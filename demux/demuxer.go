package demux

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/k-danil/go-astits/internal/pidmap"
	"github.com/k-danil/go-astits/ts"
)

var ErrZeroCopyNextData = errors.New("astits: NextData is unavailable with zero-copy packets")

// Demuxer represents a demuxer
// https://en.wikipedia.org/wiki/MPEG_transport_stream
// http://seidl.cs.vsb.cz/download/dvb/DVB_Poster.pdf
// http://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.13.01_40/en_300468v011301o.pdf
type Demuxer struct {
	ctx        context.Context
	dataBuffer []*Data

	optPacketSize    uint
	optSkipErrLimit  uint
	optPacketsParser PacketsParser
	optPacketSkipper ts.PacketSkipper
	optZeroCopyBatch uint

	packetBuffer *ts.PacketBuffer
	packetPool   packetPool
	programMap   pidmap.Map[uint16]
	psiPrev      pidmap.Map[[]byte]
	dsScratch    []*Data
	dsArr        [4]*Data
	pmKeysArr    [4]uint16
	pmValsArr    [4]uint16
	r            io.Reader
}

// PacketsParser represents an object capable of parsing a set of packets containing a unique payload spanning over those packets
// Use the skip returned argument to indicate whether the default process should still be executed on the set of packets
type PacketsParser func(pl *ts.PacketList) (ds []*Data, skip bool, err error)

// New creates a new transport stream based on a reader
func New(ctx context.Context, r io.Reader, opts ...func(*Demuxer)) (d *Demuxer) {
	// Init
	d = &Demuxer{
		ctx:              ctx,
		optPacketSkipper: ts.EmptySkipper,
		r:                r,
	}
	d.programMap = pidmap.Map[uint16]{Keys: d.pmKeysArr[:0], Vals: d.pmValsArr[:0]}
	d.packetPool.init(&d.programMap)
	d.dsScratch = d.dsArr[:0]

	// Apply options
	for _, opt := range opts {
		opt(d)
	}

	return
}

func (dmx *Demuxer) GetStats() (ret map[uint64]uint) {
	var packetSize uint
	if dmx.packetBuffer != nil {
		packetSize = dmx.packetBuffer.PacketSize()
	}

	ret = make(map[uint64]uint, len(dmx.packetPool.slots.Vals))
	for i := range dmx.packetPool.slots.Vals {
		if n := dmx.packetPool.slots.Vals[i].stats; n > 0 {
			ret[uint64(dmx.packetPool.slots.Keys[i])] = uint(n) * packetSize
		}
	}

	return
}

// WithPacketSize returns the option to set the packet size
func WithPacketSize(packetSize int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optPacketSize = uint(packetSize)
	}
}

// WithPacketsParser returns the option to set the packets parser
func WithPacketsParser(p PacketsParser) func(*Demuxer) {
	return func(d *Demuxer) {
		if p != nil {
			d.optPacketsParser = p
		}
	}
}

// WithPacketSkipper returns the option to set the packet skipper
func WithPacketSkipper(s ts.PacketSkipper) func(*Demuxer) {
	return func(d *Demuxer) {
		if s != nil {
			d.optPacketSkipper = s
		}
	}
}

// WithSkipErrLimit returns the option to set the packet skipper
func WithSkipErrLimit(count int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optSkipErrLimit = uint(count)
	}
}

// WithZeroCopyPackets makes NextPacket/NextPacketTo read the stream in batches
// of batchPackets and return packets as views into the internal buffer: no per-packet
// read call and no copy. A packet's memory (Payload, adaptation private data) is only
// valid until the NextPacket/NextPacketTo call that triggers the next batch refill —
// consume it immediately. NextData is unavailable in this mode (it accumulates packets
// across refills) and returns ErrZeroCopyNextData.
func WithZeroCopyPackets(batchPackets uint) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optZeroCopyBatch = batchPackets
	}
}

func (dmx *Demuxer) nextPacket(p *ts.Packet) (err error) {
	// Create packet buffer if not exists
	if dmx.packetBuffer == nil {
		if dmx.packetBuffer, err = ts.NewPacketBuffer(dmx.r, dmx.optPacketSize, dmx.optSkipErrLimit, dmx.optPacketSkipper, dmx.optZeroCopyBatch); err != nil {
			err = fmt.Errorf("astits: creating packet buffer failed: %w", err)
			return
		}
	}

	// Fetch next packet from buffer
	if err = dmx.packetBuffer.Next(p); err != nil {
		if err != ts.ErrNoMorePackets {
			err = fmt.Errorf("astits: fetching next packet from buffer failed: %w", err)
		}
	}
	return
}

