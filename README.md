# go-astits — opinionated fork

An opinionated fork of [asticode/go-astits](https://github.com/asticode/go-astits) focused
on hot-path performance of MPEG-TS demuxing and remuxing (live video and archives) — at the
cost of upstream compatibility. The API and semantics have diverged from the original for
good; this module is not and will never be a drop-in replacement.

Module path: `github.com/k-danil/go-astits/v3` (v3 is a breaking release, see
[Migrating to v3](#migrating-to-v3)).

## Layout

Dependency arrows point strictly downwards, no cycles:

| Package      | Contents                                                                                                                                                       |
|--------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ts`         | packet, header, adaptation field: parse + serialization; `ClockReference` as 27 MHz ticks with the PCR/PTS/DTS/ESCR codecs; CRC32; the packet reader (windowed, zero-copy views, sync lock, 188/192/204 autodetect); recoverable-error model |
| `tsio`       | the reader contract (`Peeker`, optional `Tagger`) and its reference implementations: `BytesReader` over a slice or mmap, `SeekBuffer` over a seekable source with an in-buffer seek-back window |
| `pes`        | PES packets: parse + serialization, full optional header (PTS/DTS, ESCR, ES rate, DSM trick mode, CRC, pack_header, extension)                                  |
| `psi`        | PSI/SI tables — MPEG-2 Systems + DVB-SI: parse and serialize, every table, byte-exact round-trip                                                                |
| `descriptor` | MPEG-2 Systems (ISO/IEC 13818-1, Table 2-45) + DVB (EN 300 468 §6) descriptors: parse + serialize, one file per descriptor; DVB extension descriptors in `descriptor/ext`; tags defined outside these two specs degrade to `Unknown` |
| `dvbtext`    | DVB SI text fields (EN 300 468 annex A): character table selection, decoding to UTF-8 and encoding back; the text and ISO 639/3166 code types carried by the descriptors |
| `demux`      | demuxer: windowed packet walk, per-PID byte accumulator, event-based `Next`/`Events`, PSI table state and dedup, opt-in recoverable errors                       |
| `mux`        | muxer: PES packetization, table generation and retransmission, raw passthrough                                                                                 |

API conventions: `Parse(bs []byte) (n int, err error)` on slices; `Put(bs []byte)` for
fixed-size serialization (panics on short buffer, like `binary.BigEndian`); `Append(dst
[]byte) []byte` for variable-size; `CalcLength() int` everywhere; constructors `demux.New` /
`mux.New`; functional options `WithX`. The public packages ship executable examples (`go doc`
or `example_test.go`): the demux loop and its options, both `tsio` readers and a `Tagger`,
direct parse and serialization in `ts`/`psi`/`pes`/`descriptor`, the muxer. Outside the standard library it depends only on
`golang.org/x/text` for the DVB text codecs (`testify` in tests), and uses no `unsafe`:
direct slice parsing and byte appending throughout, bit-level test fixtures are built with an
internal ~80-line bit writer.

## Pros

All rows share one setup: a one-hour SD recording (2.1 Mbit/s, 952 MB `.ts`) walked end to
end on a single CPU thread (Apple M1 Pro, Go 1.26), the chunk already in RAM. The fork's
rows read it through `tsio.BytesReader` (no copy between the input and the parser); alloc
and garbage figures are **per full pass** (one hour of content). Upstream
`asticode/go-astits` v1.15.0 is measured with the identical harness and file over a
`bytes.Reader` — it is GC-bound at this scale (10–33M allocs/pass), so its throughput varies
run to run. Over a plain `bytes.Reader` (one bufio copy) the fork's event row is 7.3 GiB/s
and the packet rows 11–13 GiB/s.

| Path                                                             | This fork                               | Upstream v1.15.0                | Delta                        |
|------------------------------------------------------------------|-----------------------------------------|---------------------------------|------------------------------|
| Packet walk (`NextPacketTo`, views into the slice)               | **13.7 GiB/s**, 7 allocs, 4 KB          | 2.6 GB/s, 10.5M allocs, 1.2 GB  | ×5.7 / ×1,500,000 / ×300,000 |
| Same + skipper (most packets skipped at header level)            | **16.9 GiB/s**, 7 allocs, 4 KB          | — (no equivalent)               | —                            |
| PES + tables via events (`Next`, sync lock, unbounded limits)    | **8.7 GiB/s**, ~23 allocs, ~50 KB       | 0.33 GB/s, 32.9M allocs, 3.9 GB | ×28 / ×1,400,000 / ×78,000   |

How:

- **Full standard coverage** (parse + byte-exact serialize round-trip): every descriptor,
  PSI/SI table and PES-header field whose *syntax is defined in* ISO/IEC 13818-1 (H.222.0) or
  ETSI EN 300 468 (DVB-SI) — the complete descriptor sets of both (ISO Table 2-45 and DVB §6,
  main plus extension), every table (PAT/CAT/PMT/TSDT, NIT/BAT/SDT/EIT/TDT/TOT/RST/ST/DIT/SIT,
  ISO_IEC_14496 and metadata sections), and the full PES optional header (CRC and pack_header
  included). Structures those two documents defer to other specifications — payloads
  referencing ISO/IEC 14496, DSM-CC (13818-6) or IPMP (13818-11) — are carried verbatim
  rather than decoded; tags defined outside the two are surfaced as `Unknown`. The DVB
  baseline is EN 300 468 V1.19.1 (2025-02): `S2SatelliteDeliverySystem` follows its Table 42
  (`NotTimesliceFlag`, `TSGSMode`, `TimesliceNumber`), `ServiceType` its Table 89, and the
  extension tag list runs to 0x24.
- **Descriptor body contract**: each body is parsed against its own `descriptor_length` and
  never borrows a neighbour's bytes — a body that needs more than it declares becomes a
  `*Malformed` carrying the declared bytes verbatim, so the loop stays aligned and `Append`
  reproduces the input byte for byte. Bytes left past a fixed layout are the encoder's error,
  not data: the body becomes a `*Malformed` as well, byte-exact through `Raw`. Only bodies
  whose syntax table ends in a growing loop keep a tail of their own — `short_smoothing_buffer`
  (Table 94) and `T2MI` (Table 158) in `Reserved` (JSON `_reserved`), `VBI_data`'s
  unrecognised services — while AC-3, E-AC-3, AC-4, DTS, DTS-HD, DTS Neural and AAC keep
  theirs in the named `AdditionalInfo` field. Loop-shaped bodies (`ISO_639`, `FMC`, `content`,
  `multilingual_*`, `cell_list`, …) type what their loop can and reject the rest as
  `*Malformed`. A zero-length body goes to its parser like any other: legally
  empty tags keep their type, tags with a mandatory body become `*Malformed`, unassigned tags
  stay `Unknown`. Reserved bits are re-encoded to the value the spec mandates for their
  family (`reserved`/`reserved_future_use` 1, `reserved_zero_future_use` 0). DVB dates and
  durations are validated on the wire: a BCD nibble above 9 fails the body with an error of
  class `ts.ErrInvalidData` (a `*Malformed` descriptor, a `SectionError` for TDT/TOT/EIT), and
  on the write side out-of-range values saturate — durations at 99:59:59, dates at the 16-bit
  MJD range (1858-11-17 … 2038-04-21, one day short of the 0xFFFF undefined marker).
  Writing does not validate lengths: `Append` writes an
  8-bit body length and `AppendWithLength` a 12-bit loop length, so a body over 255 bytes or a
  list over 4095 is the caller's to split.
- **Direct parsing and serialization**: no bit-writer/byte-iterator abstractions on hot
  paths — slice cursors for reads (the 4-byte TS header lands in one big-endian `uint32`,
  its fields sliced out in registers), packet assembly in a scratch buffer with a single
  `Write` per packet; tables and descriptors serialize append-style with CRC computed over
  the produced slice.
- **Event-based demux** (`Next() (Event, error)` and the `Events()` iterator): one call
  advances to the next `EventPES` or a typed table event (`EventPAT`/`EventPMT`/`EventEIT`/…).
  A completed unit is claimed via `PES()` (pool-owned, `Close()` when done retaining it,
  carrying a `PacketSpan`: the offsets and reader tags of its first and last packet);
  `Section()` hands over the whole section behind a table event — table_id, version,
  current_next_indicator, section numbers and the typed body — and `SectionSpan()` the
  `PacketSpan` of the unit it came from, so a table can be located in the file or charged to
  the datagram that carried it; `PAT()`/`PMT()` hold the tables in effect. The full MPEG-2 systems + DVB-SI
  table set is parsed, each surfaced as its own typed event; everything beyond PAT/PMT is off
  by default (`WithDVBTables`). `WithPSIRepeats` also emits byte-identical repeats
  (`TableChanged` distinguishes them) for stream-composition analysis. Under
  `WithRecoverableErrors`, `EventError` additionally surfaces skipped corruption (below).
- **Per-PID byte accumulator**: each PID assembles its unit into one contiguous pooled
  buffer sized from the unit's own length hint (PSI section length, PES packet length) with
  a sticky-max fallback — packets are one-shot scratch, so both copy and view modes reach the
  parser with a single copy and no per-unit allocation. A unit is capped (`WithMaxUnitSize`,
  16 MB for PES and 64 KB for PSI by default; `-1` unbounded) and torn with
  `ts.ErrUnitTooLarge` past the cap; the buffer behind a PID comes from a power-of-two size
  class chosen no larger than the cap, so the limit bounds resident memory, not just the
  delivered unit, and a PID whose unit start never comes cannot buffer the stream. Null
  packets are counted but never accumulated. `PacketCounts()` reports packets seen per PID,
  every packet the reader delivered. PAT, CAT and TSDT (PIDs 0–2) are always assembled as
  PSI; `WithDVBTables` adds the DVB SI PIDs. Sections packed back to back are followed across
  the unit boundary: on a start indicator the bytes ahead of `pointer_field` finish the
  section the previous packet left open before the unit is handed over. A PAT announced for
  later (`current_next_indicator` 0) is tracked apart from the map in effect: its PMT PIDs
  parse as PSI only while the PID has never delivered an elementary-stream unit, so an
  announcement cannot take over a PID that is carrying PES.
- **Circular memory lifecycle**: payload buffers cycle through size-classed pools, PES units
  through their own pool; embedded structs instead of pointer fields (AF inside `ts.Packet`,
  PES data and an owned AF copy inside `demux.PES`, optional header inside `pes.Header`),
  compact generic `pidmap` tables instead of maps keyed by PID. The AF has no inline
  private-data buffer: `TransportPrivateData` views the packet and is copied (into a reused
  backing) only when a unit is retained.
- **Escape-analysis-friendly dispatch**: descriptor parsing dispatches through a switch, not
  a parser LUT — iterators stay on the stack; demuxer and muxer instances embed their slot
  arrays and scratch buffers, so a short-lived instance costs a handful of allocations.
- **Windowed reads**: the reader is always a `tsio.Peeker` (`*bufio.Reader` qualifies; any other
  source is wrapped in one sized to the window), and the event API parses a whole window of
  packets in place per refill — no read call, no copy and no call chain per packet. A window
  is cut to what the peeker already holds, so a live feed is never waited on for more than one
  packet (sync lock's damage paths aside), and to 1024 packets, the cancellation cadence. `demux.WithZeroCopyPackets` hands `NextPacketTo` packets as views into the window
  (valid until the next refill) instead of copying each one out, and sizes the wrapping bufio
  when there is one; `Packet.Raw()` returns the view, so packet-level passthrough and PID
  rewrite over `Raw()` stay zero-copy.
- **Reference readers** (`tsio`): `BytesReader` serves a slice — a chunk in memory, an
  mmap'd file — with no copy anywhere between the input and the parser; `SeekBuffer` reads a
  seekable source through a bounded buffer that keeps an in-buffer seek-back window, so a
  rewind after a prefix scan costs no I/O. Both satisfy `tsio.Peeker`; a live feed brings its
  own (a datagram reassembler, say). A reader that also implements `tsio.Tagger` stamps each
  window with an opaque `uint64` — a hardware or software receive timestamp, an RTP sequence —
  carried untouched as `Packet.Tag` and `PES.FirstPacketTag`/`LastPacketTag`.
- **Plain readers and sockets**: any `io.Reader` works — a reader that is not a `tsio.Peeker`
  (or a peeker whose `Size()` is below 204) is wrapped in bufio (13 KB by default; N packets,
  at least 409 bytes, under `WithZeroCopyPackets(N)` — sized for 204-byte packets until
  `WithPacketSize` pins the format) and, once that buffer is drained, read one packet at a
  time, so a live socket is never waited on for more than a packet (the sync-lock damage
  paths below are the exception). A tail shorter than a packet at EOF is left in the reader
  and reported once (`ErrorKindPacketDrop` with its length as `Dropped`), so a file or socket
  still being written keeps its alignment: once the source grows, the next call completes the
  packet the tail began. That report is a stall, not a loss: a tail that later completes was
  still counted as dropped, and one that grows and then ends is not counted again, so the
  byte balance over a growing source is approximate. Three things follow from Go's socket semantics: a read blocked on the socket is not
  interrupted by the context (cancellation is seen between reads — set a deadline or close the
  socket to wake the demuxer); a deadline error comes back wrapped and is not terminal, the next
  call continues where it left off; a datagram socket needs the read-ahead to hold a whole
  datagram (the default covers 1316/1472-byte and 8 KB datagrams, size `WithZeroCopyPackets` for
  larger ones), and RTP headers are the caller's to strip — a reassembler is the natural
  `tsio.Peeker`. The first packet waits longer: 409 bytes for packet-size autodetection (none
  with `WithPacketSize`), a full 1024-byte scan window under `WithSyncLock`.
- **Clock references as ticks**: `ts.ClockReference` is an `int64` count of 27 MHz ticks —
  PCR/OPCR/ESCR exactly, PTS/DTS as multiples of `ts.PTSTicks` — so intervals, offsets and
  jitter are plain subtractions; `Diff` takes the shortest signed distance across the 33-bit
  wrap (`ts.ClockWrap`); `Base()`/`Extension()` give the wire fields back, a negative value
  (a `Diff` result) folded onto the forward range so it serializes as the same instant. A
  non-conformant PCR/ESCR extension of 300 or more is folded into the tick count on parse:
  `Extension()` then reads below 300 and `Base()` one higher than the field on the wire, and
  the bytes as received stay in `Packet.Raw()`.
- **Multi-format packet reader**: plain TS (188), M2TS (192, with the 4-byte
  TP_extra_header exposed as `Packet.Prefix` / decoded by `ArrivalTimeStamp()`) and
  Reed-Solomon (204) are read transparently. The size is autodetected by locking onto the
  recurring sync byte — a stray `0x47` in payload or parity doesn't mislead it — or pinned
  with `WithPacketSize`.
- **Sync lock** (`demux.WithSyncLock`) — for UDP/RTP or otherwise torn feeds: aligns on the
  first position from which the packet grid can be captured — three consecutive periods
  carrying a sync byte, which keeps a 204-byte stream from masquerading as an aligned
  188-byte one; with `WithPacketSize` an input holding a single period locks on one — and
  survives damage with the TR 101 290 hysteresis — a lone corrupt sync byte is repaired and
  reported (sync_byte_error), two in a row are a sync loss re-locked only on five consecutive
  periods (TS_sync_loss; a tail shorter than that after a loss is consumed into it), an
  aligned corrupt packet is dropped — peeking ahead through a `tsio.Peeker` of at least 1024
  bytes (a raw reader is wrapped in bufio). The initial capture deliberately asks for less
  than the five-period hysteresis, so packets ahead of the first capturable position are
  reported as a sync loss; the damage paths look further ahead than one packet — a repair
  needs the next period in view, a re-lock a whole scan window. Off by default so aligned
  files stay on the zero-wrap fast path. Tolerance is explicit and strict
  by default: `WithSkipErrLimit` bounds the streak of consecutive damage events (dropped
  packets in either mode, sync losses under sync lock) and `WithResyncLimit` the scan windows
  a loss may take to re-lock — 0 tolerates nothing, -1 never gives up, N allows N — so a
  lossy feed sets both.
- **`ts.PacketSkipper`** — header-level filtering before any payload work.
- **`demux.WithKeepPIDs`** — inline PID allow-list (`ts.PIDSet`, a 13-bit bit set) checked in
  the parse hot path with a single bit test, cheaper than a `PacketSkipper` call. Filtered
  packets never reach PSI processing, so keep PID 0 (PAT) and the PMT PID(s) when program
  info is still needed. `SetKeepPIDs` swaps the list in for a later pass (e.g. after `Rewind`).
- **`Packet.Offset`** — a byte map of the stream, correct even with a skipper installed.
- **`demux.WithPacketHook`** — a callback run on every raw packet `Next` reads (after the
  skipper, before unit assembly), so one `Next` traversal can serve both packet-level work
  (indexing, PID/PCR sampling) and unit-level demuxing without a second pass; `NextPacketTo`
  hands the packet to the caller and runs no hook. The packet is
  valid only for the duration of the call.
- **PSI dedup**: byte-identical repeats of PAT/PMT/… are neither parsed nor emitted (unless
  `WithPSIRepeats` is set, and even then repeats reuse the cached parse — no re-parse).
- **Data ownership**: `AdaptationField`/`TransportPrivateData` inside a claimed `demux.PES`
  are owned copies, parsed PSI tables and descriptors own their payloads (guarded by
  dedicated ownership tests); retaining data on the consumer side is safe from pool reuse.
- **DVB text**: Annex A character tables end to end; control codes under the UTF-8 table are
  written in their Table A.2 form (`\n` → `EE 82 8A`), never as raw control bytes, and Latin
  diacritics compose through a precomputed table — one allocation per string.
- **Hardened parsers**: fuzz targets for every direct parser plus randomized byte-exact
  roundtrip properties; corrupt input never panics and yields errors matchable with
  `errors.Is` — `ts.ErrInvalidData` classifies any corrupt-input failure,
  `psi.ErrCRC32Mismatch` flags checksum errors.
- **Recoverable-error signalling** (`demux.WithRecoverableErrors`) — opt-in: instead of
  silently skipping a corrupt PSI section (CRC32 mismatch — TR 101 290 CRC_error), a torn
  table, a bad PES unit, a lost sync or a repaired sync byte, a dropped packet, a unit torn by a continuity
  gap / discontinuity / transport error / scrambling (the payload of the packet that tore it
  counts as dropped), a continuity gap that lost no bytes (`ErrorKindContinuity` — a PSI PID
  closes its unit in every packet, so a counter break between two whole tables drops nothing
  yet is a CC_error), or a unit that is neither PES nor PSI, `Next` surfaces it as
  `EventError` carrying a typed `*ts.RecoverableError` (kind, PID, byte offset, bytes
  dropped — 0 for a violation that lost nothing) and continues. The offset of a unit-level
  error is the last packet of that unit, not the packet the reader happened to be on when
  the unit was let go. `EventError` is yielded before the packet hook sees any packet past
  the damage. A unit whose start was lost — after a tear, or when the reader joins a PID
  mid-unit — is accumulated but never parsed: it is reported once as `ErrorKindTornUnit`
  with `ts.ErrHeadlessUnit` and its full byte count, so every accumulated byte of a damaged
  feed is charged either to the torn head or to the headless remainder, and
  `ErrorKindUnknownUnit` means what it says — a complete unit of an unrecognised type
  (SCTE-35, DSM-CC, AIT), not the debris of a lost packet. The error is
  non-terminal — `Events()` yields it without ending the stream, so a lossy feed keeps
  demuxing while the consumer counts damage (e.g. TR 101 290 error counters) and sums the
  loss. A unit the stream never closed (EOF) is still delivered, flagged `PES.Truncated`.
  Violations that lost nothing carry `Dropped` 0: a non-video PES with `PES_packet_length` 0
  (`pes.ErrUnboundedNonVideo`) is delivered and reported; a repeated packet whose bytes
  differ from the original (`ts.ErrDuplicateMismatch`) is dropped and reported, and a
  third repeat in a row counts as a continuity gap. A unit start whose bytes differ from the
  open unit's last packet is no repeat at all: it ends the open unit and starts the next one
  instead of being dropped; the tear — or the `Dropped` 0 continuity event, when the open
  unit was a whole PES — names `ts.ErrDuplicateMismatch`, the one counter break a
  consumer's own counter check cannot see. The third repeat and the unit size cap change
  what the silent mode delivers too: both drop the unit they hit.
  A PSI unit is parsed section by section: a damaged section is one event (with its CRC32
  checked before its body, so damage counts as CRC_error) and the sections around it are
  still delivered (`psi.Data.Errors` lists them for direct users of `psi.Parse`); a
  descriptor whose body does not parse under a valid CRC32 is kept verbatim as
  `descriptor.Malformed` rather than costing its section.
  Off by default: no events and no calls on the hot path. The damage handling is the same
  with or without the option; against earlier versions the output differs exactly where the
  handling changed — an `adaptation_field_control` '00' packet is discarded, a unit start
  with `discontinuity_indicator` flushes the previous unit instead of dropping it, a
  transport error or scrambling tears the unit in progress, and EOF tails are delivered as
  truncated instead of dropped.
- **`Demuxer.Close()`** — deterministic resource return for demuxers abandoned before EOF;
  `Rewind()` cleans up after itself and keeps its read-ahead buffer across passes.
- **Muxer**: raw packet passthrough (`WritePacket` of `Packet.Raw()` with `UpdateHeader`),
  `SetCC`, table retransmission from cache; PAT spans sections and packets when needed,
  oversize sections are rejected (`psi.ErrSectionOverflow`, against
  `TableID.MaxSectionLength()`: 1021 for PAT/CAT/PMT/TSDT and the DVB NIT/BAT/SDT, 4093 for
  EIT and private sections) instead of silently corrupted. The adaptation field is the
  muxer's to lay out: `WriteData` overwrites `StuffingLength`/`IsOneByteStuffing` before
  measuring the packet, so a field taken straight from `demux.PES` remuxes byte-exactly
  instead of counting its old stuffing twice; a field that leaves no room for payload is
  refused with `mux.ErrAdaptationFieldTooLong`, one longer than its length byte with
  `ts.ErrAdaptationFieldOverflow` — never a panic in the layout arithmetic. One-byte
  adaptation fields (`adaptation_field_length` 0) parse as `IsOneByteStuffing` and round-trip
  byte for byte. A packet with an adaptation field and no payload repeats the continuity
  counter (H.222.0 2.4.3.3); tables are packetized without a trailing all-stuffing packet;
  `WriteTables` resets the retransmission counter and leaves both table counters untouched
  when a write fails. `AddElementaryStream` refuses every PID below 0x0020 — H.222.0
  Table 2-3 reserves 0x0000–0x000F, EN 300 468 keeps 0x0010–0x001F for service information —
  plus the muxer's own PMT PID and the null PID, with `mux.ErrReservedPID`; auto-assigned PIDs
  start above the PMT PID. A structure that contradicts its own flags is refused instead of
  written: an adaptation field with `IsOneByteStuffing` beside live flags, private data or
  stuffing, or with `HasAdaptationExtensionField` and no extension, fails with
  `ts.ErrContradictoryAdaptationField`; a `Put` into a buffer too small for even the length
  byte returns `ts.ErrShortPacket`.
- **PES headers are strict both ways**: `pes.Header.IsVideoStream` follows Table 2-22
  (`stream_id & 0xF0 == 0xE0`), `pes.AllowsUnboundedLength` is the one predicate for who may
  carry `PES_packet_length` 0 (video and the extended id 0xFD) — the muxer's writer and the
  demuxer's `ErrUnboundedNonVideo` check share it — and `StreamType.ToPESStreamID` maps
  AC-3/E-AC-3 to 0xBD (`private_stream_1`) as DVB does. Writing fails instead of corrupting:
  `pes.ErrHeaderTooLong` when the optional header data exceeds 255 bytes or an extension_2
  field its 7-bit length, `pes.ErrMissingOptionalHeader` when the stream_id requires one,
  `pes.ErrMissingExtension` when `HasExtension` is set without an `Extension`,
  `pes.ErrUnboundedNonVideo` when a non-video unit would exceed 65535 bytes. Parsing rejects
  a payload without the `00 00 01` start code (`pes.ErrInvalidStartCode`) and reads optional
  fields only within `PES_header_data_length` — a header that claims fields it has no room
  for fails with `ts.ErrShortPacket` instead of inventing a PTS out of payload bytes.

## Migrating to v3

Everything that breaks against v2, in one place:

- **Module path** becomes `github.com/k-danil/go-astits/v3`.
- **`ts.Peeker` is `tsio.Peeker`** (no alias) and gains `Buffered() int` — a peeker without
  it no longer compiles; a live feed's window is cut to it. `tsio` also brings `BytesReader`,
  `SeekBuffer` and the optional `Tagger`.
- **`ts.ClockReference` is a tick count** (`int64`, 27 MHz) instead of a packed
  `base<<9|ext`. `NewClockReference(base, ext)`, `Base()` and `Extension()` keep their
  meaning; arithmetic on the value is now correct; `Time()` is gone; the JSON form is the
  tick count. A PCR or ESCR extension of 300 or more (non-conformant) is folded into the
  tick count rather than kept as a field — the packed form stored 9 bits verbatim.
- **`Demuxer.Section()` returns `(pid uint16, *psi.Section)`**; the typed body sits behind
  `Section.Syntax.Data`. `PacketCounts()` (packets per PID) replaces `GetStats()`.
- **Damage limits share one scale**: `WithSkipErrLimit`, `WithResyncLimit` and
  `WithMaxUnitSize(pes, psi)` take 0 (nothing), -1 (unbounded) or N. `WithMaxUnitSize`
  defaults to 16 MB / 64 KB.
- **Recoverable errors** carry `Kind`, `PID`, `Offset`, `Dropped` and `Err`; new kinds
  `ErrorKindSyncByte`, `ErrorKindPacketDrop`, `ErrorKindTornUnit`, `ErrorKindUnknownUnit`,
  `ErrorKindContinuity`.
  `psi.Data.Errors` lists unusable sections of a unit, `descriptor.Malformed` keeps a body the
  parser rejected. Silent-mode output changed where the handling did (see the
  recoverable-error bullet above).
- **`demux.PES`** gains `Truncated` and an embedded `demux.PacketSpan`
  (`FirstPacketOffset`/`LastPacketOffset`, `FirstPacketTag`/`LastPacketTag`) — read as
  `pes.FirstPacketOffset`, named as `PacketSpan` in a composite literal; `ts.Packet` gains `Tag`.
- **`Rewind` on a reader that cannot seek** returns -1 and continues from the current
  position instead of replaying a window.
- **`descriptor.DataStreamAligment*`** constants are spelled `DataStreamAlignment*`; the
  `descriptor`, `psi` and root package docs moved into this README.
- **`ts.PacketBuffer`** exposes `Window`/`Advance`/`Pos`/`Tag`/`Close` and
  `ts.ReadAhead`/`ReadAheadSize`; `Advance` takes a whole number of packets within the window
  and rejects anything else; `Packet.ParseAt` parses in place. Not needed by demuxer users.
- **v3.1** adds `demux.PacketSpan`/`SectionSpan`, `ts.ErrorKindContinuity`, `ts.ReadAheadSize`
  and `psi.TableID.MaxSectionLength`, and stops consuming a sub-packet tail at EOF. Source
  breaks: a `demux.PES` composite literal must name `PacketSpan`;
  `S2SatelliteDeliverySystem` loses `BackwardsCompatibilityIndicator` and gains
  `NotTimesliceFlag`/`TSGSMode`/`TimesliceNumber`; `ServiceTypeMPEG2HDDigitalTelevisionService`
  is `ServiceTypeHDDigitalTelevisionService` (JSON `HD_digital_television_service`,
  `teletext_service` in lower case); `CellList` latitudes and longitudes are `int16`;
  `pes.Header.IsVideoStream` covers 0xE0–0xEF and no longer 0xFD. Behaviour changes: AC-3
  and E-AC-3 units get `PES_packet_length` and stream_id 0xBD; a torn unit's headless
  remainder is one `ErrorKindTornUnit`/`ts.ErrHeadlessUnit` instead of an unknown-unit;
  descriptor bodies stop at their declared length (`*Malformed` where they used to read into
  a neighbour) and a tail past a fixed layout makes the body `*Malformed` too; zero-length
  bodies are typed; `terrestrial_delivery_system`'s JSON `Time_Slicing_indicator` is
  `time_slicing_indicator` and `C2_bundle_delivery_system`'s `MasterChannel` is
  `PrimaryChannel` (JSON `primary_channel`) per V1.19.1; `dvbtext.Encode` no longer decomposes U+212B into a
  base+mark pair (it goes out as UTF-8, and `Decode` now returns the same rune); invalid BCD
  fails instead of yielding a date; `WriteData` zeroes the adaptation field's stuffing
  before layout and rejects reserved PIDs and oversize fields; AC-3, DTS-HD and S2 reserved
  bits re-encode per spec. New errors: `ts.ErrAdaptationFieldOverflow`,
  `ts.ErrContradictoryAdaptationField`, `ts.ErrHeadlessUnit`, `pes.ErrHeaderTooLong`,
  `pes.ErrMissingOptionalHeader`, `pes.ErrMissingExtension`, `pes.ErrInvalidStartCode`,
  `mux.ErrAdaptationFieldTooLong`, `mux.ErrReservedPID`; all are of class `ts.ErrInvalidData`.

## Problems and deliberate trade-offs

- **Incompatible with upstream** in both API and semantics. Compatibility is a non-goal.
- **The demuxer is single-goroutine** by contract — no internal locking.
- **Close discipline**: a `demux.PES` claimed via `PES()` must be `Close()`d when you stop
  retaining it (an unclaimed unit is released by the next `Next`); a demuxer abandoned before
  EOF must be released via `Demuxer.Close()` — otherwise held resources go to the GC instead
  of the pools.
- **Context cancellation is polled, not immediate**: the packet reader checks `ctx` once per
  window, at most 1024 packets, so a cancel is observed within that many packets rather than
  at the next call boundary — the per-packet path stays free of a `select` — and never
  interrupts a `Read` blocked on the source (see the sockets bullet).
- **View mode**: packet memory is valid only until the next batch refill. The event API is
  unaffected (the accumulator copies out), but a `Packet` held from `NextPacketTo` is not.
- **PSI dedup changes emission semantics** by default: a repeated section with identical
  bytes is not delivered (only the first occurrence and any change are). Opt out with
  `WithPSIRepeats`.
- **`Next` results are borrowed**: `PES()` before `Close`, `Section()` and `SectionSpan()`
  are valid only until the next `Next`; retaining beyond that means claiming (`PES`) or
  copying.
- **`WriteData` owns the adaptation field's stuffing**: the `StuffingLength`/
  `IsOneByteStuffing` you pass are discarded, and after the call the field carries the
  stuffing that was actually written. Describe what the packet must carry (PCR, RAI, private
  data), not how it should be padded.
- **`Demuxer.Close` ends the demuxer's life**: reading after it is not supported. `PMT()` is
  the last PMT parsed for any program — on a multi-program stream tell programs apart with
  `Section()`.
- **Events of one packet are not ordered among themselves**: a unit completed by a packet is
  delivered before a recoverable error queued while that same packet was read. Ordering holds
  across packets, which is what damage counters need; to tie an error to a unit, use
  `Offset`, not arrival order.
- **`WithRecoverableErrors` changes the error contract**: without it, a non-nil error from
  `Next`/`Events` is terminal (as before). With it, a `*ts.RecoverableError` is non-terminal —
  distinguish with `ts.IsRecoverable(err)` and keep iterating; only a genuine fatal (or
  `ts.ErrNoMorePackets`) ends the stream.
- **`dvbtext.Text` marshals to JSON as the decoded string**, not as the wire bytes:
  unmarshalling re-encodes into the default table or UTF-8, so a JSON round-trip preserves the
  text, not the bytes — control codes other than the line break, and unassigned positions, are
  dropped along the way. `dvbtext.Code` likewise marshals as `"eng"` (or `"0x000000"` when the
  bytes are not printable) rather than as the byte array Go emits for `[3]byte`.
- Requires **Go ≥ 1.26**.

## Roadmap

- Packet-level primitives: in-place PCR patching over `Raw()`.
- Packet path over large in-memory input is DRAM-bound (8% gain from dropping the bufio
  copy against 34% on L2-resident data): software prefetch a few packets ahead.
- `SeekBuffer` as the demuxer's default wrapper for seekable readers, so `Rewind` after a
  prefix scan stays in memory; a larger default window (256 packets) for copy mode.
- Reserved-bit families as named constants across all descriptor writers; EIT events with
  a broken BCD marked individually instead of failing the section; the muxer's dead
  multi-section PAT path, TSID/PMT-PID setters and table CC seeding; a time-based table
  retransmission period.
