package version

import (
	_ "embed"
	"strings"
)

//go:embed version.txt
var raw string

// Get returns the version string, trimming whitespace.
// Falls back to "dev" if the embedded file is empty.
func Get() string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "dev"
	}
	return v
}
