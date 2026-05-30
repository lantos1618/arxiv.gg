package arxiv

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AdminStats is a compact, read-only snapshot for the web admin dashboard.
type AdminStats struct {
	GeneratedAt    time.Time
	DBType         DBType
	Cache          CacheStats
	Embeddings     AdminEmbeddingStats
	Users          AdminUserStats
	EmbeddingJobs  map[string]int64
	RecentUsers    []AdminUserRow
	RecentAuditLog []AdminAuditRow
}

type AdminEmbeddingStats struct {
	MiniLMAbstracts     int64
	QwenAbstracts       int64
	FullAbstracts       int64
	MissingAbstractText int64
	FullPaperChunks     int64
	FullPaperEmbeddings int64
	PendingMiniLM       int64
	PendingQwenAbstract int64
	PendingFullPaper    int64
}

type AdminUserStats struct {
	TotalUsers     int64
	New24h         int64
	New7d          int64
	New30d         int64
	Active24h      int64
	Active7d       int64
	Active30d      int64
	FreeUsers      int64
	PaidUsers      int64
	UnsetPlanUsers int64
}

type AdminUserRow struct {
	ID          string
	Email       string
	Name        string
	Plan        string
	Provider    string
	Verified    bool
	CreatedAt   time.Time
	LastLoginAt *time.Time
	LastSeenAt  *time.Time
}

type AdminAuditRow struct {
	AdminEmail string
	Action     string
	TargetType string
	TargetID   string
	Details    string
	CreatedAt  time.Time
}

// AdminStats returns real dashboard numbers from the database. Anything not
// represented here should be shown by the UI as a placeholder, not invented.
func (c *Cache) AdminStats(ctx context.Context) (*AdminStats, error) {
	now := time.Now().UTC()
	cacheStats, err := c.Stats(ctx)
	if err != nil {
		return nil, err
	}

	stats := &AdminStats{
		GeneratedAt:   now,
		DBType:        c.dbType,
		Cache:         *cacheStats,
		EmbeddingJobs: map[string]int64{},
	}

	if err := c.countEmbeddingsForAdmin(ctx, &stats.Embeddings, cacheStats.TotalPapers); err != nil {
		return nil, err
	}
	if err := c.countUsersForAdmin(ctx, &stats.Users, now); err != nil {
		return nil, err
	}
	if jobs, err := c.EmbeddingJobStats(ctx); err != nil {
		return nil, err
	} else {
		stats.EmbeddingJobs = jobs
	}
	recentUsers, err := c.RecentAdminUsers(ctx, 50)
	if err != nil {
		return nil, err
	}
	stats.RecentUsers = recentUsers
	recentAudit, err := c.RecentAdminAudit(ctx, 50)
	if err != nil {
		return nil, err
	}
	stats.RecentAuditLog = recentAudit

	return stats, nil
}

func (c *Cache) countEmbeddingsForAdmin(ctx context.Context, out *AdminEmbeddingStats, totalPapers int64) error {
	if err := c.db.WithContext(ctx).Model(&Paper{}).
		Where("COALESCE(title, '') <> '' AND COALESCE(abstract, '') <> ''").
		Count(&out.FullAbstracts).Error; err != nil {
		return err
	}
	if err := c.db.WithContext(ctx).Model(&Embedding{}).Count(&out.MiniLMAbstracts).Error; err != nil {
		return err
	}
	if err := c.db.WithContext(ctx).Model(&EmbeddingV2{}).
		Where("scope = ? AND model LIKE ? AND dim = ?", "abstract", "%Qwen%", 1024).
		Count(&out.QwenAbstracts).Error; err != nil {
		return err
	}
	if err := c.db.WithContext(ctx).Model(&PaperChunk{}).Count(&out.FullPaperChunks).Error; err != nil {
		return err
	}
	if err := c.db.WithContext(ctx).Model(&ChunkEmbeddingV2{}).Count(&out.FullPaperEmbeddings).Error; err != nil {
		return err
	}
	out.MissingAbstractText = maxInt64(totalPapers-out.FullAbstracts, 0)
	out.PendingMiniLM = maxInt64(out.FullAbstracts-out.MiniLMAbstracts, 0)
	out.PendingQwenAbstract = maxInt64(out.FullAbstracts-out.QwenAbstracts, 0)
	out.PendingFullPaper = maxInt64(out.FullPaperChunks-out.FullPaperEmbeddings, 0)
	return nil
}

