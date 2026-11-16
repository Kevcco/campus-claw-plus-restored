package ocrreview

import (
	"strings"
	"testing"
)

const validSource = `{"status":"needs_review","engine":"tesseract-5.5.4-fast-8741641","body":"课程😀","quality":"acceptable_candidate","pages":[{"page":1,"width":1000,"height":500,"rotation_applied":0,"source":"ocr","char_start":0,"char_end":3,"words":[{"text":"课程😀","char_start":0,"char_end":3,"bbox":[0.1,0.2,0.3,0.4],"engine_confidence":95}]}]}`

func TestSourceStrictCodepointsAndBounds(t *testing.T) {
	s, pages, e := DecodeSource([]byte(validSource))
	if e != nil || s.Body != "课程😀" || len(pages) != 1 || pages[0].Text != "课程😀" {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(validSource, `"status":`, `"status":"ready","status":`, 1), strings.Replace(validSource, `"status"`, `"Status"`, 1), strings.Replace(validSource, `"width":1000`, `"width":20000000`, 1), strings.Replace(validSource, `"char_end":3`, `"char_end":4`, 1), strings.Replace(validSource, `"engine_confidence":95`, `"engine_confidence":null`, 1), strings.Replace(validSource, `,"engine_confidence":95`, "", 1), strings.Replace(validSource, `[0.1,0.2,0.3,0.4]`, `[0.1,0.2,0.3,0.4,0.5]`, 1), strings.Replace(validSource, `[0.1,0.2,0.3,0.4]`, `[0.9,0.2,0.3,0.4]`, 1), strings.Replace(validSource, `"text":"课程😀"`, `"text":"伪造出处"`, 1), strings.Replace(validSource, `"rotation_applied":0`, `"rotation_applied":45`, 1)} {
		if _, _, e = DecodeSource([]byte(bad)); e == nil {
			t.Fatal("bad source accepted")
		}
	}
}
