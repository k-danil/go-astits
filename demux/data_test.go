package demux

import (
	"bytes"
	"testing"

	"github.com/asticode/go-astikit"
	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/internal/programmap"
	"github.com/k-danil/go-astits/psi"
	"github.com/k-danil/go-astits/ts"
)

func TestParseData(t *testing.T) {
	// Init
	pm := programmap.New()
	pl := ts.NewPacketList()

	// Custom parser
	cds := []*DemuxerData{{PID: 1}}
	var c = func(pl *ts.PacketList) (o []*DemuxerData, skip bool, err error) {
		o = cds
		skip = true
		return
	}
	ds, err := parseData(pl, c, pm, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, cds, ds)

	// Do nothing for CAT
	pl = ts.NewPacketList()
	pl.PushBack(&ts.Packet{Header: ts.PacketHeader{PID: ts.PIDCAT}})
	ds, err = parseData(pl, nil, pm, nil, nil)
	assert.NoError(t, err)
	assert.Empty(t, ds)

	// PES
	p := pesWithHeaderBytes()
	pl = ts.NewPacketList()
	p0 := &ts.Packet{
		Header:  ts.PacketHeader{PID: uint16(256)},
		Payload: p[:33],
	}
	pl.PushBack(p0)
	pl.PushBack(&ts.Packet{
		Header:  ts.PacketHeader{PID: uint16(256)},
		Payload: p[33:],
	})
	ds, err = parseData(pl, nil, pm, nil, nil)
	assert.NoError(t, err)
	wantPES := &DemuxerData{
		AdaptationField: p0.AdaptationField,
		PID:             uint16(256),
		internalData:    &dataPayload{p},
	}
	assert.NoError(t, wantPES.pes.Parse(p))
	wantPES.PES = &wantPES.pes
	assert.Equal(t, []*DemuxerData{wantPES}, ds)

	// PSI
	pm.SetUnlocked(uint16(256), uint16(1))
	p = psiBytes()
	pl = ts.NewPacketList()
	p0 = &ts.Packet{
		Header:  ts.PacketHeader{PID: uint16(256)},
		Payload: p[:33],
	}
	pl.PushBack(p0)
	pl.PushBack(&ts.Packet{
		Header:  ts.PacketHeader{PID: uint16(256)},
		Payload: p[33:],
	})
	ds, err = parseData(pl, nil, pm, nil, nil)
	assert.NoError(t, err)
	expPSI, err := psi.ParsePSIData(astikit.NewBytesIterator(p))
	assert.NoError(t, err)
	assert.Equal(t, psiToData(expPSI, p0.AdaptationField, uint16(256)), ds)
}

func TestPSIToData(t *testing.T) {
	d, err := psi.ParsePSIData(astikit.NewBytesIterator(psiBytes()))
	assert.NoError(t, err)
	sec := d.Sections
	assert.Equal(t, []*DemuxerData{
		{EIT: sec[0].Syntax.Data.(*psi.EITData), PID: 2},
		{NIT: sec[1].Syntax.Data.(*psi.NITData), PID: 2},
		{PAT: sec[2].Syntax.Data.(*psi.PATData), PID: 2},
		{PMT: sec[3].Syntax.Data.(*psi.PMTData), PID: 2},
		{SDT: sec[4].Syntax.Data.(*psi.SDTData), PID: 2},
		{TOT: sec[5].Syntax.Data.(*psi.TOTData), PID: 2},
	}, psiToData(d, nil, uint16(2)))
}

func TestIsPSIPayload(t *testing.T) {
	pm := programmap.New()
	var pids []int
	for i := 0; i <= 255; i++ {
		if isPSIPayload(uint16(i), pm) {
			pids = append(pids, i)
		}
	}
	assert.Equal(t, []int{0, 16, 17, 18, 19, 20, 30, 31}, pids)
	pm.SetUnlocked(uint16(1), uint16(0))
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
