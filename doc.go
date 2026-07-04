// Package astits is the documentation root of an opinionated, performance-
// focused fork of asticode/go-astits for demuxing and remuxing MPEG-TS.
//
// This package holds no code — import the sub-packages instead:
//
//	ts          packets, headers, adaptation fields, clocks, CRC32, the packet reader
//	pes         PES packets
//	psi         PSI tables (PAT/PMT/EIT/NIT/SDT/TOT)
//	descriptor  DVB/MPEG descriptors
//	demux       the event-based demuxer
//	mux         the muxer
//
// The API and semantics have diverged from upstream on purpose; this module is
// not a drop-in replacement. It has no dependencies outside the standard
// library and uses no unsafe.
//
// # Contracts and safety
//
// The sharp edges of this library are lifetime and ownership, not mechanics.
// Read this before writing a demux or mux loop.
//
// Ownership of demuxer results. demux.Demuxer.Next advances to the next event;
// the PES unit behind an EventPES and the section behind a table event belong
// to the demuxer and are valid only until the following Next call. To use a
// unit past that point, claim it with Demuxer.PES and release it with
// PES.Close; anything read from Section, PAT or PMT that you keep must be
// copied out.
//
// Close discipline. A claimed PES must be Closed once you stop retaining it (an
// unclaimed unit is released by the next Next). A demuxer or muxer abandoned
// before the end of the stream must be released with Close; otherwise its
// pooled buffers go to the garbage collector instead of back to the pool.
//
// Zero-copy view mode. With demux.WithZeroCopyPackets a packet's bytes — Raw,
// Payload and an adaptation field's TransportPrivateData — are views into a
// shared batch buffer, valid only until the next read refills it. Copy anything
// you keep beyond the next read. The event API is unaffected: the accumulator
// copies payloads out before the refill, so PES units are always owned.
//
// Serialization panics. Fixed-size writers (Put) panic on a destination buffer
// that is too short, mirroring encoding/binary.BigEndian; size the buffer with
// CalcLength first. Variable-size writers (Append) grow the destination
// instead and never panic.
//
// Corrupt input. Parsers reject malformed bytes with errors matchable through
// errors.Is against ts.ErrInvalidData; they never panic on hostile input
// (guarded by fuzzing).
//
// Concurrency. A demuxer and a muxer are single-goroutine by contract — they
// hold no internal locks.
package astits
