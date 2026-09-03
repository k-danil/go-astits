package ext

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
)

type AC4 struct {
	TOC                         []byte `json:"ac4_dsi_byte"`
	AdditionalInfo              []byte `json:"additional_info"`
	AC4ChannelMode              uint8  `json:"ac4_channel_mode"`
	AC4ConfigFlag               bool   `json:"ac4_config_flag"`
	AC4TOCFlag                  bool   `json:"ac4_toc_flag"`
	AC4DialogEnhancementEnabled bool   `json:"ac4_dialog_enhancement_enabled"`
}

func parseAC4(i *bytesiter.Iterator, offsetEnd int) (d *AC4, err error) {
	d = &AC4{}

	var b byte
	if b, err = i.NextByte(); err != nil {
		err = fmt.Errorf("astits: fetching next byte failed: %w", err)
		return
	}
	d.AC4ConfigFlag = b&0x80 > 0
	d.AC4TOCFlag = b&0x40 > 0

	if d.AC4ConfigFlag {
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}
		d.AC4DialogEnhancementEnabled = b&0x80 > 0
		d.AC4ChannelMode = b >> 5 & 0x03
	}
	if d.AC4TOCFlag {
		if err = readLengthPrefixed(i, &d.TOC); err != nil {
			return
		}
	}
	if i.Offset() < offsetEnd {
		if d.AdditionalInfo, err = i.NextBytes(offsetEnd - i.Offset()); err != nil {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}
	}
	return
}

func (d *AC4) CalcLength() (n int) {
	n = 1
	if d.AC4ConfigFlag {
		n++
	}
	if d.AC4TOCFlag {
		n += 1 + len(d.TOC)
	}
	return n + len(d.AdditionalInfo)
}

func (d *AC4) Append(dst []byte) []byte {
	var b byte
	if d.AC4ConfigFlag {
		b |= 0x80
	}
	if d.AC4TOCFlag {
		b |= 0x40
	}
	dst = append(dst, b)
	if d.AC4ConfigFlag {
		var cb byte
		if d.AC4DialogEnhancementEnabled {
			cb |= 0x80
		}
		cb |= d.AC4ChannelMode & 0x03 << 5
		dst = append(dst, cb)
	}
	if d.AC4TOCFlag {
		dst = append(dst, uint8(len(d.TOC)))
		dst = append(dst, d.TOC...)
	}
	return append(dst, d.AdditionalInfo...)
}
