package ocrreview

import (
	"context"
	"database/sql"
	"time"

	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
)

const MaxQuery = 2000
const ChunkSize = 4096
const MaxBody = 2097152

type Status struct {
	DocumentID        string  `json:"document_id"`
	Title             string  `json:"title"`
	Format            string  `json:"format"`
	OriginalVersion   int     `json:"original_version"`
	OriginalStatus    string  `json:"original_status"`
	PageCount         *int    `json:"page_count"`
	Status            string  `json:"status"`
	LatestRevision    *int    `json:"latest_revision"`
	PublishedRevision *int    `json:"published_revision"`
	Attempts          int     `json:"attempts"`
	FailureReason     *string `json:"failure_reason"`
	CanStart          bool    `json:"can_start"`
	CanRetry          bool    `json:"can_retry"`
}

// Status reads one consistent row; a student cannot discover unreviewed work.
func (s *Store) Status(ctx context.Context, p identity.Principal, id string) (out Status, err error) {
	if e := materials.Readable(p); e != nil {
		return out, e
	}
	if !materials.ValidID(p.UserID) || !materials.ValidID(p.ClassID) {
		return out, identity.ErrUnauthenticated
	}
	if !materials.ValidID(id) {
		return out, identity.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var pages, latest, published, attempts sql.NullInt64
	var state, reason sql.NullString
	query := "SELECT d.title,d.format,d.status,d.page_count,h.latest_revision,h.published_revision,j.state,j.attempts,j.failure_reason FROM documents d LEFT JOIN document_ocr_heads h ON h.document_id=d.id AND h.class_id=d.class_id LEFT JOIN document_ocr_jobs j ON j.document_id=d.id AND j.class_id=d.class_id WHERE d.id=? AND d.class_id=?"
	if p.Role == "student" {
		query += " AND h.published_revision IS NOT NULL"
	}
	e := s.DB.QueryRowContext(ctx, query, id, p.ClassID).Scan(&out.Title, &out.Format, &out.OriginalStatus, &pages, &latest, &published, &state, &attempts, &reason)
	if e != nil {
		return Status{}, failure(ctx, e)
	}
	ptr := func(n sql.NullInt64) *int {
		if !n.Valid {
			return nil
		}
		v := int(n.Int64)
		return &v
	}
	out.DocumentID, out.OriginalVersion, out.PageCount = id, 1, ptr(pages)
	out.LatestRevision, out.PublishedRevision, out.Attempts = ptr(latest), ptr(published), int(attempts.Int64)
	out.Status = "not_required"
	if out.OriginalStatus == "needs_ocr" {
		out.Status = "idle"
	}
	if state.Valid {
		out.Status = state.String
	}
	if latest.Valid {
		out.Status = "needs_review"
	}
	if published.Valid {
		if published.Int64 < 2 || !latest.Valid || published.Int64 > latest.Int64 {
			return Status{}, identity.ErrUnavailable
		}
		out.Status = "ready"
	}
	if reason.Valid {
		if !failureReason(reason.String) {
			return Status{}, identity.ErrUnavailable
		}
		out.FailureReason = &reason.String
	}
	if out.Status == "succeeded" {
		return Status{}, identity.ErrUnavailable
	}
	if out.Status == "failed" && (out.FailureReason == nil || out.Attempts < 1) {
		return Status{}, identity.ErrUnavailable
	}
	out.CanStart = p.Role == "teacher" && out.Status == "idle"
	out.CanRetry = p.Role == "teacher" && out.OriginalStatus == "needs_ocr" && out.Status == "failed" && out.Attempts < 3 && (reason.String == "ocr_timeout" || reason.String == "worker_interrupted" || reason.String == "internal_failure")
	return out, nil
}
