package astits

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Sync byte
const syncByte byte = '\x47'

// Errors
var (
	ErrNoMorePackets                = errors.New("astits: no more packets")
	ErrPacketMustStartWithASyncByte = errors.New("astits: packet must start with a sync byte")
)

// Demuxer represents a demuxer
// https://en.wikipedia.org/wiki/MPEG_transport_stream
// http://seidl.cs.vsb.cz/download/dvb/DVB_Poster.pdf
// http://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.13.01_40/en_300468v011301o.pdf
type Demuxer struct {
	ctx        context.Context
	dataBuffer []*DemuxerData
	//l          astikit.CompleteLogger

	optPacketSize    uint
	optSkipErrLimit  uint
	optPacketsParser PacketsParser
	optPacketSkipper PacketSkipper

	packetBuffer *packetBuffer
	packetPool   *packetPool
	programMap   *programMap
	r            io.Reader
}

// PacketsParser represents an object capable of parsing a set of packets containing a unique payload spanning over those packets
// Use the skip returned argument to indicate whether the default process should still be executed on the set of packets
type PacketsParser func(pl *PacketList) (ds []*DemuxerData, skip bool, err error)

// PacketSkipper represents an object capable of skipping a packet before parsing its payload. Its header and adaptation field is parsed and provided to the object.
// Use this option if you need to filter out unwanted packets from your pipeline. NextPacket() will return the next unskipped packet if any.
type PacketSkipper func(p *Packet) (skip bool)

var EmptySkipper = func(_ *Packet) (skip bool) { return }

// NewDemuxer creates a new transport stream based on a reader
func NewDemuxer(ctx context.Context, r io.Reader, opts ...func(*Demuxer)) (d *Demuxer) {
	// Init
	d = &Demuxer{
		ctx: ctx,
		//l:                astikit.AdaptStdLogger(nil),
		programMap:       newProgramMap(),
		optPacketSkipper: EmptySkipper,
		r:                r,
	}
	d.packetPool = newPacketPool(d.programMap)

	// Apply options
	for _, opt := range opts {
		opt(d)
	}

	return
}

func (dmx *Demuxer) GetStats() (ret map[uint64]uint) {
	if dmx.packetPool == nil || dmx.packetPool.stats == nil {
		return
	}
	var packetSize uint
	if dmx.packetBuffer != nil {
		packetSize = dmx.packetBuffer.packetSize
	}

	ret = make(map[uint64]uint, len(dmx.packetPool.stats))
	for k, v := range dmx.packetPool.stats {
		ret[k] = v * packetSize
	}

	return
}

// DemuxerOptLogger returns the option to set the logger
//func DemuxerOptLogger(l astikit.StdLogger) func(*Demuxer) {
//	return func(d *Demuxer) {
//		d.l = astikit.AdaptStdLogger(l)
//	}
//}

// DemuxerOptPacketSize returns the option to set the packet size
func DemuxerOptPacketSize(packetSize int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optPacketSize = uint(packetSize)
	}
}

// DemuxerOptPacketsParser returns the option to set the packets parser
func DemuxerOptPacketsParser(p PacketsParser) func(*Demuxer) {
	return func(d *Demuxer) {
		if p != nil {
			d.optPacketsParser = p
		}
	}
}

// DemuxerOptPacketSkipper returns the option to set the packet skipper
func DemuxerOptPacketSkipper(s PacketSkipper) func(*Demuxer) {
	return func(d *Demuxer) {
		if s != nil {
			d.optPacketSkipper = s
		}
	}
}

// DemuxerOptSkipErrLimit returns the option to set the packet skipper
func DemuxerOptSkipErrLimit(count int) func(*Demuxer) {
	return func(d *Demuxer) {
		d.optSkipErrLimit = uint(count)
	}
}

func (dmx *Demuxer) nextPacket(p *Packet) (err error) {
	// Create packet buffer if not exists
	if dmx.packetBuffer == nil {
		if dmx.packetBuffer, err = newPacketBuffer(dmx.r, dmx.optPacketSize, dmx.optSkipErrLimit, dmx.optPacketSkipper); err != nil {
			err = fmt.Errorf("astits: creating packet buffer failed: %w", err)
			return
		}
	}

	// Fetch next packet from buffer
	if err = dmx.packetBuffer.next(p); err != nil {
		if err != ErrNoMorePackets {
			err = fmt.Errorf("astits: fetching next packet from buffer failed: %w", err)
		}
	}
	return
}

// NextPacket retrieves the next packet. You must Close() the packet after use.
func (dmx *Demuxer) NextPacket() (p *Packet, err error) {
	p = NewPacket()

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
func (dmx *Demuxer) NextPacketTo(p *Packet) (err error) {
	select {
	case <-dmx.ctx.Done():
		err = dmx.ctx.Err()
	default:
		err = dmx.nextPacket(p)
	}

	return
}

func (dmx *Demuxer) nextData() (d *DemuxerData, err error) {
	// Check data buffer
	if len(dmx.dataBuffer) > 0 {
		d = dmx.dataBuffer[0]
		dmx.dataBuffer = dmx.dataBuffer[1:]
		return
	}

	// Loop through packets
	var p *Packet
	var pl *PacketList
	for {
		// Get next packet
		p = NewPacket()
		if err = dmx.nextPacket(p); err != nil {
			p.Close()
			// If the end of the stream has been reached, we dump the packet pool
			if errors.Is(err, ErrNoMorePackets) {
				for {
					// Dump packet pool
					if pl = dmx.packetPool.dumpUnlocked(); pl.IsEmpty() {
						break
					}

					// Parse data
					ds, errParseData := parseData(pl, dmx.optPacketsParser, dmx.programMap)
					if errParseData != nil {
						// Log error as there may be some incomplete data here
						// We still want to try to parse all packets, in case final data is complete
						//dmx.l.Error(fmt.Errorf("astits: parsing data failed: %w", errParseData))
						continue
					}

					// Update data
					if d = dmx.updateData(ds); d != nil {
						err = nil
						return
					}
				}
				return
			}
			err = fmt.Errorf("astits: fetching next packet failed: %w", err)
			return
		}

		// Add packet to the pool
		if pl = dmx.packetPool.addUnlocked(p); pl.IsEmpty() {
			continue
		}

		// Parse data
		var ds []*DemuxerData
		ds, err = parseData(pl, dmx.optPacketsParser, dmx.programMap)
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
func (dmx *Demuxer) NextData() (d *DemuxerData, err error) {
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

func (dmx *Demuxer) updateData(ds []*DemuxerData) (d *DemuxerData) {
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
						dmx.programMap.setUnlocked(pgm.ProgramMapID, pgm.ProgramNumber)
					}
				}
			}
		}
	}
	return
}

// Rewind rewinds the demuxer reader
func (dmx *Demuxer) Rewind() (n int64, err error) {
	dmx.dataBuffer = []*DemuxerData{}
	dmx.packetBuffer = nil
	dmx.packetPool = newPacketPool(dmx.programMap)
	if n, err = rewind(dmx.r); err != nil {
		err = fmt.Errorf("astits: rewinding reader failed: %w", err)
		return
	}
	return
}
