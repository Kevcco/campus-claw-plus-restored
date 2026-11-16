package ocrreview

import (
	"context"
	"database/sql"
	"errors"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
	"time"
)

type Recognize func(context.Context, []byte) ([]byte, string)

func failureReason(reason string) bool {
	switch reason {
	case "corrupt", "encrypted", "unsupported_content", "resource_limit", "ocr_timeout", "worker_interrupted", "language_missing", "internal_failure":
		return true
	}
	return false
}
func (s *Store) FailOCR(ctx context.Context, l Lease, reason string) error {
	if !validLease(l) || !failureReason(reason) {
		return identity.ErrInvalid
	}
	return s.transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		j, e := lockJob(ctx, tx, l.DocumentID, l.ClassID)
		if e != nil {
			return e
		}
		if !owns(j, l) {
			return identity.ErrConflict
		}
		r, e := tx.ExecContext(ctx, "UPDATE document_ocr_jobs SET state='failed',worker_owner=NULL,lease_until=NULL,failure_reason=? WHERE document_id=? AND class_id=? AND worker_owner=? AND execution_epoch=? AND lease_until>UTC_TIMESTAMP(3)", reason, l.DocumentID, l.ClassID, l.Owner, l.Epoch)
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return e
		}
		if n != 1 {
			return identity.ErrConflict
		}
		return nil
	})
}
func (s *Store) RetryOCR(ctx context.Context, p identity.Principal, id string) error {
	if e := materials.Writable(p); e != nil {
		return e
	}
	if !materials.ValidID(id) || !materials.ValidID(p.ClassID) || !materials.ValidID(p.UserID) {
		return identity.ErrInvalid
	}
	return s.transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		j, e := lockJob(ctx, tx, id, p.ClassID)
		if e != nil {
			return e
		}
		var reason sql.NullString
		if e = tx.QueryRowContext(ctx, "SELECT failure_reason FROM document_ocr_jobs WHERE document_id=? AND class_id=?", id, p.ClassID).Scan(&reason); e != nil {
			return e
		}
		if j.status != "needs_ocr" || j.state != "failed" || j.attempts >= 3 || (reason.String != "ocr_timeout" && reason.String != "worker_interrupted" && reason.String != "internal_failure") {
			return identity.ErrConflict
		}
		_, e = tx.ExecContext(ctx, "UPDATE document_ocr_jobs SET state='queued',failure_reason=NULL WHERE document_id=? AND class_id=?", id, p.ClassID)
		return e
	})
}
func (s *Store) executeOCR(ctx context.Context, w Work, recognize Recognize, heartbeat time.Duration) error {
	callCtx, cancel := context.WithTimeout(ctx, 190*time.Second)
	defer cancel()
	type answer struct {
		raw    []byte
		reason string
	}
	done := make(chan answer, 1)
	go func() { raw, reason := recognize(callCtx, w.Original); done <- answer{raw, reason} }()
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if e := s.HeartbeatOCR(ctx, w.Lease); e != nil {
				return e
			}
		case a := <-done:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if a.reason != "" {
				if !failureReason(a.reason) {
					a.reason = "internal_failure"
				}
				return s.FailOCR(ctx, w.Lease, a.reason)
			}
			if e := s.CommitOCR(ctx, w.Lease, a.raw); e != nil {
				if errors.Is(e, identity.ErrInvalid) {
					return s.FailOCR(ctx, w.Lease, "internal_failure")
				}
				return e
			}
			return nil
		case <-callCtx.Done():
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return s.FailOCR(ctx, w.Lease, "ocr_timeout")
		}
	}
}

// RunOCR is the sequential default worker implementation; composition must
// activate its migration/service/contracts before starting it in production.
func (s *Store) RunOCR(ctx context.Context, recognize Recognize) error {
	return s.runOCR(ctx, recognize, time.Second, 20*time.Second)
}
func (s *Store) runOCR(ctx context.Context, recognize Recognize, poll, heartbeat time.Duration) error {
	if recognize == nil || poll <= 0 || heartbeat <= 0 {
		return identity.ErrInvalid
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		ids, e := s.ReadyOCR(ctx)
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return e
		}
		for _, id := range ids {
			if ctx.Err() != nil {
				return nil
			}
			w, e := s.ClaimOCR(ctx, id[0], id[1])
			if errors.Is(e, identity.ErrConflict) || errors.Is(e, identity.ErrNotFound) {
				continue
			}
			if e != nil {
				return e
			}
			if e = s.executeOCR(ctx, w, recognize, heartbeat); e != nil && !errors.Is(e, identity.ErrConflict) {
				if ctx.Err() != nil {
					return nil
				}
				return e
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
