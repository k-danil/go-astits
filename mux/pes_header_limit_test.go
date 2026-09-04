package mux

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v3/pes"
	"github.com/k-danil/go-astits/v3/psi"
	"github.com/k-danil/go-astits/v3/ts"
)

func TestWriteDataRejectsUnwritableHeader(t *testing.T) {
	pts := ts.NewClockReference(90000, 0)
	for _, tc := range []struct {
		name       string
		streamType psi.StreamType
		data       *pes.Data
		wantErr    error
	}{
		{
			name:       "pack header past the length field",
			streamType: psi.StreamTypeH264Video,
			data: &pes.Data{
				Data: []byte("access unit"),
				Header: pes.Header{OptionalHeader: &pes.OptionalHeader{
					PTS: pts, PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS, HasExtension: true,
					Extension: &pes.OptionalHeaderExtension{
						HasPackHeaderField: true, PackHeader: make([]byte, 300),
						HasProgramPacketSequenceCounter: true,
					},
				}},
			},
			wantErr: pes.ErrHeaderTooLong,
		},
		{
			name:       "extension_2 reserved past its 7-bit length",
			streamType: psi.StreamTypeH264Video,
			data: &pes.Data{
				Data: []byte("access unit"),
				Header: pes.Header{OptionalHeader: &pes.OptionalHeader{
					PTS: pts, PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS, HasExtension: true,
					Extension: &pes.OptionalHeaderExtension{HasExtension2: true, Extension2Reserved: make([]byte, 130)},
				}},
			},
			wantErr: pes.ErrHeaderTooLong,
		},
		{
			name:       "audio unit past 65535",
			streamType: psi.StreamTypeAACAudio,
			data: &pes.Data{
				Data: make([]byte, 70000),
				Header: pes.Header{OptionalHeader: &pes.OptionalHeader{
					PTS: pts, PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS,
				}},
			},
			wantErr: pes.ErrUnboundedNonVideo,
		},
		{
			name:       "stream_id without its optional header",
			streamType: psi.StreamTypeH264Video,
			data:       &pes.Data{Data: []byte("access unit")},
			wantErr:    pes.ErrMissingOptionalHeader,
		},
		{
			name:       "extension flag without the extension header",
			streamType: psi.StreamTypeH264Video,
			data: &pes.Data{
				Data: []byte("access unit"),
				Header: pes.Header{OptionalHeader: &pes.OptionalHeader{
					PTS: pts, PTSDTSIndicator: pes.PTSDTSIndicatorOnlyPTS, HasExtension: true,
				}},
			},
			wantErr: pes.ErrMissingExtension,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const pid = 0x100
			buf := &bytes.Buffer{}
			m := New(context.Background(), buf)
			require.NoError(t, m.AddElementaryStream(psi.ElementaryStream{ElementaryPID: pid, StreamType: tc.streamType}))
			m.SetPCRPID(pid)

			_, err := m.WriteData(&Data{PID: pid, PES: tc.data})
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}
