package dvbtext

import "testing"

func BenchmarkDecodeLatin(b *testing.B) {
	for _, bm := range []struct {
		name string
		text Text
	}{
		{"no mark", Text("Channel 1 news")},
		{"one mark", Text{'C', 'h', 'a', 'n', 'n', 'e', 'l', ' ', 0xc2, 'e', 'w', 's'}},
		{"five marks", Text{0xc2, 'e', 0xc1, 'a', 0xc3, 'o', 0xc4, 'n', 0xc8, 'u', 'x', 'y'}},
	} {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := bm.text.Decode(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
