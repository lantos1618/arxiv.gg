package arxiv

import "testing"

func TestValidArxivIDLegacyCategories(t *testing.T) {
	for _, id := range []string{"math.GT/0309136", "math.GT/0309136v1", "cs.AI/9901001", "hep-th/9901001v2", "1706.03762V7"} {
		if !ValidArxivID(id) {
			t.Errorf("rejected supported ID %q", id)
		}
	}
	for _, id := range []string{"evil.GT/0309136", "math../0309136", "math.GT/0309136?x=1", "math.GT/0309136/extra", "math.GT/0309136v"} {
		if ValidArxivID(id) {
			t.Errorf("accepted malformed ID %q", id)
		}
	}
}

func TestCanonicalArxivID(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"1706.03762V7", "1706.03762"},
		{" math.GT/0309136v2 ", "math/0309136"},
		{"MATH.GT/0309136", "math/0309136"},
		{"q-bio.CB/0401004v1", "q-bio/0401004"},
		{"hep-th/9901001v2", "hep-th/9901001"},
		{"solv-int/9901001", "solv-int/9901001"},
		{"evil.GT/0309136", ""},
		{"1706.03762?x=1", ""},
	} {
		if got := CanonicalArxivID(tc.input); got != tc.want {
			t.Errorf("CanonicalArxivID(%q)=%q, want %q", tc.input, got, tc.want)
		}
	}
}
