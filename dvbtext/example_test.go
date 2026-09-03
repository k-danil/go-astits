package dvbtext_test

import (
	"encoding/json"
	"fmt"

	"github.com/k-danil/go-astits/v3/dvbtext"
)

// A service name as it arrives in an SDT: the leading 0x01 selects
// ISO/IEC 8859-5, the rest is the name.
func ExampleText_String() {
	name := dvbtext.Text{0x01, 0xbf, 0xe0, 0xd8, 0xd2, 0xd5, 0xe2}
	js, err := json.Marshal(name)
	if err != nil {
		panic(err)
	}

	fmt.Println(name.String())
	fmt.Println(string(js))
	fmt.Printf("%x\n", []byte(dvbtext.Encode("Привет")))
	// Output:
	// Привет
	// "Привет"
	// 15d09fd180d0b8d0b2d0b5d182
}
