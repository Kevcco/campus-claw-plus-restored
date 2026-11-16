// Package privateocr is the fixed-origin OCR HTTP adapter, independently bounded
// from the existing 35-second PDF/Word parser client.
package privateocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"github.com/zwh9616/campus-claw-plus/backend/internal/ocrreview"
	"github.com/zwh9616/campus-claw-plus/backend/internal/strictjson"
	"io"
	"mime"
	"net"
	"net/http"
	"time"
)

const Origin = "http://document-ocr:8092"

var ErrConfig = errors.New("invalid private OCR configuration")

type Client struct {
	token string
	http  *http.Client
}

func New(origin, token string) (*Client, error) {
	key, e := base64.RawURLEncoding.Strict().DecodeString(token)
	if origin != Origin || e != nil || len(key) != 32 || len(token) != 43 || base64.RawURLEncoding.EncodeToString(key) != token {
		return nil, ErrConfig
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, ResponseHeaderTimeout: 188 * time.Second, MaxResponseHeaderBytes: 8192, MaxConnsPerHost: 2, IdleConnTimeout: 30 * time.Second}
	return &Client{token, &http.Client{Transport: transport, Timeout: 190 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Close() { c.http.CloseIdleConnections() }
func (c *Client) call(ctx context.Context, method, path string, input []byte) (int, []byte, error) {
	req, e := http.NewRequestWithContext(ctx, method, Origin+path, bytes.NewReader(input))
	if e != nil {
		return 0, nil, e
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/pdf")
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return 0, nil, e
	}
	defer resp.Body.Close()
	typ, _, e := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if e != nil || typ != "application/json" || len(resp.Header.Values("Content-Type")) != 1 {
		return 0, nil, strictjson.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 8388609))
	if e != nil {
		return 0, nil, e
	}
	if len(raw) > 8388608 {
		return 0, nil, strictjson.ErrInvalid
	}
	return resp.StatusCode, raw, nil
}
func (c *Client) Healthy(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	code, raw, e := c.call(ctx, "GET", "/health", nil)
	if e != nil || code != 200 {
		return false
	}
	var p struct {
		Status string `json:"status"`
	}
	return strictjson.Decode(raw, &p) == nil && p.Status == "ok"
}
func (c *Client) Recognize(ctx context.Context, input []byte) ([]byte, string) {
	if len(input) == 0 || len(input) > 10485760 || !bytes.HasPrefix(input, []byte("%PDF-")) {
		return nil, "unsupported_content"
	}
	code, raw, e := c.call(ctx, "POST", "/internal/ocr", input)
	if e != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, "worker_interrupted"
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, "ocr_timeout"
		}
		var timeout net.Error
		if errors.As(e, &timeout) && timeout.Timeout() {
			return nil, "ocr_timeout"
		}
		return nil, "internal_failure"
	}
	if code == 200 {
		if _, _, e = ocrreview.DecodeSource(raw); e == nil {
			return raw, ""
		}
		return nil, "internal_failure"
	}
	var failed struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if strictjson.Decode(raw, &failed) != nil || failed.Status != "failed" {
		return nil, "internal_failure"
	}
	codes := map[string]int{"corrupt": 422, "encrypted": 422, "unsupported_content": 422, "resource_limit": 422, "ocr_timeout": 504, "worker_interrupted": 503, "language_missing": 503, "internal_failure": 503}
	if expected, ok := codes[failed.Reason]; ok && code == expected {
		return nil, failed.Reason
	}
	return nil, "internal_failure"
}
