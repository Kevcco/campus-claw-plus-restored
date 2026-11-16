//go:build ocrpublic

package ocrpublic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zwh9616/campus-claw-plus/backend/internal/config"
	"github.com/zwh9616/campus-claw-plus/backend/internal/documentqueue"
	"github.com/zwh9616/campus-claw-plus/backend/internal/documents"
	"github.com/zwh9616/campus-claw-plus/backend/internal/httpserver"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
	"github.com/zwh9616/campus-claw-plus/backend/internal/migrations"
	"github.com/zwh9616/campus-claw-plus/backend/internal/mysqlstore"
	"github.com/zwh9616/campus-claw-plus/backend/internal/ocrreview"
)

type response struct {
	Path, Method string
	Code         int
	Body         json.RawMessage
	Location     string
}
type fixture struct {
	t        *testing.T
	db       *sql.DB
	store    *ocrreview.Store
	auth     *identity.Service
	server   *httptest.Server
	actors   map[string]identity.Issued
	minute   atomic.Int64
	rows     []response
	log      bytes.Buffer
	original []byte
	upload   documentqueue.Upload
	accepted []byte
	id       string
}

func setup(t *testing.T, index int) *fixture {
	t.Helper()
	password := os.Getenv("MYSQL_PWD")
	if password == "" {
		t.Fatal("owned credential missing")
	}
	root, e := sql.Open("mysql", "root:"+password+"@tcp(127.0.0.1:3306)/?parseTime=true&charset=utf8mb4&timeout=2s&readTimeout=5s&writeTimeout=5s")
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("ocr_public_%d", index)
	if _, e = root.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin"); e != nil {
		t.Fatal(e)
	}
	root.Close()
	db, e := sql.Open("mysql", "root:"+password+"@tcp(127.0.0.1:3306)/"+name+"?parseTime=true&charset=utf8mb4&timeout=2s&readTimeout=5s&writeTimeout=5s")
	if e != nil {
		t.Fatal(e)
	}
	db.SetMaxOpenConns(20)
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if e = (migrations.Executor{DB: db, Database: name}).Up(ctx, 7); e != nil {
		t.Fatal(e)
	}
	repo := &mysqlstore.Store{DB: db}
	seed := mysqlstore.Seed{Classes: []mysqlstore.SeedClass{{Key: "a", Name: "A"}, {Key: "b", Name: "B"}}}
	for _, row := range []struct {
		name, role string
		classes    []string
	}{{"teacher-a", "teacher", []string{"a", "b"}}, {"teacher-b", "teacher", []string{"b"}}, {"student-a", "student", []string{"a"}}, {"student-b", "student", []string{"b"}}} {
		seed.Users = append(seed.Users, mysqlstore.SeedUser{Username: row.name, Password: "test-only-" + row.name, Role: row.role, ClassKeys: row.classes})
	}
	if e = repo.Initialize(ctx, seed, nil); e != nil {
		t.Fatal(e)
	}
	f := &fixture{t: t, db: db, store: &ocrreview.Store{DB: db}, actors: map[string]identity.Issued{}}
	f.auth, e = identity.New(repo, nil, "https://campus.test")
	if e != nil {
		t.Fatal(e)
	}
	for _, user := range seed.Users {
		issued, e := f.auth.Login(ctx, user.Username, user.Password, strings.Repeat("a", 32))
		if e != nil {
			t.Fatal("real fixture login", e)
		}
		f.actors[user.Username] = issued
	}
	f.minute.Store(time.Now().Unix() / 60)
	limiter := materials.NewLimiter(func() time.Time { return time.Unix(f.minute.Load()*60, 0) })
	logger := slog.New(slog.NewJSONHandler(&f.log, nil))
	base := httpserver.IdentityHandler(logger, f.auth, repo, config.Identity{}, identity.NewLimiter(nil))
	base = httpserver.MaterialsHandler(logger, f.auth, &materials.Service{Repo: repo}, "https://campus.test", limiter, base)
	base = httpserver.DocumentsHandler(logger, f.auth, &documents.Service{Repo: repo}, "https://campus.test", limiter, base)
	f.server = httptest.NewTLSServer(httpserver.OCRHandler(logger, f.auth, f.store, "https://campus.test", limiter, base))
	t.Cleanup(func() {
		f.server.Close()
		raw, _ := json.Marshal(f.rows)
		if e := os.WriteFile("/evidence/"+t.Name()+".json", raw, 0644); e != nil {
			t.Error(e)
		}
		if e := os.WriteFile("/evidence/"+t.Name()+"-logs.txt", f.log.Bytes(), 0644); e != nil {
			t.Error(e)
		}
	})
	f.original, e = os.ReadFile("/fixtures/development-positive.pdf")
	if e != nil {
		t.Fatal(e)
	}
	p := f.actors["teacher-a"].Body.Principal
	uid, _ := strconv.ParseUint(p.UserID, 10, 64)
	cid, _ := strconv.ParseUint(p.ClassID, 10, 64)
	f.upload = documentqueue.Upload{UserID: uid, ClassID: cid, Key: "original-upload-key01", Title: "原件扫描PDF", Format: "pdf", Original: f.original}
	queue := &documentqueue.Store{DB: db}
	accepted, e := queue.Enqueue(ctx, f.upload)
	if e != nil {
		t.Fatal(e)
	}
	f.accepted = accepted.Body
	f.id = strconv.FormatUint(accepted.ID, 10)
	owner, e := documentqueue.NewOwner()
	if e != nil {
		t.Fatal(e)
	}
	work, e := queue.Claim(ctx, accepted.ID, owner)
	if e != nil {
		t.Fatal(e)
	}
	pages := 4
	if e = queue.Finish(ctx, work, documentqueue.Result{Status: "needs_ocr", Pages: &pages}); e != nil {
		t.Fatal(e)
	}
	return f
}
func (f *fixture) path(suffix string) string { return "/api/v1/documents/" + f.id + "/" + suffix }
func (f *fixture) call(method, path, actor, key string, body []byte, want int) []byte {
	f.t.Helper()
	req, _ := http.NewRequest(method, f.server.URL+path, bytes.NewReader(body))
	if actor != "" {
		req.Header.Set("Authorization", "Bearer "+f.actors[actor].Body.AccessToken)
	}
	if method == "POST" {
		req.Header.Set("Origin", "https://campus.test")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, e := f.server.Client().Do(req)
	if e != nil {
		f.t.Fatal("real HTTPS request", e)
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(resp.Body)
	if e != nil || resp.StatusCode != want {
		f.t.Fatalf("%s %s: status%d want%d, read%v", method, path, resp.StatusCode, want, e)
	}
	if resp.Header.Get("Cache-Control") != "no-store" || len(resp.Header.Get("X-Request-ID")) != 32 {
		f.t.Fatal("missing private response headers")
	}
	location := resp.Header.Get("Location")
	if want == 201 && !strings.Contains(location, "/ocr-revisions/") {
		f.t.Fatal("missing immutable201 location")
	}
	if want == 202 && location != f.path("ocr") {
		f.t.Fatal("missing202 location")
	}
	pattern := path
	if strings.HasPrefix(path, "/api/v1/ocr-search") {
		pattern = "/api/v1/ocr-search"
	} else {
		parts := strings.Split(strings.TrimPrefix(strings.Split(path, "?")[0], "/api/v1/documents/"), "/")
		if len(parts) == 3 {
			pattern = "/api/v1/documents/{id}/ocr-revisions/{revision}"
		} else if len(parts) == 2 {
			pattern = "/api/v1/documents/{id}/" + parts[1]
		}
	}
	if len(raw) > 0 && json.Valid(raw) {
		f.rows = append(f.rows, response{pattern, method, want, json.RawMessage(raw), location})
	}
	return raw
}
func (f *fixture) source() {
	f.t.Helper()
	f.call("POST", f.path("ocr-jobs"), "teacher-a", "", nil, 202)
	p := f.actors["teacher-a"].Body.Principal
	work, e := f.store.ClaimOCR(context.Background(), f.id, p.ClassID)
	if e != nil {
		f.t.Fatal(e)
	}
	raw, e := os.ReadFile("/fixtures/actual-source.json")
	if e != nil {
		f.t.Fatal(e)
	}
	if e = f.store.CommitOCR(context.Background(), work.Lease, raw); e != nil {
		f.t.Fatal(e)
	}
}
func review() ocrreview.Review {
	return ocrreview.Review{BaseRevision: 1, Acknowledged: true, Pages: []ocrreview.Page{{Page: 1, Text: "教师核对😀"}, {Page: 2, Text: "保留分解字符e\u0301"}, {Page: 3, Text: "第三页"}, {Page: 4, Text: "第四页"}}}
}
func encode(v any) []byte { raw, _ := json.Marshal(v); return raw }

func TestOCRPublicRealIdentityVersionsAndCitations(t *testing.T) {
	f := setup(t, 1)
	ctx := context.Background()
	f.call("GET", f.path("ocr"), "", "", nil, 401)
	f.call("GET", f.path("ocr"), "teacher-b", "", nil, 404)
	f.call("GET", f.path("ocr"), "student-a", "", nil, 404)
	f.source()
	original := f.call("GET", f.path("ocr-source"), "teacher-a", "", nil, 200)
	f.call("GET", f.path("ocr-source"), "student-a", "", nil, 403)
	f.call("GET", f.path("ocr-revisions/1"), "student-a", "", nil, 404)
	in := review()
	query := strings.Repeat("界", 1999) + "😀"
	in.Pages[0].Text = strings.Repeat("😀", 4095) + query + "尾"
	first := f.call("POST", f.path("ocr-reviews"), "teacher-a", "review-same-key001", encode(in), 201)
	f.call("GET", f.path("ocr"), "student-a", "", nil, 200)
	second := f.call("POST", f.path("ocr-reviews"), "teacher-a", "review-same-key001", encode(in), 201)
	if !bytes.Equal(first, second) {
		t.Fatal("original201 drift")
	}
	f.call("POST", f.path("ocr-reviews"), "student-a", "student-review-001", encode(in), 403)
	old := f.call("GET", f.path("ocr-revisions/2"), "student-a", "", nil, 200)
	hits := f.call("GET", "/api/v1/ocr-search?q="+url.QueryEscape(query), "student-a", "", nil, 200)
	var found struct{ Items []ocrreview.Citation }
	if json.Unmarshal(hits, &found) != nil || len(found.Items) == 0 {
		t.Fatal("2000 codepoint query missing")
	}
	var persisted ocrreview.Revision
	if json.Unmarshal(old, &persisted) != nil {
		t.Fatal("revision")
	}
	if persisted.Pages[1].Text != in.Pages[1].Text {
		t.Fatal("teacher Unicode was rewritten")
	}
	for _, c := range found.Items {
		if c.Revision != 2 || c.Page != 1 || !strings.Contains(c.Quote, query) || string([]rune(persisted.Body)[c.Start:c.End]) != c.Quote {
			t.Fatal("persistent codepoint citation")
		}
	}
	cross := f.call("GET", "/api/v1/ocr-search?q="+url.QueryEscape(query), "student-b", "", nil, 200)
	if string(cross) != "{\"items\":[],\"mode\":\"keyword\"}\n" {
		t.Fatal("cross-class search")
	}
	missing := f.call("GET", "/api/v1/documents/999/ocr-revisions/2", "student-a", "", nil, 404)
	denied := f.call("GET", f.path("ocr-revisions/2"), "student-b", "", nil, 404)
	var a, b map[string]any
	json.Unmarshal(missing, &a)
	json.Unmarshal(denied, &b)
	delete(a, "request_id")
	delete(b, "request_id")
	if !bytes.Equal(encode(a), encode(b)) {
		t.Fatal("404 existence distinction")
	}
	in.BaseRevision = 2
	in.Pages[0].Text = "新版本"
	f.call("POST", f.path("ocr-reviews"), "teacher-a", "review-new-key001", encode(in), 201)
	if !bytes.Equal(old, f.call("GET", f.path("ocr-revisions/2"), "student-a", "", nil, 200)) {
		t.Fatal("old revision changed")
	}
	if !bytes.Equal(original, f.call("GET", f.path("ocr-source"), "teacher-a", "", nil, 200)) {
		t.Fatal("source overwritten")
	}
	replay := review()
	replay.Pages[0].Text = strings.Repeat("😀", 4095) + query + "尾"
	if !bytes.Equal(first, f.call("POST", f.path("ocr-reviews"), "teacher-a", "review-same-key001", encode(replay), 201)) {
		t.Fatal("replay after new publish")
	}
	result := f.call("GET", "/api/v1/ocr-search?q="+url.QueryEscape(query), "student-a", "", nil, 200)
	if !bytes.Contains(result, []byte(`"items":[]`)) {
		t.Fatal("old index leaked")
	}
	replayed, ok, e := (&documentqueue.Store{DB: f.db}).Replay(ctx, f.upload)
	if e != nil || !ok || !bytes.Equal(replayed.Body, f.accepted) {
		t.Fatal("original202 changed")
	}
	var raw []byte
	var version int
	if e = f.db.QueryRow("SELECT original_bytes,version FROM documents WHERE id=?", f.id).Scan(&raw, &version); e != nil || version != 1 || sha256.Sum256(raw) != sha256.Sum256(f.original) {
		t.Fatal("original v1 drift")
	}
	actor := f.actors["teacher-a"]
	csrf, e := f.auth.GetCSRF(ctx, actor.Refresh)
	if e != nil {
		t.Fatal(e)
	}
	switched, e := f.auth.Mutate(ctx, identity.Mutation{Action: "switch-class", Access: actor.Body.AccessToken, Refresh: actor.Refresh, Origin: "https://campus.test", CSRF: csrf, RequestID: strings.Repeat("b", 32), ClassID: func() (string, error) { return f.actors["teacher-b"].Body.Principal.ClassID, nil }})
	if e != nil {
		t.Fatal(e)
	}
	f.actors["switched"] = switched
	f.call("GET", f.path("ocr"), "teacher-a", "", nil, 401)
	f.call("GET", f.path("ocr"), "switched", "", nil, 404)
	f.call("POST", f.path("ocr-reviews"), "teacher-a", "review-same-key001", encode(replay), 401)
	f.call("POST", f.path("ocr-reviews"), "switched", "review-same-key001", encode(replay), 404)
	if e = f.auth.Revoke(ctx, f.actors["student-a"].Body.Principal.UserID, strings.Repeat("c", 32)); e != nil {
		t.Fatal(e)
	}
	f.call("GET", f.path("ocr-revisions/2"), "student-a", "", nil, 401)
}

func TestOCRPublicQueueRetryAndSharedQuota(t *testing.T) {
	f := setup(t, 2)
	ctx := context.Background()
	first := f.call("POST", f.path("ocr-jobs"), "teacher-a", "", nil, 202)
	if !bytes.Equal(first, f.call("POST", f.path("ocr-jobs"), "teacher-a", "", nil, 202)) {
		t.Fatal("natural start replay")
	}
	p := f.actors["teacher-a"].Body.Principal
	work, e := f.store.ClaimOCR(ctx, f.id, p.ClassID)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.store.FailOCR(ctx, work.Lease, "ocr_timeout"); e != nil {
		t.Fatal(e)
	}
	f.call("GET", f.path("ocr"), "teacher-a", "", nil, 200)
	f.call("POST", f.path("ocr-retry"), "teacher-a", "", nil, 202)
	f.call("POST", f.path("ocr-retry"), "teacher-a", "", nil, 409)
	work, e = f.store.ClaimOCR(ctx, f.id, p.ClassID)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.store.FailOCR(ctx, work.Lease, "corrupt"); e != nil {
		t.Fatal(e)
	}
	f.call("POST", f.path("ocr-retry"), "teacher-a", "", nil, 409)
	f.minute.Add(1)
	for i := 0; i < 12; i++ {
		var body bytes.Buffer
		m := multipart.NewWriter(&body)
		m.WriteField("title", "共享配额TXT")
		part, _ := m.CreateFormFile("file", "quota.txt")
		part.Write([]byte("txt"))
		m.Close()
		req, _ := http.NewRequest("POST", f.server.URL+"/api/v1/materials", &body)
		req.Header.Set("Authorization", "Bearer "+f.actors["teacher-a"].Body.AccessToken)
		req.Header.Set("Origin", "https://campus.test")
		req.Header.Set("Content-Type", m.FormDataContentType())
		req.Header.Set("Idempotency-Key", fmt.Sprintf("shared-material-%04d", i))
		r, e := f.server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode != 201 {
			t.Fatal("old TXT write", r.StatusCode)
		}
	}
	f.call("POST", f.path("ocr-jobs"), "teacher-a", "", nil, 429)
}

func TestOCRPublicStrictBodyRollbackAndDependency(t *testing.T) {
	f := setup(t, 3)
	f.source()
	valid := encode(review())
	for _, body := range [][]byte{[]byte(`{"base_revision":1,"base_revision":1,"acknowledged":true,"pages":[]}`), []byte(`{"base_revision":1,"acknowledged":true,"pages":[],"class_id":"2"}`), []byte(`{"Base_revision":1,"acknowledged":true,"pages":[]}`), []byte(`null`), []byte(`{"base_revision":1,"acknowledged":false,"pages":[]}`)} {
		f.call("POST", f.path("ocr-reviews"), "teacher-a", "invalid-review-001", body, 400)
	}
	f.call("GET", f.path("ocr")+"?class_id=2", "teacher-a", "", nil, 400)
	f.call("GET", f.path("ocr-revisions/02"), "teacher-a", "", nil, 400)
	f.call("GET", "/api/v1/ocr-search?q="+strings.Repeat("a", 2001), "teacher-a", "", nil, 400)
	f.minute.Add(1)
	f.store.Hook = func(stage string) error {
		if stage == "before-commit" {
			return fmt.Errorf("controlled rollback")
		}
		return nil
	}
	f.call("POST", f.path("ocr-reviews"), "teacher-a", "rollback-review01", valid, 503)
	var count int
	if e := f.db.QueryRow("SELECT COUNT(*) FROM document_ocr_revisions").Scan(&count); e != nil || count != 1 {
		t.Fatal("partial HTTP publication", e)
	}
	for _, table := range []string{"document_ocr_chunks", "document_ocr_review_keys"} {
		if e := f.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); e != nil || count != 0 {
			t.Fatal("HTTP rollback leaked", table, e)
		}
	}
	f.store.Hook = nil
	f.call("POST", f.path("ocr-reviews"), "teacher-a", "rollback-review01", valid, 201)
	f.db.Close()
	f.call("GET", f.path("ocr"), "teacher-a", "", nil, 503)
}

func TestOCRPublicExactDecodedAndEscapedBodyBudget(t *testing.T) {
	f := setup(t, 4)
	f.source()
	in := review()
	other := len(in.Pages[1].Text) + len(in.Pages[2].Text) + len(in.Pages[3].Text) + 6
	count := ocrreview.MaxBody - other
	in.Pages[0].Text = strings.Repeat("a", count)
	body := encode(in)
	body = bytes.Replace(body, []byte(in.Pages[0].Text), []byte(strings.Repeat(`\u0061`, count)), 1)
	if len(body) > httpserver.OCRMaxRequest {
		t.Fatal("JSON encoding budget too small")
	}
	f.call("POST", f.path("ocr-reviews"), "teacher-a", "maximum-review001", body, 201)
	f.call("GET", f.path("ocr-revisions/2"), "student-a", "", nil, 200)
	in.BaseRevision = 2
	in.Pages[0].Text += "a"
	f.call("POST", f.path("ocr-reviews"), "teacher-a", "over-limit-key001", encode(in), 400)
}
