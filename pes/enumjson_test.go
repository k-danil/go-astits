package pes

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func testEnumJSONRoundtrip[T interface{ ~uint8 }](t *testing.T) {
	for i := range 256 {
		v := T(uint8(i))
		b, err := json.Marshal(v)
		require.NoError(t, err)
		var got T
		require.NoError(t, json.Unmarshal(b, &got), "%s", b)
		require.Equal(t, v, got, "%s", b)
	}
}

func TestEnumJSONRoundtrip(t *testing.T) {
	t.Run("PSTDBufferScale", testEnumJSONRoundtrip[PSTDBufferScale])
	t.Run("PTSDTSIndicator", testEnumJSONRoundtrip[PTSDTSIndicator])
	t.Run("StreamID", testEnumJSONRoundtrip[StreamID])
	t.Run("TrickModeControl", testEnumJSONRoundtrip[TrickModeControl])
	t.Run("ScramblingControl", testEnumJSONRoundtrip[ScramblingControl])
	t.Run("FieldID", testEnumJSONRoundtrip[FieldID])
	t.Run("FrequencyTruncation", testEnumJSONRoundtrip[FrequencyTruncation])
}
