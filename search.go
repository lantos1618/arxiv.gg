package arxiv

import (
	"context"
	"strings"
	"time"
)

// Search searches papers by title/abstract text using full-text search.
// Uses FTS5 for SQLite or tsvector for PostgreSQL.
func (c *Cache) Search(ctx context.Context, query, category string, limit int) ([]Paper, error) {
	if limit <= 0 {
		limit = 20
	}

	if c.dbType == DBTypePostgres {
		return c.searchPostgres(ctx, query, category, limit)
	}
	return c.searchSQLite(ctx, query, category, limit)
}

// searchSQLite uses FTS5 for full-text search
func (c *Cache) searchSQLite(ctx context.Context, query, category string, limit int) ([]Paper, error) {
	sql := `
		SELECT p.id, p.created, p.updated, p.title, p.abstract, p.authors, p.categories,
		       p.comments, p.journal_ref, p.doi, p.license, p.pdf_downloaded, p.src_downloaded
		FROM papers p
		JOIN papers_fts fts ON p.rowid = fts.rowid
		WHERE papers_fts MATCH ?
	`
	args := []any{query}

	if category != "" {
		sql += " AND p.categories LIKE '%' || ? || '%'"
		args = append(args, category)
	}

	sql += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	sqlDB, _ := c.db.DB()
	rows, err := sqlDB.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var papers []Paper
	for rows.Next() {
		var p Paper
		var created, updated string
		var pdfDl, srcDl int

		err := rows.Scan(
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

		papers = append(papers, p)
	}

	return papers, rows.Err()
}

// searchPostgres uses tsvector for full-text search
func (c *Cache) searchPostgres(ctx context.Context, query, category string, limit int) ([]Paper, error) {
	sql := `
		SELECT id, created, updated, title, abstract, authors, categories,
		       comments, journal_ref, doi, license, pdf_downloaded, src_downloaded,
		       ts_rank(search_vector, plainto_tsquery('english', $1)) AS rank
		FROM papers
		WHERE search_vector @@ plainto_tsquery('english', $1)
	`
	args := []any{query}
	argNum := 2

	if category != "" {
		sql += " AND categories ILIKE '%' || $" + string(rune('0'+argNum)) + " || '%'"
		args = append(args, category)
		argNum++
	}

	sql += " ORDER BY rank DESC LIMIT $" + string(rune('0'+argNum))
	args = append(args, limit)

	var papers []Paper
	err := c.db.WithContext(ctx).Raw(sql, args...).Scan(&papers).Error
	if err != nil {
		// Fallback to ILIKE search if tsvector not populated
		return c.searchPostgresFallback(ctx, query, category, limit)
	}

	if len(papers) == 0 {
		// Try fallback if no results (might be tsvector not populated)
		return c.searchPostgresFallback(ctx, query, category, limit)
	}

	return papers, nil
}

// searchPostgresFallback uses ILIKE for searching when tsvector not available
func (c *Cache) searchPostgresFallback(ctx context.Context, query, category string, limit int) ([]Paper, error) {
	q := c.db.WithContext(ctx).Model(&Paper{}).
		Where("title ILIKE ? OR abstract ILIKE ?", "%"+query+"%", "%"+query+"%")

	if category != "" {
		q = q.Where("categories ILIKE ?", "%"+category+"%")
	}

	var papers []Paper
	err := q.Order("created DESC").Limit(limit).Find(&papers).Error
	return papers, err
}

// SearchByAuthor searches papers by author name.
func (c *Cache) SearchByAuthor(ctx context.Context, author string, limit int) ([]Paper, error) {
	if limit <= 0 {
		limit = 100
	}

	var papers []Paper
	// Use ILIKE for PostgreSQL, LIKE for SQLite (SQLite LIKE is case-insensitive by default)
	likeOp := "LIKE"
	if c.dbType == DBTypePostgres {
		likeOp = "ILIKE"
	}

	err := c.db.WithContext(ctx).
		Where("authors "+likeOp+" ?", "%"+author+"%").
		Order("created DESC").
		Limit(limit).
		Find(&papers).Error
	return papers, err
}

// PaperExists checks if a paper exists in the cache.
func (c *Cache) PaperExists(ctx context.Context, id string) bool {
	var count int64
	err := c.db.WithContext(ctx).Model(&Paper{}).Where("id = ?", id).Count(&count).Error
	return err == nil && count > 0
}

// CategoryCount represents a category with its paper count.
type CategoryCount struct {
	Name  string
	Count int
}

// ListCategories returns all categories with their paper counts.
func (c *Cache) ListCategories(ctx context.Context) ([]CategoryCount, error) {
	// Categories are space-separated in the categories column
	// We need to split and count each individual category
	var papers []Paper
	if err := c.db.WithContext(ctx).Select("categories").Where("categories != ?", "").Find(&papers).Error; err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, p := range papers {
		for _, cat := range strings.Fields(p.Categories) {
			counts[cat]++
		}
	}

	var result []CategoryCount
	for name, count := range counts {
		result = append(result, CategoryCount{Name: name, Count: count})
	}

	// Sort by count descending
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Count > result[i].Count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// ListPapers lists papers, optionally filtered by category.
// Sorted by FetchedAt (most recently fetched first), with fallback to ID for legacy papers.
func (c *Cache) ListPapers(ctx context.Context, category string, offset, limit int) ([]Paper, error) {
	if limit <= 0 {
		limit = 100
	}

	query := c.db.WithContext(ctx).Model(&Paper{})
	if category != "" {
		// Use ILIKE for PostgreSQL
		if c.dbType == DBTypePostgres {
			query = query.Where("categories ILIKE ?", "%"+category+"%")
		} else {
			query = query.Where("categories LIKE ?", "%"+category+"%")
		}
	}

	var papers []Paper
	orderClause := "fetched_at DESC, id DESC"
	if c.dbType == DBTypePostgres {
		orderClause = "fetched_at DESC NULLS LAST, id DESC"
	}

	err := query.
		Where("src_downloaded = ?", true).
		Order(orderClause).
		Limit(limit).
		Offset(offset).
		Find(&papers).Error

	return papers, err
}

// ListPapersFiltered lists papers with various filter options.
func (c *Cache) ListPapersFiltered(ctx context.Context, category string, srcOnly, all bool, limit int) ([]Paper, error) {
	query := c.db.WithContext(ctx).Model(&Paper{})

	if category != "" {
		if c.dbType == DBTypePostgres {
			query = query.Where("categories ILIKE ?", "%"+category+"%")
		} else {
			query = query.Where("categories LIKE ?", "%"+category+"%")
		}
	}

	if srcOnly {
		query = query.Where("src_downloaded = ?", true)
	} else if !all {
		// Default: show papers with source OR title (exclude metadata-only without useful info)
		query = query.Where("src_downloaded = ? OR title != ?", true, "")
	}

	query = query.Order("id DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	var papers []Paper
	err := query.Find(&papers).Error
	return papers, err
}

// DownloadCategory downloads papers for a category.
func (c *Cache) DownloadCategory(ctx context.Context, category string, limit int, opts *DownloadOptions) error {
	// Use parameter placeholders appropriate for database type
	placeholder := "?"
	if c.dbType == DBTypePostgres {
		placeholder = "$"
	}

	var sql string
	var args []any

	if c.dbType == DBTypePostgres {
		sql = `
			SELECT id FROM papers
			WHERE categories ILIKE '%' || $1 || '%'
			AND (
				($2 = 1 AND pdf_downloaded = false) OR
				($3 = 1 AND src_downloaded = false)
			)
			ORDER BY created DESC
		`
	} else {
		sql = `
			SELECT id FROM papers
			WHERE categories LIKE '%' || ? || '%'
			AND (
				(? = 1 AND pdf_downloaded = 0) OR
				(? = 1 AND src_downloaded = 0)
			)
			ORDER BY created DESC
		`
	}

	args = []any{category}

	dlPDF := 0
	dlSrc := 0
	if opts != nil && opts.DownloadPDF {
		dlPDF = 1
	}
	if opts == nil || opts.DownloadSource {
		dlSrc = 1
	}
	args = append(args, dlPDF, dlSrc)

	if limit > 0 {
		if c.dbType == DBTypePostgres {
			sql += " LIMIT $4"
		} else {
			sql += " LIMIT " + placeholder
		}
		args = append(args, limit)
	}

	sqlDB, _ := c.db.DB()
	rows, err := sqlDB.QueryContext(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i, id := range ids {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if opts != nil && opts.Progress != nil {
			opts.Progress(id, i+1, len(ids))
		}

		if err := c.DownloadPaper(ctx, id, opts); err != nil {
			// Log and continue
			continue
		}

		// Rate limit
		time.Sleep(3 * time.Second)
	}

	return nil
}
