ALTER TABLE documents
 MODIFY COLUMN format ENUM('pdf','docx','png','jpeg') NOT NULL,
 DROP CHECK ck_document_state,
 ADD CONSTRAINT ck_document_state CHECK(
  (format IN('pdf','docx') AND (
   (status='queued' AND attempts<=2 AND body IS NULL AND page_count IS NULL AND failure_reason IS NULL) OR
   (status='processing' AND attempts BETWEEN 1 AND 3 AND body IS NULL AND page_count IS NULL AND failure_reason IS NULL) OR
   (status='ready' AND attempts BETWEEN 1 AND 3 AND body IS NOT NULL AND OCTET_LENGTH(body) BETWEEN 1 AND 2097152 AND failure_reason IS NULL AND
    ((format='pdf' AND page_count IS NOT NULL AND page_count BETWEEN 1 AND 200) OR (format='docx' AND page_count IS NULL))) OR
   (status='needs_ocr' AND format='pdf' AND attempts BETWEEN 1 AND 3 AND body IS NULL AND page_count IS NOT NULL AND page_count BETWEEN 1 AND 200 AND failure_reason IS NULL) OR
   (status='failed' AND attempts BETWEEN 1 AND 3 AND body IS NULL AND page_count IS NULL AND failure_reason IS NOT NULL)
  )) OR
  (format IN('png','jpeg') AND status='needs_ocr' AND attempts=0 AND body IS NULL AND page_count=1 AND page_count IS NOT NULL AND failure_reason IS NULL)
 );
