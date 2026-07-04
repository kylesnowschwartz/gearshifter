package catalog

import (
	"bufio"
	"io"
	"strings"
)

// parseFrontmatter reads the leading YAML frontmatter block (--- ... ---)
// and returns its top-level scalar fields, lowercased keys. It is a minimal
// parser on purpose: skill/command frontmatter in the wild is flat
// `key: value` scalars, and a YAML dependency isn't warranted (CQS-006).
// Nested structures and list items are skipped.
func parseFrontmatter(r io.Reader) map[string]string {
	fields := map[string]string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return fields
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(strings.TrimSpace(line), "-") {
			continue // nested value or list item: not ours to parse
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key != "" && value != "" {
			fields[key] = value
		}
	}
	return fields
}
