//go:build ocrreview

package ocrreview

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"os"
	"sync"
	"testing"
)

func db(t *testing.T) *sql.DB {
	t.Helper()
	password := os.Getenv("MYSQL_PWD")
	if password == "" {
		t.Fatal("isolated credential required")
	}
	d, e := sql.Open("mysql", "root:"+password+"@tcp(127.0.0.1:3306)/ocr_probe?parseTime=true&charset=utf8mb4&timeout=2s&readTimeout=5s&writeTimeout=5s")
	if e != nil {
		t.Fatal(e)
	}
	d.SetMaxOpenConns(25)
	t.Cleanup(func() { d.Close() })
	return d
}
func setup(t *testing.T) (*Store, identity.Principal, Review) {
	t.Helper()
	d := db(t)
	for _, table := range []string{"document_ocr_review_keys", "document_ocr_chunks", "document_ocr_heads", "document_ocr_revisions", "document_idempotency", "document_parse_jobs", "document_chunks", "document_locations", "documents"} {
		if _, e := d.Exec("DELETE FROM " + table); e != nil {
			t.Fatal(e)
		}
	}
	_, e := d.Exec("INSERT INTO documents(id,class_id,created_by,title,format,version,status,original_bytes,original_digest,size_bytes,attempts,page_count,created_at,updated_at) VALUES(91,1,1,'扫描','pdf',1,'needs_ocr','abc',UNHEX(SHA2('abc',256)),3,1,2,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))")
	if e != nil {
		t.Fatal(e)
	}
	s := &Store{DB: d}
	if e = s.SeedDraft(context.Background(), "1", "91", []Page{{1, "OCR原稿😀"}, {2, "第二页"}}); e != nil {
		t.Fatal(e)
	}
	return s, identity.Principal{UserID: "1", ClassID: "1", SessionID: "trusted", Role: "teacher"}, Review{1, true, []Page{{1, "教师校对😀"}, {2, "新第二页"}}}
}
func requireErr(t *testing.T, e, want error) {
	t.Helper()
	if !errors.Is(e, want) {
		t.Fatalf("got %v want %v", e, want)
	}
}
func TestMySQLReviewIdempotentRaceAndOldSource(t *testing.T) {
	s, p, in := setup(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	out := make(chan []byte, 20)
	fail := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); b, e := s.Save(ctx, p, "91", "same-review-key-0001", in); out <- b; fail <- e }()
	}
	wg.Wait()
	close(out)
	close(fail)
	for e := range fail {
		if e != nil {
			t.Fatal(e)
		}
	}
	for b := range out {
		if string(b) != `{"document_id":"91","revision":2,"status":"ready"}` {
			t.Fatal(string(b))
		}
	}
	for table, want := range map[string]int{"document_ocr_revisions": 2, "document_ocr_review_keys": 1, "document_ocr_chunks": 2} {
		var n int
		if e := s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != want {
			t.Fatal(table, n, e)
		}
	}
	in.Pages[0].Text = "changed"
	_, e := s.Save(ctx, p, "91", "same-review-key-0001", in)
	requireErr(t, e, identity.ErrConflict)
	old, e := s.Read(ctx, p, "91", 1)
	if e != nil || old.Body != "OCR原稿😀\n\n第二页" {
		t.Fatal(old, e)
	}
	var version int
	var original string
	var body sql.NullString
	if e = s.DB.QueryRow("SELECT version,original_bytes,body FROM documents WHERE id=91").Scan(&version, &original, &body); e != nil || version != 1 || original != "abc" || body.Valid {
		t.Fatal("original changed", e)
	}
	var quote string
	var start, end int
	if e = s.DB.QueryRow("SELECT quote,char_start,char_end FROM document_ocr_chunks WHERE page=1").Scan(&quote, &start, &end); e != nil || quote != "教师校对😀" || end-start != 5 {
		t.Fatal("bad quote", quote, start, end, e)
	}
}
func TestMySQLReviewCASRaceAndAuthorization(t *testing.T) {
	s, p, in := setup(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := s.Save(ctx, p, "91", fmt.Sprintf("different-key-%04d", i), in)
			errs <- e
		}(i)
	}
	wg.Wait()
	close(errs)
	success, conflicts := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else if errors.Is(e, identity.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal(success, conflicts)
	}
	other := p
	other.ClassID = "2"
	_, e := s.Save(ctx, other, "91", "cross-class-key-001", in)
	requireErr(t, e, identity.ErrNotFound)
	_, e = s.Read(ctx, other, "91", 2)
	requireErr(t, e, identity.ErrNotFound)
	student := p
	student.Role = "student"
	_, e = s.Save(ctx, student, "91", "student-write-key01", in)
	requireErr(t, e, identity.ErrForbidden)
	_, e = s.Read(ctx, student, "91", 1)
	requireErr(t, e, identity.ErrNotFound)
	if _, e = s.Read(ctx, student, "91", 2); e != nil {
		t.Fatal(e)
	}
	in.BaseRevision = 2
	in.Pages[0].Text = "校对第三版"
	if _, e = s.Save(ctx, p, "91", "another-review-0001", in); e != nil {
		t.Fatal(e)
	}
	old, e := s.Read(ctx, student, "91", 2)
	if e != nil || old.Pages[0].Text != "教师校对😀" {
		t.Fatal("old published source drift", e)
	}
	// Replay the original request after another revision was published.
	in.BaseRevision = 1
	in.Pages = []Page{{1, "教师校对😀"}, {2, "新第二页"}}
	var key string
	if e = s.DB.QueryRow("SELECT idem_key FROM document_ocr_review_keys WHERE revision=2").Scan(&key); e != nil {
		t.Fatal(e)
	}
	if b, e := s.Save(ctx, p, "91", key, in); e != nil || string(b) != `{"document_id":"91","revision":2,"status":"ready"}` {
		t.Fatal("replay changed", e, string(b))
	}
}
func TestMySQLReviewRollbackAndPageCount(t *testing.T) {
	s, p, in := setup(t)
	for _, stage := range []string{"after-index", "before-commit"} {
		s.Hook = func(got string) error {
			if got == stage {
				return errors.New("injected storage boundary")
			}
			return nil
		}
		_, e := s.Save(context.Background(), p, "91", "rollback-review-001", in)
		requireErr(t, e, identity.ErrUnavailable)
		var rev int
		var pub sql.NullInt64
		if e = s.DB.QueryRow("SELECT latest_revision,published_revision FROM document_ocr_heads").Scan(&rev, &pub); e != nil {
			t.Fatal(e)
		}
		if rev != 1 || pub.Valid {
			t.Fatal("pointer published after rollback")
		}
		for _, table := range []string{"document_ocr_review_keys", "document_ocr_chunks"} {
			var n int
			if e = s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil {
				t.Fatal(e)
			}
			if n != 0 {
				t.Fatal("partial rows", table)
			}
		}
	}
	s.Hook = nil
	bad := in
	bad.Pages = []Page{{1, "missing page"}}
	_, e := s.Save(context.Background(), p, "91", "page-count-key-001", bad)
	requireErr(t, e, identity.ErrInvalid)
	if _, e = s.Save(context.Background(), p, "91", "rollback-review-001", in); e != nil {
		t.Fatal(e)
	}
}
func TestMySQLReviewForeignClassAndRevisionBounds(t *testing.T) {
	s, _, _ := setup(t)
	for _, q := range []string{"INSERT INTO document_ocr_revisions(document_id,class_id,revision,kind,body,pages,created_at) VALUES(91,2,2,'review','a',JSON_ARRAY(JSON_OBJECT('page',1,'text','a')),UTC_TIMESTAMP(3))", "UPDATE document_ocr_revisions SET revision=0", "UPDATE document_ocr_revisions SET original_version=2", "UPDATE document_ocr_heads SET published_revision=999"} {
		if _, e := s.DB.Exec(q); e == nil {
			t.Fatal("constraint allowed", q)
		}
	}
}
func TestMySQLReviewPublishedKeywordIsolation(t *testing.T) {
	s, p, in := setup(t)
	ctx := context.Background()
	student := p
	student.Role = "student"
	if found, e := s.Search(ctx, student, "原稿", 20); e != nil || len(found) != 0 {
		t.Fatal("draft leaked", found, e)
	}
	if _, e := s.Save(ctx, p, "91", "publish-search-0001", in); e != nil {
		t.Fatal(e)
	}
	found, e := s.Search(ctx, student, "教师", 20)
	if e != nil || len(found) != 1 || found[0].Revision != 2 || found[0].Quote != "教师校对😀" {
		t.Fatal(found, e)
	}
	other := student
	other.ClassID = "2"
	if found, e = s.Search(ctx, other, "教师", 20); e != nil || len(found) != 0 {
		t.Fatal("cross-class search", e)
	}
	in.BaseRevision = 2
	in.Pages[0].Text = "新版正文"
	if _, e = s.Save(ctx, p, "91", "publish-search-0002", in); e != nil {
		t.Fatal(e)
	}
	if found, e = s.Search(ctx, student, "教师", 20); e != nil || len(found) != 0 {
		t.Fatal("old search index leaked", e)
	}
	if found, e = s.Search(ctx, student, "新版", 20); e != nil || len(found) != 1 || found[0].Revision != 3 {
		t.Fatal(found, e)
	}
	if found, e = s.Search(ctx, student, "' OR 1=1 --", 20); e != nil || len(found) != 0 {
		t.Fatal("query injection", e)
	}
}
