package catalog

import (
	"strconv"
	"strings"
)

// ParseClaudeVersion extracts the leading dotted version from `claude
// --version` output (e.g. "2.1.200 (Claude Code)" -> "2.1.200").
func ParseClaudeVersion(output string) string {
	fields := strings.Fields(strings.TrimSpace(output))
	if len(fields) == 0 {
		return ""
	}
	v := fields[0]
	for _, part := range strings.Split(v, ".") {
		if _, err := strconv.Atoi(part); err != nil {
			return ""
		}
	}
	return v
}

// compareVersions compares dotted integer versions: -1, 0, or 1.
// Missing segments count as zero (2.1 == 2.1.0).
func compareVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		ai, bi := 0, 0
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		switch {
		case ai < bi:
			return -1
		case ai > bi:
			return 1
		}
	}
	return 0
}
