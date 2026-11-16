//go:build ocrreview

package ocrreview

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOCRClaimHelper(t *testing.T) {
	if os.Getenv("CAMPUS_OCR_CLAIM_HELPER") != "1" {
		return
	}
	s := &Store{DB: db(t)}
	if _, e := s.ClaimOCR(context.Background(), "91", "1"); e != nil {
		t.Fatal(e)
	}
	println("OCR_CLAIMED")
	time.Sleep(time.Hour)
}
func TestMySQLOCRKilledWorkerTakeover(t *testing.T) {
	s, p, raw := sourceSetup(t)
	ctx := context.Background()
	if e := s.EnqueueOCR(ctx, p, "91"); e != nil {
		t.Fatal(e)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestOCRClaimHelper$")
	child.Env = append(os.Environ(), "CAMPUS_OCR_CLAIM_HELPER=1")
	output, e := child.StderrPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = child.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if child.Process != nil {
			child.Process.Kill()
		}
	})
	ready := make(chan bool, 1)
	go func() {
		scan := bufio.NewScanner(output)
		for scan.Scan() {
			if scan.Text() == "OCR_CLAIMED" {
				ready <- true
				return
			}
		}
		ready <- false
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("helper did not claim")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("helper claim timeout")
	}
	var owner string
	var epoch uint64
	if e = s.DB.QueryRow("SELECT worker_owner,execution_epoch FROM document_ocr_jobs").Scan(&owner, &epoch); e != nil || epoch != 1 {
		t.Fatal(e)
	}
	if e = child.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	if e = child.Wait(); e == nil {
		t.Fatal("helper was not actually killed")
	}
	_, e = s.ClaimOCR(ctx, "91", "1")
	requireErr(t, e, identity.ErrConflict)
	// Advance only this owned fixture's lease clock after observing real kill.
	if _, e = s.DB.Exec("UPDATE document_ocr_jobs SET lease_until=TIMESTAMPADD(SECOND,-1,UTC_TIMESTAMP(3))"); e != nil {
		t.Fatal(e)
	}
	next, e := s.ClaimOCR(ctx, "91", "1")
	if e != nil || next.Epoch != 2 || next.Owner == owner {
		t.Fatal(next, e)
	}
	if e = s.CommitOCR(ctx, next.Lease, raw); e != nil {
		t.Fatal(e)
	}
}

