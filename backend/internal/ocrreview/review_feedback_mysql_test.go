//go:build ocrreview

package ocrreview

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
)

// DI-SC-18/19: actual published MySQL chunks must cover the full allowed query
// at chunk boundaries, with codepoint-based immutable citations.
func TestMySQLReviewBoundaryKeywords(t *testing.T) {
	for i, query := range []string{"合同", "😀合同", strings.Repeat("界", 199) + "😀"} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			s, p, in := setup(t)
			ctx := context.Background()
			in.Pages[0].Text = strings.Repeat("😀", 511) + query + "后续文字"
			if _, e := s.Save(ctx, p, "91", "boundary-review-001", in); e != nil {
				t.Fatal(e)
			}
			student := p
			student.Role = "student"
			found, e := s.Search(ctx, student, query, 20)
			if e != nil || len(found) == 0 {
				t.Fatal("published boundary keyword missing", e)
			}
			for _, c := range found {
				persisted, e := s.Read(ctx, student, c.DocumentID, c.Revision)
				if e != nil || c.Revision != 2 || c.Page != 1 || !strings.Contains(c.Quote, query) || string([]rune(persisted.Body)[c.Start:c.End]) != c.Quote {
					t.Fatal("immutable codepoint citation mismatch", c, e)
				}
			}
			other := student
			other.ClassID = "2"
			if hits, e := s.Search(ctx, other, query, 20); e != nil || len(hits) != 0 {
				t.Fatal("cross-class boundary hit", hits, e)
			}
			in.BaseRevision = 2
			in.Pages[0].Text = "新版正文"
			if _, e := s.Save(ctx, p, "91", "boundary-review-002", in); e != nil {
				t.Fatal(e)
			}
			if hits, e := s.Search(ctx, student, query, 20); e != nil || len(hits) != 0 {
				t.Fatal("old boundary index leaked", hits, e)
			}
			if old, e := s.Read(ctx, student, "91", 2); e != nil || !strings.Contains(old.Body, query) {
				t.Fatal("old source changed", e)
			}
		})
	}
}

// DI-SC-18: same user/class/key on distinct documents is independent;
// concurrent replays for each document must publish exactly one revision.
func TestMySQLReviewDocumentScopedIdempotency(t *testing.T) {
	s, p, in := setup(t)
	ctx := context.Background()
	if _, e := s.DB.Exec("INSERT INTO documents(id,class_id,created_by,title,format,version,status,original_bytes,original_digest,size_bytes,attempts,page_count,created_at,updated_at) VALUES(92,1,1,'另一原件','pdf',1,'needs_ocr','def',UNHEX(SHA2('def',256)),3,1,2,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))"); e != nil {
		t.Fatal(e)
	}
	if e := s.seedDraft(ctx, "1", "92", []Page{{1, "另一个原稿"}, {2, "第二页"}}); e != nil {
		t.Fatal(e)
	}
	type result struct {
		id  string
		raw []byte
		err error
	}
	out := make(chan result, 20)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		id := "91"
		if i%2 == 1 {
			id = "92"
		}
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			raw, e := s.Save(ctx, p, id, "scoped-review-key01", in)
			out <- result{id, raw, e}
		}(id)
	}
	wg.Wait()
	close(out)
	first := map[string][]byte{}
	for r := range out {
		var receipt Receipt
		if r.err != nil || json.Unmarshal(r.raw, &receipt) != nil || receipt.DocumentID != r.id || receipt.Revision != 2 {
			t.Fatal("document-scoped concurrent write failed", r.id, r.err, string(r.raw))
		}
		if prior := first[r.id]; prior != nil && !bytes.Equal(prior, r.raw) {
			t.Fatal("replay changed")
		}
		first[r.id] = r.raw
	}
	for table, want := range map[string]int{"document_ocr_revisions": 4, "document_ocr_review_keys": 2, "document_ocr_chunks": 4} {
		var n int
		if e := s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != want {
			t.Fatal("unexpected side effects", table, n, e)
		}
	}
	in.Pages[0].Text = "changed"
	for _, id := range []string{"91", "92"} {
		_, e := s.Save(ctx, p, id, "scoped-review-key01", in)
		requireErr(t, e, identity.ErrConflict)
	}
}
