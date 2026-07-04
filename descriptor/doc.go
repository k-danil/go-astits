// Package descriptor parses and serializes the DVB and MPEG descriptors carried
// in PSI tables, one file per descriptor type. [Parse] reads the next
// descriptor from a byte slice; every descriptor implements [Descriptor] with
// CalcLength and Append for serialization.
package descriptor
