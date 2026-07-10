package demux

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-danil/go-astits/v2/ts"
)

// Дифференциал и бенч view-режима на реальном часовом TS (GOLDEN_TS_DIR, иначе скип).
const (
	viewBatchSize   = 4096
	copyBufferBytes = 1 << 20
)

const goldenDirEnv = "GOLDEN_TS_DIR"

func goldenFile(t testing.TB) string {
	dir := os.Getenv(goldenDirEnv)
	if dir == "" {
		t.Skipf("%s not set", goldenDirEnv)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.ts"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	sort.Strings(files)
	return files[0]
}

func pusiOrAFSkipper(p *ts.Packet) bool {
	return !p.Header.PayloadUnitStartIndicator && !p.Header.HasAdaptationField
}

// walkDigest сворачивает всё извлечённое из пакетов в один хэш: любые расхождения
// копи- и view-режимов всплывают одним числом.
func walkDigest(t testing.TB, path string, zeroCopy bool, skipper ts.PacketSkipper) (digest uint64, packets int) {
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	opts := []func(*Demuxer){WithPacketSize(ts.PacketSize), WithSkipErrLimit(1)}
	var r io.Reader = f
	if zeroCopy {
		opts = append(opts, WithZeroCopyPackets(viewBatchSize))
	} else {
		r = bufio.NewReaderSize(f, copyBufferBytes)
	}
	if skipper != nil {
		opts = append(opts, WithPacketSkipper(skipper))
	}
	dmx := New(context.Background(), r, opts...)

	h := fnv.New64a()
	var scratch [10]byte
	p := ts.NewPacket()
	defer p.Close()
	for {
		if err = dmx.NextPacketTo(p); err != nil {
			require.True(t, errors.Is(err, ts.ErrNoMorePackets))
			return h.Sum64(), packets
		}
		packets++

		scratch[0] = byte(p.Header.PID >> 8)
		scratch[1] = byte(p.Header.PID)
		scratch[2] = b2b(p.Header.PayloadUnitStartIndicator)
		scratch[3] = b2b(p.Header.HasAdaptationField)
		scratch[4] = p.Header.ContinuityCounter
		scratch[5] = byte(p.Offset >> 24)
		scratch[6] = byte(p.Offset >> 16)
		scratch[7] = byte(p.Offset >> 8)
		scratch[8] = byte(p.Offset)
		scratch[9] = 0
		if p.AdaptationField != nil {
			scratch[9] = b2b(p.AdaptationField.RandomAccessIndicator)
			_, _ = h.Write(p.AdaptationField.TransportPrivateData)
		}
		_, _ = h.Write(scratch[:])
		_, _ = h.Write(p.Payload)
	}
}

func b2b(v bool) byte {
	if v {
		return 1
	}
	return 0
}

func TestZeroCopyDigestMatchesCopyMode(t *testing.T) {
	path := goldenFile(t)
	tests := []struct {
		name    string
		skipper ts.PacketSkipper
	}{
		{"noSkipper", nil},
		{"pusiOrAFSkipper", pusiOrAFSkipper},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			copyDigest, copyPackets := walkDigest(t, path, false, tc.skipper)
			viewDigest, viewPackets := walkDigest(t, path, true, tc.skipper)
			require.Equal(t, copyPackets, viewPackets)
			require.Equal(t, copyDigest, viewDigest)
		})
	}
}

// walkBench — лёгкий проход в духе индексатора: поля хедера/AF + длины, без
// сканирования пейлоада (иначе бенч меряет хэш, а не демукс).
func walkBench(t testing.TB, path string, zeroCopy bool, skipper ts.PacketSkipper) (sink int64) {
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	opts := []func(*Demuxer){WithPacketSize(ts.PacketSize), WithSkipErrLimit(1)}
	var r io.Reader = f
	if zeroCopy {
		opts = append(opts, WithZeroCopyPackets(viewBatchSize))
	} else {
		r = bufio.NewReaderSize(f, copyBufferBytes)
	}
	if skipper != nil {
		opts = append(opts, WithPacketSkipper(skipper))
	}
	dmx := New(context.Background(), r, opts...)

	p := ts.NewPacket()
	defer p.Close()
	for {
		if err = dmx.NextPacketTo(p); err != nil {
			require.True(t, errors.Is(err, ts.ErrNoMorePackets))
			return
		}
		sink += int64(p.Header.PID) + p.Offset + int64(len(p.Payload))
		if p.AdaptationField != nil {
			sink += int64(len(p.AdaptationField.TransportPrivateData))
		}
	}
}

