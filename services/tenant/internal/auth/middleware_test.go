package auth

import (
	"context"
	"testing"

	"tenant/internal/biz"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

func TestMiddlewareStopsBannedAccountsAtOnce(t *testing.T) {
	j := testJWT(t)
	id := uuid.New()
	token, _, _ := j.Issue(id)
	accounts := fakeAccounts{ids: map[uuid.UUID]biz.AccountStatus{id: {Exists: true, Banned: true}}}

	handler := Middleware(j, accounts)(func(context.Context, any) (any, error) {
		t.Fatal("handler ran for a banned account")
		return nil, nil
	})
	if _, err := handler(contextWithAuthorization("Bearer "+token), nil); !errors.Is(err, biz.ErrAccountBanned) {
		t.Fatalf("err = %v, want ErrAccountBanned", err)
	}
}

func TestMiddlewareRejectsAdministratorTokens(t *testing.T) {
	j := testJWT(t)
	id := uuid.New()
	token, _, _ := j.IssueAdmin(id)
	accounts := fakeAccounts{ids: map[uuid.UUID]biz.AccountStatus{id: {Exists: true}}}

	handler := Middleware(j, accounts)(func(context.Context, any) (any, error) {
		t.Fatal("handler ran for an administrator token")
		return nil, nil
	})
	if _, err := handler(contextWithAuthorization("Bearer "+token), nil); !errors.Is(err, biz.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}

// fakeAdminAccounts holds the administrators that still exist.
type fakeAdminAccounts map[uuid.UUID]bool

func (f fakeAdminAccounts) Get(_ context.Context, id uuid.UUID) (*biz.Admin, error) {
	if !f[id] {
		return nil, biz.ErrUnauthenticated
	}
	return &biz.Admin{ID: id}, nil
}

func TestAdminMiddleware(t *testing.T) {
	j := testJWT(t)
	admin, removed := uuid.New(), uuid.New()
	admins := fakeAdminAccounts{admin: true}
	adminToken, _, _ := j.IssueAdmin(admin)
	removedToken, _, _ := j.IssueAdmin(removed)
	// A user who shares the administrator's id is still no administrator.
	userToken, _, _ := j.Issue(admin)

	var seen uuid.UUID
	handler := AdminMiddleware(j, admins)(func(ctx context.Context, _ any) (any, error) {
		seen, _ = AdminID(ctx)
		return nil, nil
	})
	if _, err := handler(contextWithAuthorization("Bearer "+adminToken), nil); err != nil || seen != admin {
		t.Fatalf("admin token: err = %v, admin id = %v, want %v", err, seen, admin)
	}

	rejected := map[string]string{
		"missing":               "",
		"user token":            "Bearer " + userToken,
		"removed administrator": "Bearer " + removedToken,
	}
	for name, value := range rejected {
		t.Run(name, func(t *testing.T) {
			handler := AdminMiddleware(j, admins)(func(context.Context, any) (any, error) {
				t.Fatal("handler ran for a rejected request")
				return nil, nil
			})
			if _, err := handler(contextWithAuthorization(value), nil); !errors.Is(err, biz.ErrUnauthenticated) {
				t.Fatalf("err = %v, want ErrUnauthenticated", err)
			}
		})
	}
}
