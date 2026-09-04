package descriptor

import (
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/util"
)

type AlignmentType uint8

// 0x1 is both the video value and the legacy audio "sync word", so String
// cannot tell them apart.
const (
	DataStreamAlignmentAudioSyncWord          AlignmentType = 0x1
	DataStreamAlignmentVideoSliceOrAccessUnit AlignmentType = 0x1
	DataStreamAlignmentVideoAccessUnit        AlignmentType = 0x2
	DataStreamAlignmentVideoGOPOrSEQ          AlignmentType = 0x3
	DataStreamAlignmentVideoSEQ               AlignmentType = 0x4
)

var alignmentTypeNames = map[AlignmentType]string{
	DataStreamAlignmentVideoSliceOrAccessUnit: "slice_or_video_access_unit",
	DataStreamAlignmentVideoAccessUnit:        "video_access_unit",
	DataStreamAlignmentVideoGOPOrSEQ:          "GOP_or_SEQ",
	DataStreamAlignmentVideoSEQ:               "SEQ",
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

type DataStreamAlignment struct {
	Header Header        `json:"_header"`
	Type   AlignmentType `json:"alignment_type"`
}

func newDescriptorDataStreamAlignment(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
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

	err = rejectTrailingBytes(i, offsetEnd)
	return
}

func (d *DataStreamAlignment) CalcLength() int {
	return 1
}

func (d *DataStreamAlignment) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Tag()), uint8(d.CalcLength()))
	return append(dst, uint8(d.Type))
}
