package llm

import "testing"

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "clean JSON",
			input: `{"category":"malformed_content","confidence":"high","explanation":"null byte detected"}`,
			want:  `{"category":"malformed_content","confidence":"high","explanation":"null byte detected"}`,
		},
		{
			name:  "JSON wrapped in prose",
			input: `Sure! Here is the answer: {"category":"duplicate","confidence":"medium","explanation":"same PO number"} Hope that helps.`,
			want:  `{"category":"duplicate","confidence":"medium","explanation":"same PO number"}`,
		},
		{
			name:  "JSON with trailing non-ASCII characters",
			input: "```json\n{\"category\":\"schema_mismatch\",\"confidence\":\"low\",\"explanation\":\"invalid qty\"}\n``` 好的",
			want:  `{"category":"schema_mismatch","confidence":"low","explanation":"invalid qty"}`,
		},
		{
			name:  "nested braces",
			input: `{"outer":{"inner":"value"},"key":"val"}`,
			want:  `{"outer":{"inner":"value"},"key":"val"}`,
		},
		{
			name:  "no JSON in string",
			input: "This is just plain text with no braces.",
			want:  "This is just plain text with no braces.",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "unclosed brace returns partial",
			input: "prefix {partial content without closing",
			want:  "{partial content without closing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractJSON(tt.input)
			if got != tt.want {
				t.Errorf("ExtractJSON(%q)\n got  %q\n want %q", tt.input, got, tt.want)
			}
		})
	}
}
