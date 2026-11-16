package ocrreview

import (
	"strings"
	"testing"
)

func TestContentCodepointsAndBounds(t *testing.T) {
	body, e := content([]Page{{1, "课程😀"}, {2, "第二页"}})
	if e != nil || body != "课程😀\n\n第二页" {
		t.Fatal(body, e)
	}
	for _, pages := range [][]Page{nil, {{2, "wrong"}}, {{1, ""}}, {{1, string([]byte{255})}}, {{1, strings.Repeat("中", 700000)}}} {
		if _, e = content(pages); e == nil {
			t.Fatal("accepted invalid pages")
		}
	}
}
