// Package ocrreview implements isolated candidate revision transactions (DI-SC-18).
// Production HTTP/worker and migration Target activation are separate steps.
package ocrreview

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
)

type Page struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}
type Review struct {
	BaseRevision int    `json:"base_revision"`
	Acknowledged bool   `json:"acknowledged"`
	Pages        []Page `json:"pages"`
}
type Receipt struct {
	DocumentID string `json:"document_id"`
	Revision   int    `json:"revision"`
	Status     string `json:"status"`
}
type Revision struct {
	DocumentID      string `json:"document_id"`
	OriginalVersion int    `json:"original_version"`
	Revision        int    `json:"revision"`
	Kind            string `json:"kind"`
	Body            string `json:"body"`
	Pages           []Page `json:"pages"`
}
type Citation struct {
	DocumentID string `json:"document_id"`
	Title      string `json:"title"`
	Revision   int    `json:"revision"`
	Page       int    `json:"page"`
	Start      int    `json:"char_start"`
	End        int    `json:"char_end"`
	Quote      string `json:"quote"`
}
type Store struct {
	DB   *sql.DB
	Hook func(string) error
}

func content(pages []Page) (string, error) {
	if len(pages) < 1 || len(pages) > 100 {
		return "", identity.ErrInvalid
	}
	text := make([]string, len(pages))
	totalBytes := 0
	for i, p := range pages {
		if p.Page != i+1 || !utf8.ValidString(p.Text) || len(p.Text) == 0 || len(p.Text) > 2097152 {
			return "", identity.ErrInvalid
		}
		totalBytes += len(p.Text)
		if i > 0 {
			totalBytes += 2
		}
		if totalBytes > 2097152 {
			return "", identity.ErrInvalid
		}
		for _, r := range p.Text {
			if unicode.IsControl(r) && r != '\n' && r != '\t' {
				return "", identity.ErrInvalid
			}
		}
		text[i] = p.Text
	}
	body := strings.Join(text, "\n\n")
	if len(body) > 2097152 {
		return "", identity.ErrInvalid
	}
	return body, nil
}
func failure(ctx context.Context, e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(e, context.DeadlineExceeded) {
		return identity.ErrTimeout
	}
	if errors.Is(e, sql.ErrNoRows) {
		return identity.ErrNotFound
	}
	var m *mysql.MySQLError
	if errors.As(e, &m) && m.Number == 1062 {
		return identity.ErrConflict
	}
	for _, known := range []error{identity.ErrInvalid, identity.ErrConflict, identity.ErrForbidden, identity.ErrNotFound, identity.ErrUnauthenticated} {
		if errors.Is(e, known) {
			return known
		}
	}
	return identity.ErrUnavailable
}
func (s *Store) transact(ctx context.Context, fn func(context.Context, *sql.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return failure(ctx, e)
	}
	defer tx.Rollback()
	if e = fn(ctx, tx); e != nil {
		return failure(ctx, e)
	}
	return failure(ctx, tx.Commit())
}

// seedDraft is an internal component boundary after engine validation. Full raw
// OCR word anchors remain the worker's responsibility, not teacher text fields.
func (s *Store) seedDraft(ctx context.Context, classID, id string, pages []Page) error {
	body, e := content(pages)
	if e != nil || !materials.ValidID(id) || !materials.ValidID(classID) {
		return identity.ErrInvalid
	}
	raw, _ := json.Marshal(pages)
	return s.transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var status string
		var pageCount int
		if e := tx.QueryRowContext(ctx, "SELECT status,page_count FROM documents WHERE id=? AND class_id=? FOR UPDATE", id, classID).Scan(&status, &pageCount); e != nil {
			return e
		}
		if status != "needs_ocr" {
			return identity.ErrConflict
		}
		if pageCount != len(pages) {
			return identity.ErrInvalid
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO document_ocr_revisions(document_id,class_id,revision,kind,body,pages,created_at) VALUES(?,?,1,'ocr',?,?,UTC_TIMESTAMP(3))", id, classID, body, raw); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, "INSERT INTO document_ocr_heads(document_id,class_id,latest_revision) VALUES(?,?,1)", id, classID)
		return e
	})
}

