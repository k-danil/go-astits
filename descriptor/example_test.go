package descriptor_test

import (
	"fmt"

	"github.com/k-danil/go-astits/v3/descriptor"
)

// A descriptor loop round-trips through AppendWithLength and Parse; the
// concrete type comes back behind the Descriptor interface, and a body the
// parser rejects comes back as a Malformed carrying the raw bytes.
func ExampleParse() {
	loop := descriptor.AppendWithLength(nil, []descriptor.Descriptor{
		&descriptor.StreamIdentifier{Header: descriptor.Header{Tag: descriptor.TagStreamIdentifier}, ComponentTag: 7},
	})

	ds, n, err := descriptor.Parse(loop)
	if err != nil {
		panic(err)
	}
	for _, d := range ds {
		switch d := d.(type) {
		case *descriptor.StreamIdentifier:
			fmt.Println(d.Tag(), "component", d.ComponentTag)
		case *descriptor.Malformed:
			fmt.Println(d.Tag(), "rejected:", d.Err, len(d.Raw), "raw bytes")
		}
	}
	fmt.Println(n, "bytes consumed")
	// Output:
	// stream_identifier_descriptor component 7
	// 5 bytes consumed
}
