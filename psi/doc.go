// Package psi parses and serializes PSI tables: PAT, PMT, EIT, NIT, SDT and
// TOT. [Parse] reads a [Data] from a section payload; [Data.Append] serializes
// it and appends the CRC32. Corrupt input is rejected with errors matchable via
// errors.Is against [ts.ErrInvalidData] (for example [ErrCRC32Mismatch]).
package psi
