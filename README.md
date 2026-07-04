# go-astits — opinionated fork

An opinionated fork of [asticode/go-astits](https://github.com/asticode/go-astits) focused
on hot-path performance of MPEG-TS demuxing and remuxing (live video and archives) — at the
cost of upstream compatibility. The API and semantics have diverged from the original for
good; this module is not and will never be a drop-in replacement.

Module path: `github.com/k-danil/go-astits/v2`.

## Layout

Dependency arrows point strictly downwards, no cycles:

| Package | Contents |
|---|---|
| `ts` | packet, header, adaptation field: parse + serialization, clock codecs (PCR/PTS/DTS/ESCR), CRC32, packet reader (copy and zero-copy view modes), `Packet.Raw()` |
| `pes` | PES packets: parse + serialization |
| `psi` | PSI tables (PAT/PMT/EIT/NIT/SDT/TOT): parse + serialization |
| `descriptor` | DVB/MPEG descriptors, one file per descriptor |
| `demux` | demuxer: per-PID packet accumulation, payload assembly, PSI dedup |
| `mux` | muxer: PES packetization, table generation and retransmission, raw passthrough |

API conventions: `Parse(bs []byte) (n int, err error)` on slices; `Put(bs []byte)` for
fixed-size serialization (panics on short buffer, like `binary.BigEndian`); `Append(dst
[]byte) []byte` for variable-size; `CalcLength() int` everywhere; constructors `demux.New` /
`mux.New`; functional options `WithX`. No dependencies outside the standard
library (`testify` in tests) and no `unsafe`: direct slice parsing and byte appending throughout, bit-level
test fixtures are built with an internal ~80-line bit writer.

## Pros

Measurements: one-hour SD stream (2.1 Mbit/s, 952 MB), single CPU thread (Apple M-series);
upstream — v1.15.0.

| Path | This fork | Upstream | Delta |
|---|---|---|---|
| Packet walk (`NextPacketTo`) | 5.4 GB/s, **~10 allocs/hour** | 1.85 GB/s, 10.5M allocs | ×2.9 / ×1,000,000 |
| Same, view mode + skipper | **9.2 GB/s** | — | — |
| PES assembly (`NextData`) | **3.3 GB/s**, 147k allocs, 75 MB garbage | 0.28 GB/s, 32.9M, 4.1 GB | ×11.8 / ×224 / ×54 |

How:

- **Direct parsing and serialization**: no bit-writer/byte-iterator abstractions on hot
  paths — slice cursors for reads, packet assembly in a scratch buffer with a single
  `Write` per packet; tables and descriptors serialize append-style with CRC computed over
  the produced slice.
- **Circular memory lifecycle**: packet pool + demuxer-local freelist (chain-linked returns,
  drained back to the global pool on EOF), payload and list pools, embedded structs instead
  of pointer fields (AF inside `ts.Packet`, PES and an owned AF copy inside `demux.Data`,
  optional header inside `pes.Header`), compact generic `pidmap` tables instead of maps
  keyed by PID.
- **Escape-analysis-friendly dispatch**: descriptor parsing dispatches through a switch, not
  a parser LUT — iterators stay on the stack; demuxer and muxer instances embed their slot
  arrays and scratch buffers, so a short-lived instance costs a handful of allocations.
- **Zero-copy view mode** (`demux.WithZeroCopyPackets`): batched reads, packets are views
  into the batch buffer.
- **`ts.PacketSkipper`** — header-level filtering before any payload work.
- **`Packet.Offset`** — a byte map of the stream, correct even with a skipper installed.
- **PSI dedup**: byte-identical repeats of PAT/PMT/… are neither parsed nor emitted.
- **Data ownership**: `AdaptationField`/`TransportPrivateData` inside `demux.Data` are
  owned copies, parsed PSI tables and descriptors own their payloads (guarded by
  dedicated ownership tests); retaining data on the consumer side is safe from pool reuse.
- **Hardened parsers**: fuzz targets for every direct parser plus randomized byte-exact
  roundtrip properties; corrupt input never panics and yields errors matchable with
  `errors.Is` — `ts.ErrInvalidData` classifies any corrupt-input failure,
  `psi.ErrCRC32Mismatch` flags checksum errors.
- **`Demuxer.Close()`** — deterministic resource return for demuxers abandoned before EOF;
  `Rewind()` cleans up after itself.
- **Muxer**: raw packet passthrough (`WritePacket` of `Packet.Raw()` with `UpdateHeader`),
  `SetCC`, table retransmission from cache.

## Problems and deliberate trade-offs

- **Incompatible with upstream** in both API and semantics. Compatibility is a non-goal.
- **The demuxer is single-goroutine** by contract — no internal locking.
- **Close discipline is mandatory**: `demux.Data.Close()` after use; a demuxer abandoned
  before EOF must be released via `Demuxer.Close()` — otherwise held resources go to the GC
  instead of the pools.
- **View mode**: packet memory is valid only until the next batch refill; `NextData` is
  forbidden in this mode (`ErrZeroCopyNextData`).
- **PSI dedup changes emission semantics**: a repeated section with identical bytes is not
  delivered to the consumer. The first occurrence and any change are.
- `demux.Data` is large (~1 KB due to embedded structs): cheap to allocate, expensive to
  retain by the thousands.
- Requires **Go ≥ 1.26**.

## Roadmap

- Per-PID byte accumulator: view-compatible `NextData`, one copy less.
- Full `demux.Data` pooling.
- Packet-level primitives: in-place PCR patching, PID rewrite over `Raw()`.
