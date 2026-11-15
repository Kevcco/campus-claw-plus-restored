package documentparser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const readyPDF = `{"status":"ready","body":"课😀\n\n文","locations":[{"kind":"pdf_page","unit":1,"char_start":0,"char_end":2},{"kind":"pdf_page","unit":2,"char_start":4,"char_end":5}],"source_page_count":2}`
const readyWord = `{"status":"ready","body":"表格é","locations":[{"kind":"docx_table_cell","unit":3,"char_start":0,"char_end":4}],"source_page_count":null}`

func token() string { return base64.RawURLEncoding.EncodeToString(make([]byte, 32)) }

func clientFor(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	client, err := New(Origin, token())
	if err != nil {
		t.Fatal(err)
	}
	transport := client.http.Transport.(*http.Transport)
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(func() { client.Close(); server.Close() })
	return client
}

func TestDI14ConfigurationAndBudgets(t *testing.T) {
	for _, origin := range []string{"", " " + Origin, Origin + "/", Origin + "?x=1", "http://127.0.0.1:8091", "https://document-parser:8091", "http://user@document-parser:8091"} {
		if _, err := New(origin, token()); err != ErrConfig {
			t.Fatalf("origin accepted or leaked: %v", err)
		}
	}
	for _, secret := range []string{"", " " + token(), token() + "=", strings.Repeat("a", 42), strings.Repeat("a", 42) + "B"} {
		if _, err := New(Origin, secret); err != ErrConfig {
			t.Fatal("invalid token accepted")
		}
	}
	c, err := New(Origin, token())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	transport := c.http.Transport.(*http.Transport)
	if transport.Proxy != nil || transport.ResponseHeaderTimeout != 32*time.Second || transport.MaxResponseHeaderBytes != 8192 || c.http.Timeout != 35*time.Second {
		t.Fatal("private budgets or proxy guard drifted")
	}
}

func TestDI14RealHTTPAndUnicodeLocations(t *testing.T) {
	for _, format := range []string{"pdf", "docx"} {
		t.Run(format, func(t *testing.T) {
			c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Host != "document-parser:8091" || r.Header.Get("Authorization") != "Bearer "+token() {
					t.Error("fixed origin/auth missing")
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/health" {
					if r.Method != "GET" {
						t.Error("health method")
					}
					io.WriteString(w, `{"status":"ok"}`)
					return
				}
				data, _ := io.ReadAll(r.Body)
				if r.Method != "POST" || r.URL.Path != "/v1/parse" || string(data) != "original binary bytes" || r.ContentLength != int64(len(data)) {
					t.Error("raw body framing")
				}
				if format == "pdf" {
					if r.Header.Get("Content-Type") != "application/pdf" {
						t.Error("pdf MIME")
					}
					io.WriteString(w, readyPDF)
				} else {
					if r.Header.Get("Content-Type") != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
						t.Error("docx MIME")
					}
					io.WriteString(w, readyWord)
				}
			})
			if !c.Healthy(context.Background()) {
				t.Fatal("real health request failed")
			}
			result := c.Parse(context.Background(), format, []byte("original binary bytes"))
			if result.Status != "ready" {
				t.Fatalf("valid Unicode response rejected: %+v", result)
			}
			if format == "pdf" && (result.Body != "课😀\n\n文" || result.Locations[0].End != 2) {
				t.Fatal("codepoint content was normalized or UTF16-counted")
			}
			if format == "docx" && (result.SourcePages != nil || result.Locations[0].Kind != "docx_table_cell") {
				t.Fatal("Word page invented")
			}
		})
	}
}

func TestDI14RejectsAmbiguousAndInconsistentResults(t *testing.T) {
	cases := []string{
		`null`, `[]`, `{"status":"ready","status":"needs_ocr"}`, `{"Status":"ready"}`,
		strings.Replace(readyPDF, `"ready"`, `"\ud800"`, 1),
		strings.Replace(readyPDF, `"body":"`, `"body":"\r`, 1),
		strings.Replace(readyPDF, `"body":"`, `"body":"\u0000`, 1),
		strings.Replace(readyPDF, `"char_end":2`, `"char_end":3`, 1),
		strings.Replace(readyPDF, `"char_start":4`, `"char_start":3`, 1),
		strings.Replace(readyPDF, `"unit":2`, `"unit":1`, 1),
		strings.Replace(readyPDF, `"unit":2`, `"unit":3`, 1),
		strings.Replace(readyPDF, `"source_page_count":2`, `"source_page_count":null`, 1),
		strings.Replace(readyPDF, `"char_start":0,`, ``, 1),
		strings.Replace(readyPDF, `"status":"ready",`, `"owner":"other","status":"ready",`, 1),
		`{"status":"needs_ocr","source_page_count":2,"body":"partial"}`,
		`{"status":"failed","failure_reason":"corrupt","body":"partial"}`,
		`{"status":"failed","failure_reason":"unknown"}`,
		readyPDF + `{}`, string([]byte{0xff}),
		`{"status":"ready","body":"\ud800","locations":[{"kind":"pdf_page","unit":1,"char_start":0,"char_end":1}],"source_page_count":1}`,
		`{"status":"ready","body":"` + string([]byte{0xff}) + `","locations":[{"kind":"pdf_page","unit":1,"char_start":0,"char_end":1}],"source_page_count":1}`,
	}
	for i, raw := range cases {
		if result := decodeResult("pdf", 200, []byte(raw)); result.FailureReason != "internal_failure" {
			t.Fatalf("case %d accepted: %+v", i, result)
		}
	}
	if decodeResult("docx", 200, []byte(readyPDF)).Status != "failed" {
		t.Fatal("Word accepted PDF pages")
	}
	if decodeResult("docx", 200, []byte(`{"status":"needs_ocr","source_page_count":1}`)).Status != "failed" {
		t.Fatal("Word accepted needs_ocr")
	}
	if decodeResult("pdf", 503, []byte(readyPDF)).FailureReason != "internal_failure" {
		t.Fatal("ready on 503 accepted")
	}
}

