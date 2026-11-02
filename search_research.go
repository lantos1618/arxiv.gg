package arxiv

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ResearchFilter is applied before ranking and LIMIT, including vector searches.
type ResearchFilter struct {
	Category      string
	Since, Until  time.Time
	MinSimilarity float64
}

// Publication dates from imported feeds can describe a revision. arXiv IDs
// reliably preserve the original submission month, including legacy IDs.
const researchPublicationMonthSQL = `(CASE
 WHEN p.id ~ '^[0-9]{2}(0[1-9]|1[0-2])[.][0-9]{4,5}$' THEN '20'||substr(p.id,1,2)||'-'||substr(p.id,3,2)
 WHEN p.id ~ '^[a-z.-]+/[0-9]{2}(0[1-9]|1[0-2])[0-9]{3}$' THEN
 (CASE WHEN substr(split_part(p.id,'/',2),1,2)>='91' THEN '19' ELSE '20' END)||substr(split_part(p.id,'/',2),1,2)||'-'||substr(split_part(p.id,'/',2),3,2)
 ELSE to_char(p.created AT TIME ZONE 'UTC','YYYY-MM') END)`

var researchModernID = regexp.MustCompile(`^[0-9]{2}(0[1-9]|1[0-2])[.][0-9]{4,5}$`)
var researchLegacyID = regexp.MustCompile(`^[a-z.-]+/[0-9]{2}(0[1-9]|1[0-2])[0-9]{3}$`)

// PaperSubmissionMonth returns original arXiv publication precision without
// inventing a day. Unrecognized identifiers fall back to the stored date.
func PaperSubmissionMonth(p *Paper) string {
	id := p.ID
	century := "20"
	if researchLegacyID.MatchString(id) {
		id = strings.SplitN(id, "/", 2)[1]
		year, _ := strconv.Atoi(id[:2])
		if year >= 91 {
			century = "19"
		}
	} else if !researchModernID.MatchString(id) {
		return p.Created.UTC().Format("2006-01")
	}
	return century + id[:2] + "-" + id[2:4]
}

func researchWhere(f ResearchFilter) (string, []any) {
	where := ""
	args := []any{}
	if f.Category != "" {
		where += " AND p.categories LIKE ? AND ? = ANY(string_to_array(p.categories, ' '))"
		args = append(args, "%"+f.Category+"%", f.Category)
	}
	if !f.Since.IsZero() {
		where += " AND " + researchPublicationMonthSQL + " >= ?"
		args = append(args, f.Since.UTC().Format("2006-01"))
	}
	if !f.Until.IsZero() {
		where += " AND " + researchPublicationMonthSQL + " < ?"
		args = append(args, f.Until.UTC().Format("2006-01"))
	}
	return where, args
}

// ResearchKeywords avoids the exact-count work used by the autocomplete API.
// Broader term matching is intentional for research questions; exact names rank first.
func (c *Cache) ResearchKeywords(ctx context.Context, query string, f ResearchFilter, limit int) ([]Paper, error) {
	if c.dbType != DBTypePostgres {
		return nil, c.capabilityError("research search")
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("invalid research limit")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	where, args := researchWhere(f)
	terms := strings.Fields(query)
	if len(terms) > 32 {
		terms = terms[:32]
	}
	textQuery := strings.Join(terms, " OR ")
	sql := `SELECT p.id, p.title, p.abstract, p.authors, p.categories, p.created, p.updated, p.comments, p.doi
 FROM papers p WHERE p.search_vector @@ websearch_to_tsquery('english', ?)` + where + `
 ORDER BY CASE WHEN p.id = ? OR lower(trim(p.title)) = lower(trim(?)) THEN 0
 WHEN p.title ILIKE ? ESCAPE '\' THEN 1 ELSE 2 END,
 ts_rank_cd(p.search_vector, websearch_to_tsquery('english', ?)) DESC, p.created DESC, p.id LIMIT ?`
	pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(query) + "%"
	all := append([]any{textQuery}, args...)
	all = append(all, query, query, pattern, textQuery, limit)
	papers := []Paper{}
	if ValidArxivID(query) {
		sql = `SELECT p.id,p.title,p.abstract,p.authors,p.categories,p.created,p.updated,p.comments,p.doi FROM papers p WHERE p.id = ?` + where
		all = append([]any{query}, args...)
	}
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL statement_timeout = '8000ms'").Error; err != nil {
			return err
		}
		if err := tx.Exec("SET LOCAL jit = off").Error; err != nil {
			return err
		}
		return tx.Raw(sql, all...).Scan(&papers).Error
	})
	return papers, err
}

