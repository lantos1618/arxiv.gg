package arxiv

import (
	"testing"
	"time"
)

func TestPaperDatesNeedReview(t *testing.T) {
	day := func(s string) time.Time {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	for _, tc := range []struct {
		id, created, updated string
		review               bool
	}{
		{"1706.03762", "2017-06-12", "2023-08-02", false},
		{"1706.03762", "2023-08-02", "2023-08-03", true},
		{"0704.0001", "2007-03-31", "2007-04-02", false},
		{"hep-th/9901001", "1999-01-04", "2001-01-01", false},
		{"1810.04805", "2018-10-11", "2018-10-10", true},
	} {
		p := Paper{ID: tc.id, Created: day(tc.created), Updated: day(tc.updated)}
		if got := p.DatesNeedReview(); got != tc.review {
			t.Errorf("%+v: review=%v", tc, got)
		}
	}
	if !(Paper{ID: "1706.03762"}).DatesNeedReview() {
		t.Fatal("missing dates must be surfaced")
	}
}

func TestHTMLMetadataDoesNotInventFirstSubmissionFromRevision(t *testing.T) {
	p, err := parseArxivHTMLMetadata("1706.03762", `<meta name="citation_title" content="Example"><meta name="citation_abstract" content="Abstract"><meta name="citation_online_date" content="2023/08/02">`)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Created.IsZero() || p.Updated.Format("2006-01-02") != "2023-08-02" {
		t.Fatalf("invented submission history: %+v", p)
	}
}
