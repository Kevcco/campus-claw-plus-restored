package httpserver

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zwh9616/campus-claw-plus/backend/internal/documents"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
	"github.com/zwh9616/campus-claw-plus/backend/internal/ocrreview"
)

type ocrAuthProbe struct{ role string }

func (a ocrAuthProbe) Me(_ context.Context, token string) (identity.Principal, error) {
	if token != "probe" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return identity.Principal{UserID: "1", ClassID: "1", Role: a.role, SessionID: "unit-fixture"}, nil
}

type ocrRepoProbe struct {
	OCRRepository
	saves int
}

func (r *ocrRepoProbe) Status(_ context.Context, _ identity.Principal, id string) (ocrreview.Status, error) {
	if id != "91" {
		return ocrreview.Status{}, identity.ErrNotFound
	}
	return ocrreview.Status{DocumentID: id}, nil
}
func (r *ocrRepoProbe) Save(_ context.Context, _ identity.Principal, id, key string, input ocrreview.Review) ([]byte, error) {
	r.saves++
	return []byte(`{"document_id":"91","revision":2,"status":"ready"}`), nil
}
func ocrProbe(role string) (http.Handler, *ocrRepoProbe) {
	r := &ocrRepoProbe{}
	return OCRHandler(slog.New(slog.NewJSONHandler(io.Discard, nil)), ocrAuthProbe{role}, r, "https://campus.test", materials.NewLimiter(nil), http.NotFoundHandler()), r
}

func TestOCRHTTPBoundaryOrderAndStrictJSON(t *testing.T) {
	for _, row := range []struct {
		method, path, role, token, body, key string
		status                               int
	}{
		{"GET", "/unknown", "teacher", "probe", "", "", 404},
		{"PUT", "/api/v1/documents/91/ocr", "teacher", "", "", "", 405},
		{"POST", "/api/v1/documents/91/ocr-reviews", "teacher", "", "not-json", "", 401},
		{"POST", "/api/v1/documents/91/ocr-reviews", "student", "probe", "not-json", "", 403},
		{"POST", "/api/v1/documents/92/ocr-reviews", "teacher", "probe", "not-json", "", 404},
		{"POST", "/api/v1/documents/91/ocr-reviews", "teacher", "probe", `{"base_revision":1,"acknowledged":true,"pages":[],"role":"teacher"}`, "valid-review-key01", 400},
		{"POST", "/api/v1/documents/91/ocr-reviews", "teacher", "probe", `{"base_revision":1,"base_revision":2,"acknowledged":true,"pages":[]}`, "valid-review-key01", 400},
		{"POST", "/api/v1/documents/91/ocr-reviews", "teacher", "probe", `{"base_revision":1,"acknowledged":true,"pages":[{"page":1,"text":"😀"}]}`, "valid-review-key01", 201},
	} {
		t.Run(row.path+row.method+row.role+row.token+fmt.Sprint(row.status), func(t *testing.T) {
			h, repo := ocrProbe(row.role)
			req := httptest.NewRequest(row.method, row.path, strings.NewReader(row.body))
			if row.token != "" {
				req.Header.Set("Authorization", "Bearer "+row.token)
			}
			req.Header.Set("Origin", "https://campus.test")
			req.Header.Set("Content-Type", "application/json")
			if row.key != "" {
				req.Header.Set("Idempotency-Key", row.key)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != row.status {
				t.Fatal(w.Code, row.status)
			}
			if row.status != 201 && repo.saves != 0 {
				t.Fatal("invalid input reached write")
			}
			if row.status == 201 && w.Header().Get("Location") != "/api/v1/documents/91/ocr-revisions/2" {
				t.Fatal("revision location")
			}
		})
	}
}
func TestOCRHTTPOriginLengthAndQuotaBeforeBody(t *testing.T) {
	for _, kind := range []string{"origin", "duplicate-origin", "length", "media", "missing-key", "quota"} {
		t.Run(kind, func(t *testing.T) {
			repo := &ocrRepoProbe{}
			limiter := materials.NewLimiter(nil)
			h := OCRHandler(slog.New(slog.NewJSONHandler(io.Discard, nil)), ocrAuthProbe{"teacher"}, repo, "https://campus.test", limiter, http.NotFoundHandler())
			req := httptest.NewRequest("POST", "/api/v1/documents/91/ocr-reviews", strings.NewReader(`{}`))
			req.Header.Set("Authorization", "Bearer probe")
			req.Header.Set("Origin", "https://campus.test")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "valid-review-key01")
			want := 400
			switch kind {
			case "origin":
				req.Header.Set("Origin", "https://untrusted.test")
				want = 403
			case "duplicate-origin":
				req.Header.Add("Origin", "https://campus.test")
				want = 403
			case "length":
				req.ContentLength = OCRMaxRequest + 1
				want = 413
			case "media":
				req.Header.Set("Content-Type", "application/json; charset=latin1")
			case "missing-key":
				req.Header.Del("Idempotency-Key")
			case "quota":
				for i := 0; i < 12; i++ {
					limiter.Allow("1")
				}
				want = 429
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != want || repo.saves != 0 {
				t.Fatal(w.Code, want, repo.saves)
			}
			if want == 429 && w.Header().Get("Retry-After") == "" {
				t.Fatal("missing retry hint")
			}
		})
	}
}
func TestOCRHTTPWholeBodyDeadline(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	for _, kind := range []string{"ocr", "document", "txt"} {
		t.Run(kind, func(t *testing.T) {
			h, repo := ocrProbe("teacher")
			path, media := "/api/v1/documents/91/ocr-reviews", "application/json"
			if kind == "document" {
				path, media = "/api/v1/documents", "multipart/form-data; boundary=slow"
				h = DocumentsHandler(logger, ocrAuthProbe{"teacher"}, &documents.Service{}, "https://campus.test", materials.NewLimiter(nil), http.NotFoundHandler())
			}
			if kind == "txt" {
				path, media = "/api/v1/materials", "multipart/form-data; boundary=slow"
				h = MaterialsHandler(logger, ocrAuthProbe{"teacher"}, &materials.Service{}, "https://campus.test", materials.NewLimiter(nil), http.NotFoundHandler())
			}
			server := httptest.NewTLSServer(h)
			defer server.Close()
			endpoint, _ := url.Parse(server.URL)
			clientTLS := server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			connection, e := tls.Dial("tcp", endpoint.Host, clientTLS)
			if e != nil {
				t.Fatal(e)
			}
			defer connection.Close()
			started := time.Now()
			connection.SetDeadline(started.Add(7 * time.Second))
			fmt.Fprintf(connection, "POST %s HTTP/1.1\r\nHost: campus.test\r\nAuthorization: Bearer probe\r\nOrigin: https://campus.test\r\nIdempotency-Key: valid-review-key01\r\nContent-Type: %s\r\nContent-Length: 10000\r\n\r\nx", path, media)
			response, e := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: "POST"})
			if e != nil {
				t.Fatal("keep-alive response exceeded5s body budget", e)
			}
			defer response.Body.Close()
			raw, _ := io.ReadAll(response.Body)
			if response.StatusCode != 504 || !bytes.Contains(raw, []byte(`"UPSTREAM_TIMEOUT"`)) || repo.saves != 0 || time.Since(started) > 6500*time.Millisecond {
				t.Fatal("incorrect bounded slow body", response.StatusCode, time.Since(started))
			}
		})
	}
}
