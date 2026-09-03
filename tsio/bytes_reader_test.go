package tsio

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Past the end of input Peek and Discard hand back what is left with io.EOF,
// the bufio signalling the packet buffer's refill relies on.
func TestBytesReaderEnd(t *testing.T) {
	data := seq(10)
	for _, tc := range []struct {
		name    string
		skip, n int
		want    []byte
		err     error
	}{
		{"whole", 0, 10, data, nil},
		{"short", 0, 4, data[:4], nil},
		{"past end", 6, 8, data[6:], io.EOF},
		{"at end", 10, 1, []byte{}, io.EOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewBytesReader(data)
			n, err := r.Discard(tc.skip)
			require.NoError(t, err)
			require.Equal(t, tc.skip, n)

			bs, err := r.Peek(tc.n)
			assert.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.want, bs)
			assert.Equal(t, len(data)-tc.skip, r.Buffered())
		})
	}

	r := NewBytesReader(data)
	n, err := r.Discard(12)
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, 10, n)
	assert.Equal(t, 0, r.Buffered())
}

func ExampleNewBytesReader() {
	r := NewBytesReader([]byte("0123456789"))
	head, _ := r.Peek(4)
	_, _ = r.Discard(4)
	rest, err := r.Peek(10)
	fmt.Println(string(head), string(rest), err, r.Buffered())
	// Output: 0123 456789 EOF 6
}
