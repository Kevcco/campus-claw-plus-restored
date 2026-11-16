package ocrreview

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"math"
	"time"
	"unicode"

	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
	"github.com/zwh9616/campus-claw-plus/backend/internal/strictjson"
)

type SourceWord struct {
	Text       string    `json:"text"`
	Start      *int      `json:"char_start"`
	End        int       `json:"char_end"`
	Box        []float64 `json:"bbox"`
	Confidence *float64  `json:"engine_confidence"`
}
type SourcePage struct {
	Page     int          `json:"page"`
	Width    int          `json:"width"`
	Height   int          `json:"height"`
	Rotation *int         `json:"rotation_applied"`
	Source   string       `json:"source"`
	Start    *int         `json:"char_start"`
	End      int          `json:"char_end"`
	Words    []SourceWord `json:"words"`
}
type Source struct {
	Status  string       `json:"status"`
	Engine  string       `json:"engine"`
	Body    string       `json:"body"`
	Quality string       `json:"quality"`
	Pages   []SourcePage `json:"pages"`
}

func DecodeSource(raw []byte) (s Source, pages []Page, err error) {
	invalid := func() (Source, []Page, error) { return Source{}, nil, identity.ErrInvalid }
	if len(raw) == 0 || len(raw) > 8388608 || strictjson.Decode(raw, &s) != nil {
		return invalid()
	}
	if s.Status != "needs_review" || s.Engine != "tesseract-5.5.4-fast-8741641" || (s.Quality != "low_quality" && s.Quality != "acceptable_candidate") || len(s.Body) == 0 || len(s.Body) > 2097152 || len(s.Pages) < 1 || len(s.Pages) > 100 {
		return invalid()
	}
	for _, r := range s.Body {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return invalid()
		}
	}
	body := []rune(s.Body)
	last, totalWords := 0, 0
	for i, p := range s.Pages {
		if p.Page != i+1 || p.Width < 1 || p.Height < 1 || p.Width > 20000000 || p.Height > 20000000 || int64(p.Width)*int64(p.Height) > 20000000 || p.Start == nil || p.Rotation == nil || (*p.Rotation != 0 && *p.Rotation != 90 && *p.Rotation != 180 && *p.Rotation != 270) || p.Source != "ocr" || p.Words == nil || len(p.Words) < 1 || len(p.Words) > 20000 {
			return invalid()
		}
		start := *p.Start
		if i == 0 && start != 0 {
			return invalid()
		}
		if i > 0 && (start != last+2 || start > len(body) || string(body[last:start]) != "\n\n") {
			return invalid()
		}
		if start < 0 || p.End <= start || p.End > len(body) {
			return invalid()
		}
		wordEnd := start
		for _, w := range p.Words {
			if w.Start == nil || *w.Start < wordEnd || w.End <= *w.Start || w.End > p.End || len([]rune(w.Text)) > 2000 || string(body[*w.Start:w.End]) != w.Text || len(w.Box) != 4 || w.Confidence == nil || math.IsNaN(*w.Confidence) || math.IsInf(*w.Confidence, 0) || *w.Confidence < 0 || *w.Confidence > 100 {
				return invalid()
			}
			for _, n := range w.Box {
				if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1 {
					return invalid()
				}
			}
			if w.Box[0] >= w.Box[2] || w.Box[1] >= w.Box[3] {
				return invalid()
			}
			wordEnd = w.End
			totalWords++
			if totalWords > 100000 {
				return invalid()
			}
		}
		pages = append(pages, Page{p.Page, string(body[start:p.End])})
		last = p.End
	}
	if last != len(body) {
		return invalid()
	}
	return s, pages, nil
}

// ReadSource is teacher-only. Corrected revisions never overwrite this byte
// snapshot, so original OCR text, confidence and input-raster boxes survive.
func (s *Store) ReadSource(ctx context.Context, p identity.Principal, id string) ([]byte, error) {
	if e := materials.Writable(p); e != nil {
		return nil, e
	}
	if !materials.ValidID(id) || !materials.ValidID(p.ClassID) || !materials.ValidID(p.UserID) {
		return nil, identity.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var raw, digest, original, current []byte
	e := s.DB.QueryRowContext(ctx, "SELECT s.source_bytes,s.source_digest,s.original_digest,d.original_digest FROM document_ocr_sources s JOIN documents d ON d.id=s.document_id AND d.class_id=s.class_id WHERE s.document_id=? AND s.class_id=?", id, p.ClassID).Scan(&raw, &digest, &original, &current)
	if e != nil {
		return nil, failure(ctx, e)
	}
	hash := sha256.Sum256(raw)
	if string(hash[:]) != string(digest) || string(original) != string(current) {
		return nil, identity.ErrUnavailable
	}
	if _, _, e = DecodeSource(raw); e != nil {
		return nil, identity.ErrUnavailable
	}
	return raw, nil
}

func insertSource(ctx context.Context, tx *sql.Tx, id, classID string, raw, originalDigest []byte, decoded Source, pages []Page) error {
	pageJSON, _ := json.Marshal(pages)
	if _, e := tx.ExecContext(ctx, "INSERT INTO document_ocr_revisions(document_id,class_id,revision,kind,body,pages,created_at) VALUES(?,?,1,'ocr',?,?,UTC_TIMESTAMP(3))", id, classID, decoded.Body, pageJSON); e != nil {
		return e
	}
	if _, e := tx.ExecContext(ctx, "INSERT INTO document_ocr_heads(document_id,class_id,latest_revision) VALUES(?,?,1)", id, classID); e != nil {
		return e
	}
	hash := sha256.Sum256(raw)
	_, e := tx.ExecContext(ctx, "INSERT INTO document_ocr_sources(document_id,class_id,source_bytes,source_digest,original_digest) VALUES(?,?,?,?,?)", id, classID, raw, hash[:], originalDigest)
	return e
}
