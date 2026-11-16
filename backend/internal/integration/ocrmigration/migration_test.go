//go:build ocrmigration

package ocrmigration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zwh9616/campus-claw-plus/backend/internal/documentqueue"
	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
	"github.com/zwh9616/campus-claw-plus/backend/internal/materials"
	"github.com/zwh9616/campus-claw-plus/backend/internal/migrations"
	"github.com/zwh9616/campus-claw-plus/backend/internal/mysqlstore"
	"github.com/zwh9616/campus-claw-plus/backend/internal/ocrreview"
	"os"
	"testing"
)

func connection(t *testing.T, name string, create bool) *sql.DB {
	t.Helper()
	password := os.Getenv("MYSQL_PWD")
	if password == "" {
		t.Fatal("owned fixture credential required")
	}
	root, e := sql.Open("mysql", "root:"+password+"@tcp(127.0.0.1:3306)/?parseTime=true&charset=utf8mb4&timeout=2s&readTimeout=5s&writeTimeout=5s")
	if e != nil {
		t.Fatal(e)
	}
	if create {
		if _, e = root.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin"); e != nil {
			t.Fatal(e)
		}
	}
	root.Close()
	db, e := sql.Open("mysql", "root:"+password+"@tcp(127.0.0.1:3306)/"+name+"?parseTime=true&charset=utf8mb4&timeout=2s&readTimeout=5s&writeTimeout=5s")
	if e != nil {
		t.Fatal(e)
	}
	db.SetMaxOpenConns(10)
	t.Cleanup(func() { db.Close() })
	return db
}
func execSQL(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, e := db.Exec(q, args...); e != nil {
		t.Fatal("fixture SQL", e)
	}
}
func seed6(t *testing.T, db *sql.DB) {
	t.Helper()
	execSQL(t, db, "INSERT INTO users(id,username,password_hash,role) VALUES(1,'ocr_migration_teacher','unusable','teacher')")
	execSQL(t, db, "INSERT INTO classes(id,stable_key,name) VALUES(1,'ocr_migration_class','课程班')")
	p := identity.Principal{UserID: "1", ClassID: "1", Role: "teacher", SessionID: "trusted-migration-fixture"}
	u, e := materials.PrepareUpload("旧TXT", "old.txt", []byte("old txt"))
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = (&mysqlstore.Store{DB: db}).CreateMaterial(context.Background(), p, "old-material-key01", u); e != nil {
		t.Fatal(e)
	}
	original, e := os.ReadFile("/fixtures/development-positive.pdf")
	if e != nil {
		t.Fatal(e)
	}
	execSQL(t, db, "ALTER TABLE documents AUTO_INCREMENT=91")
	q := &documentqueue.Store{DB: db}
	accepted, e := q.Enqueue(context.Background(), documentqueue.Upload{UserID: 1, ClassID: 1, Key: "old-upload-key-001", Title: "旧扫描原件", Format: "pdf", Original: original})
	if e != nil || accepted.ID != 91 {
		t.Fatal(e)
	}
	if e = os.WriteFile("/evidence/old202.json", accepted.Body, 0644); e != nil {
		t.Fatal(e)
	}
	owner, e := documentqueue.NewOwner()
	if e != nil {
		t.Fatal(e)
	}
	w, e := q.Claim(context.Background(), 91, owner)
	if e != nil {
		t.Fatal(e)
	}
	pages := 4
	if e = q.Finish(context.Background(), w, documentqueue.Result{Status: "needs_ocr", Pages: &pages}); e != nil {
		t.Fatal(e)
	}
}
func check6Data(t *testing.T, db *sql.DB) {
	t.Helper()
	var raw []byte
	var version int
	if e := db.QueryRow("SELECT original_bytes,version FROM documents WHERE id=91").Scan(&raw, &version); e != nil {
		t.Fatal(e)
	}
	want, e := os.ReadFile("/fixtures/development-positive.pdf")
	if e != nil || !bytes.Equal(raw, want) || version != 1 {
		t.Fatal("v1 bytes drift", e)
	}
	expected, e := os.ReadFile("/evidence/old202.json")
	if e != nil {
		t.Fatal(e)
	}
	got, found, e := (&documentqueue.Store{DB: db}).Replay(context.Background(), documentqueue.Upload{UserID: 1, ClassID: 1, Key: "old-upload-key-001", Title: "旧扫描原件", Format: "pdf", Original: want})
	if e != nil || !found || !bytes.Equal(got.Body, expected) {
		t.Fatal("real original202 replay drift", e)
	}
	p := identity.Principal{UserID: "1", ClassID: "1", Role: "teacher", SessionID: "trusted-migration-fixture"}
	material, e := (&mysqlstore.Store{DB: db}).GetMaterial(context.Background(), p, "1")
	if e != nil || material.Body != "old txt" || string(material.Raw) != "old txt" {
		t.Fatal("real old TXT retrieval drift", e)
	}
}
func TestOCRMigrationFreshAndUpgrade(t *testing.T) {
	ctx := context.Background()
	for _, old := range []bool{false, true} {
		name := fmt.Sprintf("ocr_migration_fresh_%v", old)
		db := connection(t, name, true)
		e := migrations.Executor{DB: db, Database: name}
		if old {
			if e.Up(ctx, 6) != nil {
				t.Fatal("up6")
			}
			seed6(t, db)
		}
		if err := e.Up(ctx, 7); err != nil {
			t.Fatal("up7", err)
		}
		if e.Up(ctx, 7) != nil || e.Check(ctx, 7) != nil || e.Check(ctx, 6) == nil || migrations.Target != 6 {
			t.Fatal("exact version gate")
		}
		if old {
			check6Data(t, db)
		}
		var n int
		if db.QueryRow("SELECT COUNT(*) FROM identity_migration_checksums").Scan(&n) != nil || n != 7 {
			t.Fatal("history")
		}
	}
}
func TestOCRMigrationSixInterruptionsAndDrift(t *testing.T) {
	ctx := context.Background()
	for step := 1; step <= 6; step++ {
		t.Run(fmt.Sprint(step), func(t *testing.T) {
			name := fmt.Sprintf("ocr_migration_dirty_%d", step)
			db := connection(t, name, true)
			e := migrations.Executor{DB: db, Database: name}
			if e.Up(ctx, 6) != nil {
				t.Fatal("up6")
			}
			seed6(t, db)
			e.AfterDDL = func(v, n int) error {
				if v == 7 && n == step {
					return errors.New("controlled interruption")
				}
				return nil
			}
			if e.Up(ctx, 7) == nil || e.Check(ctx, 7) == nil {
				t.Fatal("dirty accepted")
			}
			var version int
			var dirty bool
			if db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty) != nil || version != 7 || !dirty {
				t.Fatal("dirty7 missing")
			}
			e.AfterDDL = nil
			execSQL(t, db, "ALTER TABLE document_ocr_revisions MODIFY body MEDIUMTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL")
			if e.Repair(ctx, 7) == nil {
				t.Fatal("drift force-cleaned")
			}
			if db.QueryRow("SELECT dirty FROM schema_migrations").Scan(&dirty) != nil || !dirty {
				t.Fatal("dirty cleared")
			}
			execSQL(t, db, "ALTER TABLE document_ocr_revisions MODIFY body MEDIUMTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL")
			if err := e.Repair(ctx, 7); err != nil {
				t.Fatal("forward repair", err)
			}
			if e.Repair(ctx, 7) != nil || e.Check(ctx, 7) != nil {
				t.Fatal("repeat repair")
			}
			check6Data(t, db)
		})
	}
}
func TestOCRMigrationExactStructureAndHistory(t *testing.T) {
	ctx := context.Background()
	name := "ocr_migration_drift"
	db := connection(t, name, true)
	e := migrations.Executor{DB: db, Database: name}
	if e.Up(ctx, 7) != nil {
		t.Fatal("up7")
	}
	for _, pair := range [][2]string{{"ALTER TABLE document_ocr_sources MODIFY source_digest BINARY(31) NOT NULL", "ALTER TABLE document_ocr_sources MODIFY source_digest BINARY(32) NOT NULL"}, {"ALTER TABLE document_ocr_jobs ALTER CHECK ck_ocr_job NOT ENFORCED", "ALTER TABLE document_ocr_jobs ALTER CHECK ck_ocr_job ENFORCED"}, {"ALTER TABLE document_ocr_jobs ALTER INDEX ix_ocr_job_claim INVISIBLE", "ALTER TABLE document_ocr_jobs ALTER INDEX ix_ocr_job_claim VISIBLE"}, {"ALTER TABLE document_ocr_heads ROW_FORMAT=COMPACT", "ALTER TABLE document_ocr_heads ROW_FORMAT=DYNAMIC"}} {
		execSQL(t, db, pair[0])
		if e.Check(ctx, 7) == nil || e.Repair(ctx, 7) == nil {
			t.Fatal("structure drift accepted")
		}
		execSQL(t, db, pair[1])
		if e.Check(ctx, 7) != nil {
			t.Fatal("restore drift")
		}
	}
	execSQL(t, db, "UPDATE identity_migration_checksums SET up_sha256=REPEAT('0',64) WHERE version=7")
	if e.Check(ctx, 7) == nil || e.Up(ctx, 7) == nil || e.Repair(ctx, 7) == nil {
		t.Fatal("checksum drift")
	}
}
func TestOCRMigrationEmptyDownNeverPretendsRollback(t *testing.T) {
	ctx := context.Background()
	name := "ocr_migration_empty"
	db := connection(t, name, true)
	e := migrations.Executor{DB: db, Database: name}
	if e.Up(ctx, 7) != nil {
		t.Fatal("up7")
	}
	seed6(t, db)
	execSQL(t, db, "INSERT INTO document_ocr_revisions(document_id,class_id,revision,kind,body,pages,created_at) VALUES(91,1,1,'ocr','a',JSON_ARRAY(JSON_OBJECT('page',1,'text','a')),UTC_TIMESTAMP(3))")
	if e.DownEmptyOCRForTest(ctx, name) == nil || e.Check(ctx, 7) != nil {
		t.Fatal("nonempty reverse allowed")
	}
	execSQL(t, db, "DELETE FROM document_ocr_revisions")
	if e.DownEmptyOCRForTest(ctx, "wrong") == nil {
		t.Fatal("confirmation bypass")
	}
	if e.DownEmptyOCRForTest(ctx, name) != nil {
		t.Fatal("empty reverse")
	}
	if e.Check(ctx, 6) == nil || e.Check(ctx, 7) == nil || e.Up(ctx, 7) == nil {
		t.Fatal("dirty7 was silently downgraded")
	}
	check6Data(t, db)
}