func sourceSetup(t *testing.T) (*Store, identity.Principal, []byte) {
	t.Helper()
	s, p, _ := setup(t)
	for _, table := range []string{"document_ocr_heads", "document_ocr_revisions"} {
		if _, e := s.DB.Exec("DELETE FROM " + table); e != nil {
			t.Fatal(e)
		}
	}
	raw, e := os.ReadFile("/fixtures/real-source.json")
	if e != nil {
		t.Fatal(e)
	}
	decoded, _, e := DecodeSource(raw)
	if e != nil {
		t.Fatal("real PDFium/Tesseract source invalid", e)
	}
	original, e := os.ReadFile("/fixtures/development-positive.pdf")
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256(original)
	if _, e = s.DB.Exec("UPDATE documents SET original_bytes=?,size_bytes=?,original_digest=?,page_count=? WHERE id=91", original, len(original), digest[:], len(decoded.Pages)); e != nil {
		t.Fatal(e)
	}
	return s, p, raw
}
func TestMySQLOCRRealSourceAndReview(t *testing.T) {
	s, p, raw := sourceSetup(t)
	ctx := context.Background()
	if e := s.EnqueueOCR(ctx, p, "91"); e != nil {
		t.Fatal(e)
	}
	if e := s.EnqueueOCR(ctx, p, "91"); e != nil {
		t.Fatal("enqueue replay", e)
	}
	var wg sync.WaitGroup
	work := make(chan Work, 20)
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w, e := s.ClaimOCR(ctx, "91", "1")
			if e == nil {
				work <- w
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	close(work)
	success := 0
	for e := range errs {
		if e == nil {
			success++
		} else if !errors.Is(e, identity.ErrConflict) {
			t.Fatal(e)
		}
	}
	if success != 1 {
		t.Fatal("claim race", success)
	}
	w := <-work
	if !bytes.HasPrefix(w.Original, []byte("%PDF-")) {
		t.Fatal("lost original")
	}
	if e := s.HeartbeatOCR(ctx, w.Lease); e != nil {
		t.Fatal(e)
	}
	forged := w.Lease
	forged.OriginalDigest[0] ^= 1
	requireErr(t, s.CommitOCR(ctx, forged, raw), identity.ErrConflict)
	if e := s.CommitOCR(ctx, w.Lease, raw); e != nil {
		t.Fatal(e)
	}
	got, e := s.ReadSource(ctx, p, "91")
	if e != nil || !bytes.Equal(got, raw) {
		t.Fatal("source bytes drift", e)
	}
	_, pages, e := DecodeSource(raw)
	if e != nil {
		t.Fatal(e)
	}
	pages[0].Text += " 教师校对"
	if _, e = s.Save(ctx, p, "91", "source-review-0001", Review{1, true, pages}); e != nil {
		t.Fatal(e)
	}
	got, e = s.ReadSource(ctx, p, "91")
	if e != nil || !bytes.Equal(got, raw) {
		t.Fatal("review overwrote OCR boxes", e)
	}
	old, e := s.Read(ctx, p, "91", 1)
	if e != nil || strings.Contains(old.Body, "教师校对") {
		t.Fatal("old text drift", e)
	}
	student := p
	student.Role = "student"
	_, e = s.ReadSource(ctx, student, "91")
	requireErr(t, e, identity.ErrForbidden)
	other := p
	other.ClassID = "2"
	_, e = s.ReadSource(ctx, other, "91")
	requireErr(t, e, identity.ErrNotFound)
	var state string
	if e = s.DB.QueryRow("SELECT state FROM document_ocr_jobs").Scan(&state); e != nil || state != "succeeded" {
		t.Fatal(state, e)
	}
	requireErr(t, s.CommitOCR(ctx, w.Lease, raw), identity.ErrConflict)
	if _, e = s.DB.Exec("UPDATE document_ocr_sources SET source_bytes=?", append(append([]byte{}, raw...), byte(' '))); e != nil {
		t.Fatal(e)
	}
	_, e = s.ReadSource(ctx, p, "91")
	requireErr(t, e, identity.ErrUnavailable)
}
func TestMySQLOCRSourceRollbackAndFinalLeaseExpiry(t *testing.T) {
	s, p, raw := sourceSetup(t)
	ctx := context.Background()
	if e := s.EnqueueOCR(ctx, p, "91"); e != nil {
		t.Fatal(e)
	}
	w, e := s.ClaimOCR(ctx, "91", "1")
	if e != nil {
		t.Fatal(e)
	}
	s.Hook = func(stage string) error {
		if stage == "ocr-source" {
			return errors.New("source transaction interruption")
		}
		return nil
	}
	requireErr(t, s.CommitOCR(ctx, w.Lease, raw), identity.ErrUnavailable)
	for _, table := range []string{"document_ocr_sources", "document_ocr_revisions", "document_ocr_heads"} {
		var n int
		if e = s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatal("partial source", table, n, e)
		}
	}
	if _, e = s.DB.Exec("UPDATE document_ocr_jobs SET lease_until=TIMESTAMPADD(MICROSECOND,100000,UTC_TIMESTAMP(3))"); e != nil {
		t.Fatal(e)
	}
	enteredSource := false
	s.Hook = func(stage string) error {
		if stage == "ocr-source" {
			enteredSource = true
			time.Sleep(150 * time.Millisecond)
		}
		return nil
	}
	requireErr(t, s.CommitOCR(ctx, w.Lease, raw), identity.ErrConflict)
	if !enteredSource {
		t.Fatal("lease expired before the transaction source-write boundary")
	}
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM document_ocr_sources").Scan(&n); e != nil || n != 0 {
		t.Fatal("expired owner published", n, e)
	}
	s.Hook = nil
	next, e := s.ClaimOCR(ctx, "91", "1")
	if e != nil || next.Epoch != 2 {
		t.Fatal(next, e)
	}
	requireErr(t, s.CommitOCR(ctx, w.Lease, raw), identity.ErrConflict)
	if e = s.CommitOCR(ctx, next.Lease, raw); e != nil {
		t.Fatal(e)
	}
}
func TestMySQLOCRTakeoverAndExhaustion(t *testing.T) {
	s, p, raw := sourceSetup(t)
	ctx := context.Background()
	if e := s.EnqueueOCR(ctx, p, "91"); e != nil {
		t.Fatal(e)
	}
	var old Lease
	for attempt := 1; attempt <= 3; attempt++ {
		w, e := s.ClaimOCR(ctx, "91", "1")
		if e != nil || w.Epoch != uint64(attempt) {
			t.Fatal(attempt, e)
		}
		if old.Epoch > 0 {
			requireErr(t, s.HeartbeatOCR(ctx, old), identity.ErrConflict)
			requireErr(t, s.CommitOCR(ctx, old, raw), identity.ErrConflict)
		}
		old = w.Lease
		if _, e = s.DB.Exec("UPDATE document_ocr_jobs SET lease_until=TIMESTAMPADD(SECOND,-1,UTC_TIMESTAMP(3))"); e != nil {
			t.Fatal(e)
		}
	}
	_, e := s.ClaimOCR(ctx, "91", "1")
	requireErr(t, e, identity.ErrConflict)
	var state string
	var attempts int
	if e = s.DB.QueryRow("SELECT state,attempts FROM document_ocr_jobs").Scan(&state, &attempts); e != nil || state != "failed" || attempts != 3 {
		t.Fatal(state, attempts, e)
	}
	var count int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM document_ocr_sources").Scan(&count); e != nil || count != 0 {
		t.Fatal("exhausted source", e)
	}
}
