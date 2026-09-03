package demux

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/ts"
)

// frameTagger serves a stream in datagram-sized frames, each stamped with its
// index: a window never crosses a frame, a repair peek may.
type frameTagger struct {
	data  []byte
	pos   int
	frame int
}

func (r *frameTagger) frameEnd() int {
	return min((r.pos/r.frame+1)*r.frame, len(r.data))
}

func (r *frameTagger) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:r.frameEnd()])
	r.pos += n
	return
}

func (r *frameTagger) Peek(n int) (bs []byte, err error) {
	bs = r.data[r.pos:min(r.pos+n, len(r.data))]
	if len(bs) < n {
		err = io.EOF
	}
	return
}

func (r *frameTagger) Discard(n int) (int, error) {
	n = min(n, len(r.data)-r.pos)
	r.pos += n
	return n, nil
}

func (r *frameTagger) Buffered() int { return r.frameEnd() - r.pos }
func (r *frameTagger) Size() int     { return r.frame }
func (r *frameTagger) Tag() uint64   { return uint64(r.pos/r.frame) + 1 }

// A cached repeat reports the packets it arrived on, not the cached copy's.
func TestSectionSpanFollowsRepeats(t *testing.T) {
	const frameBytes = 3 * ts.PacketSize
	sec := patSection(0, true, 0x100)
	raw := psiPacket(ts.PIDPAT, 0, sec)
	raw = append(raw, payloadPacket(ts.PIDNull, 0, false, nil)...)
	raw = append(raw, payloadPacket(ts.PIDNull, 1, false, nil)...)
	raw = append(raw, psiPacket(ts.PIDPAT, 1, sec)...)

	dmx := New(context.Background(), &frameTagger{data: raw, frame: frameBytes},
		WithPacketSize(ts.PacketSize), WithPSIRepeats())
	defer dmx.Close()

	var spans []PacketSpan
	var changed []bool
	for ev, err := range dmx.Events() {
		require.NoError(t, err)
		require.Equal(t, EventPAT, ev)
		spans = append(spans, dmx.SectionSpan())
		changed = append(changed, dmx.TableChanged())
	}
	require.Equal(t, []bool{true, false}, changed, "the second arrival is served from the cache")
	assert.Equal(t, []PacketSpan{
		{FirstPacketTag: 1, LastPacketTag: 1},
		{FirstPacketOffset: frameBytes, LastPacketOffset: frameBytes, FirstPacketTag: 2, LastPacketTag: 2},
	}, spans)
}

// The reader's tag must reach every packet and both ends of every PES through
// each read path: the batch window, the sync-lock window and the per-packet
// repair path after a corrupt packet.
func TestTagFollowsPacketsAndUnits(t *testing.T) {
	const frameBytes = 7 * ts.PacketSize
	pesStart := []byte{0, 0, 1, 0xe0, 0, 0, 0x80, 0, 0}
	var raw []byte
	for i := range 40 {
		cc := uint8(2 * i) // two packets per unit on 0x100, so the next start is never a CC duplicate
		raw = append(raw, payloadPacket(0x100, cc&0xf, true, append(pesStart, byte(i)))...)
		raw = append(raw, payloadPacket(0x100, (cc+1)&0xf, false, []byte{1, 2, 3})...)
		raw = append(raw, payloadPacket(0x200, uint8(i)&0xf, true, append(pesStart, byte(i)))...)
		if i%3 == 0 {
			raw = append(raw, corruptAlignedPacket()...)
		}
	}
	want := func(offset int64) uint64 { return uint64(offset/frameBytes) + 1 }

	for _, tc := range []struct {
		name string
		opts []func(*Demuxer)
	}{
		{"batch", []func(*Demuxer){WithZeroCopyPackets(4096)}},
		{"sync-lock", []func(*Demuxer){WithSyncLock(), WithZeroCopyPackets(4096)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]func(*Demuxer){WithPacketSize(ts.PacketSize), WithSkipErrLimit(-1), WithResyncLimit(-1)}, tc.opts...)

			dmx := New(context.Background(), &frameTagger{data: raw, frame: frameBytes}, opts...)
			p := ts.NewPacket()
			packets := 0
			for {
				err := dmx.NextPacketTo(p)
				if errors.Is(err, ts.ErrNoMorePackets) {
					break
				}
				require.NoError(t, err)
				require.Equal(t, want(p.Offset), p.Tag, "packet at %d", p.Offset)
				packets++
			}
			p.Close()
			dmx.Close()
			require.Greater(t, packets, 100)

			dmx = New(context.Background(), &frameTagger{data: raw, frame: frameBytes}, opts...)
			defer dmx.Close()
			units, crossing := 0, 0
			for ev, err := range dmx.Events() {
				require.NoError(t, err)
				if ev != EventPES {
					continue
				}
				d := dmx.PES()
				assert.Equal(t, want(d.FirstPacketOffset), d.FirstPacketTag)
				assert.Equal(t, want(d.LastPacketOffset), d.LastPacketTag)
				if d.FirstPacketTag != d.LastPacketTag {
					crossing++
				}
				units++
			}
			require.Greater(t, units, 70)
			require.Positive(t, crossing, "some unit must straddle a frame")

		})
	}
}
