package migrations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

var ocrTables = []string{"document_ocr_revisions", "document_ocr_heads", "document_ocr_review_keys", "document_ocr_chunks", "document_ocr_sources", "document_ocr_jobs"}

const ocrStructureSHA = "53d195d92dddf19a668a57f772eb07cfed628d0df26461dba7dbe45f062f6a3d"

func (e Executor) ocrStructure(ctx context.Context, table string) error {
	allowed := false
	for _, t := range ocrTables {
		if t == table {
			allowed = true
		}
	}
	if !allowed {
		return ErrState
	}
	data, err := files.ReadFile("ocr-structure.json")
	if err != nil {
		return ErrState
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != ocrStructureSHA {
		return ErrState
	}
	var expected map[string]string
	if json.Unmarshal(data, &expected) != nil || len(expected) != 6 || expected[table] == "" {
		return ErrState
	}
	var name, actual, engine, format string
	if e.DB.QueryRowContext(ctx, "SELECT ENGINE,ROW_FORMAT FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?", e.Database, table).Scan(&engine, &format) != nil || engine != "InnoDB" || format != "Dynamic" {
		return ErrState
	}
	if e.DB.QueryRowContext(ctx, "SHOW CREATE TABLE `"+table+"`").Scan(&name, &actual) != nil || name != table || canonicalDocumentDefinition(actual) != canonicalDocumentDefinition(expected[table]) {
		return ErrState
	}
	return nil
}
