package demux

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/ts"
)

func TestTruncatedPSIUnitAtEOFIsNotAnError(t *testing.T) {
	sec := patSection(0, true, 0x300)
	tests := []struct {
		name string
		unit []byte
		want []Event
	}{
		{name: "cut inside the section", unit: append([]byte{0}, sec[:len(sec)-5]...)},
		{name: "cut inside the next header", unit: slices.Concat([]byte{0}, sec, []byte{0x02, 0xb0}), want: []Event{EventPAT}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dmx := New(context.Background(), bytes.NewReader(stuffedPacket(ts.PIDPAT, 0, true, tt.unit)),
				WithPacketSize(ts.PacketSize), WithRecoverableErrors())
			defer dmx.Close()
			var got []Event
			for ev, err := range dmx.Events() {
				require.NoError(t, err)
				got = append(got, ev)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
