package documents

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
)

func sample() Detail {
	return Detail{Document: Document{ID: "9223372036854775807", Title: "课程", Format: "docx", Version: 1, Status: "ready", Size: 5, Attempts: 1, Created: "2026-11-15T00:00:00.000Z", Updated: "2026-11-15T00:00:01.100Z"}, Body: ptr("甲😀\n\n乙"), Locations: []Location{{"docx_paragraph", 1, 0, 2}, {"docx_table_cell", 3, 4, 5}}}
}
func ptr[T any](v T) *T { return &v }
func TestDIInputBoundariesAndSourceSignatures(t *testing.T) {
	for _, v := range []struct {
		name  string
		bytes []byte
	}{{"LECTURE.PDF", []byte("%PDF-")}, {"x.docx", []byte("PK\x03\x04")}} {
		u, e := PrepareUpload("\u0085课程 \u0085", v.name, v.bytes)
		if e != nil || u.Title != "课程" {
			t.Fatal("valid format/title")
		}
	}
	for _, v := range []struct {
		title, name string
		bytes       []byte
	}{{"", "x.pdf", []byte("%PDF-")}, {"a", "x.doc", []byte("PK\x03\x04")}, {"a", "x.docm", []byte("PK\x03\x04")}, {"a", "x.docx", []byte("%PDF-")}, {"a", "x.pdf", nil}, {"a", "x.pdf", []byte("fake")}, {"x\x00", "x.pdf", []byte("%PDF-")}, {strings.Repeat("题", 201), "x.pdf", []byte("%PDF-")}} {
		if _, e := PrepareUpload(v.title, v.name, v.bytes); e == nil {
			t.Fatal("bad input accepted")
		}
	}
	valid := append([]byte("%PDF-"), make([]byte, MaxFile-5)...)
	if _, e := PrepareUpload("title", "x.pdf", valid); e != nil {
		t.Fatal("exact byte maximum")
	}
	if _, e := PrepareUpload("title", "x.pdf", append(valid, 0)); e != ErrTooLarge {
		t.Fatal("maximum+1")
	}
}
func TestDIUnicodeLocationsSnapshotAndCorruption(t *testing.T) {
	d := sample()
	if !ValidDetail(d) {
		t.Fatal("nonBMP and logical units")
	}
	c := Citation{ID: d.Document.ID, ChunkID: "2", Version: 1, Title: d.Document.Title, Location: d.Locations[0], Quote: "甲😀", URL: "/api/v1/documents/" + d.Document.ID}
	if !CheckCitation(c, d, 1) {
		t.Fatal("exact source")
	}
	c.Quote = "甲�"
	if CheckCitation(c, d, 1) {
		t.Fatal("wrong quote")
	}
	for _, mutate := range []func(*Detail){func(d *Detail) { d.Locations[0].End++ }, func(d *Detail) { d.Locations[1].Start-- }, func(d *Detail) { d.Locations[1].Unit = 1 }, func(d *Detail) { d.Body = ptr("甲\r\n乙") }, func(d *Detail) { d.Document.Format = "pdf" }, func(d *Detail) { d.Document.Status = "failed" }, func(d *Detail) { d.Document.Created = "2026-11-15T00:00:00Z" }, func(d *Detail) { d.Document.Updated = "2026-11-15T00:00:01.1Z" }} {
		copy := sample()
		mutate(&copy)
		if ValidDetail(copy) {
			t.Fatal("corrupt source accepted")
		}
	}
	q := d.Document
	q.Status = "queued"
	q.Attempts = 0
	q.Updated = q.Created
	u := Upload{Title: q.Title, Format: q.Format, Raw: make([]byte, q.Size)}
	raw, _ := json.Marshal(q)
	if !ValidSnapshot(raw, q.ID, u) {
		t.Fatal("original202")
	}
	for _, raw := range [][]byte{[]byte(strings.Replace(string(raw), ".000Z", "Z", 1)), []byte(strings.Replace(string(raw), `"attempts":0`, `"attempts":null`, 1)), []byte(strings.TrimSuffix(string(raw), "}") + `,"unknown":1}`)} {
		if ValidSnapshot(raw, q.ID, u) {
			t.Fatal("malformed original202 accepted")
		}
	}
}

type replayRepo struct {
	Repository
	calls *[]string
	found bool
}

func (r replayRepo) ReplayDocument(context.Context, identity.Principal, string, Upload) ([]byte, bool, error) {
	*r.calls = append(*r.calls, "replay")
	return []byte("original202"), r.found, nil
}
func (r replayRepo) CreateDocument(context.Context, identity.Principal, string, Upload) ([]byte, error) {
	*r.calls = append(*r.calls, "transaction")
	return []byte("new202"), nil
}
func TestDIReplayAuthorizationAndPreflightOrder(t *testing.T) {
	p := identity.Principal{UserID: "1", ClassID: "1", SessionID: "session", Role: "teacher"}
	for _, found := range []bool{true, false} {
		calls := []string{}
		s := Service{Repo: replayRepo{calls: &calls, found: found}, Healthy: func(context.Context) bool { calls = append(calls, "preflight"); return true }}
		_, e := s.Create(context.Background(), p, "valid_upload_key", Upload{})
		want := []string{"replay"}
		if !found {
			want = append(want, "preflight", "transaction")
		}
		if e != nil || !reflect.DeepEqual(calls, want) {
			t.Fatal("replay must precede any preflight or transaction")
		}
	}
	calls := []string{}
	s := Service{Repo: replayRepo{calls: &calls}, Healthy: func(context.Context) bool { calls = append(calls, "preflight"); return false }}
	if _, e := s.Create(context.Background(), p, "valid_upload_key", Upload{}); e != identity.ErrUnavailable || !reflect.DeepEqual(calls, []string{"replay", "preflight", "replay"}) {
		t.Fatal("outage created data")
	}
	p.Role = "student"
	calls = nil
	if _, e := s.Create(context.Background(), p, "valid_upload_key", Upload{}); e != identity.ErrForbidden || len(calls) != 0 {
		t.Fatal("authorization before all side effects")
	}
}
