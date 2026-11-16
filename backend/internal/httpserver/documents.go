package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/zwh9616/campus-claw-plus/backend/internal/documents"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
)

func documentRoute(path string) (kind, id, methods string) {
	if path == "/api/v1/documents" {
		return "collection", "", "GET, POST"
	}
	if path == "/api/v1/document-search" {
		return "search", "", "GET"
	}
	if !strings.HasPrefix(path, "/api/v1/documents/") {
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/documents/"), "/")
	if parts[0] == "" {
		return
	}
	if len(parts) == 1 {
		return "detail", parts[0], "GET"
	}
	if len(parts) == 2 && parts[1] == "download" {
		return "download", parts[0], "GET"
	}
	if len(parts) == 2 && parts[1] == "retry" {
		return "retry", parts[0], "POST"
	}
	return
}
func documentUpload(r *http.Request) (documents.Upload, error) {
	if len(r.Header.Values("Content-Type")) != 1 {
		return documents.Upload{}, identity.ErrInvalid
	}
	media, params, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "multipart/form-data" || params["boundary"] == "" {
		return documents.Upload{}, identity.ErrInvalid
	}
	if r.ContentLength > documents.MaxRequest {
		return documents.Upload{}, documents.ErrTooLarge
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, documents.MaxRequest+1))
	if len(raw) > documents.MaxRequest {
		return documents.Upload{}, documents.ErrTooLarge
	}
	if e != nil {
		return documents.Upload{}, requestReadError(r.Context(), e)
	}
	reader := multipart.NewReader(bytes.NewReader(raw), params["boundary"])
	seen := map[string]bool{}
	var title, filename string
	var file []byte
	for {
		part, e := reader.NextRawPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			return documents.Upload{}, identity.ErrInvalid
		}
		name := part.FormName()
		if (name != "title" && name != "file") || seen[name] || part.Header.Get("Content-Transfer-Encoding") != "" || len(part.Header.Values("Content-Disposition")) != 1 {
			part.Close()
			return documents.Upload{}, identity.ErrInvalid
		}
		seen[name] = true
		content, e := io.ReadAll(part)
		part.Close()
		if e != nil {
			return documents.Upload{}, identity.ErrInvalid
		}
		if name == "title" {
			if part.FileName() != "" {
				return documents.Upload{}, identity.ErrInvalid
			}
			title = string(content)
		} else {
			filename, file = part.FileName(), content
		}
	}
	if !seen["title"] || !seen["file"] {
		return documents.Upload{}, identity.ErrInvalid
	}
	return documents.PrepareUpload(title, filename, file)
}

