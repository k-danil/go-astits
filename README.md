# go-astits — opinionated fork

An opinionated fork of [asticode/go-astits](https://github.com/asticode/go-astits) focused
on hot-path performance of MPEG-TS demuxing and remuxing (live video and archives) — at the
cost of upstream compatibility. The API and semantics have diverged from the original for
good; this module is not and will never be a drop-in replacement.

Module path: `github.com/k-danil/go-astits/v2`.

## Layout

Dependency arrows point strictly downwards, no cycles:

| Package      | Contents                                                                                                                                                       |
|--------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ts`         | packet, header, adaptation field: parse + serialization, clock codecs (PCR/PTS/DTS/ESCR), CRC32, packet reader (copy and zero-copy view modes, 188/192/204 autodetect), `Packet.Raw()` |
| `pes`        | PES packets: parse + serialization, full optional header (PTS/DTS, ESCR, ES rate, DSM trick mode, CRC, pack_header, extension)                                  |
| `psi`        | PSI/SI tables — MPEG-2 Systems + DVB-SI: parse and serialize, every table, byte-exact round-trip                                                                |
| `descriptor` | MPEG-2 Systems (ISO/IEC 13818-1, Table 2-45) + DVB (EN 300 468 §6) descriptors: parse + serialize, one file per descriptor; DVB extension descriptors in `descriptor/ext`; tags defined outside these two specs degrade to `Unknown` |
| `demux`      | demuxer: per-PID byte accumulator, event-based `Next`/`Events`, PSI table state, PSI dedup                                                                     |
| `mux`        | muxer: PES packetization, table generation and retransmission, raw passthrough                                                                                 |

API conventions: `Parse(bs []byte) (n int, err error)` on slices; `Put(bs []byte)` for
fixed-size serialization (panics on short buffer, like `binary.BigEndian`); `Append(dst
[]byte) []byte` for variable-size; `CalcLength() int` everywhere; constructors `demux.New` /
`mux.New`; functional options `WithX`. No dependencies outside the standard
library (`testify` in tests) and no `unsafe`: direct slice parsing and byte appending throughout, bit-level
test fixtures are built with an internal ~80-line bit writer.

## Pros

All rows share one setup: a one-hour SD recording (2.1 Mbit/s, 952 MB `.ts`) walked end to
end on a single CPU thread (Apple M1 Pro, Go 1.26). Every run reads from an **in-memory
`bytes.Reader`** (the consumer hands us a chunk already in RAM), builds a fresh demuxer, and
walks to EOF; alloc and garbage figures are **per full pass** (one hour of content). Upstream
`asticode/go-astits` v1.15.0 is measured with the identical harness, file, and substrate — it
is GC-bound at this scale (10–33M allocs/pass), so its throughput varies run to run.

| Path                                                             | This fork                                  | Upstream v1.15.0                | Delta                        |
|------------------------------------------------------------------|--------------------------------------------|---------------------------------|------------------------------|
| Packet walk (`NextPacketTo`)                                     | **11 GB/s**, 3 allocs, 2.8 KB              | 2.6 GB/s, 10.5M allocs, 1.2 GB  | ×4.2 / ×3,500,000 / ×420,000 |
| Same, view mode + skipper (most packets skipped at header level) | **17 GB/s**, 5 allocs, 0.8 MB batch buffer | — (no equivalent)               | —                            |
| PES + tables via events (`Next`)                                 | **6.4 GB/s**, ~22 allocs, 90 KB garbage    | 0.33 GB/s, 32.9M allocs, 3.9 GB | ×19 / ×1,500,000 / ×43,000   |

How:

- **Full standard coverage** (parse + byte-exact serialize round-trip): every descriptor,
  PSI/SI table and PES-header field whose *syntax is defined in* ISO/IEC 13818-1 (H.222.0) or
  ETSI EN 300 468 (DVB-SI) — the complete descriptor sets of both (ISO Table 2-45 and DVB §6,
  main plus extension), every table (PAT/CAT/PMT/TSDT, NIT/BAT/SDT/EIT/TDT/TOT/RST/ST/DIT/SIT,
  ISO_IEC_14496 and metadata sections), and the full PES optional header (CRC and pack_header
  included). Structures those two documents defer to other specifications — payloads
  referencing ISO/IEC 14496, DSM-CC (13818-6) or IPMP (13818-11) — are carried verbatim
  rather than decoded; tags defined outside the two are surfaced as `Unknown`.
- **Direct parsing and serialization**: no bit-writer/byte-iterator abstractions on hot
  paths — slice cursors for reads (the 4-byte TS header lands in one big-endian `uint32`,
  its fields sliced out in registers), packet assembly in a scratch buffer with a single
  `Write` per packet; tables and descriptors serialize append-style with CRC computed over
  the produced slice.
- **Event-based demux** (`Next() (Event, error)` and the `Events()` iterator): one call
  advances to the next `EventPES` or a typed table event (`EventPAT`/`EventPMT`/`EventEIT`/…).
  A completed unit is claimed via `PES()` (pool-owned, `Close()` when done retaining it);
  table state is read through `Section()`/`PAT()`/`PMT()`. The full MPEG-2 systems + DVB-SI
  table set is parsed, each surfaced as its own typed event; everything beyond PAT/PMT is off
  by default (`WithDVBTables`). `WithPSIRepeats` also emits byte-identical repeats
  (`TableChanged` distinguishes them) for stream-composition analysis.
- **Per-PID byte accumulator**: each PID assembles its unit into one contiguous pooled
  buffer sized from the unit's own length hint (PSI section length, PES packet length) with
  a sticky-max fallback — packets are one-shot scratch, so both copy and view modes reach the
  parser with a single copy and no per-unit allocation.
- **Circular memory lifecycle**: payload buffers cycle through size-classed pools, PES units
  through their own pool; embedded structs instead of pointer fields (AF inside `ts.Packet`,
  PES data and an owned AF copy inside `demux.PES`, optional header inside `pes.Header`),
  compact generic `pidmap` tables instead of maps keyed by PID. The AF has no inline
  private-data buffer: `TransportPrivateData` views the packet and is copied (into a reused
  backing) only when a unit is retained.
- **Escape-analysis-friendly dispatch**: descriptor parsing dispatches through a switch, not
  a parser LUT — iterators stay on the stack; demuxer and muxer instances embed their slot
  arrays and scratch buffers, so a short-lived instance costs a handful of allocations.
- **Zero-copy view mode** (`demux.WithZeroCopyPackets`): batched reads, packets are views
  into the batch buffer; the accumulator copies payloads out before the refill, so the event
  API works unchanged in this mode. `Packet.Raw()` returns the view as well, so packet-level
  passthrough and PID rewrite over `Raw()` run without leaving zero-copy. A `*bufio.Reader`
  source is not re-buffered: the batch peeks views straight into the reader's own buffer, so
  a buffered reader — which already holds the bytes — is never copied a second time.
- **Multi-format packet reader**: plain TS (188), M2TS (192, with the 4-byte
  TP_extra_header exposed as `Packet.Prefix` / decoded by `ArrivalTimeStamp()`) and
  Reed-Solomon (204) are read transparently. The size is autodetected by locking onto the
  recurring sync byte — a stray `0x47` in payload or parity doesn't mislead it — or pinned
  with `WithPacketSize`.
- **Sync lock** (`demux.WithSyncLock`) — for UDP/RTP or otherwise torn feeds: aligns to the
  first sync byte at any offset within a packet and re-locks after a lost or corrupt packet,
  peeking ahead through a `ts.Peeker` (a raw reader is wrapped in bufio). Off by default so
  aligned files stay on the zero-wrap fast path; `WithResyncLimit` bounds recovery.
- **`ts.PacketSkipper`** — header-level filtering before any payload work.
- **`Packet.Offset`** — a byte map of the stream, correct even with a skipper installed.
- **`demux.WithPacketHook`** — a callback run on every raw packet as it is read (after the
  skipper, before unit assembly), so one `Next` traversal can serve both packet-level work
  (indexing, PID/PCR sampling) and unit-level demuxing without a second pass. The packet is
  valid only for the duration of the call.
- **PSI dedup**: byte-identical repeats of PAT/PMT/… are neither parsed nor emitted (unless
  `WithPSIRepeats` is set, and even then repeats reuse the cached parse — no re-parse).
- **Data ownership**: `AdaptationField`/`TransportPrivateData` inside a claimed `demux.PES`
  are owned copies, parsed PSI tables and descriptors own their payloads (guarded by
  dedicated ownership tests); retaining data on the consumer side is safe from pool reuse.
- **Hardened parsers**: fuzz targets for every direct parser plus randomized byte-exact
  roundtrip properties; corrupt input never panics and yields errors matchable with
  `errors.Is` — `ts.ErrInvalidData` classifies any corrupt-input failure,
  `psi.ErrCRC32Mismatch` flags checksum errors.
- **`Demuxer.Close()`** — deterministic resource return for demuxers abandoned before EOF;
  `Rewind()` cleans up after itself.
- **Muxer**: raw packet passthrough (`WritePacket` of `Packet.Raw()` with `UpdateHeader`),
  `SetCC`, table retransmission from cache; PAT spans sections and packets when needed,
  oversize sections are rejected (`psi.ErrSectionOverflow`) instead of silently corrupted.

## Problems and deliberate trade-offs

- **Incompatible with upstream** in both API and semantics. Compatibility is a non-goal.
- **The demuxer is single-goroutine** by contract — no internal locking.
- **Close discipline**: a `demux.PES` claimed via `PES()` must be `Close()`d when you stop
  retaining it (an unclaimed unit is released by the next `Next`); a demuxer abandoned before
  EOF must be released via `Demuxer.Close()` — otherwise held resources go to the GC instead
  of the pools.
- **View mode**: packet memory is valid only until the next batch refill. The event API is
  unaffected (the accumulator copies out), but a `Packet` held from `NextPacketTo` is not.
- **PSI dedup changes emission semantics** by default: a repeated section with identical
  bytes is not delivered (only the first occurrence and any change are). Opt out with
  `WithPSIRepeats`.
- **`Next` results are borrowed**: `PES()` before `Close` and `Section()` are valid only
  until the next `Next`; retaining beyond that means claiming (`PES`) or copying.
- Requires **Go ≥ 1.26**.

## Roadmap

- Packet-level primitives: in-place PCR patching, PID rewrite over `Raw()`.
