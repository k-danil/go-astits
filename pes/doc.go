// Package pes parses and serializes PES packets. [Data.Parse] reads a packet
// from a byte slice, exposing the header (stream id, PTS/DTS, flags) and the
// payload; [Header.Put] serializes the header and a chunk of payload into a
// caller buffer, and [Header.CalcDataLength] sizes that write ahead of time so
// a muxer can lay out adaptation-field stuffing.
package pes
