package mysqlstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/zwh9616/campus-claw-plus/backend/internal/documentqueue"
	"github.com/zwh9616/campus-claw-plus/backend/internal/documents"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
)

const documentColumns = "id,title,format,version,status,size_bytes,attempts,page_count,failure_reason,created_at,updated_at"

type documentQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func documentScan(row interface{ Scan(...any) error }) (d documents.Document, e error) {
	var created, updated time.Time
	e = row.Scan(&d.ID, &d.Title, &d.Format, &d.Version, &d.Status, &d.Size, &d.Attempts, &d.Pages, &d.Failure, &created, &updated)
	d.Created = created.UTC().Format("2006-01-02T15:04:05.000Z")
	d.Updated = updated.UTC().Format("2006-01-02T15:04:05.000Z")
	if e == nil && !documents.ValidDocument(d) {
		e = identity.ErrUnavailable
	}
	return
}
func documentError(ctx context.Context, e error) error {
	switch {
	case e == nil:
		return nil
	case errors.Is(e, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(e, documentqueue.ErrTimeout):
		return identity.ErrTimeout
	case errors.Is(e, documentqueue.ErrConflict) || errors.Is(e, documentqueue.ErrExhausted):
		return identity.ErrConflict
	case errors.Is(e, documentqueue.ErrInvalid):
		return identity.ErrInvalid
	case errors.Is(e, sql.ErrNoRows) || errors.Is(e, documentqueue.ErrLost):
		return identity.ErrNotFound
	default:
		return identity.ErrUnavailable
	}
}
func documentFilter(p identity.Principal) string {
	f := " WHERE class_id=?"
	if p.Role == "student" {
		f += " AND status='ready'"
	}
	return f
}
func visibleDocument(ctx context.Context, q documentQuery, p identity.Principal, id string) (out documents.Detail, e error) {
	out.Locations = []documents.Location{}
	row := q.QueryRowContext(ctx, "SELECT "+documentColumns+",body,original_bytes,original_digest FROM documents"+documentFilter(p)+" AND id=?", p.ClassID, id)
	var created, updated time.Time
	var digest []byte
	d := &out.Document
	e = row.Scan(&d.ID, &d.Title, &d.Format, &d.Version, &d.Status, &d.Size, &d.Attempts, &d.Pages, &d.Failure, &created, &updated, &out.Body, &out.Raw, &digest)
	if e != nil {
		return out, documentError(ctx, e)
	}
	d.Created = created.UTC().Format("2006-01-02T15:04:05.000Z")
	d.Updated = updated.UTC().Format("2006-01-02T15:04:05.000Z")
	hash := sha256.Sum256(out.Raw)
	if len(out.Raw) != d.Size || string(hash[:]) != string(digest) {
		return out, identity.ErrUnavailable
	}
	rows, e := q.QueryContext(ctx, "SELECT kind,unit,char_start,char_end FROM document_locations WHERE document_id=? AND class_id=? AND document_version=? ORDER BY unit", id, p.ClassID, d.Version)
	if e != nil {
		return out, documentError(ctx, e)
	}
	for rows.Next() {
		var loc documents.Location
		if e = rows.Scan(&loc.Kind, &loc.Unit, &loc.Start, &loc.End); e != nil {
			break
		}
		out.Locations = append(out.Locations, loc)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return out, documentError(ctx, e)
	}
	if !documents.ValidDetail(out) {
		return out, identity.ErrUnavailable
	}
	return out, nil
}
func (s *Store) GetDocument(ctx context.Context, p identity.Principal, id string) (out documents.Detail, e error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return out, documentError(ctx, e)
	}
	defer tx.Rollback()
	out, e = visibleDocument(ctx, tx, p, id)
	if e != nil {
		return out, e
	}
	return out, documentError(ctx, tx.Commit())
}
func (s *Store) ListDocuments(ctx context.Context, p identity.Principal, page, size int) (out documents.Page, e error) {
	out = documents.Page{Items: []documents.Document{}, Page: page, PageSize: size}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return out, documentError(ctx, e)
	}
	defer tx.Rollback()
	f := documentFilter(p)
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM documents"+f, p.ClassID).Scan(&out.Total); e != nil {
		return out, documentError(ctx, e)
	}
	if s.DocumentHook != nil {
		if e = s.DocumentHook("list-count"); e != nil {
			return out, documentError(ctx, e)
		}
	}
	rows, e := tx.QueryContext(ctx, "SELECT "+documentColumns+" FROM documents"+f+" ORDER BY id DESC LIMIT ? OFFSET ?", p.ClassID, size, (int64(page)-1)*int64(size))
	if e != nil {
		return out, documentError(ctx, e)
	}
	for rows.Next() {
		d, err := documentScan(rows)
		if err != nil {
			e = err
			break
		}
		out.Items = append(out.Items, d)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return out, documentError(ctx, e)
	}
	return out, documentError(ctx, tx.Commit())
}
func queueUpload(p identity.Principal, key string, u documents.Upload) (documentqueue.Upload, error) {
	user, e := strconv.ParseUint(p.UserID, 10, 63)
	if e != nil || user == 0 {
		return documentqueue.Upload{}, identity.ErrUnauthenticated
	}
	class, e := strconv.ParseUint(p.ClassID, 10, 63)
	if e != nil || class == 0 {
		return documentqueue.Upload{}, identity.ErrUnauthenticated
	}
	return documentqueue.Upload{UserID: user, ClassID: class, Key: key, Title: u.Title, Format: u.Format, Original: u.Raw}, nil
}
func (s *Store) ReplayDocument(ctx context.Context, p identity.Principal, key string, u documents.Upload) ([]byte, bool, error) {
	input, e := queueUpload(p, key, u)
	if e != nil {
		return nil, false, e
	}
	a, found, e := (&documentqueue.Store{DB: s.DB}).Replay(ctx, input)
	if e != nil {
		return nil, found, documentError(ctx, e)
	}
	if found && !documents.ValidSnapshot(a.Body, strconv.FormatUint(a.ID, 10), u) {
		return nil, true, identity.ErrUnavailable
	}
	return a.Body, found, nil
}
func (s *Store) CreateDocument(ctx context.Context, p identity.Principal, key string, u documents.Upload) ([]byte, error) {
	input, e := queueUpload(p, key, u)
	if e != nil {
		return nil, e
	}
	a, e := (&documentqueue.Store{DB: s.DB, Hook: s.DocumentHook}).Enqueue(ctx, input)
	if e != nil {
		return nil, documentError(ctx, e)
	}
	if !documents.ValidSnapshot(a.Body, strconv.FormatUint(a.ID, 10), u) {
		return nil, identity.ErrUnavailable
	}
	return a.Body, nil
}
func (s *Store) RetryDocument(ctx context.Context, p identity.Principal, id string) (documents.Document, error) {
	n, e := strconv.ParseUint(id, 10, 63)
	if e != nil {
		return documents.Document{}, identity.ErrInvalid
	}
	class, e := strconv.ParseUint(p.ClassID, 10, 63)
	if e != nil {
		return documents.Document{}, identity.ErrUnauthenticated
	}
	e = (&documentqueue.Store{DB: s.DB, Hook: s.DocumentHook}).Retry(ctx, n, class)
	if e != nil {
		return documents.Document{}, documentError(ctx, e)
	}
	out, e := s.GetDocument(ctx, p, id)
	return out.Document, e
}
func (s *Store) SearchDocuments(ctx context.Context, p identity.Principal, query string, top int) (out documents.Search, e error) {
	out = documents.Search{Mode: "keyword", Items: []documents.Citation{}}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return out, documentError(ctx, e)
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT c.document_id,c.id,c.document_version,d.title,l.kind,c.unit,c.ordinal,c.char_start,c.char_end,c.quote
 FROM document_chunks c JOIN documents d ON d.id=c.document_id
 LEFT JOIN document_locations l ON l.document_id=c.document_id AND l.class_id=c.class_id AND l.document_version=c.document_version AND l.unit=c.unit
 WHERE c.class_id=? AND d.class_id=? AND d.status='ready' AND INSTR(CAST(c.quote AS BINARY),CAST(? AS BINARY))>0
 ORDER BY d.id DESC,c.unit,c.ordinal,c.id LIMIT ?`, p.ClassID, p.ClassID, query, top)
	if e != nil {
		return out, documentError(ctx, e)
	}
	type candidate struct {
		citation documents.Citation
		ordinal  int
	}
	candidates := []candidate{}
	for rows.Next() {
		var c candidate
		var kind sql.NullString
		v := &c.citation
		if e = rows.Scan(&v.ID, &v.ChunkID, &v.Version, &v.Title, &kind, &v.Location.Unit, &c.ordinal, &v.Location.Start, &v.Location.End, &v.Quote); e != nil {
			break
		}
		if !kind.Valid {
			e = identity.ErrUnavailable
			break
		}
		v.Location.Kind = kind.String
		v.URL = "/api/v1/documents/" + v.ID
		candidates = append(candidates, c)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return out, documentError(ctx, e)
	}
	if s.DocumentHook != nil {
		if e = s.DocumentHook("search-candidates"); e != nil {
			return out, documentError(ctx, e)
		}
	}
	cache := map[string]documents.Detail{}
	for _, c := range candidates {
		v := c.citation
		d, ok := cache[v.ID]
		if !ok {
			d, e = visibleDocument(ctx, tx, p, v.ID)
			if e != nil {
				return out, e
			}
			cache[v.ID] = d
		}
		if !documents.CheckCitation(v, d, c.ordinal) {
			return out, identity.ErrUnavailable
		}
		out.Items = append(out.Items, v)
	}
	return out, documentError(ctx, tx.Commit())
}
