//go:build ocrpipeline

package ocrpipeline

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zwh9616/campus-claw-plus/backend/internal/ocrreview"
	"github.com/zwh9616/campus-claw-plus/backend/internal/privateocr"
)

// DI-SC-16/17/19: actual image bytes cross Go and HTTP, with no gold mounted
// into the recognition service. The host independently scores saved responses.
func TestRealImageOCRDevelopment(t *testing.T) {
	client, e := privateocr.New(privateocr.Origin, os.Getenv("DOCUMENT_PARSER_TOKEN"))
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	var fixtures []struct {
		ID, File, Split string
		Width, Height   int
	}
	manifest, e := os.ReadFile("/fixtures/image-inputs.json")
	if e != nil || json.Unmarshal(manifest, &fixtures) != nil || len(fixtures) != 14 {
		t.Fatal("development-only manifest", e)
	}
	type row struct {
		ID        string          `json:"id"`
		Reason    string          `json:"reason"`
		Result    json.RawMessage `json:"result,omitempty"`
		ElapsedMS int64           `json:"elapsed_ms"`
	}
	rows := []row{}
	metadataChecks := 0
	for _, fixture := range fixtures {
		if filepath.Base(fixture.File) != fixture.File {
			t.Fatal("unsafe test fixture path")
		}
		input, e := os.ReadFile("/fixtures/images/" + fixture.File)
		if e != nil {
			t.Fatal(e)
		}
		started := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		raw, reason := client.Recognize(ctx, input)
		cancel()
		if reason == "" {
			source, _, e := ocrreview.DecodeSource(raw)
			if e != nil || len(source.Pages) != 1 || source.Pages[0].Width != fixture.Width || source.Pages[0].Height != fixture.Height {
				t.Fatal("original image geometry", fixture.ID, e)
			}
			if fixture.ID == "dev-01" || fixture.ID == "dev-02" {
				var variant []byte
				if fixture.ID == "dev-01" {
					variant = append(append(append([]byte{}, input[:33]...), pngChunk("eXIf", []byte("untrusted EXIF metadata"))...), input[33:]...)
				} else {
					// Valid TIFF EXIF orientation6; encoded raster stays authoritative.
					exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
					variant = append(append(append([]byte{}, input[:2]...), append([]byte{255, 225, 0, byte(len(exif) + 2)}, exif...)...), input[2:]...)
				}
				ctx, done := context.WithTimeout(context.Background(), 25*time.Second)
				got, why := client.Recognize(ctx, variant)
				done()
				if why != "" || !bytes.Equal(got, raw) {
					t.Fatal("metadata changed encoded-pixel OCR", fixture.ID, why)
				}
				metadataChecks++
			}
		} else if fixture.Split == "development-positive" || reason == "internal_failure" || reason == "worker_interrupted" {
			t.Fatal("development recognition failed", fixture.ID, reason)
		}
		rows = append(rows, row{fixture.ID, reason, raw, time.Since(started).Milliseconds()})
	}
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[:4], 4001)
	binary.BigEndian.PutUint32(header[4:8], 5000)
	header[8], header[9] = 8, 2
	huge := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", header)...)
	for _, bad := range []struct {
		body   []byte
		reason string
	}{{huge, "resource_limit"}, {[]byte("\x89PNG\r\n\x1a\nbroken"), "corrupt"}, {[]byte{255, 216, 255, 217}, "corrupt"}} {
		ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
		_, reason := client.Recognize(ctx, bad.body)
		done()
		if reason != bad.reason {
			t.Fatal("native image rejection", reason, bad.reason)
		}
	}
	raw, e := json.Marshal(rows)
	if e != nil || os.WriteFile("/evidence/images.json", raw, 0644) != nil {
		t.Fatal("save actual image response", e)
	}
	t.Logf("IMAGE_RPC development_inputs=14 metadata_invariance=%d bounded_rejections=3 original_geometry_checked=true gold_in_service=false blind_inputs=0", metadataChecks)
}

func pngChunk(kind string, payload []byte) []byte {
	var out bytes.Buffer
	binary.Write(&out, binary.BigEndian, uint32(len(payload)))
	out.WriteString(kind)
	out.Write(payload)
	binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(kind), payload...)))
	return out.Bytes()
}
