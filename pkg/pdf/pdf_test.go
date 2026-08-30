package pdf

import (
	"strings"
	"testing"
)

func TestExtractSection(t *testing.T) {
	sampleDoc := `
1 Introduction
Here is some intro text.

6 Conclusion
In this work, we presented the Transformer architecture.
It replaces recurrence with self-attention.

References
[1] Vaswani et al.
`

	section := ExtractSection(sampleDoc, []string{"conclusion", "conclusions"}, 1000)
	if !strings.Contains(section, "we presented the Transformer architecture") {
		t.Errorf("ExtractSection() failed to extract conclusion. Got:\n%s", section)
	}
	if strings.Contains(section, "References") || strings.Contains(section, "Vaswani et al.") {
		t.Errorf("ExtractSection() failed to cut off before References. Got:\n%s", section)
	}
}
