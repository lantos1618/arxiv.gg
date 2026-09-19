package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	arxiv "github.com/lantos1618/arxiv.gg"
)

func TestCategoryTaxonomyPreservesSubjectsAndLegacyNames(t *testing.T) {
	for code, want := range map[string]categoryDefinition{
		"cs.CV":          {"Computer Vision and Pattern Recognition", "Computer Science", false},
		"math.AG":        {"Algebraic Geometry", "Mathematics", false},
		"physics.med-ph": {"Medical Physics", "Physics", false},
		"q-bio.NC":       {"Neurons and Cognition", "Quantitative Biology", false},
		"q-fin.RM":       {"Risk Management", "Quantitative Finance", false},
		"stat.ML":        {"Machine Learning", "Statistics", false},
		"eess.IV":        {"Image and Video Processing", "Electrical Engineering and Systems Science", false},
		"econ.EM":        {"Econometrics", "Economics", false},
		"q-alg":          {"Quantum Algebra and Topology", "Mathematics", true},
		"adap-org":       {"Adaptation, Noise, and Self-Organizing Systems", "Physics", true},
		"mtrl-th":        {"Materials Theory", "Physics", true},
	} {
		if got := categoryDefinitionFor(code); got != want {
			t.Errorf("%s: got %+v, want %+v", code, got, want)
		}
	}
	for _, code := range []string{"math.future", "unrecognized"} {
		if got := categoryDefinitionFor(code); got.Name != code || got.Discipline != "Other subjects" {
			t.Errorf("unknown category %q was assigned a guessed subject: %+v", code, got)
		}
	}
}

func TestCategoryGroupsKeepCountsAndLegacyCodesDistinct(t *testing.T) {
	categories := []arxiv.CategoryCount{
		{Name: "stat.ML", Count: 7}, {Name: "math.AG", Count: 11},
		{Name: "cs.LG", Count: 13}, {Name: "alg-geom", Count: 17},
		{Name: "math.future", Count: 19}, {Name: "cs.AI", Count: 23},
	}
	original := append([]arxiv.CategoryCount(nil), categories...)
	groups := groupCategories(categories)
	var names []string
	var items []categoryDirectoryItem
	for _, group := range groups {
		names = append(names, group.Name)
		items = append(items, group.Categories...)
	}
	if !reflect.DeepEqual(names, []string{"Computer Science", "Mathematics", "Statistics", "Other subjects"}) {
		t.Fatalf("discipline order: %v", names)
	}
	if !reflect.DeepEqual(categories, original) {
		t.Fatal("grouping changed the cached category input")
	}
	if len(items) != len(categories) {
		t.Fatalf("grouping lost or merged subjects: %v", items)
	}
	for _, input := range original {
		found := false
		for _, item := range items {
			if item.Code == input.Name {
				found = true
				if item.Count != int64(input.Count) {
					t.Errorf("%s count = %d, want %d", item.Code, item.Count, input.Count)
				}
			}
		}
		if !found {
			t.Errorf("subject %s disappeared", input.Name)
		}
	}
	if got := groups[1].Categories; got[0].Code != "alg-geom" || !got[0].Legacy || got[1].Code != "math.AG" || got[1].Legacy {
		t.Fatalf("legacy and current subjects must keep separate links and scope: %v", got)
	}
}

func TestCategoryDirectoryLinksGroupsAndFullNames(t *testing.T) {
	categories := []arxiv.CategoryCount{{Name: "cs.LG", Count: 1234}, {Name: "hep-ph", Count: 42}, {Name: "<unknown>", Count: 1}}
	var out bytes.Buffer
	if err := templates.ExecuteTemplate(&out, "categories", map[string]any{
		"Categories": categories, "CategoryGroups": groupCategories(categories),
	}); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{
		`aria-label="Subject disciplines"`, `href="#discipline-computer-science"`,
		`id="discipline-computer-science" tabindex="-1"`,
		`href="/category/cs.LG"><span>Machine Learning</span> <code>cs.LG</code>`,
		`High Energy Physics - Phenomenology`, `1,234 indexed papers`, `&lt;unknown&gt;`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("category directory missing %q", want)
		}
	}
	if strings.Contains(body, "<unknown>") {
		t.Fatal("unrecognized category was not escaped")
	}
}

func TestCategoryListShowsFetchDatesAndCoverage(t *testing.T) {
	fetched := time.Date(2026, 9, 19, 1, 30, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	zero := time.Time{}
	papers := []arxiv.Paper{
		{ID: "1706.03762", Title: "Fetched paper", Created: time.Date(2017, 6, 12, 0, 0, 0, 0, time.UTC), FetchedAt: &fetched},
		{ID: "1706.00001", Title: "Missing fetch date"},
		{ID: "1706.00002", Title: "Zero fetch date", FetchedAt: &zero},
	}
	var out bytes.Buffer
	if err := templates.ExecuteTemplate(&out, "category", map[string]any{
		"Category": "cs.LG", "DisplayName": "Machine Learning", "Papers": papers,
	}); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{
		"Showing 3 papers with downloaded source (up to 200)",
		"Directory totals include all indexed paper records",
		`datetime="2026-09-18T23:30:00Z"`, "2026-09-18 23:30 UTC",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("category list missing %q", want)
		}
	}
	if strings.Count(body, "Fetch date unavailable") != 2 {
		t.Fatal("missing and zero fetch timestamps must be visibly unknown")
	}
	if strings.Contains(body, "2017-06-12") || strings.Contains(body, "0001-01-01") {
		t.Fatal("category list presented a submission date or zero timestamp as its fetch date")
	}
}
