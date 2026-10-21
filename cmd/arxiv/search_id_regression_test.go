package main

import "testing"

func TestExtractArxivIDCanonicalInputs(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"1706.03762", "1706.03762"},
		{" 1706.03762v7 ", "1706.03762"},
		{"1706.03762V7", "1706.03762"},
		{"hep-th/9901001v2", "hep-th/9901001"},
		{"math.GT/0309136v1", "math.GT/0309136"},
		{"https://arxiv.org/abs/1706.03762v7?context=cs#paper", "1706.03762"},
		{"https://arxiv.org/pdf/math.GT/0309136v1.pdf?download=1#paper", "math.GT/0309136"},
		{"export.arxiv.org/pdf/1706.03762v7.pdf", "1706.03762"},
		{"http://www.arxiv.org/abs/1706.03762", "1706.03762"},
		{"https://example.com/abs/1706.03762", ""},
		{"https://arxiv.org.evil.test/abs/1706.03762", ""},
		{"https://user@arxiv.org/abs/1706.03762", ""},
		{"https://arxiv.org:123/abs/1706.03762", ""},
		{"javascript:arxiv.org/abs/1706.03762", ""},
		{"https://arxiv.org/abs/1706.03762/extra", ""},
		{"1706.03762&max_results=1000", ""},
		{"evil/1234567", ""},
	} {
		if got := extractArxivID(tc.input); got != tc.want {
			t.Errorf("extractArxivID(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
