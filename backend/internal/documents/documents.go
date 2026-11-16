// Package documents owns DI-02..04 public authorization, input and source rules.
package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
	"github.com/zwh9616/campus-claw-plus/backend/internal/strictjson"
)

const MaxFile = 10485760
const MaxRequest = 10551296

var ErrTooLarge = errors.New("document request too large")

type Document struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Format   string  `json:"format"`
	Version  int     `json:"version"`
	Status   string  `json:"status"`
	Size     int     `json:"size_bytes"`
	Attempts int     `json:"attempts"`
	Pages    *int    `json:"page_count"`
	Failure  *string `json:"failure_reason"`
	Created  string  `json:"created_at"`
	Updated  string  `json:"updated_at"`
}
type Location struct {
	Kind  string `json:"kind"`
	Unit  int    `json:"unit"`
	Start int    `json:"char_start"`
	End   int    `json:"char_end"`
}
type Detail struct {
	Document  Document   `json:"document"`
	Body      *string    `json:"body"`
	Locations []Location `json:"locations"`
	Raw       []byte     `json:"-"`
}
type Page struct {
	Items    []Document `json:"items"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Total    int64      `json:"total"`
}
type Citation struct {
	ID       string   `json:"document_id"`
	ChunkID  string   `json:"chunk_id"`
	Version  int      `json:"document_version"`
	Title    string   `json:"title"`
	Location Location `json:"location"`
	Quote    string   `json:"quote"`
	URL      string   `json:"url"`
}
type Search struct {
	Mode  string     `json:"mode"`
	Items []Citation `json:"items"`
}
type Upload struct {
	Title, Format string
	Raw           []byte
}
type Repository interface {
	ListDocuments(context.Context, identity.Principal, int, int) (Page, error)
	GetDocument(context.Context, identity.Principal, string) (Detail, error)
	ReplayDocument(context.Context, identity.Principal, string, Upload) ([]byte, bool, error)
	CreateDocument(context.Context, identity.Principal, string, Upload) ([]byte, error)
	RetryDocument(context.Context, identity.Principal, string) (Document, error)
	SearchDocuments(context.Context, identity.Principal, string, int) (Search, error)
}
type Service struct {
	Repo    Repository
	Healthy func(context.Context) bool
}

func Readable(p identity.Principal) error { return materials.Readable(p) }
func Writable(p identity.Principal) error { return materials.Writable(p) }
func PrepareUpload(title, filename string, raw []byte) (Upload, error) {
	if len(raw) > MaxFile {
		return Upload{}, ErrTooLarge
	}
	title = strings.TrimSpace(title)
	if !materials.Text(title, 1, 200) || len(raw) < 1 {
		return Upload{}, identity.ErrInvalid
	}
	format := ""
	filename = strings.ToLower(filename)
	if strings.HasSuffix(filename, ".pdf") && bytes.HasPrefix(raw, []byte("%PDF-")) {
		format = "pdf"
	}
	if strings.HasSuffix(filename, ".docx") && len(raw) >= 4 && string(raw[:4]) == "PK\x03\x04" {
		format = "docx"
	}
	if format == "" {
		return Upload{}, identity.ErrInvalid
	}
	return Upload{Title: title, Format: format, Raw: raw}, nil
}
func (s *Service) Create(ctx context.Context, p identity.Principal, key string, u Upload) ([]byte, error) {
	if e := Writable(p); e != nil {
		return nil, e
	}
	if !materials.ValidKey(key) {
		return nil, identity.ErrInvalid
	}
	prior, found, e := s.Repo.ReplayDocument(ctx, p, key, u)
	if e != nil || found {
		return prior, e
	}
	if s.Healthy == nil || !s.Healthy(ctx) {
		// A concurrent commit can make the snapshot available during preflight.
		prior, found, e = s.Repo.ReplayDocument(ctx, p, key, u)
		if e != nil || found {
			return prior, e
		}
		if ctx.Err() != nil {
			return nil, identity.ErrTimeout
		}
		return nil, identity.ErrUnavailable
	}
	return s.Repo.CreateDocument(ctx, p, key, u)
}
func ValidDocument(d Document) bool {
	if !materials.ValidID(d.ID) || !materials.Text(d.Title, 1, 200) || strings.TrimSpace(d.Title) != d.Title || d.Version != 1 || d.Size < 1 || d.Size > MaxFile || d.Attempts < 0 || d.Attempts > 3 {
		return false
	}
	for _, v := range []string{d.Created, d.Updated} {
		t, e := time.Parse("2006-01-02T15:04:05.000Z", v)
		if e != nil || t.UTC().Format("2006-01-02T15:04:05.000Z") != v {
			return false
		}
	}
	if d.Updated < d.Created {
		return false
	}
	switch d.Status {
	case "queued":
		if d.Attempts > 2 {
			return false
		}
	case "processing", "ready", "needs_ocr", "failed":
		if d.Attempts < 1 {
			return false
		}
	default:
		return false
	}
	if d.Format != "pdf" && d.Format != "docx" {
		return false
	}
	if d.Status == "failed" {
		if d.Failure == nil {
			return false
		}
		switch *d.Failure {
		case "corrupt", "encrypted", "unsupported_content", "resource_limit", "parse_timeout", "worker_interrupted", "internal_failure":
		default:
			return false
		}
	} else if d.Failure != nil {
		return false
	}
	if d.Format == "pdf" && (d.Status == "ready" || d.Status == "needs_ocr") {
		if d.Pages == nil || *d.Pages < 1 || *d.Pages > 200 {
			return false
		}
	} else if d.Pages != nil {
		return false
	}
	return d.Format != "docx" || d.Status != "needs_ocr"
}
func ValidDetail(d Detail) bool {
	if !ValidDocument(d.Document) {
		return false
	}
	if d.Document.Status != "ready" {
		return d.Body == nil && len(d.Locations) == 0
	}
	if d.Body == nil || !utf8.ValidString(*d.Body) || len(*d.Body) < 1 || len(*d.Body) > 2097152 || len(d.Locations) < 1 || len(d.Locations) > 20000 {
		return false
	}
	for _, c := range *d.Body {
		if unicode.IsControl(c) && c != '\n' && c != '\t' {
			return false
		}
	}
	text := []rune(*d.Body)
	end, unit := 0, 0
	for i, p := range d.Locations {
		start := end
		if i > 0 {
			if end+2 > len(text) || string(text[end:end+2]) != "\n\n" {
				return false
			}
			start += 2
		}
		if p.Unit <= unit || p.Unit > 20000 || p.Start != start || p.End <= p.Start || p.End > len(text) || strings.TrimSpace(string(text[p.Start:p.End])) == "" {
			return false
		}
		if d.Document.Format == "pdf" && (p.Kind != "pdf_page" || p.Unit > *d.Document.Pages) {
			return false
		}
		if d.Document.Format == "docx" && p.Kind != "docx_paragraph" && p.Kind != "docx_table_cell" {
			return false
		}
		end, unit = p.End, p.Unit
	}
	return end == len(text)
}
func ValidSnapshot(raw []byte, id string, u Upload) bool {
	// The credential decoder deliberately forbids every null. This contract has
	// two nullable fields; retain its duplicate/Unicode checks on a map, then
	// independently require the exact case-sensitive, non-null scalar fields.
	var fields map[string]json.RawMessage
	if strictjson.Decode(raw, &fields) != nil || len(fields) != 11 {
		return false
	}
	for _, name := range []string{"id", "title", "format", "version", "status", "size_bytes", "attempts", "page_count", "failure_reason", "created_at", "updated_at"} {
		v, ok := fields[name]
		if !ok || ((name != "page_count" && name != "failure_reason") && bytes.Equal(bytes.TrimSpace(v), []byte("null"))) {
			return false
		}
	}
	var d Document
	return json.Unmarshal(raw, &d) == nil && ValidDocument(d) && d.ID == id && d.Title == u.Title && d.Format == u.Format && d.Size == len(u.Raw) && d.Status == "queued" && d.Attempts == 0 && d.Created == d.Updated
}
func CheckCitation(c Citation, d Detail, ordinal int) bool {
	if !ValidDetail(d) || d.Document.Status != "ready" || c.ID != d.Document.ID || c.Version != d.Document.Version || c.Title != d.Document.Title || !materials.ValidID(c.ChunkID) || c.URL != "/api/v1/documents/"+c.ID || ordinal < 1 {
		return false
	}
	p := c.Location
	for _, source := range d.Locations {
		if source.Kind != p.Kind || source.Unit != p.Unit {
			continue
		}
		start := source.Start + (ordinal-1)*1800
		end := min(start+2000, source.End)
		if start < source.Start || start >= source.End || p.Start != start || p.End != end {
			return false
		}
		return c.Quote == string([]rune(*d.Body)[start:end])
	}
	return false
}
