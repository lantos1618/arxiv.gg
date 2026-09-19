package arxiv

import (
	"os"
	"testing"
)

func TestReviewedPaperDateRepairPreservesNewerAndUnrelatedRecords(t *testing.T) {
	cache := quickSearchPostgresTestCache(t)
	if err := cache.db.Exec(`INSERT INTO papers(id,title,created,updated) VALUES
		('1706.03762','Attention','2023-08-02','2023-08-03'),
		('1312.6114','Already refreshed','2013-12-20','2026-01-01'),
		('1810.04805','BERT','2019-05-24','2019-05-28'),
		('2601.00001','Unrelated','2023-08-02','2023-08-03')`).Error; err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("deploy/sql/2026-09-19-reviewed-paper-dates.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := cache.db.Exec(string(script)).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ id, created, updated string }{
		{"1706.03762", "2017-06-12", "2023-08-02"},
		{"1312.6114", "2013-12-20", "2026-01-01"},
		{"1810.04805", "2018-10-11", "2019-05-24"},
		{"2601.00001", "2023-08-02", "2023-08-03"},
	} {
		var paper Paper
		if err := cache.db.Where("id = ?", tc.id).First(&paper).Error; err != nil {
			t.Fatal(err)
		}
		if paper.Created.Format("2006-01-02") != tc.created || paper.Updated.Format("2006-01-02") != tc.updated {
			t.Errorf("incorrect repair %+v: %+v", tc, paper)
		}
	}
}
