// Package demux turns an MPEG-TS byte stream into events. [New] builds a
// [Demuxer]; [Demuxer.Next] — or the [Demuxer.Events] iterator — advances to
// the next [EventPES] or typed table event (EventPAT, EventPMT, …). Claim a
// completed unit with [Demuxer.PES], and read table state with
// [Demuxer.Section], [Demuxer.PAT] and [Demuxer.PMT].
//
// Results are borrowed until the next Next call: a claimed [PES] must be
// [PES.Close]d, an abandoned demuxer released with [Demuxer.Close], and
// anything kept from Section/PAT/PMT copied out. DVB tables are parsed only
// with [WithDVBTables]; [WithZeroCopyPackets] enables the view read mode. See
// the module documentation for the full ownership and view-mode contracts.
package demux