func benchPackets(b *testing.B, zeroCopy bool, skipper ts.PacketSkipper) {
	path := goldenFile(b)
	fi, err := os.Stat(path)
	require.NoError(b, err)
	b.SetBytes(fi.Size())
	b.ReportAllocs()
	for b.Loop() {
		walkBench(b, path, zeroCopy, skipper)
	}
}

func BenchmarkPacketsCopy(b *testing.B)        { benchPackets(b, false, nil) }
func BenchmarkPacketsView(b *testing.B)        { benchPackets(b, true, nil) }
func BenchmarkPacketsCopySkipper(b *testing.B) { benchPackets(b, false, pusiOrAFSkipper) }
func BenchmarkPacketsViewSkipper(b *testing.B) { benchPackets(b, true, pusiOrAFSkipper) }

// Claimed PES ownership: fields must survive pooled buffer reuse by
// subsequent demuxing. Runs against a real hour-long file (env-gated).
func TestDemuxerPESOwnership(t *testing.T) {
	const (
		retainCount  = 5
		churnCount   = 200
		readBufBytes = 1 << 20
	)

	f, err := os.Open(goldenFile(t))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	dmx := New(context.Background(), bufio.NewReaderSize(f, readBufBytes),
		WithPacketSize(ts.PacketSize),
		WithSkipErrLimit(1),
	)

	type pesSnap struct {
		rai     bool
		priv    []byte
		pcr     uint64
		payload []byte
	}
	var retained []*PES
	var snaps []pesSnap
	for len(retained) < retainCount {
		ev, derr := dmx.Next()
		require.NoError(t, derr)
		if ev != EventPES {
			continue
		}
		d := dmx.PES()
		if d.AdaptationField == nil {
			continue // released by the next Next
		}
		retained = append(retained, d)
		snaps = append(snaps, pesSnap{
			rai:     d.AdaptationField.RandomAccessIndicator,
			priv:    bytes.Clone(d.AdaptationField.TransportPrivateData),
			pcr:     d.AdaptationField.PCR.Base(),
			payload: bytes.Clone(d.Data.Data),
		})
	}

	for i := 0; i < churnCount; i++ {
		_, derr := dmx.Next()
		require.NoError(t, derr)
	}

	for i, d := range retained {
		assert.Equal(t, snaps[i].rai, d.AdaptationField.RandomAccessIndicator, "pes %d", i)
		assert.Equal(t, snaps[i].priv, d.AdaptationField.TransportPrivateData, "pes %d", i)
		assert.Equal(t, snaps[i].pcr, d.AdaptationField.PCR.Base(), "pes %d", i)
		assert.Equal(t, snaps[i].payload, d.Data.Data, "pes %d", i)
		d.Close()
	}
	dmx.Close()
}

// walkNextDigest сворачивает весь событийный поток в один хэш: PES-эмиссии
// байт-в-байт + таблицы. Дифференциал copy- vs view-режима для Next-пути.
func walkNextDigest(t testing.TB, path string, zeroCopy bool) (digest uint64, events int) {
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	opts := []func(*Demuxer){WithPacketSize(ts.PacketSize), WithSkipErrLimit(1), WithDVBTables()}
	var r io.Reader = f
	if zeroCopy {
		opts = append(opts, WithZeroCopyPackets(viewBatchSize))
	} else {
		r = bufio.NewReaderSize(f, copyBufferBytes)
	}
	dmx := New(context.Background(), r, opts...)
	defer dmx.Close()

	h := fnv.New64a()
	var scratch [8]byte
	for {
		ev, derr := dmx.Next()
		if derr != nil {
			require.True(t, errors.Is(derr, ts.ErrNoMorePackets))
			return h.Sum64(), events
		}
		events++
		if ev == EventPES {
			d := dmx.PES()
			scratch[0] = byte(d.PID >> 8)
			scratch[1] = byte(d.PID)
			scratch[2] = d.ContinuityCounter
			scratch[3] = b2b(d.AdaptationField != nil)
			scratch[4] = byte(d.Data.Header.StreamID)
			scratch[5] = b2b(d.Data.Header.OptionalHeader != nil)
			if d.AdaptationField != nil {
				scratch[6] = b2b(d.AdaptationField.RandomAccessIndicator)
				scratch[7] = b2b(d.AdaptationField.HasPCR)
				if d.AdaptationField.HasPCR {
					var pcr [8]byte
					binary.BigEndian.PutUint64(pcr[:], uint64(d.AdaptationField.PCR.Base()))
					_, _ = h.Write(pcr[:])
				}
			}
			_, _ = h.Write(scratch[:])
			_, _ = h.Write(d.Data.Data)
		} else {
			pid, _ := dmx.Section()
			scratch[0] = byte(pid >> 8)
			scratch[1] = byte(pid)
			scratch[2] = byte(ev)
			_, _ = h.Write(scratch[:3])
		}
	}
}

