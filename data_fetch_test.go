package arxiv

import "testing"

func TestParseArxivHTMLMetadata(t *testing.T) {
	body := `<!doctype html>
<html>
<head>
<meta name="citation_title" content="Sycophancy is an Educational Safety Risk: Why LLM Tutors Need Sycophancy Benchmarks" />
<meta name="citation_author" content="Kasneci, Enkelejda" />
<meta name="citation_author" content="Kasneci, Gjergji" />
<meta name="citation_date" content="2026/05/14" />
<meta name="citation_online_date" content="2026/05/14" />
<meta name="citation_arxiv_id" content="2605.14604" />
<meta name="citation_abstract" content="This position paper argues that effective tutoring requires corrective friction. It treats kind-but-correct behavior as a safety requirement." />
</head>
<body>
<td class="tablecell subjects">
<span class="primary-subject">Artificial Intelligence (cs.AI)</span>; Human-Computer Interaction (cs.HC)
</td>
</body>
</html>`

	paper, err := parseArxivHTMLMetadata("2605.14604", body)
	if err != nil {
		t.Fatalf("parseArxivHTMLMetadata: %v", err)
	}
	if paper.ID != "2605.14604" {
		t.Fatalf("ID = %q", paper.ID)
	}
	if paper.Title != "Sycophancy is an Educational Safety Risk: Why LLM Tutors Need Sycophancy Benchmarks" {
		t.Fatalf("Title = %q", paper.Title)
	}
	if paper.Authors != "Enkelejda Kasneci, Gjergji Kasneci" {
		t.Fatalf("Authors = %q", paper.Authors)
	}
	if paper.Categories != "cs.AI cs.HC" {
		t.Fatalf("Categories = %q", paper.Categories)
	}
	if paper.Created.Format("2006-01-02") != "2026-05-14" {
		t.Fatalf("Created = %s", paper.Created.Format("2006-01-02"))
	}
	if paper.Abstract == "" {
		t.Fatal("Abstract is empty")
	}
}

func TestParseArxivHTMLMetadataMissingRequiredFields(t *testing.T) {
	_, err := parseArxivHTMLMetadata("2605.14604", `<html><head></head><body></body></html>`)
	if err == nil {
		t.Fatal("expected error")
	}
}
