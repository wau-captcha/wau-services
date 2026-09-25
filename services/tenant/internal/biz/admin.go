package biz

import (
	"cmp"
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	defaultUserPageSize = 20
	maxUserPageSize     = 100
	maxUserQueryLen     = 100
)

// AdminStats counts accounts and keys across the system.
type AdminStats struct {
	Users            int64
	BannedUsers      int64
	ApiKeys          int64
	SuspendedApiKeys int64
	ScrapingApiKeys  int64
}

// AdminUser is an account as administrators see it.
type AdminUser struct {
	User
	ApiKeyCount int
	// Successful verifications today.
	QuotaUsed int64
}

// UserPage is one page of accounts and how many match in total.
type UserPage struct {
	Users     []*AdminUser
	TotalSize int
}

// AdminApiKey is a key as administrators see it: never with its secret key.
type AdminApiKey struct {
	ID            uuid.UUID
	Name          string
	SiteKey       string
	IsActive      bool
	IsSuspended   bool
	SuspendReason string
	CreateDate    time.Time
	// Today's scraping counters.
	ScrapingCount
}

// AdminRepo reads and changes any account.
type AdminRepo interface {
	Stats(ctx context.Context) (*AdminStats, error)
	// ListUsers matches pattern, a LIKE pattern, against email and display name; empty matches all.
	ListUsers(ctx context.Context, pattern string, limit, offset int) ([]*AdminUser, int, error)
	// GetUser returns ErrNoSuchUser when the account does not exist.
	GetUser(ctx context.Context, id uuid.UUID) (*AdminUser, error)
	// SetUserBanned returns ErrNoSuchUser when the account does not exist.
	SetUserBanned(ctx context.Context, id uuid.UUID, banned bool) error
	ListApiKeys(ctx context.Context, userID uuid.UUID) ([]*AdminApiKey, error)
	// SetApiKeySuspended returns ErrApiKeyNotFound for a missing or deleted key.
	SetApiKeySuspended(ctx context.Context, id uuid.UUID, suspended bool) (*AdminApiKey, error)
}

// AdminUsecase oversees every account. Callers are administrators; the server checks that.
type AdminUsecase struct {
	accounts AdminRepo
	counters CounterRepo
	usage    *UsageUsecase
	calendar *Calendar
}

func NewAdminUsecase(accounts AdminRepo, counters CounterRepo, usage *UsageUsecase, calendar *Calendar) *AdminUsecase {
	return &AdminUsecase{accounts: accounts, counters: counters, usage: usage, calendar: calendar}
}

func (uc *AdminUsecase) Stats(ctx context.Context) (*AdminStats, error) {
	return uc.accounts.Stats(ctx)
}

// ListUsers pages through accounts, newest first. page is 1-based; 0 means the first page.
func (uc *AdminUsecase) ListUsers(ctx context.Context, query string, page, pageSize int) (*UserPage, error) {
	query = strings.TrimSpace(query)
	switch {
	case utf8.RuneCountInString(query) > maxUserQueryLen:
		return nil, InvalidField("query", "query must be at most 100 characters")
	case page < 0:
		return nil, InvalidField("page", "page must not be negative")
	case pageSize < 0 || pageSize > maxUserPageSize:
		return nil, InvalidField("page_size", "page_size must be between 1 and 100")
	}
	page, pageSize = max(page, 1), cmp.Or(pageSize, defaultUserPageSize)

	users, total, err := uc.accounts.ListUsers(ctx, likePattern(query), pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	if err := uc.fillQuota(ctx, users...); err != nil {
		return nil, err
	}
	return &UserPage{Users: users, TotalSize: total}, nil
}

func (uc *AdminUsecase) GetUser(ctx context.Context, id uuid.UUID) (*AdminUser, error) {
	user, err := uc.accounts.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := uc.fillQuota(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// SetUserBanned bans or unbans an account. Administrators are not accounts, so a ban never
// locks one out, even when the account has the same email.
func (uc *AdminUsecase) SetUserBanned(ctx context.Context, id uuid.UUID, banned bool) (*AdminUser, error) {
	if err := uc.accounts.SetUserBanned(ctx, id, banned); err != nil {
		return nil, err
	}
	return uc.GetUser(ctx, id)
}

// ListApiKeys returns the keys of an account that are not deleted, with today's counters.
func (uc *AdminUsecase) ListApiKeys(ctx context.Context, userID uuid.UUID) ([]*AdminApiKey, error) {
	if _, err := uc.accounts.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	keys, err := uc.accounts.ListApiKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := uc.fillScraping(ctx, keys...); err != nil {
		return nil, err
	}
	return keys, nil
}

// SetApiKeySuspended suspends a key or lifts its suspension. Lifting it also clears
// today's scraping counters: they would suspend the key again on its next challenge.
func (uc *AdminUsecase) SetApiKeySuspended(ctx context.Context, id uuid.UUID, suspended bool) (*AdminApiKey, error) {
	key, err := uc.accounts.SetApiKeySuspended(ctx, id, suspended)
	if err != nil {
		return nil, err
	}
	if !suspended {
		if err := uc.counters.ResetScraping(ctx, id, uc.calendar.Today()); err != nil {
			return nil, err
		}
	}
	if err := uc.fillScraping(ctx, key); err != nil {
		return nil, err
	}
	return key, nil
}

// Usage reports verifications of the whole system, of one account, or of one key.
func (uc *AdminUsecase) Usage(ctx context.Context, filter UsageFilter, window UsageWindow) (*Usage, error) {
	return uc.usage.ForAnyone(ctx, filter, window)
}

func (uc *AdminUsecase) fillQuota(ctx context.Context, users ...*AdminUser) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(users))
	for i, user := range users {
		ids[i] = user.ID
	}
	quotaUsed, err := uc.counters.QuotaUsed(ctx, ids, uc.calendar.Today())
	if err != nil {
		return err
	}
	for i, user := range users {
		user.QuotaUsed = quotaUsed[i]
	}
	return nil
}

func (uc *AdminUsecase) fillScraping(ctx context.Context, keys ...*AdminApiKey) error {
	if len(keys) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(keys))
	for i, key := range keys {
		ids[i] = key.ID
	}
	counts, err := uc.counters.ScrapingCounts(ctx, ids, uc.calendar.Today())
	if err != nil {
		return err
	}
	for i, key := range keys {
		key.ScrapingCount = counts[i]
	}
	return nil
}

// likePattern matches query anywhere, taking %, _ and \ in it literally.
func likePattern(query string) string {
	if query == "" {
		return ""
	}
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	return "%" + escaped + "%"
}