func TestNextDigestMatchesAcrossModes(t *testing.T) {
	path := goldenFile(t)
	copyDigest, copyEvents := walkNextDigest(t, path, false)
	viewDigest, viewEvents := walkNextDigest(t, path, true)
	require.Equal(t, copyEvents, viewEvents)
	require.Equal(t, copyDigest, viewDigest)
}

// BenchmarkNextFile is the phase-C reper: full event pass over the hour-long
// asset.
func BenchmarkNextFile(b *testing.B) {
	data, err := os.ReadFile(goldenFile(b))
	require.NoError(b, err)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()

	for b.Loop() {
		dmx := New(context.Background(), bytes.NewReader(data),
			WithPacketSize(ts.PacketSize), WithSkipErrLimit(1))
		for {
			ev, derr := dmx.Next()
			if derr != nil {
				if errors.Is(derr, ts.ErrNoMorePackets) {
					break
				}
				continue
			}
			if ev == EventPES {
				dmx.PES().Close()
			}
		}
		dmx.Close()
	}
}

func BenchmarkPacketsInMem(b *testing.B) {
	data, err := os.ReadFile(goldenFile(b))
	require.NoError(b, err)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	p := ts.NewPacket()
	defer p.Close()
	var sink int64
	for b.Loop() {
		dmx := New(context.Background(), bytes.NewReader(data),
			WithPacketSize(ts.PacketSize), WithSkipErrLimit(1))
		for {
			if err := dmx.NextPacketTo(p); err != nil {
				break
			}
			sink += int64(p.Header.PID)
		}
		dmx.Close()
	}
	_ = sink
}

// View-режим БЕЗ скиппера: honest-гейт для консьюмера, который смотрит каждый
// пакет (чанкер) — жадный parse header+AF на всём потоке, ничего не пропущено.
// Контролируемое A/B к BenchmarkPacketsInMem (тот же sink; переменная — copy vs view).
func BenchmarkPacketsInMemView(b *testing.B) {
	data, err := os.ReadFile(goldenFile(b))
	require.NoError(b, err)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	p := ts.NewPacket()
	defer p.Close()
	var sink int64
	for b.Loop() {
		dmx := New(context.Background(), bytes.NewReader(data),
			WithPacketSize(ts.PacketSize), WithSkipErrLimit(1),
			WithZeroCopyPackets(viewBatchSize))
		for {
			if err := dmx.NextPacketTo(p); err != nil {
				break
			}
			sink += int64(p.Header.PID)
		}
		dmx.Close()
	}
	_ = sink
}

func BenchmarkPacketsInMemViewSkipper(b *testing.B) {
	data, err := os.ReadFile(goldenFile(b))
	require.NoError(b, err)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	p := ts.NewPacket()
	defer p.Close()
	var sink int64
	for b.Loop() {
		dmx := New(context.Background(), bytes.NewReader(data),
			WithPacketSize(ts.PacketSize), WithSkipErrLimit(1),
			WithZeroCopyPackets(viewBatchSize), WithPacketSkipper(pusiOrAFSkipper))
		for {
			if err := dmx.NextPacketTo(p); err != nil {
				break
			}
			sink += int64(p.Header.PID)
		}
		dmx.Close()
	}
	_ = sink
}