// NextPacket retrieves the next packet. You must Close() the packet after use.
func (dmx *Demuxer) NextPacket() (p *ts.Packet, err error) {
	p = ts.NewPacket()

	select {
	case <-dmx.ctx.Done():
		err = dmx.ctx.Err()
	default:
		err = dmx.nextPacket(p)
	}

	if err != nil {
		p.Close()
		return nil, err
	}

	return
}

// NextPacketTo unpack packet to provided p.
func (dmx *Demuxer) NextPacketTo(p *ts.Packet) (err error) {
	select {
	case <-dmx.ctx.Done():
		err = dmx.ctx.Err()
	default:
		err = dmx.nextPacket(p)
	}

	return
}

func (dmx *Demuxer) nextData() (d *Data, err error) {
	// packetPool accumulates packets across batch refills, which would alias
	// reused view memory — hard error instead of silent corruption.
	if dmx.optZeroCopyBatch > 0 {
		return nil, ErrZeroCopyNextData
	}

	// Check data buffer
	if len(dmx.dataBuffer) > 0 {
		d = dmx.dataBuffer[0]
		dmx.dataBuffer = dmx.dataBuffer[1:]
		return
	}

	// Loop through packets
	var p *ts.Packet
	var pl *ts.PacketList
	for {
		// Get next packet
		p = dmx.packetPool.getPacket()
		if err = dmx.nextPacket(p); err != nil {
			dmx.packetPool.recyclePacket(p)
			// If the end of the stream has been reached, we dump the packet pool
			if errors.Is(err, ts.ErrNoMorePackets) {
				for {
					// Dump packet pool
					if pl = dmx.packetPool.dumpUnlocked(); pl.IsEmpty() {
						break
					}

					// Parse data
					ds, errParseData := parseData(pl, dmx.optPacketsParser, &dmx.programMap, &dmx.psiPrev, dmx.dsScratch)
					dmx.packetPool.recycle(pl)
					if errParseData != nil {
						// Swallow the error: there may be some incomplete data here,
						// we still want to try to parse all packets, in case final data is complete
						continue
					}

					// Update data
					if d = dmx.updateData(ds); d != nil {
						err = nil
						return
					}
				}
				// Stream fully read: return the freelist to the global pool,
				// otherwise a short-lived demuxer hands its packets to the GC
				dmx.packetPool.drain()
				return
			}
			err = fmt.Errorf("astits: fetching next packet failed: %w", err)
			return
		}

		// Add packet to the pool
		if pl = dmx.packetPool.addUnlocked(p); pl.IsEmpty() {
			if pl != nil {
				pl.Close()
			}
			continue
		}

		// Parse data
		var ds []*Data
		ds, err = parseData(pl, dmx.optPacketsParser, &dmx.programMap, &dmx.psiPrev, dmx.dsScratch)
		dmx.packetPool.recycle(pl)
		if err != nil {
			err = fmt.Errorf("astits: building new data failed: %w", err)
			return
		}

		// Update data
		if d = dmx.updateData(ds); d != nil {
			return
		}
	}
}

// NextData retrieves the next data
func (dmx *Demuxer) NextData() (d *Data, err error) {
	select {
	case <-dmx.ctx.Done():
		if err = dmx.ctx.Err(); err != nil {
			return
		}
	default:
		return dmx.nextData()
	}
	return
}

func (dmx *Demuxer) updateData(ds []*Data) (d *Data) {
	// Check whether there is data to be processed
	if len(ds) > 0 {
		// Process data
		d = ds[0]
		dmx.dataBuffer = append(dmx.dataBuffer, ds[1:]...)

		// Update program map
		for _, v := range ds {
			if v.PAT != nil {
				for _, pgm := range v.PAT.Programs {
					// Program number 0 is reserved to NIT
					if pgm.ProgramNumber > 0 {
						dmx.programMap.Set(pgm.ProgramMapID, pgm.ProgramNumber)
					}
				}
			}
		}
	}
	return
}

// Close returns everything the demuxer holds to the pools (buffered data,
// unfinished lists, the freelist). The demuxer must not be used after Close.
// Mandatory for demuxers abandoned before the end of the stream.
func (dmx *Demuxer) Close() {
	for _, d := range dmx.dataBuffer {
		d.Close()
	}
	dmx.dataBuffer = nil
	dmx.packetPool.close()
}

// Rewind rewinds the demuxer reader
func (dmx *Demuxer) Rewind() (n int64, err error) {
	dmx.Close()
	dmx.dataBuffer = []*Data{}
	dmx.packetBuffer = nil
	dmx.packetPool.init(&dmx.programMap)
	dmx.psiPrev = pidmap.Map[[]byte]{}
	if n, err = ts.Rewind(dmx.r); err != nil {
		err = fmt.Errorf("astits: rewinding reader failed: %w", err)
		return
	}
	return
}
