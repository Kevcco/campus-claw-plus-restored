CREATE TABLE IF NOT EXISTS document_ocr_revisions (
 document_id BIGINT UNSIGNED NOT NULL,
 class_id BIGINT UNSIGNED NOT NULL,
 original_version INT UNSIGNED NOT NULL DEFAULT 1,
 revision INT UNSIGNED NOT NULL,
 kind ENUM('ocr','review') NOT NULL,
 body MEDIUMTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL,
 pages JSON NOT NULL,
 created_at DATETIME(3) NOT NULL,
 PRIMARY KEY(document_id,class_id,revision),
 CONSTRAINT fk_ocr_revision_original FOREIGN KEY(document_id,class_id,original_version) REFERENCES documents(id,class_id,version),
 CONSTRAINT ck_ocr_revision CHECK(original_version=1 AND revision BETWEEN 1 AND 2147483647),
 CONSTRAINT ck_ocr_body CHECK(OCTET_LENGTH(body) BETWEEN 1 AND 2097152),
 CONSTRAINT ck_ocr_pages CHECK(JSON_TYPE(pages)='ARRAY' AND JSON_LENGTH(pages) BETWEEN 1 AND 100)
) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;
CREATE TABLE IF NOT EXISTS document_ocr_heads (
 document_id BIGINT UNSIGNED NOT NULL,
 class_id BIGINT UNSIGNED NOT NULL,
 latest_revision INT UNSIGNED NOT NULL,
 published_revision INT UNSIGNED NULL,
 PRIMARY KEY(document_id,class_id),
 CONSTRAINT fk_ocr_head_latest FOREIGN KEY(document_id,class_id,latest_revision) REFERENCES document_ocr_revisions(document_id,class_id,revision),
 CONSTRAINT fk_ocr_head_published FOREIGN KEY(document_id,class_id,published_revision) REFERENCES document_ocr_revisions(document_id,class_id,revision),
 CONSTRAINT ck_ocr_head_order CHECK(published_revision IS NULL OR published_revision<=latest_revision)
) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;
CREATE TABLE IF NOT EXISTS document_ocr_review_keys (
 user_id BIGINT UNSIGNED NOT NULL,
 class_id BIGINT UNSIGNED NOT NULL,
 idem_key VARBINARY(64) NOT NULL,
 document_id BIGINT UNSIGNED NOT NULL,
 revision INT UNSIGNED NOT NULL,
 payload_digest BINARY(32) NOT NULL,
 response_bytes VARBINARY(512) NOT NULL,
 PRIMARY KEY(user_id,class_id,document_id,idem_key),
 CONSTRAINT fk_ocr_review_user FOREIGN KEY(user_id) REFERENCES users(id),
 CONSTRAINT fk_ocr_review_revision FOREIGN KEY(document_id,class_id,revision) REFERENCES document_ocr_revisions(document_id,class_id,revision),
 CONSTRAINT ck_ocr_review_key CHECK(OCTET_LENGTH(idem_key) BETWEEN 16 AND 64)
) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;
CREATE TABLE IF NOT EXISTS document_ocr_chunks (
 document_id BIGINT UNSIGNED NOT NULL,
 class_id BIGINT UNSIGNED NOT NULL,
 revision INT UNSIGNED NOT NULL,
 ordinal INT UNSIGNED NOT NULL,
 page SMALLINT UNSIGNED NOT NULL,
 char_start INT UNSIGNED NOT NULL,
 char_end INT UNSIGNED NOT NULL,
 quote TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL,
 PRIMARY KEY(document_id,class_id,revision,ordinal),
 CONSTRAINT fk_ocr_chunk_revision FOREIGN KEY(document_id,class_id,revision) REFERENCES document_ocr_revisions(document_id,class_id,revision),
 CONSTRAINT ck_ocr_chunk CHECK(ordinal>0 AND page BETWEEN 1 AND 100 AND char_end>char_start AND char_end<=2097152 AND CHAR_LENGTH(quote)=char_end-char_start)
) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;
CREATE TABLE IF NOT EXISTS document_ocr_sources (
 document_id BIGINT UNSIGNED NOT NULL,
 class_id BIGINT UNSIGNED NOT NULL,
 revision INT UNSIGNED NOT NULL DEFAULT 1,
 source_bytes MEDIUMBLOB NOT NULL,
 source_digest BINARY(32) NOT NULL,
 original_digest BINARY(32) NOT NULL,
 PRIMARY KEY(document_id,class_id),
 CONSTRAINT fk_ocr_source_revision FOREIGN KEY(document_id,class_id,revision) REFERENCES document_ocr_revisions(document_id,class_id,revision),
 CONSTRAINT ck_ocr_source CHECK(revision=1 AND OCTET_LENGTH(source_bytes) BETWEEN 1 AND 8388608)
) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;
CREATE TABLE IF NOT EXISTS document_ocr_jobs (
 document_id BIGINT UNSIGNED NOT NULL,
 class_id BIGINT UNSIGNED NOT NULL,
 original_version INT UNSIGNED NOT NULL DEFAULT 1,
 state ENUM('queued','processing','succeeded','failed') NOT NULL,
 original_digest BINARY(32) NOT NULL,
 attempts TINYINT UNSIGNED NOT NULL DEFAULT 0,
 execution_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0,
 worker_owner BINARY(32) NULL,
 lease_until DATETIME(3) NULL,
 failure_reason ENUM('corrupt','encrypted','unsupported_content','resource_limit','ocr_timeout','worker_interrupted','language_missing','internal_failure') NULL,
 PRIMARY KEY(document_id,class_id),
 KEY ix_ocr_job_claim(state,lease_until,document_id),
 CONSTRAINT fk_ocr_job_original FOREIGN KEY(document_id,class_id,original_version) REFERENCES documents(id,class_id,version),
 CONSTRAINT ck_ocr_job CHECK(original_version=1 AND attempts<=3 AND
 ((state='queued' AND attempts<=2 AND execution_epoch=attempts AND worker_owner IS NULL AND lease_until IS NULL AND failure_reason IS NULL) OR
  (state='processing' AND attempts BETWEEN 1 AND 3 AND execution_epoch=attempts AND worker_owner IS NOT NULL AND lease_until IS NOT NULL AND failure_reason IS NULL) OR
  (state='succeeded' AND attempts BETWEEN 1 AND 3 AND execution_epoch=attempts AND worker_owner IS NULL AND lease_until IS NULL AND failure_reason IS NULL) OR
  (state='failed' AND attempts BETWEEN 1 AND 3 AND execution_epoch=attempts AND worker_owner IS NULL AND lease_until IS NULL AND failure_reason IS NOT NULL)))
) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;