func (c *Cache) countUsersForAdmin(ctx context.Context, out *AdminUserStats, now time.Time) error {
	if err := c.db.WithContext(ctx).Model(&User{}).Count(&out.TotalUsers).Error; err != nil {
		return err
	}
	for _, window := range []struct {
		since time.Time
		count *int64
	}{
		{now.Add(-24 * time.Hour), &out.New24h},
		{now.Add(-7 * 24 * time.Hour), &out.New7d},
		{now.Add(-30 * 24 * time.Hour), &out.New30d},
	} {
		if err := c.db.WithContext(ctx).Model(&User{}).Where("created_at >= ?", window.since).Count(window.count).Error; err != nil {
			return err
		}
	}
	for _, window := range []struct {
		since time.Time
		count *int64
	}{
		{now.Add(-24 * time.Hour), &out.Active24h},
		{now.Add(-7 * 24 * time.Hour), &out.Active7d},
		{now.Add(-30 * 24 * time.Hour), &out.Active30d},
	} {
		if err := c.db.WithContext(ctx).Model(&UserSession{}).
			Where("last_seen_at >= ? AND expires_at > ?", window.since, now).
			Distinct("user_id").
			Count(window.count).Error; err != nil {
			return err
		}
	}
	if err := c.db.WithContext(ctx).Model(&User{}).Where("plan = ?", "free").Count(&out.FreeUsers).Error; err != nil {
		return err
	}
	if err := c.db.WithContext(ctx).Model(&User{}).Where("plan = ?", "paid").Count(&out.PaidUsers).Error; err != nil {
		return err
	}
	return c.db.WithContext(ctx).Model(&User{}).Where("plan = '' OR plan IS NULL").Count(&out.UnsetPlanUsers).Error
}

func (c *Cache) RecentAdminUsers(ctx context.Context, limit int) ([]AdminUserRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var users []User
	if err := c.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&users).Error; err != nil {
		return nil, err
	}

	rows := make([]AdminUserRow, 0, len(users))
	for _, user := range users {
		var lastSeen *time.Time
		var session UserSession
		err := c.db.WithContext(ctx).
			Where("user_id = ?", user.ID).
			Order("last_seen_at DESC").
			Limit(1).
			First(&session).Error
		if err == nil {
			lastSeen = &session.LastSeenAt
		}
		rows = append(rows, AdminUserRow{
			ID:          user.ID,
			Email:       user.Email,
			Name:        user.Name,
			Plan:        normalizedPlan(user.Plan),
			Provider:    user.AuthProvider,
			Verified:    user.EmailVerified,
			CreatedAt:   user.CreatedAt,
			LastLoginAt: user.LastLoginAt,
			LastSeenAt:  lastSeen,
		})
	}
	return rows, nil
}

func (c *Cache) RecentAdminAudit(ctx context.Context, limit int) ([]AdminAuditRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var logs []AdminAuditLog
	if err := c.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}
	rows := make([]AdminAuditRow, 0, len(logs))
	for _, log := range logs {
		rows = append(rows, AdminAuditRow{
			AdminEmail: log.AdminEmail,
			Action:     log.Action,
			TargetType: log.TargetType,
			TargetID:   log.TargetID,
			Details:    log.Details,
			CreatedAt:  log.CreatedAt,
		})
	}
	return rows, nil
}

func (c *Cache) RecordAdminAudit(ctx context.Context, adminEmail, action, targetType, targetID, details string) error {
	adminEmail = trimForStorage(strings.TrimSpace(adminEmail), 320)
	if adminEmail == "" {
		adminEmail = "admin-token"
	}
	log := &AdminAuditLog{
		ID:         "audit_" + mustRandomToken(18),
		AdminEmail: adminEmail,
		Action:     trimForStorage(action, 128),
		TargetType: trimForStorage(targetType, 128),
		TargetID:   trimForStorage(targetID, 256),
		Details:    trimForStorage(details, 2048),
		CreatedAt:  time.Now().UTC(),
	}
	if log.Action == "" {
		return fmt.Errorf("audit action is required")
	}
	return c.db.WithContext(ctx).Create(log).Error
}

func normalizedPlan(plan string) string {
	plan = strings.ToLower(strings.TrimSpace(plan))
	switch plan {
	case "paid":
		return "paid"
	case "free":
		return "free"
	default:
		return "free"
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
