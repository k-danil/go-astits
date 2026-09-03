// Package demux turns an MPEG-TS byte stream into events. Results are borrowed until the next Next call: a claimed [PES] must be [PES.Close]d, an abandoned demuxer released with [Demuxer.Close], and anything kept from Section/PAT/PMT copied out.
package demux
