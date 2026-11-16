//go:build ocrpipeline

package ocrpipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/ocrreview"
	"github.com/zwh9616/campus-claw-plus/backend/internal/privateocr"
	"os"
	"testing"
	"time"
)

func TestRealPDFOCRWorkerAndReviewedSearch(t *testing.T) {
	password := os.Getenv("MYSQL_PWD")
	token := os.Getenv("DOCUMENT_PARSER_TOKEN")
	if password == "" || token == "" {
		t.Fatal("isolated credentials required")
	}
	db, e := sql.Open("mysql", "root:"+password+"@tcp(mysql:3306)/ocr_probe?parseTime=true&charset=utf8mb4&timeout=2s&readTimeout=5s&writeTimeout=5s")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, table := range []string{"document_ocr_jobs", "document_ocr_sources", "document_ocr_review_keys", "document_ocr_chunks", "document_ocr_heads", "document_ocr_revisions", "document_idempotency", "document_parse_jobs", "document_chunks", "document_locations", "documents"} {
		if _, e = db.ExecContext(ctx, "DELETE FROM "+table); e != nil {
			t.Fatal(e)
		}
	}
	original, e := os.ReadFile("/fixtures/development-positive.pdf")
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256(original)
	_, e = db.ExecContext(ctx, "INSERT INTO documents(id,class_id,created_by,title,format,version,status,original_bytes,original_digest,size_bytes,attempts,page_count,created_at,updated_at) VALUES(91,1,1,'真实扫描PDF','pdf',1,'needs_ocr',?,?,?,1,4,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))", original, digest[:], len(original))
	if e != nil {
		t.Fatal(e)
	}
	client, e := privateocr.New(privateocr.Origin, token)
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	if !client.Healthy(ctx) {
		t.Fatal("real OCR health")
	}
	badToken := token
	if token[0] == 'A' {
		badToken = "B" + token[1:]
	} else {
		badToken = "A" + token[1:]
	}
	bad, e := privateocr.New(privateocr.Origin, badToken)
	if e != nil {
		t.Fatal(e)
	}
	if bad.Healthy(ctx) {
		t.Fatal("wrong token accepted")
	}
	bad.Close()
	store := &ocrreview.Store{DB: db}
	p := identity.Principal{UserID: "1", ClassID: "1", Role: "teacher", SessionID: "trusted-pipeline-fixture"}
	if e = store.EnqueueOCR(ctx, p, "91"); e != nil {
		t.Fatal(e)
	}
	workerCtx, stop := context.WithCancel(ctx)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- store.RunOCR(workerCtx, client.Recognize) }()
	var state string
	for {
		if e = db.QueryRowContext(ctx, "SELECT state FROM document_ocr_jobs").Scan(&state); e != nil {
			t.Fatal(e)
		}
		if state == "succeeded" {
			break
		}
		if state == "failed" {
			var reason string
			db.QueryRowContext(ctx, "SELECT failure_reason FROM document_ocr_jobs").Scan(&reason)
			t.Fatal("real OCR failed", reason)
		}
		select {
		case <-ctx.Done():
			t.Fatal("worker pipeline timeout")
		case <-time.After(50 * time.Millisecond):
		}
	}
	stop()
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	raw, e := store.ReadSource(ctx, p, "91")
	if e != nil {
		t.Fatal(e)
	}
	source, pages, e := ocrreview.DecodeSource(raw)
	if e != nil || len(pages) != 4 {
		t.Fatal("source invalid", e)
	}
	if e = os.WriteFile("/evidence/source.json", raw, 0644); e != nil {
		t.Fatal("preserve actual response for independent Schema/CER checks", e)
	}
	words := 0
	for _, page := range source.Pages {
		words += len(page.Words)
	}
	if words < 100 {
		t.Fatal("not real OCR", words)
	}
	if _, e = store.Save(ctx, p, "91", "real-pipeline-review-001", ocrreview.Review{BaseRevision: 1, Acknowledged: true, Pages: pages}); e != nil {
		t.Fatal(e)
	}
	student := p
	student.Role = "student"
	query := source.Pages[0].Words[0].Text
	citations, e := store.Search(ctx, student, query, 20)
	if e != nil || len(citations) == 0 {
		t.Fatal("reviewed search", e)
	}
	for _, c := range citations {
		if c.Revision != 2 {
			t.Fatal("wrong revision")
		}
	}
	other := student
	other.ClassID = "2"
	if cross, e := store.Search(ctx, other, query, 20); e != nil || len(cross) != 0 {
		t.Fatal("cross-class leak", e)
	}
	after, e := store.ReadSource(ctx, p, "91")
	if e != nil || !bytes.Equal(after, raw) {
		t.Fatal("source changed after review", e)
	}
	evidence, _ := json.Marshal(map[string]any{"pages": len(pages), "words": words, "source_sha256": sha256.Sum256(raw), "original_sha256": digest, "review_revision": 2, "citations": len(citations), "cross_class_results": 0, "worker_state": state, "source_unchanged": true})
	t.Log("PIPELINE " + string(evidence))
}
func TestRealOCRCancellationReleasesService(t *testing.T) {
	client, e := privateocr.New(privateocr.Origin, os.Getenv("DOCUMENT_PARSER_TOKEN"))
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	original, e := os.ReadFile("/fixtures/development-positive.pdf")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan string, 1)
	go func() { _, reason := client.Recognize(ctx, original); done <- reason }()
	busy := false
	for i := 0; i < 20; i++ {
		time.Sleep(20 * time.Millisecond)
		probe, c := context.WithTimeout(context.Background(), time.Second)
		_, reason := client.Recognize(probe, []byte("%PDF-corrupt"))
		c()
		if reason == "internal_failure" {
			busy = true
			break
		}
		select {
		case reason := <-done:
			t.Fatal("recognition finished before cancellation probe", reason)
		default:
		}
	}
	if !busy {
		t.Fatal("did not observe occupied OCR service")
	}
	cancel()
	select {
	case reason := <-done:
		if reason != "worker_interrupted" {
			t.Fatal(reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("private request did not cancel")
	}
	released := false
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		probe, c := context.WithTimeout(context.Background(), time.Second)
		_, reason := client.Recognize(probe, []byte("%PDF-corrupt"))
		c()
		if reason == "corrupt" {
			released = true
			break
		}
	}
	if !released {
		t.Fatal("canceled child kept OCR service occupied")
	}
	t.Log("CANCEL observed_busy=true request_canceled=true subprocess_reaped=true service_reusable=true")
}
