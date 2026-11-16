package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
	"github.com/zwh9616/campus-claw-plus/backend/internal/ocrreview"
	"github.com/zwh9616/campus-claw-plus/backend/internal/strictjson"
)

const OCRMaxRequest = 6*ocrreview.MaxBody + 65536

type OCRRepository interface {
	Status(context.Context, identity.Principal, string) (ocrreview.Status, error)
	Read(context.Context, identity.Principal, string, int) (ocrreview.Revision, error)
	ReadSource(context.Context, identity.Principal, string) ([]byte, error)
	Save(context.Context, identity.Principal, string, string, ocrreview.Review) ([]byte, error)
	Search(context.Context, identity.Principal, string, int) ([]ocrreview.Citation, error)
	EnqueueOCR(context.Context, identity.Principal, string) error
	RetryOCR(context.Context, identity.Principal, string) error
}

func ocrRoute(path string) (kind, id, revision, method string) {
	if path == "/api/v1/ocr-search" {
		return "search", "", "", "GET"
	}
	if !strings.HasPrefix(path, "/api/v1/documents/") {
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/documents/"), "/")
	if len(parts) == 3 && parts[1] == "ocr-revisions" && parts[2] != "" {
		return "revision", parts[0], parts[2], "GET"
	}
	if len(parts) != 2 || parts[0] == "" {
		return
	}
	switch parts[1] {
	case "ocr":
		return "status", parts[0], "", "GET"
	case "ocr-source":
		return "source", parts[0], "", "GET"
	case "ocr-reviews":
		return "review", parts[0], "", "POST"
	case "ocr-jobs":
		return "start", parts[0], "", "POST"
	case "ocr-retry":
		return "retry", parts[0], "", "POST"
	}
	return
}

// OCRHandler is an explicit component boundary, not installed by cmd/server.
// Production activation must register operations and require migration7 first.
func OCRHandler(logger *slog.Logger, auth materialAuthenticator, repo OCRRepository, origin string, limiter *materials.Limiter, fallback http.Handler) http.Handler {
	handler := Observe(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind, id, rawRevision, method := ocrRoute(r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		r, cancel := businessRequest(w, r)
		defer cancel()
		ctx := r.Context()
		if r.Method != method {
			w.Header().Set("Allow", method)
			w.WriteHeader(405)
			return
		}
		fail := func(e error) { writeIdentityError(w, e) }
		token := ""
		if v := r.Header.Values("Authorization"); len(v) == 1 && strings.HasPrefix(v[0], "Bearer ") {
			token = strings.TrimPrefix(v[0], "Bearer ")
		}
		p, e := auth.Me(ctx, token)
		if e != nil {
			fail(e)
			return
		}
		if e = materials.Readable(p); e != nil {
			fail(e)
			return
		}
		if r.Method == "POST" || kind == "source" {
			if e = materials.Writable(p); e != nil {
				fail(e)
				return
			}
		}
		if r.Method == "POST" {
			v := r.Header.Values("Origin")
			if len(v) > 1 || (len(v) == 1 && v[0] != origin) {
				fail(identity.ErrForbidden)
				return
			}
		}
		var state ocrreview.Status
		if id != "" {
			if !materials.ValidID(id) {
				fail(identity.ErrInvalid)
				return
			}
			state, e = repo.Status(ctx, p, id)
			if e != nil {
				fail(e)
				return
			}
		}
		if r.Method == "POST" {
			if ok, retry := limiter.Allow(p.UserID); !ok {
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				writeIdentityCode(w, 429, "RATE_LIMITED", "请稍后重试")
				return
			}
		}
		if kind != "review" {
			if e = emptyMaterialBody(r); e != nil {
				fail(requestReadError(ctx, e))
				return
			}
		}
		if kind != "search" {
			if _, e = queryValues(r); e != nil {
				fail(e)
				return
			}
		}
		switch kind {
		case "status":
			json.NewEncoder(w).Encode(state)
		case "source":
			raw, e := repo.ReadSource(ctx, p, id)
			if e != nil {
				fail(e)
				return
			}
			w.Write(raw)
		case "revision":
			n, e := strconv.ParseInt(rawRevision, 10, 32)
			if e != nil || n < 1 || strconv.FormatInt(n, 10) != rawRevision {
				fail(identity.ErrInvalid)
				return
			}
			out, e := repo.Read(ctx, p, id, int(n))
			if e != nil {
				fail(e)
				return
			}
			json.NewEncoder(w).Encode(out)
		case "review":
			keys := r.Header.Values("Idempotency-Key")
			if len(keys) != 1 || !materials.ValidKey(keys[0]) {
				fail(identity.ErrInvalid)
				return
			}
			if len(r.Header.Values("Content-Type")) != 1 {
				fail(identity.ErrInvalid)
				return
			}
			media, params, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if e != nil || media != "application/json" || len(params) > 1 || (len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8")) {
				fail(identity.ErrInvalid)
				return
			}
			if r.ContentLength > OCRMaxRequest {
				writeIdentityCode(w, 413, "PAYLOAD_TOO_LARGE", "请求过大")
				return
			}
			raw, e := io.ReadAll(io.LimitReader(r.Body, OCRMaxRequest+1))
			if e != nil {
				fail(requestReadError(ctx, e))
				return
			}
			if len(raw) > OCRMaxRequest {
				writeIdentityCode(w, 413, "PAYLOAD_TOO_LARGE", "请求过大")
				return
			}
			var input ocrreview.Review
			if strictjson.Decode(raw, &input) != nil {
				fail(identity.ErrInvalid)
				return
			}
			out, e := repo.Save(ctx, p, id, keys[0], input)
			if e != nil {
				fail(e)
				return
			}
			var receipt ocrreview.Receipt
			if json.Unmarshal(out, &receipt) != nil || receipt.DocumentID != id || receipt.Revision < 2 || receipt.Status != "ready" {
				fail(identity.ErrUnavailable)
				return
			}
			w.Header().Set("Location", r.URL.Path[:strings.LastIndex(r.URL.Path, "/")]+"/ocr-revisions/"+strconv.Itoa(receipt.Revision))
			w.WriteHeader(201)
			w.Write(out)
		case "start", "retry":
			if kind == "start" {
				e = repo.EnqueueOCR(ctx, p, id)
			} else {
				e = repo.RetryOCR(ctx, p, id)
			}
			if e != nil {
				fail(e)
				return
			}
			w.Header().Set("Location", "/api/v1/documents/"+id+"/ocr")
			w.WriteHeader(202)
			json.NewEncoder(w).Encode(struct {
				DocumentID string `json:"document_id"`
				Status     string `json:"status"`
			}{id, "accepted"})
		case "search":
			q, e := queryValues(r, "q", "mode", "top_k")
			if e != nil {
				fail(e)
				return
			}
			text := strings.TrimSpace(q.Get("q"))
			if !materials.Text(text, 1, ocrreview.MaxQuery) || (q.Has("mode") && q.Get("mode") != "keyword") {
				fail(identity.ErrInvalid)
				return
			}
			top, e := integerQuery(q, "top_k", 4, 10)
			if e != nil {
				fail(e)
				return
			}
			items, e := repo.Search(ctx, p, text, top)
			if e != nil {
				fail(e)
				return
			}
			type citation struct {
				ocrreview.Citation
				URL string `json:"url"`
			}
			out := []citation{}
			for _, c := range items {
				out = append(out, citation{c, "/api/v1/documents/" + c.DocumentID + "/ocr-revisions/" + strconv.Itoa(c.Revision)})
			}
			json.NewEncoder(w).Encode(struct {
				Items []citation `json:"items"`
				Mode  string     `json:"mode"`
			}{out, "keyword"})
		}
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if kind, _, _, _ := ocrRoute(r.URL.Path); kind == "" {
			fallback.ServeHTTP(w, r)
		} else {
			handler.ServeHTTP(w, r)
		}
	})
}
