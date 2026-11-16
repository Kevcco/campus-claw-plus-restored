//go:build integration

package integration

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zwh9616/campus-claw-plus/backend/internal/config"
	"github.com/zwh9616/campus-claw-plus/backend/internal/documentqueue"
	"github.com/zwh9616/campus-claw-plus/backend/internal/documents"
	"github.com/zwh9616/campus-claw-plus/backend/internal/httpserver"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
	"github.com/zwh9616/campus-claw-plus/backend/internal/migrations"
	"github.com/zwh9616/campus-claw-plus/backend/internal/mysqlstore"
)

// These real TLS/MySQL tests deliberately use a controlled parser availability
// callback/result to isolate public HTTP/SQL rules. Production parser extraction
// is separately required by TestDocumentHTTPProduction on default Compose.
type dhFixture struct {
	t           *testing.T
	db, app     *sql.DB
	repo        *mysqlstore.Store
	server      *httptest.Server
	auth        *identity.Service
	actors      map[string]identity.Issued
	minute      atomic.Int64
	available   atomic.Bool
	healthCalls atomic.Int64
}

func dhSetup(t *testing.T, index int) *dhFixture {
	app, db, name := database(t, index)
	ctx := context.Background()
	if (migrations.Executor{DB: db, Database: name}).Up(ctx, 6) != nil || (&mysqlstore.Store{DB: db}).Initialize(ctx, seed(t), nil) != nil {
		t.Fatal("document fixture migrate/initialize")
	}
	f := &dhFixture{t: t, db: db, app: app, repo: &mysqlstore.Store{DB: app}, actors: map[string]identity.Issued{}}
	f.available.Store(true)
	f.minute.Store(time.Now().Unix() / 60)
	auth, e := identity.New(f.repo, nil, "https://campus.test")
	if e != nil {
		t.Fatal("document auth")
	}
	f.auth = auth
	for _, user := range []string{"teacher-a", "teacher-b", "student-a", "student-a2", "student-b"} {
		s, e := auth.Login(ctx, user, "test-only-"+user, strings.Repeat("a", 32))
		if e != nil {
			t.Fatal("fixture login")
		}
		f.actors[user] = s
	}
	limiter := materials.NewLimiter(func() time.Time { return time.Unix(f.minute.Load()*60, 0) })
	service := &documents.Service{Repo: f.repo, Healthy: func(context.Context) bool { f.healthCalls.Add(1); return f.available.Load() }}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	base := httpserver.IdentityHandler(logger, auth, f.repo, config.Identity{}, identity.NewLimiter(nil))
	base = httpserver.MaterialsHandler(logger, auth, &materials.Service{Repo: f.repo}, "https://campus.test", limiter, base)
	f.server = httptest.NewTLSServer(httpserver.DocumentsHandler(logger, auth, service, "https://campus.test", limiter, base))
	t.Cleanup(f.server.Close)
	return f
}
func (f *dhFixture) call(method, path, actor, ctype, key string, body []byte) ([]byte, http.Header, int) {
	f.t.Helper()
	req, _ := http.NewRequest(method, f.server.URL+path, bytes.NewReader(body))
	if actor != "" {
		req.Header.Set("Authorization", "Bearer "+f.actors[actor].Body.AccessToken)
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	req.Header.Set("Origin", "https://campus.test")
	req.Header.Set("X-Class-ID", f.actors["teacher-a"].Body.Principal.ClassID)
	req.Header.Set("X-Role", "teacher")
	res, e := f.server.Client().Do(req)
	if e != nil {
		f.t.Fatal("public trusted HTTPS transport")
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, 16*1024*1024))
	if e != nil {
		f.t.Fatal("public response read")
	}
	if res.Header.Get("Cache-Control") != "no-store" || len(res.Header.Get("X-Request-ID")) != 32 {
		f.t.Fatal("public cache/request-id")
	}
	if method == "POST" && res.StatusCode == 202 && strings.HasPrefix(path, "/api/v1/documents") {
		var accepted documents.Document
		if json.Unmarshal(raw, &accepted) != nil || res.Header.Get("Location") != "/api/v1/documents/"+accepted.ID {
			f.t.Fatal("DI-SC-07/10 actual upload, original replay and retry Location")
		}
	}
	return raw, res.Header, res.StatusCode
}
func (f *dhFixture) want(method, path, actor, ctype, key string, body []byte, want int) []byte {
	f.t.Helper()
	raw, _, status := f.call(method, path, actor, ctype, key, body)
	if status != want {
		f.t.Fatalf("public %s %s status%d want%d", method, path, status, want)
	}
	return raw
}
func (f *dhFixture) upload(actor, key, name string, raw []byte, want int) (documents.Document, []byte) {
	body, ctype := mrMultipart(f.t, "课程", name, raw)
	response := f.want("POST", "/api/v1/documents", actor, ctype, key, body, want)
	var d documents.Document
	if want == 202 && (json.Unmarshal(response, &d) != nil || !documents.ValidDocument(d)) {
		f.t.Fatal("invalid202 contract")
	}
	return d, response
}
func (f *dhFixture) finish(d documents.Document) {
	f.t.Helper()
	id, _ := strconvID(d.ID)
	class, _ := strconvID(f.actors["teacher-a"].Body.Principal.ClassID)
	q := &documentqueue.Store{DB: f.app}
	owner, e := documentqueue.NewOwner()
	if e != nil {
		f.t.Fatal("owner")
	}
	work, e := q.Claim(context.Background(), id, owner)
	if e != nil || work.ClassID != class {
		f.t.Fatal("claim")
	}
	pages := 2
	r := documentqueue.Result{Status: "ready", Body: "甲😀乙\n\nneedle%_𠮷", Pages: &pages, Locations: []documentqueue.Location{{Kind: "pdf_page", Unit: 1, Start: 0, End: 3}, {Kind: "pdf_page", Unit: 2, Start: 5, End: 14}}}
	if d.Format == "docx" {
		r.Pages = nil
		r.Locations[0].Kind = "docx_paragraph"
		r.Locations[1].Kind = "docx_table_cell"
	}
	if e = q.Finish(context.Background(), work, r); e != nil {
		f.t.Fatal("controlled typed result", e)
	}
}
func strconvID(id string) (uint64, error) { var v uint64; _, e := fmt.Sscan(id, &v); return v, e }
func TestDHExplicitRetryStopsAtThreeAndPreservesOriginal202(t *testing.T) {
	f := dhSetup(t, 69)
	d, original := f.upload("teacher-a", "public_retry_attempts", "a.pdf", []byte("%PDF-retry"), 202)
	q := &documentqueue.Store{DB: f.app}
	id, _ := strconvID(d.ID)
	for attempt := 1; attempt <= 3; attempt++ {
		owner, _ := documentqueue.NewOwner()
		work, e := q.Claim(context.Background(), id, owner)
		if e != nil {
			t.Fatal("retry fixture claim", e)
		}
		if e = q.Finish(context.Background(), work, documentqueue.Result{Status: "failed", Failure: "parse_timeout"}); e != nil {
			t.Fatal("retry fixture failure", e)
		}
		f.want("GET", "/api/v1/documents/"+d.ID, "student-a", "", "", nil, 404)
		var got documents.Document
		if attempt < 3 {
			raw := f.want("POST", "/api/v1/documents/"+d.ID+"/retry", "teacher-a", "", "", nil, 202)
			if json.Unmarshal(raw, &got) != nil || got.Status != "queued" || got.Attempts != attempt || !documents.ValidDocument(got) {
				t.Fatal("explicit retry response")
			}
		} else {
			f.want("POST", "/api/v1/documents/"+d.ID+"/retry", "teacher-a", "", "", nil, 409)
		}
	}
	_, replay := f.upload("teacher-a", "public_retry_attempts", "a.pdf", []byte("%PDF-retry"), 202)
	if !bytes.Equal(original, replay) || count(t, f.app, "documents") != 1 || count(t, f.app, "document_parse_jobs") != 1 {
		t.Fatal("retry changed original acceptance or duplicated job")
	}
}
func TestDHPublicAuthorizationOriginalsAndCitations(t *testing.T) {
	f := dhSetup(t, 65)
	raw := []byte("%PDF-controlled-public-boundary")
	d, original := f.upload("teacher-a", "public_upload_key_0001", "a.pdf", raw, 202)
	f.want("GET", "/api/v1/documents/"+d.ID, "student-a", "", "", nil, 404)
	f.want("GET", "/api/v1/documents/"+d.ID, "teacher-b", "", "", nil, 404)
	f.want("GET", "/api/v1/documents/"+d.ID, "", "", "", nil, 401)
	f.upload("student-a", "public_student_key_01", "a.pdf", raw, 403)
	f.finish(d)
	for _, user := range []string{"teacher-a", "student-a", "student-a2"} {
		response := f.want("GET", "/api/v1/documents/"+d.ID, user, "", "", nil, 200)
		var detail documents.Detail
		if json.Unmarshal(response, &detail) != nil || !documents.ValidDetail(detail) {
			t.Fatal("detail source")
		}
		downloaded, headers, status := f.call("GET", "/api/v1/documents/"+d.ID+"/download", user, "", "", nil)
		if status != 200 || !bytes.Equal(downloaded, raw) || headers.Get("Content-Disposition") != `attachment; filename="document-`+d.ID+`.pdf"` || headers.Get("X-Content-Type-Options") != "nosniff" || headers.Get("Content-Type") != "application/pdf" {
			t.Fatal("authorized exact original")
		}
		search := f.want("GET", "/api/v1/document-search?q="+url.QueryEscape("needle%_"), user, "", "", nil, 200)
		var result documents.Search
		if json.Unmarshal(search, &result) != nil || len(result.Items) != 1 || !documents.CheckCitation(result.Items[0], detail, 1) {
			t.Fatal("authoritative literal keyword citation")
		}
	}
	f.want("GET", "/api/v1/documents/"+d.ID+"/download", "student-b", "", "", nil, 404)
	out := f.want("GET", "/api/v1/document-search?q=needle", "student-b", "", "", nil, 200)
	var empty documents.Search
	if json.Unmarshal(out, &empty) != nil || len(empty.Items) != 0 {
		t.Fatal("cross-class leak")
	}
	f.available.Store(false)
	before := f.healthCalls.Load()
	_, replay := f.upload("teacher-a", "public_upload_key_0001", "a.pdf", raw, 202)
	if !bytes.Equal(original, replay) || f.healthCalls.Load() != before {
		t.Fatal("original202 must replay before outage preflight")
	}
	f.upload("teacher-a", "public_outage_key_01", "b.pdf", raw, 503)
	if count(t, f.app, "documents") != 1 || count(t, f.app, "document_parse_jobs") != 1 {
		t.Fatal("outage created partial business data")
	}
	for _, path := range []string{"/api/v1/documents?page=0", "/api/v1/documents?page=1&page=1", "/api/v1/documents?class_id=1", "/api/v1/document-search?q=", "/api/v1/document-search?q=x&mode=hybrid", "/api/v1/document-search?q=x&top_k=11"} {
		f.want("GET", path, "teacher-a", "", "", nil, 400)
	}
	f.want("GET", "/api/v1/documents/"+d.ID+"?unknown=1", "student-b", "", "", nil, 404)
	f.want("POST", "/api/v1/documents/"+d.ID+"/retry", "student-a", "", "", nil, 403)
	f.want("POST", "/api/v1/documents/"+d.ID+"/retry", "teacher-a", "", "", nil, 409)
	for _, mutation := range []string{"UPDATE document_chunks SET quote='needle%_X' WHERE unit=2", "UPDATE documents SET original_bytes=REPEAT('x',size_bytes)"} {
		tx, e := f.db.Begin()
		if e != nil {
			t.Fatal("corrupt fixture tx")
		}
		if _, e = tx.Exec(mutation); e != nil {
			t.Fatal("corrupt fixture mutation")
		}
		tx.Commit()
		if strings.Contains(mutation, "chunks") {
			f.want("GET", "/api/v1/document-search?q=needle", "teacher-a", "", "", nil, 503)
			atExec(t, f.db, "UPDATE document_chunks SET quote='needle%_𠮷' WHERE unit=2")
		} else {
			f.want("GET", "/api/v1/documents/"+d.ID, "teacher-a", "", "", nil, 503)
			f.want("GET", "/api/v1/documents/"+d.ID+"/download", "teacher-a", "", "", nil, 503)
		}
	}
}
func TestDHConcurrentIdempotencySharedQuotaAndRollback(t *testing.T) {
	f := dhSetup(t, 66)
	body, ctype := mrMultipart(t, "课程", "a.pdf", []byte("%PDF-concurrent"))
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted, limited := 0, 0
	var original []byte
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw, _, status := f.call("POST", "/api/v1/documents", "teacher-a", ctype, "concurrent_public_key", body)
			mu.Lock()
			defer mu.Unlock()
			switch status {
			case 202:
				accepted++
				if original == nil {
					original = raw
				} else if !bytes.Equal(raw, original) {
					t.Error("different original202")
				}
			case 429:
				limited++
			default:
				t.Errorf("concurrent status%d", status)
			}
		}()
	}
	wg.Wait()
	if accepted != 12 || limited != 8 || count(t, f.app, "documents") != 1 || count(t, f.app, "document_parse_jobs") != 1 || count(t, f.app, "document_idempotency") != 1 {
		t.Fatal("20 contenders must preserve shared12/minute quota and one transaction")
	}
	f.minute.Add(1)
	f.available.Store(false)
	f.upload("teacher-a", "concurrent_public_key", "a.pdf", []byte("%PDF-different"), 409)
	f.minute.Add(1)
	for i := 0; i < 12; i++ {
		if i%2 == 0 {
			f.upload("teacher-a", fmt.Sprintf("quota_document_%03d", i), "a.pdf", []byte("%PDF-quota"), 503)
		} else {
			b, c := mrMultipart(t, "text", "a.txt", []byte("text"))
			f.want("POST", "/api/v1/materials", "teacher-a", c, fmt.Sprintf("quota_material_%03d", i), b, 201)
		}
	}
	_, h, status := f.call("POST", "/api/v1/documents", "teacher-a", ctype, "quota_exhausted_key", body)
	if status != 429 || h.Get("Retry-After") == "" {
		t.Fatal("TXT and Document must share one per-user quota")
	}
	f.minute.Add(1)
	f.available.Store(true)
	for _, stage := range []string{"enqueue-document", "enqueue-job", "enqueue-snapshot"} {
		f.repo.DocumentHook = func(s string) error {
			if s == stage {
				return errors.New("controlled write failure")
			}
			return nil
		}
		f.upload("teacher-a", "rollback_public_"+stage, "a.pdf", []byte("%PDF-rollback"), 503)
		if count(t, f.app, "documents") != 1 || count(t, f.app, "document_idempotency") != 1 || count(t, f.app, "document_parse_jobs") != 1 {
			t.Fatal("partial transaction escaped rollback")
		}
	}
}
func TestDHSnapshotConsistencyAndStrictInput(t *testing.T) {
	f := dhSetup(t, 67)
	d, _ := f.upload("teacher-a", "snapshot_document_01", "a.docx", []byte("PK\x03\x04public"), 202)
	f.finish(d)
	var changed atomic.Bool
	f.repo.DocumentHook = func(stage string) error {
		if stage == "search-candidates" && changed.CompareAndSwap(false, true) {
			tx, e := f.db.Begin()
			if e != nil {
				return e
			}
			defer tx.Rollback()
			if _, e = tx.Exec("UPDATE documents SET body='甲😀乙\n\nchanged!!' WHERE id=?", d.ID); e != nil {
				return e
			}
			if _, e = tx.Exec("UPDATE document_chunks SET quote='changed!!' WHERE document_id=? AND unit=2", d.ID); e != nil {
				return e
			}
			return tx.Commit()
		}
		return nil
	}
	response := f.want("GET", "/api/v1/document-search?q=needle", "teacher-a", "", "", nil, 200)
	var result documents.Search
	if json.Unmarshal(response, &result) != nil || len(result.Items) != 1 || result.Items[0].Quote != "needle%_𠮷" {
		t.Fatal("candidate and authoritative original must use same read snapshot")
	}
	f.repo.DocumentHook = nil
	for _, name := range []string{"x.doc", "x.docm", "x.txt", "x.docx"} {
		f.upload("teacher-a", "bad_extension_"+strings.ReplaceAll(name, ".", "_"), name, []byte("%PDF-not-word"), 400)
	}
	malformed, ctype := mrMultipart(t, "课程", "a.pdf", []byte("fake"))
	f.want("POST", "/api/v1/documents", "teacher-a", ctype, "bad_signature_key", malformed, 400)
	f.want("GET", "/api/v1/documents", "teacher-a", "", "", []byte("body"), 400)
	// Force a zero-millisecond SQL clock on the one fixture DML connection. The
	// exact public Timestamp schema must retain .000 rather than RFC3339Nano.
	f.minute.Add(1)
	f.app.SetMaxOpenConns(1)
	atExec(t, f.app, "SET timestamp=?", time.Now().Unix())
	defer f.app.Exec("SET timestamp=DEFAULT")
	exact, raw := f.upload("teacher-a", "fixed_clock_document", "a.pdf", []byte("%PDF-time"), 202)
	if !strings.HasSuffix(exact.Created, ".000Z") || !bytes.Contains(raw, []byte(".000Z")) {
		t.Fatal("public timestamp omitted exact millisecond precision")
	}
}
func TestDHSlowBodyUsesWholeFiveSecondBudget(t *testing.T) {
	f := dhSetup(t, 68)
	target := strings.TrimPrefix(f.server.URL, "https://")
	cfg := f.server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	connection, e := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", target, cfg)
	if e != nil {
		t.Fatal("trusted slow-body transport")
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(7 * time.Second))
	started := time.Now()
	fmt.Fprintf(connection, "POST /api/v1/documents HTTP/1.1\r\nHost: campus.test\r\nAuthorization: Bearer %s\r\nOrigin: https://campus.test\r\nIdempotency-Key: slow_body_document\r\nContent-Type: multipart/form-data; boundary=slow\r\nContent-Length: 10000\r\nConnection: close\r\n\r\n--slow\r\n", f.actors["teacher-a"].Body.AccessToken)
	response, e := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: "POST"})
	if e != nil {
		t.Fatal("slow body did not receive bounded error")
	}
	defer response.Body.Close()
	if response.StatusCode != 504 || time.Since(started) > 6*time.Second || count(t, f.app, "documents") != 0 {
		t.Fatal("slow body exceeded5s or created data")
	}
}
