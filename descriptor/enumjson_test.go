package descriptor

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
	t.Run("Tag", testEnumJSONRoundtrip[Tag])
	t.Run("AudioType", testEnumJSONRoundtrip[AudioType])
	t.Run("ServiceType", testEnumJSONRoundtrip[ServiceType])
	t.Run("TeletextType", testEnumJSONRoundtrip[TeletextType])
	t.Run("VBIDataServiceID", testEnumJSONRoundtrip[VBIDataServiceID])
	t.Run("AnnouncementType", testEnumJSONRoundtrip[AnnouncementType])
	t.Run("AnnouncementReference", testEnumJSONRoundtrip[AnnouncementReference])
	t.Run("LinkageType", testEnumJSONRoundtrip[LinkageType])
	t.Run("HierarchyType", testEnumJSONRoundtrip[HierarchyType])
	t.Run("AlignmentType", testEnumJSONRoundtrip[AlignmentType])
	t.Run("MosaicCellLinkage", testEnumJSONRoundtrip[MosaicCellLinkage])
}
