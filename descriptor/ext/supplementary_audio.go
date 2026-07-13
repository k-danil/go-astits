package ext

import (
	"fmt"

	"github.com/k-danil/go-astits/v2/internal/bytesiter"
	"github.com/k-danil/go-astits/v2/internal/util"
)

// SupplementaryAudio represents a supplementary audio extension descriptor
// Chapter: 6.4.10 | Link: https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
type SupplementaryAudio struct {
	PrivateData             []byte  `json:"private_data"`
	LanguageCode            [3]byte `json:"language_code"`
	EditorialClassification uint8   `json:"editorial_classification"`
	HasLanguageCode         bool    `json:"language_code_present"`
	MixType                 bool    `json:"mix_type"`
}

func parseSupplementaryAudio(i *bytesiter.Iterator, offsetEnd int) (d *SupplementaryAudio, err error) {
	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}

	d = &SupplementaryAudio{
		EditorialClassification: b >> 2 & 0x1f,
		HasLanguageCode:         b&0x1 > 0,
		MixType:                 b&0x80 > 0,
	}

	if d.HasLanguageCode {
		var bs []byte
		if bs, err = i.NextBytesNoCopy(3); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
		copy(d.LanguageCode[:], bs)
	}

	if i.Offset() < offsetEnd {
		if d.PrivateData, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *SupplementaryAudio) CalcLength() int {
	ret := 1
	ret += 3 * int(util.B2U(d.HasLanguageCode))
	ret += len(d.PrivateData)
	return ret
}

func (d *SupplementaryAudio) Append(dst []byte) []byte {
	dst = append(dst, util.B2U(d.MixType)<<7|d.EditorialClassification&0x1f<<2|1<<1|util.B2U(d.HasLanguageCode))

	if d.HasLanguageCode {
		dst = append(dst, d.LanguageCode[:]...)
	}

	return append(dst, d.PrivateData...)
}