// ResearchVectors returns abstract or full-text matches using the same filters.
func (c *Cache) ResearchVectors(ctx context.Context, vector []float32, f ResearchFilter, limit int, deep bool) ([]DeepSearchResult, error) {
	if c.dbType != DBTypePostgres {
		return nil, c.capabilityError("research vector search")
	}
	if len(vector) != QwenEmbeddingDim || limit < 1 || limit > 100 {
		return nil, fmt.Errorf("invalid research vector or limit")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	where, args := researchWhere(f)
	selectFrom := `SELECT p.id AS paper_id, 1-(e.vector <=> ?::vector) AS similarity, '' AS chunk_id, '' AS section, '' AS snippet
 FROM embeddings_v2 e JOIN papers p ON p.id=e.paper_id
 WHERE e.scope='abstract' AND e.model=? AND e.dim=? AND e.vector IS NOT NULL`
	if deep {
		selectFrom = `SELECT p.id AS paper_id, 1-(e.vector <=> ?::vector) AS similarity, c.id AS chunk_id, COALESCE(c.section,'') AS section,
 left(regexp_replace(c.text, '[[:space:]]+', ' ', 'g'), 700) AS snippet
 FROM chunk_embeddings_v2 e JOIN paper_chunks c ON c.id=e.chunk_id JOIN papers p ON p.id=c.paper_id
 WHERE c.scope='pdf_text' AND e.source_hash=c.text_hash AND COALESCE(c.text,'')<>'' AND e.model=? AND e.dim=? AND e.vector IS NOT NULL`
	}
	vec := float32SliceToVectorString(vector)
	var query string
	all := append([]any{vec, qwenEmbeddingModel, qwenEmbeddingDim}, args...)
	candidates := limit
	if deep {
		candidates = min(limit*8, 800)
	}

	if f.Category != "" {
		// A trigram index narrows categories before exact distance ranking. Sparse
		// categories otherwise force ANN to traverse thousands of unrelated papers.
		query = `WITH filtered AS MATERIALIZED (` + selectFrom + where + `)
 SELECT * FROM filtered WHERE similarity >= ? ORDER BY similarity DESC, paper_id, chunk_id LIMIT ?`
		all = append(all, f.MinSimilarity, candidates)
	} else {
		// Apply the threshold outside the ordered nearest-neighbor scan. Filtering
		// inside makes an iterative index keep scanning when fewer than LIMIT match.
		query = `WITH nearest AS MATERIALIZED (` + selectFrom + where + ` ORDER BY e.vector <=> ?::vector LIMIT ?)
 SELECT * FROM nearest WHERE similarity >= ? ORDER BY similarity DESC, paper_id, chunk_id`
		all = append(all, vec, candidates, f.MinSimilarity)
	}
	rows := []DeepSearchResult{}
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL statement_timeout = '10000ms'").Error; err != nil {
			return err
		}
		if err := tx.Exec("SET LOCAL jit = off").Error; err != nil {
			return err
		}
		if err := tx.Exec("SET LOCAL hnsw.ef_search = 400").Error; err != nil {
			return err
		}
		if err := tx.Exec("SET LOCAL hnsw.iterative_scan = strict_order").Error; err != nil {
			return err
		}
		return tx.Raw(query, all...).Scan(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	results := []DeepSearchResult{}
	ids := []string{}
	seen := map[string]bool{}
	for _, row := range rows {
		if !seen[row.PaperID] {
			seen[row.PaperID] = true
			results = append(results, row)
			ids = append(ids, row.PaperID)
			if len(results) == limit {
				break
			}
		}
	}
	if len(ids) == 0 {
		return results, nil
	}
	var papers []Paper
	if err := c.db.WithContext(ctx).Select("id,title,abstract,authors,categories,created,updated,comments,doi").Where("id IN ?", ids).Find(&papers).Error; err != nil {
		return nil, err
	}
	byID := map[string]*Paper{}
	for i := range papers {
		byID[papers[i].ID] = &papers[i]
	}
	for i := range results {
		results[i].Paper = byID[results[i].PaperID]
	}
	return results, nil
}

// QwenQueryStatusByHash exposes readiness without returning another client's query text.
func (c *Cache) QwenQueryStatusByHash(ctx context.Context, hash string) (*QwenQueryEmbeddingStatus, error) {
	var row struct{ QueryText string }
	if err := c.db.WithContext(ctx).Table("qwen_query_embeddings").Select("query_text").Where("query_hash=? AND model=? AND dim=?", hash, qwenEmbeddingModel, qwenEmbeddingDim).Take(&row).Error; err != nil {
		return nil, err
	}
	return c.QwenQueryEmbeddingStatus(ctx, row.QueryText)
}
