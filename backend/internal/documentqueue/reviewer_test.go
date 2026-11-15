package documentqueue

import (
	"strings"
	"testing"
)

// DI-SC-07/08: the storage boundary must reject out-of-contract inputs even
// before the future authorized HTTP adapter calls it.
func TestReviewerUploadBoundaries(t *testing.T) {
	base := Upload{UserID: 1, ClassID: 1, Key: strings.Repeat("a", 16), Title: "课程😀", Format: "pdf", Original: []byte("source")}
	for _, change := range []struct {
		name string
		edit func(*Upload)
	}{
		{"empty user", func(u *Upload) { u.UserID = 0 }},
		{"unsigned class overflow", func(u *Upload) { u.ClassID = 1 << 63 }},
		{"short key", func(u *Upload) { u.Key = strings.Repeat("a", 15) }},
		{"long key", func(u *Upload) { u.Key = strings.Repeat("a", 65) }},
		{"Unicode key", func(u *Upload) { u.Key = strings.Repeat("a", 15) + "😀" }},
		{"path key", func(u *Upload) { u.Key = "path/" + strings.Repeat("a", 16) }},
		{"blank title", func(u *Upload) { u.Title = "" }},
		{"padded title", func(u *Upload) { u.Title = " 课程" }},
		{"invalid UTF8 title", func(u *Upload) { u.Title = string([]byte{0xff}) }},
		{"control title", func(u *Upload) { u.Title = "课程\n" }},
		{"too many title codepoints", func(u *Upload) { u.Title = strings.Repeat("😀", 201) }},
		{"empty source", func(u *Upload) { u.Original = nil }},
		{"oversize source", func(u *Upload) { u.Original = make([]byte, 10485761) }},
		{"open format", func(u *Upload) { u.Format = "txt" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			u := base
			change.edit(&u)
			if validUpload(u) {
				t.Fatal("invalid upload accepted")
			}
		})
	}
	base.Key = strings.Repeat("_", 64)
	base.Title = strings.Repeat("😀", 200)
	base.Original = make([]byte, 10485760)
	for _, format := range []string{"pdf", "docx"} {
		base.Format = format
		if !validUpload(base) {
			t.Fatal("valid exact boundary rejected", format)
		}
	}
}

// DI-SC-08: payload equality preserves byte and field boundaries; the SQL key
// separately scopes this digest to the authorized user and class.
func TestReviewerDigestPreservesBoundaries(t *testing.T) {
	a := Upload{Format: "pdf", Title: "ab", Original: []byte("c")}
	b := Upload{Format: "pdf", Title: "a", Original: []byte("bc")}
	if digest(a) == digest(b) {
		t.Fatal("field concatenation collision")
	}
	b = a
	b.Format = "docx"
	if digest(a) == digest(b) {
		t.Fatal("format omitted from payload identity")
	}
	b = a
	b.Original = []byte("c\r\n")
	if digest(a) == digest(b) {
		t.Fatal("original bytes normalized")
	}
	b = a
	b.Original = append([]byte(nil), a.Original...)
	if digest(a) != digest(b) {
		t.Fatal("equal bytes changed identity")
	}
}
