package configs

import "strings"

// parseEpics splits a comma- or newline-separated epics input into trimmed, de-duplicated names,
// in the order given.
func parseEpics(input string) []string {
	var epics []string
	seen := map[string]bool{}
	for _, e := range strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == '\n' }) {
		e = strings.TrimSpace(e)
		if e == "" || seen[e] {
			continue
		}
		seen[e] = true
		epics = append(epics, e)
	}
	return epics
}
