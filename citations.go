package arxiv

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UpdateCitations extracts references from a paper's source and stores citation edges.
// This should be called after downloading source files.
func (c *Cache) UpdateCitations(ctx context.Context, paperID, srcPath string) error {
	if srcPath == "" {
		return nil
	}

	refs, err := extractReferences(srcPath)
	if err != nil {
		return fmt.Errorf("extract references: %w", err)
	}
	err = c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("from_id = ?", paperID).Delete(&Citation{}).Error; err != nil {
			return fmt.Errorf("delete citations: %w", err)
		}
		citations := make([]Citation, 0, len(refs))
		for _, refID := range refs {
			if refID != paperID {
				citations = append(citations, Citation{FromID: paperID, ToID: refID})
			}
		}
		if len(citations) == 0 {
			return nil
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&citations).Error; err != nil {
			return fmt.Errorf("insert citations: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	c.detailLRU.Clear()
	return nil
}

// CitedByCount returns the number of cached papers that cite this paper.
func (c *Cache) CitedByCount(ctx context.Context, paperID string) (int, error) {
	cacheKey := detailKey("cited_by_count", paperID)
	if cached, ok := c.getDetailCache(cacheKey); ok {
		if count, ok := cached.(int); ok {
			return count, nil
		}
	}

	value, err, _ := c.detailFlights.Do(cacheKey, func() (interface{}, error) {
		if cached, ok := c.getDetailCache(cacheKey); ok {
			if count, ok := cached.(int); ok {
				return count, nil
			}
		}
		count, err := c.citedByCountUncached(ctx, paperID)
		if err != nil {
			return 0, err
		}
		c.putDetailCache(cacheKey, citationTTL, count)
		return count, nil
	})
	if err != nil {
		return 0, err
	}
	count, _ := value.(int)
	return count, nil
}

func (c *Cache) citedByCountUncached(ctx context.Context, paperID string) (int, error) {
	var count int64
	err := c.withCitationQuery(ctx, func() error {
		return c.db.WithContext(ctx).Model(&Citation{}).Where("to_id = ?", paperID).Count(&count).Error
	})
	return int(count), err
}

// CitedBy returns papers that cite this paper (only cached papers with metadata).
type CitingPaper struct {
	ID    string
	Title string
}

func (c *Cache) CitedBy(ctx context.Context, paperID string, limit int) ([]CitingPaper, error) {
	if limit <= 0 {
		limit = 50
	}
	cacheKey := detailKey("cited_by", paperID, fmt.Sprint(limit))
	if cached, ok := c.getDetailCache(cacheKey); ok {
		if papers, ok := cached.([]CitingPaper); ok {
			return cloneCitingPapers(papers), nil
		}
	}

	value, err, _ := c.detailFlights.Do(cacheKey, func() (interface{}, error) {
		if cached, ok := c.getDetailCache(cacheKey); ok {
			if papers, ok := cached.([]CitingPaper); ok {
				return cloneCitingPapers(papers), nil
			}
		}
		papers, err := c.citedByUncached(ctx, paperID, limit)
		if err != nil {
			return nil, err
		}
		c.putDetailCache(cacheKey, citationTTL, cloneCitingPapers(papers))
		return papers, nil
	})
	if err != nil {
		return nil, err
	}
	papers, _ := value.([]CitingPaper)
	return cloneCitingPapers(papers), nil
}

func (c *Cache) citedByUncached(ctx context.Context, paperID string, limit int) ([]CitingPaper, error) {
	var papers []CitingPaper
	err := c.withCitationQuery(ctx, func() error {
		return c.db.WithContext(ctx).
			Table("citations").
			Select("papers.id, papers.title").
			Joins("JOIN papers ON citations.from_id = papers.id").
			Where("citations.to_id = ?", paperID).
			Order("papers.created DESC").
			Limit(limit).
			Scan(&papers).Error
	})
	return papers, err
}

// Reference represents a paper that is cited.
type Reference struct {
	ID        string
	Title     string
	HasTitle  bool // True if we have metadata (title available)
	HasSource bool // True if we have source downloaded
}

func (c *Cache) References(ctx context.Context, paperID string) ([]Reference, error) {
	cacheKey := detailKey("references", paperID)
	if cached, ok := c.getDetailCache(cacheKey); ok {
		if refs, ok := cached.([]Reference); ok {
			return cloneReferences(refs), nil
		}
	}

	value, err, _ := c.detailFlights.Do(cacheKey, func() (interface{}, error) {
		if cached, ok := c.getDetailCache(cacheKey); ok {
			if refs, ok := cached.([]Reference); ok {
				return cloneReferences(refs), nil
			}
		}
		refs, err := c.referencesUncached(ctx, paperID)
		if err != nil {
			return nil, err
		}
		c.putDetailCache(cacheKey, citationTTL, cloneReferences(refs))
		return refs, nil
	})
	if err != nil {
		return nil, err
	}
	refs, _ := value.([]Reference)
	return cloneReferences(refs), nil
}

func (c *Cache) referencesUncached(ctx context.Context, paperID string) ([]Reference, error) {
	type refRow struct {
		ID        string
		Title     string
		HasTitle  bool
		HasSource bool
	}

	var rows []refRow
	sqlDB, _ := c.db.DB()
	placeholder := c.bindVar(1)
	err := c.withCitationQuery(ctx, func() error {
		return c.db.WithContext(ctx).
			Table("citations").
			Select("citations.to_id as id, COALESCE(papers.title, '') as title, "+
				"CASE WHEN papers.id IS NOT NULL AND papers.title != '' THEN 1 ELSE 0 END as has_title, "+
				"CASE WHEN papers.src_downloaded = true THEN 1 ELSE 0 END as has_source").
			Joins("LEFT JOIN papers ON citations.to_id = papers.id").
			Where("citations.from_id = ?", paperID).
			Order("citations.to_id DESC").
			Scan(&rows).Error
	})
	if err != nil {
		// Fallback to raw SQL for complex CASE statements
		var rawRows interface {
			Close() error
			Next() bool
			Scan(dest ...interface{}) error
			Err() error
		}
		err := c.withCitationQuery(ctx, func() error {
			rows, err := sqlDB.QueryContext(ctx, `
				SELECT c.to_id, COALESCE(p.title, ''),
				       CASE WHEN p.id IS NOT NULL AND p.title != '' THEN 1 ELSE 0 END,
				       CASE WHEN p.src_downloaded = true THEN 1 ELSE 0 END
					FROM citations c
					LEFT JOIN papers p ON c.to_id = p.id
					WHERE c.from_id = `+placeholder+`
					ORDER BY c.to_id DESC
				`, paperID)
			if err != nil {
				return err
			}
			rawRows = rows
			return nil
		})
		if err != nil {
			return nil, err
		}
		defer rawRows.Close()
		var refs []Reference
		for rawRows.Next() {
			var r Reference
			var hasTitle, hasSource int
			if err := rawRows.Scan(&r.ID, &r.Title, &hasTitle, &hasSource); err != nil {
				return nil, err
			}
			r.HasTitle = hasTitle == 1
			r.HasSource = hasSource == 1
			refs = append(refs, r)
		}
		return refs, rawRows.Err()
	}

	refs := make([]Reference, len(rows))
	for i, row := range rows {
		refs[i] = Reference{
			ID:        row.ID,
			Title:     row.Title,
			HasTitle:  row.HasTitle,
			HasSource: row.HasSource,
		}
	}

	return refs, nil
}

// UncachedReferenceCount returns the number of references without metadata.
func (c *Cache) UncachedReferenceCount(ctx context.Context, paperID string) (int, error) {
	cacheKey := detailKey("uncached_reference_count", paperID)
	if cached, ok := c.getDetailCache(cacheKey); ok {
		if count, ok := cached.(int); ok {
			return count, nil
		}
	}

	var count int64
	sqlDB, _ := c.db.DB()
	placeholder := c.bindVar(1)
	err := c.withCitationQuery(ctx, func() error {
		return sqlDB.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM citations c
			LEFT JOIN papers p ON c.to_id = p.id
			WHERE c.from_id = `+placeholder+` AND (p.id IS NULL OR p.title = '')
		`, paperID).Scan(&count)
	})
	if err == nil {
		c.putDetailCache(cacheKey, citationTTL, int(count))
	}
	return int(count), err
}

// RebuildAllCitations rebuilds the citations table by re-extracting references from all papers.
func (c *Cache) RebuildAllCitations(ctx context.Context) error {
	// Clear existing citations
	c.db.WithContext(ctx).Exec("DELETE FROM citations")

	// Get all papers with source downloaded
	var papers []Paper
	if err := c.db.WithContext(ctx).
		Select("id", "src_path").
		Where("src_downloaded = ? AND src_path IS NOT NULL", true).
		Find(&papers).Error; err != nil {
		return err
	}

	// Extract and store citations for each paper
	for _, p := range papers {
		if err := c.UpdateCitations(ctx, p.ID, p.SourcePath); err != nil {
			return err
		}
	}

	return nil
}

// GetPaperWithCitations returns a paper along with its citation count.
func (c *Cache) GetPaperWithCitations(ctx context.Context, id string) (*Paper, int, error) {
	paper, err := c.GetPaper(ctx, id)
	if err != nil {
		return nil, 0, err
	}

	count, err := c.CitedByCount(ctx, id)
	if err != nil {
		return paper, 0, err
	}

	return paper, count, nil
}

// scanPaper is a helper to scan paper rows (used internally).
func scanPaperRow(row interface {
	Scan(dest ...any) error
}) (*Paper, error) {
	var p Paper
	var created, updated string
	var pdfDl, srcDl int

	err := row.Scan(
		&p.ID, &created, &updated, &p.Title, &p.Abstract, &p.Authors,
		&p.Categories, &p.Comments, &p.JournalRef, &p.DOI, &p.License,
		&pdfDl, &srcDl,
	)
	if err != nil {
		return nil, err
	}

	p.Created, _ = time.Parse("2006-01-02", created)
	p.Updated, _ = time.Parse("2006-01-02", updated)
	p.PDFDownloaded = pdfDl == 1
	p.SourceDownloaded = srcDl == 1

	return &p, nil
}

// GraphNode represents a node in the citation graph.
type GraphNode struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Authors   string `json:"authors"`
	Year      int    `json:"year"`
	Citations int    `json:"citations"` // How many papers cite this one
	Cached    bool   `json:"cached"`
}

// GraphEdge represents an edge in the citation graph.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// CitationGraph represents a citation graph for visualization.
type CitationGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// PaperListItem represents a paper in the sidebar list.
type PaperListItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Authors   string `json:"authors"`
	Year      int    `json:"year"`
	Citations int    `json:"citations"`
	Cached    bool   `json:"cached"`
	IsRef     bool   `json:"isRef"`    // True if this paper is a reference
	IsCiting  bool   `json:"isCiting"` // True if this paper cites the main paper
}

// GetCitationGraph returns a citation graph centered on the given paper.
// Includes: the paper itself, its references, papers that cite it,
// and edges between references if they cite each other.
func (c *Cache) GetCitationGraph(ctx context.Context, paperID string) (*CitationGraph, error) {
	graph := &CitationGraph{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	nodeSet := make(map[string]GraphNode)
	edgeSet := make(map[string]bool)

	addNode := func(id string, node GraphNode) {
		if _, exists := nodeSet[id]; !exists {
			nodeSet[id] = node
		}
	}

	addEdge := func(from, to string) {
		key := from + "->" + to
		if !edgeSet[key] {
			edgeSet[key] = true
			graph.Edges = append(graph.Edges, GraphEdge{Source: from, Target: to})
		}
	}

	// Helper to extract year from arXiv ID (YYMM.NNNNN -> 20YY)
	yearFromID := func(id string) int {
		if len(id) >= 2 {
			yy := id[:2]
			if yy >= "00" && yy <= "99" {
				year := 2000 + int(yy[0]-'0')*10 + int(yy[1]-'0')
				if year > 2090 {
					year -= 100 // 91-99 -> 1991-1999
				}
				return year
			}
		}
		return 0
	}

	// Get the central paper
	paper, err := c.GetPaper(ctx, paperID)
	if err != nil {
		return nil, err
	}
	refs, err := c.References(ctx, paperID)
	if err != nil {
		return nil, err
	}
	citedBy, err := c.CitedBy(ctx, paperID, 100)
	if err != nil {
		return nil, err
	}
	ids := []string{paperID}
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	for _, citing := range citedBy {
		ids = append(ids, citing.ID)
	}
	details, err := c.citationPaperDetails(ctx, ids)
	if err != nil {
		return nil, err
	}
	addNode(paper.ID, GraphNode{
		ID:        paper.ID,
		Title:     paper.Title,
		Authors:   paper.Authors,
		Year:      paper.Created.Year(),
		Citations: details[paperID].Citations,
		Cached:    true,
	})

	refIDs := make(map[string]bool)
	for _, ref := range refs {
		title := ref.Title
		if title == "" {
			title = ref.ID
		}
		year := yearFromID(ref.ID)
		detail := details[ref.ID]
		var authors string
		if ref.HasTitle && detail.HasMetadata {
			authors = detail.Authors
			year = detail.Created.Year()
		}
		addNode(ref.ID, GraphNode{
			ID:        ref.ID,
			Title:     title,
			Authors:   authors,
			Year:      year,
			Citations: detail.Citations,
			Cached:    ref.HasTitle,
		})
		addEdge(paperID, ref.ID)
		refIDs[ref.ID] = true
	}

	for _, citing := range citedBy {
		detail := details[citing.ID]
		var year int
		if detail.HasMetadata {
			year = detail.Created.Year()
		}
		addNode(citing.ID, GraphNode{
			ID:        citing.ID,
			Title:     citing.Title,
			Authors:   detail.Authors,
			Year:      year,
			Citations: detail.Citations,
			Cached:    true,
		})
		addEdge(citing.ID, paperID)
	}

	// Find edges between references (if they cite each other)
	if len(refIDs) > 0 {
		var edges []Citation
		err := c.withCitationQuery(ctx, func() error {
			return c.db.WithContext(ctx).Raw(`
				SELECT from_id, to_id FROM citations
				WHERE from_id IN (SELECT to_id FROM citations WHERE from_id = ?)
				  AND to_id IN (SELECT to_id FROM citations WHERE from_id = ?)
			`, paperID, paperID).Scan(&edges).Error
		})
		if err != nil {
			return nil, fmt.Errorf("query reference citation edges: %w", err)
		}
		for _, edge := range edges {
			addEdge(edge.FromID, edge.ToID)
		}
	}

	// Convert node map to slice
	for _, node := range nodeSet {
		graph.Nodes = append(graph.Nodes, node)
	}

	return graph, nil
}

// GetPaperList returns a combined list of references and citing papers for the sidebar.
func (c *Cache) GetPaperList(ctx context.Context, paperID string) ([]PaperListItem, error) {
	var items []PaperListItem

	// Helper to extract year from arXiv ID
	yearFromID := func(id string) int {
		if len(id) >= 2 {
			yy := id[:2]
			if yy >= "00" && yy <= "99" {
				year := 2000 + int(yy[0]-'0')*10 + int(yy[1]-'0')
				if year > 2090 {
					year -= 100
				}
				return year
			}
		}
		return 0
	}

	// Get references
	refs, err := c.References(ctx, paperID)
	if err != nil {
		return nil, err
	}
	citedBy, err := c.CitedBy(ctx, paperID, 100)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(refs)+len(citedBy))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	for _, citing := range citedBy {
		ids = append(ids, citing.ID)
	}
	details, err := c.citationPaperDetails(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		title := ref.Title
		if title == "" {
			title = ref.ID
		}
		year := yearFromID(ref.ID)
		detail := details[ref.ID]
		var authors string
		if ref.HasTitle && detail.HasMetadata {
			authors = detail.Authors
			year = detail.Created.Year()
		}
		items = append(items, PaperListItem{
			ID:        ref.ID,
			Title:     title,
			Authors:   authors,
			Year:      year,
			Citations: detail.Citations,
			Cached:    ref.HasTitle,
			IsRef:     true,
		})
	}

	for _, citing := range citedBy {
		detail := details[citing.ID]
		var year int
		if detail.HasMetadata {
			year = detail.Created.Year()
		}
		items = append(items, PaperListItem{
			ID:        citing.ID,
			Title:     citing.Title,
			Authors:   detail.Authors,
			Year:      year,
			Citations: detail.Citations,
			Cached:    true,
			IsCiting:  true,
		})
	}

	return items, nil
}

// citationPaperDetail contains only fields needed by the graph and sidebar.
// Keep this separate from paperLRU: a partial Paper must never replace full metadata.
type citationPaperDetail struct {
	Authors     string
	Created     time.Time
	Citations   int
	HasMetadata bool
}

// citationPaperDetails avoids two queries per related paper on a cold cache.
// Batches also keep large bibliographies below backend parameter limits.
func (c *Cache) citationPaperDetails(ctx context.Context, ids []string) (map[string]citationPaperDetail, error) {
	details := make(map[string]citationPaperDetail, len(ids))
	var missingPapers, missingCounts []string
	for _, id := range ids {
		if _, ok := details[id]; ok {
			continue
		}
		detail := citationPaperDetail{}
		if c.paperLRU != nil {
			if cached, ok := c.paperLRU.Get(id); ok {
				if paper, ok := cached.(*Paper); ok && paper != nil {
					detail.Authors, detail.Created, detail.HasMetadata = paper.Authors, paper.Created, true
				}
			}
		}
		if !detail.HasMetadata {
			if cached, ok := c.getDetailCache(detailKey("citation_metadata", id)); ok {
				if metadata, ok := cached.(citationPaperDetail); ok {
					detail = metadata
				} else {
					missingPapers = append(missingPapers, id)
				}
			} else {
				missingPapers = append(missingPapers, id)
			}
		}
		count, cachedCount := c.getDetailCache(detailKey("cited_by_count", id))
		if citations, ok := count.(int); cachedCount && ok {
			detail.Citations = citations
		} else {
			missingCounts = append(missingCounts, id)
		}
		details[id] = detail
	}

	const batchSize = 500
	for start := 0; start < len(missingPapers); start += batchSize {
		batch := missingPapers[start:min(start+batchSize, len(missingPapers))]
		var papers []Paper
		err := c.withCitationQuery(ctx, func() error {
			return c.db.WithContext(ctx).Select("id", "authors", "created").Where("id IN ?", batch).Find(&papers).Error
		})
		if err != nil {
			return nil, fmt.Errorf("load citation paper metadata: %w", err)
		}
		for _, paper := range papers {
			detail := details[paper.ID]
			detail.Authors, detail.Created, detail.HasMetadata = paper.Authors, paper.Created, true
			details[paper.ID] = detail
		}
		for _, id := range batch {
			metadata := details[id]
			metadata.Citations = 0 // Citation counts have their own independently refreshed cache.
			c.putDetailCache(detailKey("citation_metadata", id), citationTTL, metadata)
		}
	}
	for start := 0; start < len(missingCounts); start += batchSize {
		batch := missingCounts[start:min(start+batchSize, len(missingCounts))]
		var counts []struct {
			ToID  string
			Count int
		}
		err := c.withCitationQuery(ctx, func() error {
			return c.db.WithContext(ctx).Model(&Citation{}).Select("to_id, COUNT(*) AS count").
				Where("to_id IN ?", batch).Group("to_id").Scan(&counts).Error
		})
		if err != nil {
			return nil, fmt.Errorf("load citation counts: %w", err)
		}
		for _, count := range counts {
			detail := details[count.ToID]
			detail.Citations = count.Count
			details[count.ToID] = detail
		}
		for _, id := range batch {
			c.putDetailCache(detailKey("cited_by_count", id), citationTTL, details[id].Citations)
		}
	}
	return details, nil
}
