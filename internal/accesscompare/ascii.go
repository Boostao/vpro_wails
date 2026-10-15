package accesscompare

import "strings"

// PrintableASCIIKey implements only the calibrated ACE1033 ASCII equality.
func PrintableASCIIKey(text string) (string, bool) {
	for i := range len(text) {
		if text[i] < 32 || text[i] > 126 {
			return "", false
		}
	}
	return strings.ToLower(strings.TrimRight(text, " ")), true
}
