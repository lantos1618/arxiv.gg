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
