package demux

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k-danil/go-astits/ts"
)

func TestHasDiscontinuity(t *testing.T) {
	pl := ts.NewPacketList()
	pl.PushBack(&ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 15}})
	assert.False(t, hasDiscontinuity(pl.Tail(), &ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 0, HasPayload: true}}))
	assert.False(t, hasDiscontinuity(pl.Tail(), &ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 15}}))
	assert.True(t, hasDiscontinuity(pl.Tail(), &ts.Packet{AdaptationField: &ts.PacketAdaptationField{DiscontinuityIndicator: true}, Header: ts.PacketHeader{ContinuityCounter: 0, HasAdaptationField: true, HasPayload: true}}))
	assert.True(t, hasDiscontinuity(pl.Tail(), &ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 1, HasPayload: true}}))
	assert.True(t, hasDiscontinuity(pl.Tail(), &ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 0}}))
}

func TestIsSameAsPrevious(t *testing.T) {
	pl := ts.NewPacketList()
	pl.PushBack(&ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 1}})
	assert.False(t, isSameAsPrevious(pl.Tail(), &ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 1}}))
	assert.False(t, isSameAsPrevious(pl.Tail(), &ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 2, HasPayload: true}}))
	assert.True(t, isSameAsPrevious(pl.Tail(), &ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 1, HasPayload: true}}))
}

func TestPacketPool(t *testing.T) {
	b := newPacketPool(nil)
	ps := b.addUnlocked(&ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 0, HasPayload: true, PID: 1}})
	assert.Nil(t, ps)
	ps = b.addUnlocked(&ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 1, HasPayload: true, PayloadUnitStartIndicator: true, PID: 1}})
	assert.Equal(t, ps.Length(), 1)
	ps = b.addUnlocked(&ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 1, HasPayload: true, PayloadUnitStartIndicator: true, PID: 2}})
	assert.Nil(t, ps)
	ps = b.addUnlocked(&ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 2, HasPayload: true, PID: 1}})
	assert.Nil(t, ps)
	ps = b.addUnlocked(&ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 3, HasPayload: true, PayloadUnitStartIndicator: true, PID: 1}})
	assert.Equal(t, ps.Length(), 2)
	ps = b.addUnlocked(&ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 5, HasPayload: true, PID: 1}})
	assert.Nil(t, ps)
	ps = b.addUnlocked(&ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 6, HasPayload: true, PayloadUnitStartIndicator: true, PID: 1}})
	assert.Equal(t, ps.Length(), 1)
	ps = b.addUnlocked(&ts.Packet{Header: ts.PacketHeader{ContinuityCounter: 7, HasPayload: true, PID: 1}})
	assert.Nil(t, ps)
	ps = b.dumpUnlocked()
	assert.Equal(t, ps.Length(), 2)
	assert.Equal(t, uint16(1), ps.Head().Header.PID)
	ps = b.dumpUnlocked()
	assert.Equal(t, ps.Length(), 1)
	assert.Equal(t, uint16(2), ps.Head().Header.PID)
	ps = b.dumpUnlocked()
	assert.Nil(t, ps)
}
