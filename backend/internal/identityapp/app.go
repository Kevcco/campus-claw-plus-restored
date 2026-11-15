// Package identityapp assembles the active identity service after verifying TLS
// and offline migration state. It never performs DDL or reads migration credentials.
package identityapp

import (
	"context"
	"errors"
	"github.com/zwh9616/campus-claw-plus/backend/internal/assignments"
	"github.com/zwh9616/campus-claw-plus/backend/internal/config"
	"github.com/zwh9616/campus-claw-plus/backend/internal/httpserver"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
	"github.com/zwh9616/campus-claw-plus/backend/internal/migrations"
	"github.com/zwh9616/campus-claw-plus/backend/internal/mysqlstore"
	"log/slog"
	"net/http"
	"time"
)

func New(ctx context.Context, lookup func(string) (string, bool), logger *slog.Logger) (http.Handler, func() error, error) {
	handler, repo, e := NewWithStore(ctx, lookup, logger)
	if e != nil {
		return nil, nil, e
	}
	return handler, repo.DB.Close, nil
}

// NewWithStore shares the verified DML pool with default runtime assembly.
// It preserves the identity-only constructor used by component tests; neither
// path reads offline migration credentials or performs DDL.
func NewWithStore(ctx context.Context, lookup func(string) (string, bool), logger *slog.Logger) (http.Handler, *mysqlstore.Store, error) {
	handler, repo, _, e := Assemble(ctx, lookup, logger, materials.NewLimiter(nil))
	return handler, repo, e
}

// Assemble permits the default Document boundary to share the same verified
// authenticator, DML pool and per-user TXT/document write quota.
func Assemble(ctx context.Context, lookup func(string) (string, bool), logger *slog.Logger, limiter *materials.Limiter) (http.Handler, *mysqlstore.Store, *identity.Service, error) {
	cfg, e := config.ReadIdentity(lookup)
	if e != nil {
		return nil, nil, nil, e
	}
	db, e := mysqlstore.OpenEnvironment(lookup, false)
	if e != nil {
		return nil, nil, nil, e
	}
	fail := func() (http.Handler, *mysqlstore.Store, *identity.Service, error) {
		db.Close()
		return nil, nil, nil, errors.New("identity database startup check failed")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var name string
	if db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name) != nil {
		return fail()
	}
	if (migrations.Executor{DB: db, Database: name}).Check(ctx, migrations.Target) != nil {
		return fail()
	}
	repo := &mysqlstore.Store{DB: db}
	service, e := identity.New(repo, nil, cfg.Origin)
	if e != nil {
		return fail()
	}
	base := httpserver.IdentityHandler(logger, service, repo, cfg, identity.NewLimiter(nil))
	base = httpserver.MaterialsHandler(logger, service, &materials.Service{Repo: repo}, cfg.Origin, limiter, base)
	logger.Info("task executor is not bound; accepted tasks remain pending", "module", "assignments", "request_id", nil)
	return httpserver.AssignmentsHandler(logger, service, &assignments.Service{Repo: repo, Limiter: assignments.NewLimiter(nil)}, cfg.Origin, base), repo, service, nil
}
