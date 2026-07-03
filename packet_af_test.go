package astits

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseInto(t *testing.T, p *Packet, bs []byte) {
	skip, err := p.parse(bs, EmptySkipper)
	require.NoError(t, err)
	require.False(t, skip)
}

func TestPacketEmbeddedAFReuse(t *testing.T) {
	p := NewPacket()
	defer p.Close()

	// Полный AF (фикстура: PCR/OPCR/private/extension)
	bs1, _ := packet(packetHeader, packetAdaptationField, []byte("payload"), false)
	parseInto(t, p, bs1[:MpegTsPacketSize])
	require.NotNil(t, p.AdaptationField)
	assert.Same(t, &p.af, p.AdaptationField)
	assert.Equal(t, []byte("test"), p.AdaptationField.TransportPrivateData)
	assert.NotNil(t, p.AdaptationField.AdaptationExtensionField)

	// Пакет без AF: указатель обязан обнулиться
	bs2, _ := packetShort(PacketHeader{HasPayload: true, PID: 0x100}, []byte{0xde})
	parseInto(t, p, bs2[:MpegTsPacketSize])
	assert.Nil(t, p.AdaptationField)

	// Минимальный AF (только RAI): стейл private/extension из первого парса
	// не должны протечь
	minimalAF := make([]byte, MpegTsPacketSize)
	minimalAF[0] = syncByte
	minimalAF[1] = 0x01
	minimalAF[2] = 0x00 // PID 0x100
	minimalAF[3] = 0x30 // AF+payload, CC=0
	minimalAF[4] = 0x01 // AF length
	minimalAF[5] = 0x40 // только RAI
	parseInto(t, p, minimalAF)
	require.NotNil(t, p.AdaptationField)
	assert.True(t, p.AdaptationField.RandomAccessIndicator)
	assert.False(t, p.AdaptationField.HasTransportPrivateData)
	assert.Nil(t, p.AdaptationField.TransportPrivateData)
	assert.Nil(t, p.AdaptationField.AdaptationExtensionField)

	// AF c Length=0 (однобайтовый стаффинг): флаговый байт не парсится вовсе —
	// стейловые Has-флаги предыдущего пакета давали фантомные PCR
	parseInto(t, p, bs1[:MpegTsPacketSize]) // снова полный AF с PCR
	require.True(t, p.AdaptationField.HasPCR)
	zeroLenAF := make([]byte, MpegTsPacketSize)
	zeroLenAF[0] = syncByte
	zeroLenAF[1] = 0x01
	zeroLenAF[2] = 0x00 // PID 0x100
	zeroLenAF[3] = 0x30 // AF+payload, CC=0
	zeroLenAF[4] = 0x00 // AF length = 0
	parseInto(t, p, zeroLenAF)
	require.NotNil(t, p.AdaptationField)
	assert.False(t, p.AdaptationField.HasPCR)
	assert.False(t, p.AdaptationField.RandomAccessIndicator)
}

// Владение AF в DemuxerData: поля обязаны переживать переиспользование пуловых
// пакетов последующим демуксом. Гоняется по реальному часовому файлу (env-gated).
func TestDemuxerDataOwnsAdaptationField(t *testing.T) {
	const (
		retainCount  = 5
		churnCount   = 200
		readBufBytes = 1 << 20
	)

	f, err := os.Open(goldenFile(t))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	dmx := NewDemuxer(context.Background(), bufio.NewReaderSize(f, readBufBytes),
		DemuxerOptPacketSize(MpegTsPacketSize),
		DemuxerOptSkipErrLimit(1),
	)

	type afSnap struct {
		rai  bool
		priv []byte
		pcr  uint64
	}
	var retained []*DemuxerData
	var snaps []afSnap
	for len(retained) < retainCount {
		d, derr := dmx.NextData()
		require.NoError(t, derr)
		if d.PES == nil || d.AdaptationField == nil {
			d.Close()
			continue
		}
		retained = append(retained, d)
		snaps = append(snaps, afSnap{
			rai:  d.AdaptationField.RandomAccessIndicator,
			priv: bytes.Clone(d.AdaptationField.TransportPrivateData),
			pcr:  d.AdaptationField.PCR.Base(),
		})
	}

	for i := 0; i < churnCount; i++ {
		d, derr := dmx.NextData()
		require.NoError(t, derr)
		d.Close()
	}

	for i, d := range retained {
		assert.Equal(t, snaps[i].rai, d.AdaptationField.RandomAccessIndicator, "data %d", i)
		assert.Equal(t, snaps[i].priv, d.AdaptationField.TransportPrivateData, "data %d", i)
		assert.Equal(t, snaps[i].pcr, d.AdaptationField.PCR.Base(), "data %d", i)
		d.Close()
	}
}
