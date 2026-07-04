package demux

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/asticode/go-astikit"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/ts"
)

func TestPSIDedup(t *testing.T) {
	const psiCopies = 3
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	b := psiBytes()
	cc := uint8(0)
	writePSI := func() {
		b1, _ := packet(ts.PacketHeader{ContinuityCounter: cc, PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[:147], true)
		w.Write(b1)
		cc++
		b2, _ := packet(ts.PacketHeader{ContinuityCounter: cc, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[147:], true)
		w.Write(b2)
		cc++
	}
	singleCopyLen := 0
	for i := 0; i < psiCopies; i++ {
		writePSI()
		if i == 0 {
			singleCopyLen = buf.Len()
		}
	}

	countDatas := func(dmx *Demuxer) (n int) {
		for {
			d, err := dmx.NextData()
			if err != nil {
				require.True(t, errors.Is(err, ts.ErrNoMorePackets))
				return
			}
			n++
			d.Close()
		}
	}

	// Identical repeated sections are suppressed: same data count as from a single copy
	single := NewDemuxer(context.Background(), bytes.NewReader(buf.Bytes()[:singleCopyLen]))
	dmx := NewDemuxer(context.Background(), bytes.NewReader(buf.Bytes()))
	want := countDatas(single)
	require.NotZero(t, want)
	require.Equal(t, want, countDatas(dmx))

	// Rewind resets the cache — sections are emitted again
	_, err := dmx.Rewind()
	require.NoError(t, err)
	require.Equal(t, want, countDatas(dmx))
}