func TestDI14StatusAndReasonMapping(t *testing.T) {
	for reason, code := range map[string]int{"corrupt": 422, "encrypted": 422, "unsupported_content": 422, "resource_limit": 422, "parse_timeout": 504, "worker_interrupted": 503, "internal_failure": 503} {
		raw, _ := json.Marshal(map[string]string{"status": "failed", "failure_reason": reason})
		if result := decodeResult("pdf", code, raw); result.FailureReason != reason {
			t.Fatalf("mapping %s", reason)
		}
		if result := decodeResult("pdf", 200, raw); result.FailureReason != "internal_failure" {
			t.Fatal("failure accepted on 200")
		}
	}
	if decodeResult("pdf", 503, []byte(`{"error":"busy"}`)).FailureReason != "worker_interrupted" {
		t.Fatal("busy mapping")
	}
	if decodeResult("pdf", 200, []byte(`{"status":"needs_ocr","source_page_count":2}`)).Status != "needs_ocr" {
		t.Fatal("PDF scan rejected")
	}
}

func TestDI14NoRedirectOrEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	var visited atomic.Int32
	var requested atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { visited.Add(1) }))
	defer other.Close()
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) { requested.Add(1); http.Redirect(w, r, other.URL, 302) })
	if c.Healthy(context.Background()) {
		t.Fatal("redirect health accepted")
	}
	if c.Parse(context.Background(), "pdf", []byte("x")).FailureReason != "internal_failure" {
		t.Fatal("redirect parse accepted")
	}
	if visited.Load() != 0 {
		t.Fatal("redirect followed")
	}
	if requested.Load() != 2 {
		t.Fatal("redirect caused additional outbound requests")
	}
}

func TestDI14PreflightDeadlineAndCallerCancellation(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	})
	start := time.Now()
	if c.Healthy(context.Background()) {
		t.Fatal("stalled preflight accepted")
	}
	if elapsed := time.Since(start); elapsed < 400*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("preflight budget: %s", elapsed)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if c.Parse(ctx, "pdf", []byte("x")).FailureReason != "parse_timeout" {
		t.Fatal("deadline mapping")
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if c.Parse(ctx, "pdf", []byte("x")).FailureReason != "worker_interrupted" {
		t.Fatal("cancellation mapping")
	}
}

func TestDI14InputAndOutputBounds(t *testing.T) {
	var called atomic.Int32
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, strings.Repeat(" ", maxOutput+1))
	})
	if c.Parse(context.Background(), "pdf", nil).FailureReason != "corrupt" {
		t.Fatal("empty input")
	}
	if c.Parse(context.Background(), "pdf", make([]byte, maxInput+1)).FailureReason != "resource_limit" {
		t.Fatal("oversize input")
	}
	if called.Load() != 0 {
		t.Fatal("invalid input reached network")
	}
	if c.Parse(context.Background(), "pdf", []byte("x")).FailureReason != "internal_failure" {
		t.Fatal("oversize output")
	}
}

// DI-SC-14: required integers must not acquire Go zero values from JSON null.
// Exercise Parse over HTTP as well as the original semantic decoder cases.
func TestDI14ReviewerRejectsNullCoordinatesOverHTTP(t *testing.T) {
	for _, format := range []string{"pdf", "docx"} {
		for _, field := range []string{"unit", "char_start", "char_end"} {
			t.Run(format+"/"+field, func(t *testing.T) {
				raw := `{"status":"ready","body":"x","locations":[{"kind":"pdf_page","unit":1,"char_start":0,"char_end":1}],"source_page_count":1}`
				if format == "docx" {
					raw = strings.ReplaceAll(raw, "pdf_page", "docx_paragraph")
					raw = strings.ReplaceAll(raw, `"source_page_count":1`, `"source_page_count":null`)
				}
				value := "1"
				if field == "char_start" {
					value = "0"
				}
				raw = strings.Replace(raw, `"`+field+`":`+value, `"`+field+`":null`, 1)
				c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, raw)
				})
				result := c.Parse(context.Background(), format, []byte("original"))
				if result.Status != "failed" || result.FailureReason != "internal_failure" || result.Body != "" || len(result.Locations) != 0 {
					t.Fatalf("null coordinate escaped validation: %+v", result)
				}
			})
		}
	}
}

func TestDI14ReviewerResponseTruncationAndBodyDeadline(t *testing.T) {
	t.Run("truncated body", func(t *testing.T) {
		c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", fmt.Sprint(len(readyPDF)+10))
			io.WriteString(w, readyPDF)
		})
		if result := c.Parse(context.Background(), "pdf", []byte("original")); result.FailureReason != "worker_interrupted" || result.Body != "" {
			t.Fatalf("incomplete transport accepted or misclassified: %+v", result)
		}
	})
	t.Run("deadline after response headers", func(t *testing.T) {
		c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
			io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
		})
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		start := time.Now()
		if result := c.Parse(ctx, "pdf", []byte("original")); result.FailureReason != "parse_timeout" || result.Body != "" {
			t.Fatalf("body deadline not enforced: %+v", result)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Fatalf("body exceeded caller budget: %s", elapsed)
		}
	})
}