func TestOCRBackupFixture(t *testing.T) {
	ctx := context.Background()
	stage := os.Getenv("OCR_BACKUP_STAGE")
	if stage == "" {
		t.Fatal("explicit backup stage required")
	}
	name := "ocr_migration_backup"
	if stage == "restored6" {
		name = "ocr_migration_restore6"
	}
	if stage == "restored7" {
		name = "ocr_migration_restore7"
	}
	db := connection(t, name, stage == "prepare6")
	e := migrations.Executor{DB: db, Database: name}
	switch stage {
	case "prepare6":
		if e.Up(ctx, 6) != nil {
			t.Fatal("prepare6")
		}
		seed6(t, db)
		check6Data(t, db)
	case "upgrade7":
		if e.Up(ctx, 7) != nil {
			t.Fatal("upgrade7")
		}
		check6Data(t, db)
		raw, err := os.ReadFile("/fixtures/actual-source.json")
		if err != nil {
			t.Fatal(err)
		}
		s := &ocrreview.Store{DB: db}
		p := identity.Principal{UserID: "1", ClassID: "1", Role: "teacher", SessionID: "trusted-backup-fixture"}
		if err = s.EnqueueOCR(ctx, p, "91"); err != nil {
			t.Fatal(err)
		}
		w, err := s.ClaimOCR(ctx, "91", "1")
		if err != nil {
			t.Fatal(err)
		}
		if err = s.CommitOCR(ctx, w.Lease, raw); err != nil {
			t.Fatal(err)
		}
		_, pages, err := ocrreview.DecodeSource(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Save(ctx, p, "91", "backup-review-key01", ocrreview.Review{BaseRevision: 1, Acknowledged: true, Pages: pages}); err != nil {
			t.Fatal(err)
		}
	case "restored6":
		if e.Check(ctx, 6) != nil || e.Check(ctx, 7) == nil {
			var version int
			var dirty bool
			db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty)
			t.Logf("RESTORE_META version=%d dirty=%v", version, dirty)
			for v := 1; v <= 6; v++ {
				t.Logf("STRUCTURE_%d %v", v, e.Structure(ctx, v))
			}
			definitions := map[string]string{}
			for _, table := range []string{"documents", "document_locations", "document_chunks", "document_parse_jobs", "document_idempotency"} {
				var name, ddl string
				if err := db.QueryRow("SHOW CREATE TABLE `"+table+"`").Scan(&name, &ddl); err == nil {
					definitions[name] = ddl
				}
			}
			raw, _ := json.MarshalIndent(definitions, "", "  ")
			os.WriteFile("/evidence/restore6-structure.json", raw, 0644)
			t.Fatal("restored6 version")
		}
		check6Data(t, db)
	case "restored7", "source7":
		if e.Check(ctx, 7) != nil || e.Check(ctx, 6) == nil {
			t.Fatal("version7 check")
		}
		check6Data(t, db)
		s := &ocrreview.Store{DB: db}
		p := identity.Principal{UserID: "1", ClassID: "1", Role: "teacher", SessionID: "trusted-backup-fixture"}
		got, err := s.ReadSource(ctx, p, "91")
		want, readErr := os.ReadFile("/fixtures/actual-source.json")
		if err != nil || readErr != nil || !bytes.Equal(got, want) {
			t.Fatal("source snapshot restore", err)
		}
		v, err := s.Read(ctx, p, "91", 2)
		if err != nil || v.Revision != 2 {
			t.Fatal("review restore", err)
		}
		var receipt []byte
		if db.QueryRow("SELECT response_bytes FROM document_ocr_review_keys").Scan(&receipt) != nil || string(receipt) != `{"document_id":"91","revision":2,"status":"ready"}` {
			t.Fatal("review receipt restore")
		}
	default:
		t.Fatal("unknown stage")
	}
	t.Log("BACKUP_STAGE " + stage + " passed")
}
