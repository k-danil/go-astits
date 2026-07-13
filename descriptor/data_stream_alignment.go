package descriptor

import (
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

type AlignmentType uint8

// Data stream alignments
// Page: 85 | Chapter:2.6.11 | Link: http://ecee.colorado.edu/~ecen5653/ecen5653/papers/iso13818-1.pdf
const (
	DataStreamAligmentAudioSyncWord          AlignmentType = 0x1
	DataStreamAligmentVideoSliceOrAccessUnit AlignmentType = 0x1
	DataStreamAligmentVideoAccessUnit        AlignmentType = 0x2
	DataStreamAligmentVideoGOPOrSEQ          AlignmentType = 0x3
	DataStreamAligmentVideoSEQ               AlignmentType = 0x4
)

var alignmentTypeNames = map[AlignmentType]string{
	DataStreamAligmentVideoSliceOrAccessUnit: "slice_or_video_access_unit",
	DataStreamAligmentVideoAccessUnit:        "video_access_unit",
	DataStreamAligmentVideoGOPOrSEQ:          "GOP_or_SEQ",
	DataStreamAligmentVideoSEQ:               "SEQ",
}

func (t AlignmentType) String() (s string) {
	var ok bool
	if s, ok = alignmentTypeNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t AlignmentType) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *AlignmentType) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, alignmentTypeNames)
	return
}

// DataStreamAlignment represents a data stream alignment descriptor
type DataStreamAlignment struct {
	Header Header        `json:"_header"`
	Type   AlignmentType `json:"alignment_type"`
}

func newDescriptorDataStreamAlignment(i *bytesiter.Iterator, h Header, _ int) (dd Descriptor, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d := &DataStreamAlignment{
		Header: h,
		Type:   AlignmentType(b),
	}
	dd = d
	return
}

func (*DataStreamAlignment) CalcLength() int {
	return 1
}

func (d *DataStreamAlignment) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	return append(dst, uint8(d.Type))
}
