//go:build integration && documenthttp

package wiring

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zwh9616/campus-claw-plus/backend/internal/documents"
)

// DI-SC-02/07/11..14: no SQL insertion, fake parser, or alternate handler.
// All document data is produced through the default trusted HTTPS artifact.
func TestDocumentHTTPProduction(t *testing.T) {
	ca, e := os.ReadFile("/fixtures/ca.pem")
	if e != nil {
		t.Fatal("CA unavailable")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		t.Fatal("CA invalid")
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 7 * time.Second, Jar: jar, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
	defer client.CloseIdleConnections()
	csrf := ""
	call := func(method, path, ct, key, token string, body []byte, want int) []byte {
		t.Helper()
		req, _ := http.NewRequest(method, "https://campus.test"+path, bytes.NewReader(body))
		req.Header.Set("Origin", "https://campus.test")
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal("trusted document HTTPS failed")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 12<<20))
		if err != nil || res.StatusCode != want {
			t.Fatalf("%s %s status=%d want=%d read_ok=%t", method, path, res.StatusCode, want, err == nil)
		}
		if len(res.Header.Get("X-Request-ID")) != 32 || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("response correlation/cache contract")
		}
		if method == "POST" && want == 202 && strings.HasPrefix(path, "/api/v1/documents") {
			var accepted documents.Document
			if json.Unmarshal(raw, &accepted) != nil || res.Header.Get("Location") != "/api/v1/documents/"+accepted.ID {
				t.Fatal("DI-SC-07/10 default HTTPS accepted resource Location")
			}
		}
		if strings.HasSuffix(path, "/download") && want == 200 {
			if res.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment; filename=\"document-") {
				t.Fatal("binary attachment contract")
			}
		}
		return raw
	}
	var seed struct {
		Users []struct{ Username, Password string }
	}
	raw, _ := os.ReadFile("/fixtures/document-seed.json")
	if json.Unmarshal(raw, &seed) != nil {
		t.Fatal("private seed unavailable")
	}
	login := func(name string) string {
		t.Helper()
		for _, u := range seed.Users {
			if u.Username == name {
				b, _ := json.Marshal(map[string]string{"username": u.Username, "password": u.Password})
				var out struct {
					Access string `json:"access_token"`
				}
				if json.Unmarshal(call("POST", "/api/v1/auth/login", "application/json", "", "", b, 200), &out) != nil || out.Access == "" {
					t.Fatal("private login failed")
				}
				return out.Access
			}
		}
		t.Fatal("actor unavailable")
		return ""
	}
	teacher := login("document-teacher")
	upload := func(name, key, token string, want int) []byte {
		t.Helper()
		file, err := os.ReadFile("/corpus/" + name)
		if err != nil {
			t.Fatal("real source unavailable")
		}
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		writer.WriteField("title", "公开资料 "+name)
		p, _ := writer.CreateFormFile("file", name)
		p.Write(file)
		writer.Close()
		return call("POST", "/api/v1/documents", writer.FormDataContentType(), key, token, body.Bytes(), want)
	}
	type item struct {
		ID, File, Key, SHA string
		Accepted           json.RawMessage
	}
	var state struct {
		Items                 []item
		Main, Outage, Persist bool `json:",omitempty"`
	}
	path := "/fixtures/document-http-proof.json"
	raw, _ = os.ReadFile(path)
	if json.Unmarshal(raw, &state) != nil {
		t.Fatal("host-owned proof unavailable")
	}
	detail := func(id, token string) documents.Detail {
		t.Helper()
		var d documents.Detail
		if json.Unmarshal(call("GET", "/api/v1/documents/"+id, "", "", token, nil, 200), &d) != nil || !documents.ValidDetail(d) {
			t.Fatal("persistent detail invalid")
		}
		return d
	}
	check := func(it item, token string) {
		t.Helper()
		d := detail(it.ID, token)
		if d.Document.Status != "ready" || d.Body == nil || len(d.Locations) == 0 {
			t.Fatal("actual parser did not produce ready sources")
		}
		original := call("GET", "/api/v1/documents/"+it.ID+"/download", "", "", token, nil, 200)
		sum := sha256.Sum256(original)
		if hex.EncodeToString(sum[:]) != it.SHA {
			t.Fatal("original bytes changed")
		}
		var found documents.Search
		if json.Unmarshal(call("GET", "/api/v1/document-search?mode=keyword&q="+url.QueryEscape("跨班资料不可访问")+"&top_k=10", "", "", token, nil, 200), &found) != nil || found.Mode != "keyword" {
			t.Fatal("keyword response")
		}
		match := false
		for _, c := range found.Items {
			if c.ID == it.ID {
				match = true
				r := []rune(*d.Body)
				if c.Location.Start < 0 || c.Location.End > len(r) || string(r[c.Location.Start:c.Location.End]) != c.Quote || c.Title != d.Document.Title || c.Version != d.Document.Version {
					t.Fatal("quote differs from persisted Unicode source")
				}
			}
		}
		if !match {
			t.Fatal("ready document missing literal search citation")
		}
	}
	stage := os.Getenv("DOCUMENT_HTTP_STAGE")
	switch stage {
	case "main":
		if len(state.Items) != 0 {
			t.Fatal("duplicate public stage")
		}
		for _, file := range []string{"clear-01.pdf", "clear-01.docx"} {
			key := "public-document-" + strings.ReplaceAll(file, ".", "-")
			accepted := upload(file, key, teacher, 202)
			var d documents.Document
			if json.Unmarshal(accepted, &d) != nil || !documents.ValidDocument(d) || d.Status != "queued" {
				t.Fatal("original 202 contract")
			}
			b, _ := os.ReadFile("/corpus/" + file)
			sum := sha256.Sum256(b)
			it := item{d.ID, file, key, hex.EncodeToString(sum[:]), accepted}
			state.Items = append(state.Items, it)
			deadline := time.Now().Add(40 * time.Second)
			for {
				v := detail(d.ID, teacher)
				if v.Document.Status == "ready" {
					break
				}
				if v.Document.Status == "failed" || v.Document.Status == "needs_ocr" || time.Now().After(deadline) {
					t.Fatal("real public parse failed or exceeded budget")
				}
				time.Sleep(150 * time.Millisecond)
			}
			check(it, teacher)
			if !bytes.Equal(upload(file, key, teacher, 202), accepted) {
				t.Fatal("original 202 not preserved after ready")
			}
			call("POST", "/api/v1/documents/"+it.ID+"/retry", "", "", teacher, nil, 409)
		}
		for _, name := range []string{"document-student-a", "document-student-a2"} {
			token := login(name)
			for _, it := range state.Items {
				check(it, token)
			}
			upload(state.Items[0].File, "student-public-denied", token, 403)
			call("POST", "/api/v1/documents/"+state.Items[0].ID+"/retry", "", "", token, nil, 403)
		}
		for _, name := range []string{"document-teacher-b", "document-student-b"} {
			token := login(name)
			for _, it := range state.Items {
				call("GET", "/api/v1/documents/"+it.ID, "", "", token, nil, 404)
				call("GET", "/api/v1/documents/"+it.ID+"/download", "", "", token, nil, 404)
			}
			var found documents.Search
			json.Unmarshal(call("GET", "/api/v1/document-search?mode=keyword&q="+url.QueryEscape("跨班资料不可访问"), "", "", token, nil, 200), &found)
			if len(found.Items) != 0 {
				t.Fatal("cross-class search disclosure")
			}
		}
		var page documents.Page
		if json.Unmarshal(call("GET", "/api/v1/documents?page=1&page_size=20", "", "", teacher, nil, 200), &page) != nil || page.Total != 2 || len(page.Items) != 2 {
			t.Fatal("real paginated list")
		}
		for _, p := range []string{"/api/v1/documents", "/api/v1/documents/" + state.Items[0].ID, "/api/v1/document-search?mode=keyword&q=x"} {
			call("GET", p, "", "", "", nil, 401)
		}
		// A real session/class rotation must revoke old access on all six paths.
		other := login("document-teacher-b")
		var me struct {
			ClassID string `json:"class_id"`
		}
		if json.Unmarshal(call("GET", "/api/v1/me", "", "", other, nil, 200), &me) != nil || me.ClassID == "" {
			t.Fatal("trusted target class")
		}
		teacher = login("document-teacher")
		var anti struct {
			Token string `json:"csrf_token"`
		}
		json.Unmarshal(call("GET", "/api/v1/auth/csrf", "", "", "", nil, 200), &anti)
		csrf = anti.Token
		change, _ := json.Marshal(map[string]string{"class_id": me.ClassID})
		var switched struct {
			Access string `json:"access_token"`
		}
		if json.Unmarshal(call("POST", "/api/v1/auth/switch-class", "application/json", "", teacher, change, 200), &switched) != nil || switched.Access == "" {
			t.Fatal("real class switch")
		}
		for _, it := range state.Items {
			call("GET", "/api/v1/documents/"+it.ID, "", "", teacher, nil, 401)
			call("GET", "/api/v1/documents/"+it.ID, "", "", switched.Access, nil, 404)
		}
		// Restore the fixture teacher's default class for outage/recreation stages.
		var first struct {
			ClassID string `json:"class_id"`
		}
		student := login("document-student-a")
		json.Unmarshal(call("GET", "/api/v1/me", "", "", student, nil, 200), &first)
		teacher = login("document-teacher")
		json.Unmarshal(call("GET", "/api/v1/auth/csrf", "", "", "", nil, 200), &anti)
		csrf = anti.Token
		change, _ = json.Marshal(map[string]string{"class_id": first.ClassID})
		call("POST", "/api/v1/auth/switch-class", "application/json", "", teacher, change, 200)
		csrf = ""
		state.Main = true
		t.Log("public main: 2 real formats; 5 actors; exact originals and persisted codepoint citations; replay and authorization PASS")
	case "outage":
		if !state.Main || len(state.Items) != 2 {
			t.Fatal("missing public main")
		}
		for _, it := range state.Items {
			if !bytes.Equal(upload(it.File, it.Key, teacher, 202), it.Accepted) {
				t.Fatal("parser outage blocks committed replay")
			}
			check(it, teacher)
		}
		upload(state.Items[0].File, "public-parser-outage-new", teacher, 503)
		var page documents.Page
		json.Unmarshal(call("GET", "/api/v1/documents", "", "", teacher, nil, 200), &page)
		if page.Total != 2 {
			t.Fatal("preflight outage left a half-upload")
		}
		state.Outage = true
		t.Log("public outage: actual stopped parser; replay/read PASS; new503 and no half-upload PASS")
	case "persist":
		if !state.Main || !state.Outage {
			t.Fatal("missing public stages")
		}
		for _, it := range state.Items {
			check(it, teacher)
			if !bytes.Equal(upload(it.File, it.Key, teacher, 202), it.Accepted) {
				t.Fatal("container recreation lost original202")
			}
		}
		state.Persist = true
		t.Log("public persist: full container recreation retained originals citations and original202 PASS")
	default:
		t.Fatal("explicit public stage required")
	}
	raw, e = json.Marshal(state)
	if e != nil || os.WriteFile(path, raw, 0600) != nil {
		t.Fatal("private proof write")
	}
	fmt.Println("DI public HTTP stage:", stage, "PASS")
}
