package privateocr

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const source = `{"status":"needs_review","engine":"tesseract-5.5.4-fast-8741641","body":"课程","quality":"acceptable_candidate","pages":[{"page":1,"width":100,"height":100,"rotation_applied":0,"source":"ocr","char_start":0,"char_end":2,"words":[{"text":"课程","char_start":0,"char_end":2,"bbox":[0.1,0.1,0.8,0.8],"engine_confidence":95}]}]}`

func client(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	token := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	c, e := New(Origin, token)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(c.Close)
	c.http.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	return c
}
func TestOCRClientClosedOriginAndCanonicalToken(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	for _, origin := range []string{"http://127.0.0.1:8092", Origin + "/", Origin + "?x=1", "http://user@document-ocr:8092"} {
		if _, e := New(origin, token); e == nil {
			t.Fatal(origin)
		}
	}
	if _, e := New(Origin, token+"="); e == nil {
		t.Fatal("token accepted")
	}
}
func TestOCRClientStrictSuccessAndFailureMapping(t *testing.T) {
	for _, row := range []struct {
		code         int
		body, reason string
	}{{200, source, ""}, {200, `{"status":"ready"}`, "internal_failure"}, {422, `{"status":"failed","reason":"corrupt"}`, "corrupt"}, {504, `{"status":"failed","reason":"ocr_timeout"}`, "ocr_timeout"}, {200, `{"status":"failed","reason":"corrupt"}`, "internal_failure"}, {503, `{"status":"failed","reason":"corrupt"}`, "internal_failure"}, {503, `{"status":"failed","reason":"language_missing","extra":1}`, "internal_failure"}} {
		t.Run(row.body[:min(30, len(row.body))], func(t *testing.T) {
			c := client(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/internal/ocr" || r.Header.Get("Content-Type") != "application/pdf" || r.Header.Get("Authorization") == "" {
					t.Error("wrong private request")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(row.code)
				w.Write([]byte(row.body))
			})
			raw, reason := c.Recognize(context.Background(), []byte("%PDF-synthetic"))
			if reason != row.reason || (reason == "" && string(raw) != source) {
				t.Fatal(reason)
			}
		})
	}
}
func TestOCRClientCancelAndRejectRedirect(t *testing.T) {
	c := client(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "http://127.0.0.1:1/untrusted")
		w.WriteHeader(302)
		w.Write([]byte(`{}`))
	})
	if _, reason := c.Recognize(context.Background(), []byte("%PDF-test")); reason != "internal_failure" {
		t.Fatal(reason)
	}
	c = client(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, reason := c.Recognize(ctx, []byte("%PDF-test")); reason != "ocr_timeout" {
		t.Fatal(reason)
	}
}