// DocumentsHandler supplies only the six approved DI operations. Authentication,
// action and resource checks precede business input; no parser call enters SQL.
func DocumentsHandler(logger *slog.Logger, auth materialAuthenticator, service *documents.Service, origin string, limiter *materials.Limiter, fallback http.Handler) http.Handler {
	handler := Observe(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind, id, methods := documentRoute(r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		r, cancel := businessRequest(w, r)
		defer cancel()
		ctx := r.Context()
		if !strings.Contains(","+strings.ReplaceAll(methods, " ", "")+",", ","+r.Method+",") {
			w.Header().Set("Allow", methods)
			w.WriteHeader(405)
			return
		}
		fail := func(e error) {
			if errors.Is(e, documents.ErrTooLarge) {
				writeIdentityCode(w, 413, "PAYLOAD_TOO_LARGE", "文件或请求过大")
			} else {
				writeIdentityError(w, e)
			}
		}
		tokens := r.Header.Values("Authorization")
		token := ""
		if len(tokens) == 1 && strings.HasPrefix(tokens[0], "Bearer ") {
			token = strings.TrimPrefix(tokens[0], "Bearer ")
		}
		p, e := auth.Me(ctx, token)
		if e != nil {
			fail(e)
			return
		}
		if e = documents.Readable(p); e != nil {
			fail(e)
			return
		}
		if r.Method == "POST" {
			if e = documents.Writable(p); e != nil {
				fail(e)
				return
			}
			origins := r.Header.Values("Origin")
			if len(origins) > 1 || (len(origins) == 1 && origins[0] != origin) {
				fail(identity.ErrForbidden)
				return
			}
		}
		var detail documents.Detail
		if id != "" {
			if !materials.ValidID(id) {
				fail(identity.ErrInvalid)
				return
			}
			detail, e = service.Repo.GetDocument(ctx, p, id)
			if e != nil {
				fail(e)
				return
			}
		}
		if r.Method == "POST" {
			if allowed, retry := limiter.Allow(p.UserID); !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				writeIdentityCode(w, 429, "RATE_LIMITED", "请稍后重试")
				return
			}
		}
		if r.Method == "GET" || kind == "retry" {
			if e = emptyMaterialBody(r); e != nil {
				if ctx.Err() != nil {
					e = identity.ErrTimeout
				}
				fail(e)
				return
			}
		}
		switch kind {
		case "collection":
			if r.Method == "GET" {
				q, e := queryValues(r, "page", "page_size")
				if e != nil {
					fail(e)
					return
				}
				page, e := integerQuery(q, "page", 1, 2147483647)
				if e != nil {
					fail(e)
					return
				}
				size, e := integerQuery(q, "page_size", 20, 50)
				if e != nil {
					fail(e)
					return
				}
				out, e := service.Repo.ListDocuments(ctx, p, page, size)
				if e != nil {
					fail(e)
					return
				}
				json.NewEncoder(w).Encode(out)
				return
			}
			if _, e = queryValues(r); e != nil {
				fail(e)
				return
			}
			keys := r.Header.Values("Idempotency-Key")
			if len(keys) != 1 || !materials.ValidKey(keys[0]) {
				fail(identity.ErrInvalid)
				return
			}
			u, e := documentUpload(r)
			if e != nil {
				fail(e)
				return
			}
			out, e := service.Create(ctx, p, keys[0], u)
			if e != nil {
				fail(e)
				return
			}
			var accepted documents.Document
			if json.Unmarshal(out, &accepted) != nil || !materials.ValidID(accepted.ID) {
				fail(identity.ErrUnavailable)
				return
			}
			w.Header().Set("Location", "/api/v1/documents/"+accepted.ID)
			w.WriteHeader(202)
			w.Write(out)
		case "detail", "download", "retry":
			if _, e = queryValues(r); e != nil {
				fail(e)
				return
			}
			if kind == "download" {
				media := "application/pdf"
				if detail.Document.Format == "docx" {
					media = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
				}
				w.Header().Set("Content-Type", media)
				w.Header().Set("Content-Disposition", `attachment; filename="document-`+id+`.`+detail.Document.Format+`"`)
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Write(detail.Raw)
				return
			}
			if kind == "retry" {
				out, e := service.Repo.RetryDocument(ctx, p, id)
				if e != nil {
					fail(e)
					return
				}
				w.Header().Set("Location", "/api/v1/documents/"+id)
				w.WriteHeader(202)
				json.NewEncoder(w).Encode(out)
				return
			}
			json.NewEncoder(w).Encode(detail)
		case "search":
			q, e := queryValues(r, "q", "mode", "top_k")
			if e != nil {
				fail(e)
				return
			}
			text := strings.TrimSpace(q.Get("q"))
			if !materials.Text(text, 1, 2000) || (q.Has("mode") && q.Get("mode") != "keyword") {
				fail(identity.ErrInvalid)
				return
			}
			top, e := integerQuery(q, "top_k", 4, 10)
			if e != nil {
				fail(e)
				return
			}
			out, e := service.Repo.SearchDocuments(ctx, p, text, top)
			if e != nil {
				fail(e)
				return
			}
			json.NewEncoder(w).Encode(out)
		}
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if kind, _, _ := documentRoute(r.URL.Path); kind == "" {
			fallback.ServeHTTP(w, r)
		} else {
			handler.ServeHTTP(w, r)
		}
	})
}
