package migrations

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOCRExactSnapshotAndImmutableFiles(t *testing.T) {
	if verifyFiles() != nil {
		t.Fatal("embedded checksum mismatch")
	}
	raw, e := files.ReadFile("ocr-structure.json")
	if e != nil {
		t.Fatal(e)
	}
	var all map[string]string
	if json.Unmarshal(raw, &all) != nil || len(all) != 6 {
		t.Fatal("missing OCR snapshot")
	}
	base := all["document_ocr_jobs"]
	for _, pair := range [][2]string{{"binary(32)", "binary(31)"}, {"`attempts` <= 3", "`attempts` <= 4"}, {"utf8mb4_0900_bin", "utf8mb4_0900_ai_ci"}, {"`execution_epoch` = `attempts`", "`execution_epoch` >= `attempts`"}} {
		changed := strings.ReplaceAll(base, pair[0], pair[1])
		if changed == base || canonicalDocumentDefinition(changed) == canonicalDocumentDefinition(base) {
			t.Fatal("OCR mutation not detected", pair)
		}
	}
	if Target != 6 {
		t.Fatal("preparing migration7 must not silently activate production")
	}
}
