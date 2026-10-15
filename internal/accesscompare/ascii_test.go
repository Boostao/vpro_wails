package accesscompare

import "testing"

func TestPrintableASCIIKey(t *testing.T) {
	for _, item := range []struct{ text, key string }{
		{"", ""}, {" ", ""}, {"A ", "a"}, {"abc", "abc"}, {" AbC  ", " abc"},
		{"=1+1", "=1+1"}, {"[A]?", "[a]?"}, {"001", "001"}, {"~", "~"},
	} {
		actual, ok := PrintableASCIIKey(item.text)
		if !ok || actual != item.key {
			t.Fatalf("%q: key=%q ok=%v, want %q", item.text, actual, ok, item.key)
		}
	}
	for value := range 256 {
		text := string([]byte{byte(value)})
		key, ok := PrintableASCIIKey(text)
		if ok != (value >= 32 && value <= 126) {
			t.Fatalf("byte %d: unexpected availability %v", value, ok)
		}
		if !ok && key != "" {
			t.Fatalf("byte %d: rejected text returned a key", value)
		}
	}
	for _, text := range []string{"a\t", "a\n", "a\x00", "a\u00a0", "\u00c9", "\u00df", "\ufffd", "\xff"} {
		if key, ok := PrintableASCIIKey(text); ok || key != "" {
			t.Fatalf("%q: unsupported text acquired key %q", text, key)
		}
	}
}
