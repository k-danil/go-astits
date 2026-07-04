package demux

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/internal/bitstest"
	"github.com/k-danil/go-astits/v2/ts"
)

func TestPSIDedup(t *testing.T) {
	const psiCopies = 3
	buf := &bytes.Buffer{}
	w := bitstest.NewWriter(buf)
	b := psiBytes()
	cc := uint8(0)
	writePSI := func() {
		b1, _ := packet(ts.PacketHeader{ContinuityCounter: cc, PayloadUnitStartIndicator: true, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[:147], true)
		_ = w.Write(b1)
		cc++
		b2, _ := packet(ts.PacketHeader{ContinuityCounter: cc, PID: ts.PIDPAT}, &ts.PacketAdaptationField{}, b[147:], true)
		_ = w.Write(b2)
		cc++
	}
	singleCopyLen := 0
	for i := 0; i < psiCopies; i++ {
		writePSI()
		if i == 0 {
			singleCopyLen = buf.Len()
		}
	}

	countEvents := func(dmx *Demuxer) (n int) {
		for {
			_, err := dmx.Next()
			if err != nil {
				require.True(t, errors.Is(err, ts.ErrNoMorePackets))
				return
			}
			n++
		}
	}

	// Identical repeated sections are suppressed: same event count as from a single copy
	single := New(context.Background(), bytes.NewReader(buf.Bytes()[:singleCopyLen]))
	dmx := New(context.Background(), bytes.NewReader(buf.Bytes()))
	want := countEvents(single)
	require.NotZero(t, want)
	require.Equal(t, want, countEvents(dmx))

	// Rewind resets the cache — sections are emitted again
	_, err := dmx.Rewind()
	require.NoError(t, err)
	require.Equal(t, want, countEvents(dmx))
}
