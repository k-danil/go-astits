package astits

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/asticode/go-astikit"
	"github.com/stretchr/testify/require"
)

func TestPSIDedup(t *testing.T) {
	const psiCopies = 3
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	b := psiBytes()
	cc := uint8(0)
	writePSI := func() {
		b1, _ := packet(PacketHeader{ContinuityCounter: cc, PayloadUnitStartIndicator: true, PID: PIDPAT}, &PacketAdaptationField{}, b[:147], true)
		w.Write(b1)
		cc++
		b2, _ := packet(PacketHeader{ContinuityCounter: cc, PID: PIDPAT}, &PacketAdaptationField{}, b[147:], true)
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
				require.True(t, errors.Is(err, ErrNoMorePackets))
				return
			}
			n++
			d.Close()
		}
	}

	// Повторные идентичные секции подавлены: данных столько же, сколько от одной копии
	single := NewDemuxer(context.Background(), bytes.NewReader(buf.Bytes()[:singleCopyLen]))
	dmx := NewDemuxer(context.Background(), bytes.NewReader(buf.Bytes()))
	want := countDatas(single)
	require.NotZero(t, want)
	require.Equal(t, want, countDatas(dmx))

	// Rewind сбрасывает кэш — секции эмитятся заново
	_, err := dmx.Rewind()
	require.NoError(t, err)
	require.Equal(t, want, countDatas(dmx))
}
