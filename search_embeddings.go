package arxiv

import (
	"context"
	"fmt"
	"strings"
)

type SemanticResult struct {
	PaperID    string  `json:"paperId"`
	Similarity float64 `json:"similarity"`
	Paper      *Paper  `json:"paper,omitempty"`
}

// SearchSemantic performs semantic search using pgvector's native similarity search.
// Uses cosine distance operator (<=>) for fast approximate nearest neighbor search.
func (c *Cache) SearchSemantic(ctx context.Context, queryEmbedding []float32, limit int) ([]SemanticResult, error) {
	if limit <= 0 {
		limit = 20
	}

	// Convert query embedding to pgvector format: [0.1,0.2,...]
	vecStr := float32SliceToVectorString(queryEmbedding)

	// Use pgvector's cosine distance operator (<=>)
	// 1 - distance = similarity
	query := `
		SELECT paper_id, 1 - (vector <=> $1::vector) as similarity
		FROM embeddings
		WHERE vector IS NOT NULL
		ORDER BY vector <=> $1::vector
		LIMIT $2
	`

	sqlDB, err := c.db.DB()
	if err != nil {
		return nil, err
	}

	rows, err := sqlDB.QueryContext(ctx, query, vecStr, limit)
	if err != nil {
		return nil, fmt.Errorf("semantic search query failed: %w", err)
	}
	defer rows.Close()

	var results []SemanticResult
	for rows.Next() {
		var r SemanticResult
		if err := rows.Scan(&r.PaperID, &r.Similarity); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return []SemanticResult{}, nil
	}

	// Fetch paper details
	papers, err := c.GetPapersByIDs(ctx, getPaperIDs(results))
	if err == nil {
		paperMap := make(map[string]*Paper)
		for _, paper := range papers {
			p := paper // avoid closure issue
			paperMap[paper.ID] = &p
		}

		for i := range results {
			if paper, exists := paperMap[results[i].PaperID]; exists {
				results[i].Paper = paper
			}
		}
	}

	return results, nil
}

// float32SliceToVectorString converts a float32 slice to pgvector format: [0.1,0.2,...]
func float32SliceToVectorString(v []float32) string {
	strs := make([]string, len(v))
	for i, f := range v {
		strs[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(strs, ",") + "]"
}

func getPaperIDs(results []SemanticResult) []string {
	ids := make([]string, len(results))
	for i, result := range results {
		ids[i] = result.PaperID
	}
	return ids
}

func (c *Cache) GetPapersByIDs(ctx context.Context, ids []string) ([]Paper, error) {
	var papers []Paper
	err := c.db.WithContext(ctx).Where("id IN ?", ids).Find(&papers).Error
	return papers, err
}
