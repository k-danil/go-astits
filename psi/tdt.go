package psi

import (
	"fmt"
	"time"

	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/dvb"
)

type TDT struct {
	UTCTime time.Time `json:"UTC_time"`
}

const dvbTimeBytesSize = 5

func parseTDTSection(i *bytesiter.Iterator) (d *TDT, err error) {
	d = &TDT{}
	if d.UTCTime, err = dvb.ParseTime(i); err != nil {
		err = fmt.Errorf("astits: parsing DVB time failed: %w", err)
		return
	}
	return
}

func (d *TDT) CalcSectionLength() int { return dvbTimeBytesSize }

func (d *TDT) appendSection(dst []byte) []byte { return dvb.AppendTime(dst, d.UTCTime) }
