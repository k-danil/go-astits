# go-astits — opinionated fork

An opinionated fork of [asticode/go-astits](https://github.com/asticode/go-astits) focused
on hot-path performance of MPEG-TS demuxing and remuxing (live video and archives) — at the
cost of upstream compatibility. The API and semantics have diverged from the original for
good; this module is not and will never be a drop-in replacement.

## Pros

Measurements: one-hour SD stream (2.1 Mbit/s, 952 MB), single CPU thread (Apple M-series);
upstream — v1.15.0.

| Path | This fork | Upstream | Delta |
|---|---|---|---|
| Packet walk (`NextPacketTo`) | 5.4 GB/s, **15 allocs/hour** | 1.85 GB/s, 10.5M allocs | ×2.9 / ×700,000 |
| Same, view mode + skipper | **9.0 GB/s** | — | — |
| PES assembly (`NextData`) | **3.3 GB/s**, 147k allocs, 75 MB garbage | 0.28 GB/s, 32.9M, 4.1 GB | ×11.8 / ×224 / ×54 |

How:

- **Direct parsing and serialization**: no `BytesIterator`/`BitsWriter` on hot paths —
  slice cursors for reads, packet assembly in a scratch buffer with a single `Write` per
  packet for writes.
- **Circular memory lifecycle**: packet pool + demuxer-local freelist (chain-linked returns,
  drained back to the global pool on EOF), payload and list pools, embedded structs instead
  of pointer fields (AF inside `Packet`, PES and an owned AF copy inside `DemuxerData`,
  optional header inside `PESHeader`), compact generic `pidMap` tables instead of maps
  keyed by PID.
- **Zero-copy view mode** (`DemuxerOptZeroCopyPackets`): batched reads, packets are views
  into the batch buffer.
- **`PacketSkipper`** — header-level filtering before any payload work.
- **`Packet.Offset`** — a byte map of the stream, correct even with a skipper installed.
- **PSI dedup**: byte-identical repeats of PAT/PMT/… are neither parsed nor emitted.
- **Data ownership**: `AdaptationField`/`TransportPrivateData` inside `DemuxerData` are
  owned copies; retaining data on the consumer side is safe from pool reuse.
- **`Demuxer.Close()`** — deterministic resource return for demuxers abandoned before EOF;
  `Rewind()` cleans up after itself.
- **Muxer**: raw packet passthrough (`WritePacket` with `UpdateHeader`), `SetCC`,
  table retransmission from cache.
- Byte-for-byte output identity across optimizations is guarded by golden tests on real
  streams (demux differential against an independent parser + remux references).

## Problems and deliberate trade-offs

- **Incompatible with upstream** in both API and semantics. Compatibility is a non-goal.
- **The demuxer is single-goroutine** by contract — no internal locking.
- **Close discipline is mandatory**: `DemuxerData.Close()` after use; a demuxer abandoned
  before EOF must be released via `Demuxer.Close()` — otherwise held resources go to the GC
  instead of the pools.
- **View mode**: packet memory is valid only until the next batch refill; `NextData` is
  forbidden in this mode (`ErrZeroCopyNextData`).
- **PSI dedup changes emission semantics**: a repeated section with identical bytes is not
  delivered to the consumer. The first occurrence and any change are.
- **The PSI path retains its payload buffer forever** (descriptors are views into it):
  a deliberate leak of one buffer per section change.
- `DemuxerData` is large (~1 KB due to embedded structs): cheap to allocate, expensive to
  retain by the thousands.
- Upstream `cmd/` tools are unsupported and do not build.
- Requires **Go ≥ 1.26**.
- Part of the test suite runs against real assets (env `GOLDEN_TS_DIR`); without it only
  unit tests run.

## Roadmap

- Split into packages (`ts` / `pes` / `psi` / `demux` / `mux`) with a move to `/v2`.
- Fuzz and roundtrip tests for parsers/serializers.
- Per-PID byte accumulator: view-compatible `NextData`, one copy less.
- Full `DemuxerData` pooling and PSI ownership (descriptor copying).