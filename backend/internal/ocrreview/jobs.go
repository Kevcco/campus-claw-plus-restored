package ocrreview

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
	"time"
)

type Lease struct {
	DocumentID, ClassID, Owner string
	Epoch                      uint64
	OriginalDigest             [32]byte
}
type Work struct {
	Lease
	Original []byte
}
type jobState struct {
	status, state               string
	pages, attempts             int
	epoch                       uint64
	owner                       sql.NullString
	live                        bool
	original, digest, jobDigest []byte
}

func lockJob(ctx context.Context, tx *sql.Tx, id, class string) (j jobState, e error) {
	e = tx.QueryRowContext(ctx, "SELECT status,page_count,original_bytes,original_digest FROM documents WHERE id=? AND class_id=? FOR UPDATE", id, class).Scan(&j.status, &j.pages, &j.original, &j.digest)
	if e != nil {
		return
	}
	e = tx.QueryRowContext(ctx, "SELECT state,attempts,execution_epoch,worker_owner,COALESCE(lease_until>UTC_TIMESTAMP(3),FALSE),original_digest FROM document_ocr_jobs WHERE document_id=? AND class_id=? FOR UPDATE", id, class).Scan(&j.state, &j.attempts, &j.epoch, &j.owner, &j.live, &j.jobDigest)
	return
}
func validLease(l Lease) bool {
	if !materials.ValidID(l.DocumentID) || !materials.ValidID(l.ClassID) || len(l.Owner) != 32 || l.Epoch == 0 {
		return false
	}
	_, e := hex.DecodeString(l.Owner)
	return e == nil
}
func owns(j jobState, l Lease) bool {
	return j.status == "needs_ocr" && j.state == "processing" && j.live && j.owner.String == l.Owner && j.epoch == l.Epoch && string(j.jobDigest) == string(l.OriginalDigest[:]) && string(j.digest) == string(j.jobDigest)
}

func (s *Store) EnqueueOCR(ctx context.Context, p identity.Principal, id string) error {
	if e := materials.Writable(p); e != nil {
		return e
	}
	if !materials.ValidID(id) || !materials.ValidID(p.ClassID) || !materials.ValidID(p.UserID) {
		return identity.ErrInvalid
	}
	return s.transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var status string
		var digest []byte
		if e := tx.QueryRowContext(ctx, "SELECT status,original_digest FROM documents WHERE id=? AND class_id=? FOR UPDATE", id, p.ClassID).Scan(&status, &digest); e != nil {
			return e
		}
		if status != "needs_ocr" {
			return identity.ErrConflict
		}
		var state string
		e := tx.QueryRowContext(ctx, "SELECT state FROM document_ocr_jobs WHERE document_id=? AND class_id=?", id, p.ClassID).Scan(&state)
		if e == nil {
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO document_ocr_jobs(document_id,class_id,state,original_digest) VALUES(?,?,'queued',?)", id, p.ClassID, digest)
		return e
	})
}
func (s *Store) ClaimOCR(ctx context.Context, id, class string) (out Work, err error) {
	if !materials.ValidID(id) || !materials.ValidID(class) {
		return out, identity.ErrInvalid
	}
	exhausted := false
	err = s.transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		j, e := lockJob(ctx, tx, id, class)
		if e != nil {
			return e
		}
		if j.status != "needs_ocr" || (j.state != "queued" && !(j.state == "processing" && !j.live)) {
			return identity.ErrConflict
		}
		if j.attempts >= 3 {
			_, e = tx.ExecContext(ctx, "UPDATE document_ocr_jobs SET state='failed',worker_owner=NULL,lease_until=NULL WHERE document_id=? AND class_id=?", id, class)
			exhausted = true
			return e
		}
		hash := sha256.Sum256(j.original)
		if string(hash[:]) != string(j.digest) || string(j.digest) != string(j.jobDigest) {
			return identity.ErrUnavailable
		}
		var token [16]byte
		if _, e = rand.Read(token[:]); e != nil {
			return identity.ErrUnavailable
		}
		owner := hex.EncodeToString(token[:])
		if _, e = tx.ExecContext(ctx, "UPDATE document_ocr_jobs SET state='processing',attempts=attempts+1,execution_epoch=execution_epoch+1,worker_owner=?,lease_until=TIMESTAMPADD(SECOND,60,UTC_TIMESTAMP(3)) WHERE document_id=? AND class_id=?", owner, id, class); e != nil {
			return e
		}
		out = Work{Lease{id, class, owner, j.epoch + 1, hash}, j.original}
		return nil
	})
	if err != nil {
		return Work{}, err
	}
	if exhausted {
		return Work{}, identity.ErrConflict
	}
	return out, nil
}
func (s *Store) HeartbeatOCR(ctx context.Context, l Lease) error {
	if !validLease(l) {
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
		result, e := tx.ExecContext(ctx, "UPDATE document_ocr_jobs SET lease_until=TIMESTAMPADD(SECOND,60,UTC_TIMESTAMP(3)) WHERE document_id=? AND class_id=? AND worker_owner=? AND execution_epoch=? AND lease_until>UTC_TIMESTAMP(3)", l.DocumentID, l.ClassID, l.Owner, l.Epoch)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if n != 1 {
			return identity.ErrConflict
		}
		return nil
	})
}
func (s *Store) CommitOCR(ctx context.Context, l Lease, raw []byte) error {
	if !validLease(l) {
		return identity.ErrInvalid
	}
	decoded, pages, e := DecodeSource(raw)
	if e != nil {
		return e
	}
	return s.transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		j, e := lockJob(ctx, tx, l.DocumentID, l.ClassID)
		if e != nil {
			return e
		}
		if !owns(j, l) {
			return identity.ErrConflict
		}
		if len(pages) != j.pages {
			return identity.ErrInvalid
		}
		if e = insertSource(ctx, tx, l.DocumentID, l.ClassID, raw, j.digest, decoded, pages); e != nil {
			return e
		}
		if s.Hook != nil {
			if e = s.Hook("ocr-source"); e != nil {
				return e
			}
		}
		// Recheck against DB time at the final guarded update: a transaction can
		// hold the lock past lease expiry and must roll its inserted source back.
		result, e := tx.ExecContext(ctx, "UPDATE document_ocr_jobs SET state='succeeded',worker_owner=NULL,lease_until=NULL WHERE document_id=? AND class_id=? AND state='processing' AND worker_owner=? AND execution_epoch=? AND lease_until>UTC_TIMESTAMP(3)", l.DocumentID, l.ClassID, l.Owner, l.Epoch)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if n != 1 {
			return identity.ErrConflict
		}
		return nil
	})
}
func (s *Store) ReadyOCR(ctx context.Context) (out [][2]string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, e := s.DB.QueryContext(ctx, "SELECT document_id,class_id FROM document_ocr_jobs WHERE state='queued' OR (state='processing' AND lease_until<=UTC_TIMESTAMP(3)) ORDER BY CASE WHEN state='processing' THEN 0 ELSE 1 END,document_id LIMIT 20")
	if e != nil {
		return nil, failure(ctx, e)
	}
	defer rows.Close()
	for rows.Next() {
		var id, class string
		if e = rows.Scan(&id, &class); e != nil {
			return nil, failure(ctx, e)
		}
		out = append(out, [2]string{id, class})
	}
	return out, failure(ctx, rows.Err())
}
