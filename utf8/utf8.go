package utf8

import _ "github.com/kamalshkeir/kasm/cpu"

// Valid reports whether p consists entirely of valid UTF-8-encoded runes.
func Valid(p []byte) bool {
	return Validate(p).IsUTF8()
}
