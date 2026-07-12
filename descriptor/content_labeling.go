package descriptor

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
)

// ContentLabeling represents a content labelling descriptor (ISO/IEC 13818-1).
type ContentLabeling struct {
	ContentReferenceID                  []byte
	TimeBaseAssociationData             []byte
	PrivateData                         []byte
	ContentTimeBaseValue                uint64 // 90 kHz units
	MetadataTimeBaseValue               uint64 // 90 kHz units
	MetadataApplicationFormatIdentifier uint32
	Header                              Header
	MetadataApplicationFormat           uint16
	ContentTimeBaseIndicator            uint8
	ContentID                           uint8
	ContentReferenceIDRecordFlag        bool
}

func newDescriptorContentLabeling(i *bytesiter.Iterator, h Header, offsetEnd int) (dd Descriptor, err error) {
	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d := &ContentLabeling{
		Header:                    h,
		MetadataApplicationFormat: uint16(bs[0])<<8 | uint16(bs[1]),
	}
	dd = d

	if d.MetadataApplicationFormat == 0xffff {
		if bs, err = i.NextBytesNoCopy(4); err != nil || len(bs) < 4 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.MetadataApplicationFormatIdentifier = uint32(bs[0])<<24 | uint32(bs[1])<<16 | uint32(bs[2])<<8 | uint32(bs[3])
	}

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.ContentReferenceIDRecordFlag = b&0x80 > 0
	d.ContentTimeBaseIndicator = b >> 3 & 0x0f

	if d.ContentReferenceIDRecordFlag {
		var n byte
		if n, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		if d.ContentReferenceID, err = i.NextBytes(int(n)); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}

	if d.ContentTimeBaseIndicator == 1 || d.ContentTimeBaseIndicator == 2 {
		if bs, err = i.NextBytesNoCopy(10); err != nil || len(bs) < 10 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		d.ContentTimeBaseValue = uint64(bs[0]&0x01)<<32 | uint64(bs[1])<<24 | uint64(bs[2])<<16 | uint64(bs[3])<<8 | uint64(bs[4])
		d.MetadataTimeBaseValue = uint64(bs[5]&0x01)<<32 | uint64(bs[6])<<24 | uint64(bs[7])<<16 | uint64(bs[8])<<8 | uint64(bs[9])
	}

	if d.ContentTimeBaseIndicator == 2 {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.ContentID = b & 0x7f
	}

	if d.ContentTimeBaseIndicator >= 3 && d.ContentTimeBaseIndicator <= 7 {
		var n byte
		if n, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		if d.TimeBaseAssociationData, err = i.NextBytes(int(n)); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}

	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *ContentLabeling) CalcLength() int {
	ret := 2
	if d.MetadataApplicationFormat == 0xffff {
		ret += 4
	}
	ret++
	if d.ContentReferenceIDRecordFlag {
		ret += 1 + len(d.ContentReferenceID)
	}
	if d.ContentTimeBaseIndicator == 1 || d.ContentTimeBaseIndicator == 2 {
		ret += 10
	}
	if d.ContentTimeBaseIndicator == 2 {
		ret++
	}
	if d.ContentTimeBaseIndicator >= 3 && d.ContentTimeBaseIndicator <= 7 {
		ret += 1 + len(d.TimeBaseAssociationData)
	}
	ret += len(d.PrivateData)
	return ret
}

func (d *ContentLabeling) Append(dst []byte) []byte {
	dst = append(dst, uint8(d.Header.Tag), uint8(d.CalcLength()))
	dst = append(dst, byte(d.MetadataApplicationFormat>>8), byte(d.MetadataApplicationFormat))

	if d.MetadataApplicationFormat == 0xffff {
		dst = append(dst, byte(d.MetadataApplicationFormatIdentifier>>24), byte(d.MetadataApplicationFormatIdentifier>>16), byte(d.MetadataApplicationFormatIdentifier>>8), byte(d.MetadataApplicationFormatIdentifier))
	}

	b := d.ContentTimeBaseIndicator&0x0f<<3 | 0x07
	if d.ContentReferenceIDRecordFlag {
		b |= 0x80
	}
	dst = append(dst, b)

	if d.ContentReferenceIDRecordFlag {
		dst = append(dst, byte(len(d.ContentReferenceID)))
		dst = append(dst, d.ContentReferenceID...)
	}

	if d.ContentTimeBaseIndicator == 1 || d.ContentTimeBaseIndicator == 2 {
		dst = append(dst, 0xfe|byte(d.ContentTimeBaseValue>>32&0x01), byte(d.ContentTimeBaseValue>>24), byte(d.ContentTimeBaseValue>>16), byte(d.ContentTimeBaseValue>>8), byte(d.ContentTimeBaseValue))
		dst = append(dst, 0xfe|byte(d.MetadataTimeBaseValue>>32&0x01), byte(d.MetadataTimeBaseValue>>24), byte(d.MetadataTimeBaseValue>>16), byte(d.MetadataTimeBaseValue>>8), byte(d.MetadataTimeBaseValue))
	}

	if d.ContentTimeBaseIndicator == 2 {
		dst = append(dst, 0x80|d.ContentID&0x7f)
	}

	if d.ContentTimeBaseIndicator >= 3 && d.ContentTimeBaseIndicator <= 7 {
		dst = append(dst, byte(len(d.TimeBaseAssociationData)))
		dst = append(dst, d.TimeBaseAssociationData...)
	}

	return append(dst, d.PrivateData...)
}
