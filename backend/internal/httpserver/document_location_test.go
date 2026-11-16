package httpserver

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zwh9616/campus-claw-plus/backend/internal/documents"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
)

type locationProbeAuth struct{}

func (locationProbeAuth) Me(context.Context, string) (identity.Principal, error) {
	return identity.Principal{UserID: "1", ClassID: "1", Role: "teacher", SessionID: "probe"}, nil
}

type locationProbeRepo struct{ documents.Repository }

func (locationProbeRepo) ReplayDocument(context.Context, identity.Principal, string, documents.Upload) ([]byte, bool, error) {
	return []byte(`{"id":"7","status":"queued"}`), true, nil
}
func (locationProbeRepo) GetDocument(context.Context, identity.Principal, string) (documents.Detail, error) {
	return documents.Detail{Document: documents.Document{ID: "7"}}, nil
}
func (locationProbeRepo) RetryDocument(context.Context, identity.Principal, string) (documents.Document, error) {
	return documents.Document{ID: "7", Status: "queued"}, nil
}
func TestReviewerDocumentAcceptanceLocation(t *testing.T) {
	h := DocumentsHandler(slog.New(slog.NewJSONHandler(io.Discard, nil)), locationProbeAuth{}, &documents.Service{Repo: locationProbeRepo{}}, "https://campus.test", materials.NewLimiter(nil), http.NotFoundHandler())
	for _, path := range []string{"/api/v1/documents", "/api/v1/documents/7/retry"} {
		t.Run(path, func(t *testing.T) {
			var body bytes.Buffer
			req := httptest.NewRequest("POST", path, &body)
			if path == "/api/v1/documents" {
				m := multipart.NewWriter(&body)
				m.WriteField("title", "probe")
				p, _ := m.CreateFormFile("file", "probe.pdf")
				p.Write([]byte("%PDF-probe"))
				m.Close()
				req = httptest.NewRequest("POST", path, &body)
				req.Header.Set("Content-Type", m.FormDataContentType())
				req.Header.Set("Idempotency-Key", "reviewer_location_probe")
			}
			req.Header.Set("Authorization", "Bearer probe")
			req.Header.Set("Origin", "https://campus.test")
			response := httptest.NewRecorder()
			h.ServeHTTP(response, req)
			if response.Code != 202 {
				t.Fatalf("status=%d want202", response.Code)
			}
			if got := response.Header().Get("Location"); got != "/api/v1/documents/7" {
				t.Errorf("202 Location=%q want /api/v1/documents/7", got)
			}
		})
	}
}
