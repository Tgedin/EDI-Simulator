package llm

// ExtractJSON finds the first complete {...} JSON object in s.
// If the model wraps its answer in prose or non-ASCII characters, this still
// extracts the payload. Returns s unchanged if no opening brace is found.
func ExtractJSON(s string) string {
	start := -1
	for i, c := range s {
		if c == '{' {
			start = i
			break
		}
	}
	if start == -1 {
		return s
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:] // unclosed brace — return what we have
}
