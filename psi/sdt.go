package psi

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v3/descriptor"
	"github.com/k-danil/go-astits/v3/internal/bytesiter"
	"github.com/k-danil/go-astits/v3/internal/util"
)

type RunningStatus uint8

const (
	RunningStatusNotRunning          RunningStatus = 1
	RunningStatusPausing             RunningStatus = 3
	RunningStatusRunning             RunningStatus = 4
	RunningStatusServiceOffAir       RunningStatus = 5
	RunningStatusStartsInAFewSeconds RunningStatus = 2
	RunningStatusUndefined           RunningStatus = 0
)

var runningStatusNames = map[RunningStatus]string{
	RunningStatusUndefined:           "undefined",
	RunningStatusNotRunning:          "not running",
	RunningStatusStartsInAFewSeconds: "starts in a few seconds",
	RunningStatusPausing:             "pausing",
	RunningStatusRunning:             "running",
	RunningStatusServiceOffAir:       "service off-air",
}

func (t RunningStatus) String() (s string) {
	var ok bool
	if s, ok = runningStatusNames[t]; !ok {
		s = fmt.Sprintf("0x%02x", uint8(t))
	}
	return
}

func (t RunningStatus) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

func (t *RunningStatus) UnmarshalJSON(b []byte) (err error) {
	*t, err = util.UnmarshalEnum(b, runningStatusNames)
	return
}

type SDT struct {
	Services          []SDTService `json:"_services"`
	OriginalNetworkID uint16       `json:"original_network_id"`
	TransportStreamID uint16       `json:"transport_stream_id"`
}

type SDTService struct {
	Descriptors            []descriptor.Descriptor `json:"_descriptors"`
	ServiceID              uint16                  `json:"service_id"`
	HasEITPresentFollowing bool                    `json:"EIT_present_following_flag"`
	HasEITSchedule         bool                    `json:"EIT_schedule_flag"`
	HasFreeCSAMode         bool                    `json:"free_CA_mode"`
	RunningStatus          RunningStatus           `json:"running_status"`
}

func parseSDTSection(i *bytesiter.Iterator, offsetSectionsEnd int, tableIDExtension uint16) (d *SDT, err error) {
	d = &SDT{TransportStreamID: tableIDExtension}

	var bs []byte
	if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
		err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
		return
	}

	d.OriginalNetworkID = binary.BigEndian.Uint16(bs)

	i.Skip(1)

	for i.Offset() < offsetSectionsEnd {
		s := SDTService{}

		if bs, err = i.NextBytesNoCopy(2); err != nil || len(bs) < 2 {
			err = fmt.Errorf("astits: fetching next bytes failed: %w", err)
			return
		}

		s.ServiceID = binary.BigEndian.Uint16(bs)

		var b byte
		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		s.HasEITSchedule = uint8(b&0x2) > 0

		s.HasEITPresentFollowing = uint8(b&0x1) > 0

		if b, err = i.NextByte(); err != nil {
			err = fmt.Errorf("astits: fetching next byte failed: %w", err)
			return
		}

		s.RunningStatus = RunningStatus(b >> 5)

		s.HasFreeCSAMode = uint8(b&0x10) > 0

		// The low bits of this byte are descriptors_loop_length; rewind for descriptor.Parse.
		i.Skip(-1)

		var dn int
		if s.Descriptors, dn, err = descriptor.Parse(i.Bytes()); err != nil {
			err = fmt.Errorf("astits: parsing descriptors failed: %w", err)
			return
		}
		i.Skip(dn)

		d.Services = append(d.Services, s)
	}
	return
}

func (d *SDT) CalcSectionLength() int {
	n := 2 + 1
	for _, s := range d.Services {
		n += 2 + 2 + 1 + descriptor.CalcLength(s.Descriptors)
	}
	return n
}

func (d *SDT) appendSection(dst []byte) []byte {
	dst = append(dst, byte(d.OriginalNetworkID>>8), byte(d.OriginalNetworkID), 0xff)
	for _, s := range d.Services {
		loopLen := descriptor.CalcLength(s.Descriptors)
		dst = append(dst,
			byte(s.ServiceID>>8), byte(s.ServiceID),
			0xfc|util.B2U(s.HasEITSchedule)<<1|util.B2U(s.HasEITPresentFollowing),
			byte(s.RunningStatus)<<5|util.B2U(s.HasFreeCSAMode)<<4|byte(loopLen>>8)&0xf,
			byte(loopLen))
		dst = descriptor.Append(dst, s.Descriptors)
	}
	return dst
}