func (s *Store) Save(ctx context.Context, p identity.Principal, id, key string, input Review) (snapshot []byte, err error) {
	if e := materials.Writable(p); e != nil {
		return nil, e
	}
	if !materials.ValidID(p.UserID) || !materials.ValidID(p.ClassID) {
		return nil, identity.ErrUnauthenticated
	}
	body, e := content(input.Pages)
	if e != nil || !materials.ValidID(id) || !materials.ValidKey(key) || !input.Acknowledged || input.BaseRevision < 1 || input.BaseRevision > 2147483646 {
		return nil, identity.ErrInvalid
	}
	raw, _ := json.Marshal(input)
	digest := sha256.Sum256(append([]byte(id+"\x00"), raw...))
	pageJSON, _ := json.Marshal(input.Pages)
	err = s.transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var current int
		var oldPages []byte
		if e := tx.QueryRowContext(ctx, "SELECT h.latest_revision,r.pages FROM document_ocr_heads h JOIN document_ocr_revisions r ON r.document_id=h.document_id AND r.class_id=h.class_id AND r.revision=h.latest_revision WHERE h.document_id=? AND h.class_id=? FOR UPDATE", id, p.ClassID).Scan(&current, &oldPages); e != nil {
			return e
		}
		var priorDigest, priorBody []byte
		e := tx.QueryRowContext(ctx, "SELECT payload_digest,response_bytes FROM document_ocr_review_keys WHERE user_id=? AND class_id=? AND document_id=? AND idem_key=?", p.UserID, p.ClassID, id, key).Scan(&priorDigest, &priorBody)
		if e == nil {
			if string(priorDigest) != string(digest[:]) {
				return identity.ErrConflict
			}
			snapshot = priorBody
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if current != input.BaseRevision {
			return identity.ErrConflict
		}
		var old []Page
		if json.Unmarshal(oldPages, &old) != nil {
			return identity.ErrUnavailable
		}
		if len(old) != len(input.Pages) {
			return identity.ErrInvalid
		}
		next := current + 1
		if _, e = tx.ExecContext(ctx, "INSERT INTO document_ocr_revisions(document_id,class_id,revision,kind,body,pages,created_at) VALUES(?,?,?,'review',?,?,UTC_TIMESTAMP(3))", id, p.ClassID, next, body, pageJSON); e != nil {
			return e
		}
		offset, ordinal := 0, 0
		for i, page := range input.Pages {
			if i > 0 {
				offset += 2
			}
			runes := []rune(page.Text)
			// Queries contain at most MaxQuery codepoints. A MaxQuery-1
			// overlap ensures every legal match fits in a persisted chunk.
			for start := 0; start < len(runes); start += ChunkSize - MaxQuery + 1 {
				end := min(start+ChunkSize, len(runes))
				ordinal++
				if _, e = tx.ExecContext(ctx, "INSERT INTO document_ocr_chunks(document_id,class_id,revision,ordinal,page,char_start,char_end,quote) VALUES(?,?,?,?,?,?,?,?)", id, p.ClassID, next, ordinal, page.Page, offset+start, offset+end, string(runes[start:end])); e != nil {
					return e
				}
				if end == len(runes) {
					break
				}
			}
			offset += len(runes)
		}
		if s.Hook != nil {
			if e = s.Hook("after-index"); e != nil {
				return e
			}
		}
		if _, e = tx.ExecContext(ctx, "UPDATE document_ocr_heads SET latest_revision=?,published_revision=? WHERE document_id=? AND class_id=?", next, next, id, p.ClassID); e != nil {
			return e
		}
		snapshot, _ = json.Marshal(Receipt{id, next, "ready"})
		if _, e = tx.ExecContext(ctx, "INSERT INTO document_ocr_review_keys(user_id,class_id,idem_key,document_id,revision,payload_digest,response_bytes) VALUES(?,?,?,?,?,?,?)", p.UserID, p.ClassID, key, id, next, digest[:], snapshot); e != nil {
			return e
		}
		if s.Hook != nil {
			return s.Hook("before-commit")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *Store) Read(ctx context.Context, p identity.Principal, id string, revision int) (out Revision, err error) {
	if e := materials.Readable(p); e != nil {
		return out, e
	}
	if !materials.ValidID(p.UserID) || !materials.ValidID(p.ClassID) {
		return out, identity.ErrUnauthenticated
	}
	if !materials.ValidID(id) || revision < 1 {
		return out, identity.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var pages []byte
	query := "SELECT revision,kind,body,pages FROM document_ocr_revisions WHERE document_id=? AND class_id=? AND revision=?"
	if p.Role == "student" {
		query += " AND kind='review'"
	}
	e := s.DB.QueryRowContext(ctx, query, id, p.ClassID, revision).Scan(&out.Revision, &out.Kind, &out.Body, &pages)
	if e != nil {
		return Revision{}, failure(ctx, e)
	}
	out.DocumentID = id
	out.OriginalVersion = 1
	if (out.Kind == "ocr" && out.Revision != 1) || (out.Kind == "review" && out.Revision < 2) {
		return Revision{}, identity.ErrUnavailable
	}
	if json.Unmarshal(pages, &out.Pages) != nil {
		return Revision{}, identity.ErrUnavailable
	}
	body, e := content(out.Pages)
	if e != nil || body != out.Body {
		return Revision{}, identity.ErrUnavailable
	}
	return out, nil
}

// Search exposes only the published revision pointer in the trusted class.
// Old revisions remain readable by version but never pollute current results.
func (s *Store) Search(ctx context.Context, p identity.Principal, query string, limit int) (out []Citation, err error) {
	out = []Citation{}
	if e := materials.Readable(p); e != nil {
		return nil, e
	}
	if !materials.ValidID(p.UserID) || !materials.ValidID(p.ClassID) {
		return nil, identity.ErrUnauthenticated
	}
	query = strings.TrimSpace(query)
	if !materials.Text(query, 1, MaxQuery) || limit < 1 || limit > 20 {
		return nil, identity.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, e := s.DB.QueryContext(ctx, "SELECT c.document_id,d.title,c.revision,c.page,c.char_start,c.char_end,c.quote,r.body FROM document_ocr_chunks c JOIN document_ocr_heads h ON h.document_id=c.document_id AND h.class_id=c.class_id AND h.published_revision=c.revision JOIN document_ocr_revisions r ON r.document_id=c.document_id AND r.class_id=c.class_id AND r.revision=c.revision JOIN documents d ON d.id=c.document_id AND d.class_id=c.class_id WHERE c.class_id=? AND INSTR(c.quote,?)>0 ORDER BY c.document_id DESC,c.ordinal LIMIT ?", p.ClassID, query, limit)
	if e != nil {
		return nil, failure(ctx, e)
	}
	defer rows.Close()
	for rows.Next() {
		var c Citation
		var body string
		if e = rows.Scan(&c.DocumentID, &c.Title, &c.Revision, &c.Page, &c.Start, &c.End, &c.Quote, &body); e != nil {
			return nil, failure(ctx, e)
		}
		runes := []rune(body)
		if c.Start < 0 || c.End > len(runes) || c.Start >= c.End || string(runes[c.Start:c.End]) != c.Quote {
			return nil, identity.ErrUnavailable
		}
		out = append(out, c)
	}
	return out, failure(ctx, rows.Err())
}
