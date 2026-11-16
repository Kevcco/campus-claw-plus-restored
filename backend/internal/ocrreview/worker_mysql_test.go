//go:build ocrreview

package ocrreview

import (
	"context"
	"errors"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"testing"
	"time"
)

func TestMySQLOCRWorkerFailureAndExplicitRetry(t *testing.T) {
	s, p, _ := sourceSetup(t)
	ctx := context.Background()
	if e := s.EnqueueOCR(ctx, p, "91"); e != nil {
		t.Fatal(e)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		w, e := s.ClaimOCR(ctx, "91", "1")
		if e != nil || w.Epoch != uint64(attempt) {
			t.Fatal(e)
		}
		recognize := func(context.Context, []byte) ([]byte, string) { return nil, "ocr_timeout" }
		if attempt == 2 {
			recognize = func(context.Context, []byte) ([]byte, string) { return []byte(`{"status":"ready"}`), "" }
		}
		if e = s.executeOCR(ctx, w, recognize, 10*time.Millisecond); e != nil {
			t.Fatal(e)
		}
		var state, reason string
		var attempts int
		if e = s.DB.QueryRow("SELECT state,failure_reason,attempts FROM document_ocr_jobs").Scan(&state, &reason, &attempts); e != nil || state != "failed" || attempts != attempt {
			t.Fatal(state, reason, attempts, e)
		}
		if (attempt == 2 && reason != "internal_failure") || (attempt != 2 && reason != "ocr_timeout") {
			t.Fatal(reason)
		}
		student := p
		student.Role = "student"
		requireErr(t, s.RetryOCR(ctx, student, "91"), identity.ErrForbidden)
		if attempt < 3 {
			if e = s.RetryOCR(ctx, p, "91"); e != nil {
				t.Fatal(e)
			}
			requireErr(t, s.RetryOCR(ctx, p, "91"), identity.ErrConflict)
		} else {
			requireErr(t, s.RetryOCR(ctx, p, "91"), identity.ErrConflict)
		}
	}
	var n int
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM document_ocr_sources").Scan(&n); e != nil || n != 0 {
		t.Fatal("failed worker published", e)
	}
}
func TestMySQLOCRWorkerCancelAndLostLease(t *testing.T) {
	s, p, _ := sourceSetup(t)
	ctx := context.Background()
	if e := s.EnqueueOCR(ctx, p, "91"); e != nil {
		t.Fatal(e)
	}
	w, e := s.ClaimOCR(ctx, "91", "1")
	if e != nil {
		t.Fatal(e)
	}
	callCtx, cancel := context.WithCancel(ctx)
	entered, stopped := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.executeOCR(callCtx, w, func(c context.Context, _ []byte) ([]byte, string) {
			close(entered)
			<-c.Done()
			close(stopped)
			return nil, "worker_interrupted"
		}, 10*time.Millisecond)
	}()
	<-entered
	cancel()
	if e = <-done; !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("recognizer not canceled")
	}
	var state string
	if e = s.DB.QueryRow("SELECT state FROM document_ocr_jobs").Scan(&state); e != nil || state != "processing" {
		t.Fatal(state, e)
	}
	// Lose authority while recognition is live; heartbeat must cancel the call.
	entered, stopped = make(chan struct{}), make(chan struct{})
	go func() {
		done <- s.executeOCR(ctx, w, func(c context.Context, _ []byte) ([]byte, string) {
			close(entered)
			<-c.Done()
			close(stopped)
			return nil, "worker_interrupted"
		}, 10*time.Millisecond)
	}()
	<-entered
	if _, e = s.DB.Exec("UPDATE document_ocr_jobs SET lease_until=TIMESTAMPADD(SECOND,-1,UTC_TIMESTAMP(3))"); e != nil {
		t.Fatal(e)
	}
	requireErr(t, <-done, identity.ErrConflict)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("lost owner kept recognizing")
	}
}
