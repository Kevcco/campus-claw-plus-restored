//go:build ocrmigration

package migrations

import (
	"context"
	"regexp"
	"strings"
)

// DownEmptyOCRForTest is not present in production binaries. It intentionally
// leaves dirty7; operational rollback uses an isolated restored v6 database.
func (e Executor) DownEmptyOCRForTest(ctx context.Context, confirmed string) error {
	if confirmed != e.Database || !regexp.MustCompile(`^ocr_migration_[a-z0-9_]+$`).MatchString(confirmed) {
		return ErrState
	}
	c, err := e.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock(c)
	if e.Check(ctx, 7) != nil {
		return ErrState
	}
	for _, t := range ocrTables {
		var n int
		if e.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+t+"`").Scan(&n) != nil || n != 0 {
			return ErrState
		}
	}
	if _, err = e.DB.ExecContext(ctx, "UPDATE schema_migrations SET dirty=1 WHERE version=7"); err != nil {
		return ErrState
	}
	raw, err := files.ReadFile("sql/000007_ocr_revisions.down.sql")
	if err != nil {
		return ErrState
	}
	for _, q := range strings.Split(string(raw), ";") {
		if strings.TrimSpace(q) == "" {
			continue
		}
		if _, err = e.DB.ExecContext(ctx, q); err != nil {
			return ErrState
		}
	}
	return nil
}
