package astits

import (
	"bytes"
	"github.com/asticode/go-astikit"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseData(t *testing.T) {
	// Init
	pm := newProgramMap()
	pl := NewPacketList()

	// Custom parser
	cds := []*DemuxerData{{PID: 1}}
	var c = func(pl *PacketList) (o []*DemuxerData, skip bool, err error) {
		o = cds
		skip = true
		return
	}
	ds, err := parseData(pl, c, pm)
	assert.NoError(t, err)
	assert.Equal(t, cds, ds)

	// Do nothing for CAT
	pl = NewPacketList()
	pl.Add(&Packet{Header: PacketHeader{PID: PIDCAT}})
	ds, err = parseData(pl, nil, pm)
	assert.NoError(t, err)
	assert.Empty(t, ds)

	// PES
	p := pesWithHeaderBytes()
	pl = NewPacketList()
	p0 := &Packet{
		Header:  PacketHeader{PID: uint16(256)},
		Payload: p[:33],
	}
	pl.Add(p0)
	pl.Add(&Packet{
		Header:  PacketHeader{PID: uint16(256)},
		Payload: p[33:],
	})
	ds, err = parseData(pl, nil, pm)
	assert.NoError(t, err)
	assert.Equal(t, []*DemuxerData{
		{
			AdaptationField: p0.AdaptationField,
			PES:             pesWithHeader(),
			PID:             uint16(256),
			internalData:    &tempPayload{p},
		}}, ds)

	// PSI
	pm.setUnlocked(uint16(256), uint16(1))
	p = psiBytes()
	pl = NewPacketList()
	p0 = &Packet{
		Header:  PacketHeader{PID: uint16(256)},
		Payload: p[:33],
	}
	pl.Add(p0)
	pl.Add(&Packet{
		Header:  PacketHeader{PID: uint16(256)},
		Payload: p[33:],
	})
	ds, err = parseData(pl, nil, pm)
	assert.NoError(t, err)
	assert.Equal(t, psi.toData(
		p0.AdaptationField,
		uint16(256),
	), ds)
}

func TestIsPSIPayload(t *testing.T) {
	pm := newProgramMap()
	var pids []int
	for i := 0; i <= 255; i++ {
		if isPSIPayload(uint16(i), pm) {
			pids = append(pids, i)
		}
	}
	assert.Equal(t, []int{0, 16, 17, 18, 19, 20, 30, 31}, pids)
	pm.setUnlocked(uint16(1), uint16(0))
	assert.True(t, isPSIPayload(uint16(1), pm))
}

func TestIsPESPayload(t *testing.T) {
	buf := &bytes.Buffer{}
	w := astikit.NewBitsWriter(astikit.BitsWriterOptions{Writer: buf})
	w.Write("000000000000000100000000")
	assert.False(t, isPESPayload(buf.Bytes()))
	buf.Reset()
	w.Write("00000000000000000000000100000000")
	assert.True(t, isPESPayload(buf.Bytes()))
}
