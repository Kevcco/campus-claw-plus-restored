import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { randomBytes, createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, unlinkSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

// DI-SC-02/07/09/17/25: exact candidate DDL only, not embedded migration8.
// Controlled mutation exists only in the SQL sent to this disposable database.
const mutate = process.argv.includes('--mutate-attempts')
assert.ok(process.argv.slice(2).every(a => a === '--mutate-attempts'))
const id = 'campus-ocr-image-ddl-' + randomBytes(6).toString('hex')
const image = id + ':probe', password = randomBytes(32).toString('base64url')
const directory = resolve('output/ocr-image-storage' + (mutate ? '-red' : ''))
mkdirSync(directory, { recursive: true })
const envfile = directory + '/' + id + '.env', results = []
const candidate = 'docs/contracts/ocr-image-originals/000008_image_originals.up.sql'
const sourcePaths = [candidate, 'scripts/check-ocr-image-storage.mjs', ...Array.from({ length: 7 }, (_, i) => {
  const names = ['identity_users_classes', 'identity_sessions', 'identity_auth_audit', 'materials_retrieval', 'assignment_tasks', 'documents', 'ocr_revisions']
  return `backend/internal/migrations/sql/${String(i + 1).padStart(6, '0')}_${names[i]}.up.sql`
})]
const sha = bytes => createHash('sha256').update(bytes).digest('hex')
const profile = JSON.parse(readFileSync('docs/contracts/ocr-image-originals/profile.json'))
assert.equal(profile.runtime_activation, false)
assert.equal(profile.production_target, 6)
assert.equal(profile.candidate_target, 8)
assert.deepEqual(profile.frozen_files, [{ path: candidate, sha256: sha(readFileSync(candidate)) }])
const report = { run_id: id, time: new Date().toISOString(), exit_code: 1, controlled_mutation: mutate,
  scope: 'isolated exact SQL candidate8 against published tables1..7; no migration executor/backup/runtime or holder deployment claim',
  sources: sourcePaths.map(path => ({ path, sha256: sha(readFileSync(path)) })) }
function docker(args, input, timeout = 30000) {
  return spawnSync('docker', args, { input, encoding: 'utf8', timeout, maxBuffer: 4 << 20,
    env: { ...process.env, MYSQL_PWD: password } })
}
function run(label, args, input, timeout = 180000) {
  const r = docker(args, input, timeout)
  results.push({ label, exit_code: r.status })
  assert.equal(r.status, 0, label + ': ' + (r.stderr ?? '').replaceAll(password, '<redacted>').slice(-1000))
  return r.stdout.trim()
}
function sql(label, text, rejectCode) {
  const r = docker(['exec', '-i', '-e', 'MYSQL_PWD', id, 'mysql', '--protocol=TCP', '-h127.0.0.1', '-uroot', '--batch', '--raw', '--skip-column-names', '--default-character-set=utf8mb4', 'ocr_images_probe'], text)
  const actual = Number(r.stderr?.match(/ERROR (\d+) /)?.[1]) || null
  results.push({ label, exit_code: r.status, expected_error: rejectCode ?? null, actual_error: actual })
  if (rejectCode) { assert.notEqual(r.status, 0, label + ' must reject'); assert.equal(actual, rejectCode, label) }
  else assert.equal(r.status, 0, label + ': ' + (r.stderr ?? '').replaceAll(password, '<redacted>').slice(-1000))
  console.log(label + ': ' + (rejectCode ? 'rejected as required' : 'passed'))
  return r.stdout.trim()
}
try {
  assert.match(readFileSync('backend/internal/migrations/target.go', 'utf8'), /const Target = 6\b/)
  writeFileSync(envfile, 'MYSQL_ROOT_PASSWORD=' + password + '\n', { mode: 0o600 })
  run('build pinned MySQL', ['build', '-t', image, 'deploy/identity/mysql'])
  report.image_id = run('exact image', ['image', 'inspect', '--format', '{{.Id}}', image])
  run('start owned isolated MySQL', ['run', '-d', '--name', id, '--network', 'none', '--memory', '1g', '--memory-swap', '1g', '--cpus', '1', '--mount', 'type=tmpfs,destination=/var/lib/mysql', '--env-file', envfile, report.image_id])
  let ready = false
  for (let i = 0; i < 120; i++) {
    const r = docker(['exec', '-e', 'MYSQL_PWD', id, 'mysql', '--protocol=TCP', '-h127.0.0.1', '-uroot', '--batch', '--skip-column-names', '-e', 'SELECT 1'], undefined, 2000)
    if (r.status === 0 && r.stdout.trim() === '1') { ready = true; break }
    await new Promise(r => setTimeout(r, 500))
  }
  assert.ok(ready)
  run('create only probe schema', ['exec', '-i', '-e', 'MYSQL_PWD', id, 'mysql', '--protocol=TCP', '-h127.0.0.1', '-uroot'], 'CREATE DATABASE ocr_images_probe CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin')
  report.mysql_version = sql('actual version', 'SELECT VERSION()'); assert.match(report.mysql_version, /^8\.4\.13/)
  for (const path of sourcePaths.slice(2)) sql('baseline ' + path.split('/').at(-1), readFileSync(path, 'utf8'))
  sql('two classes and synthetic user', "INSERT INTO users(id,username,password_hash,role) VALUES(1,'image_probe','not-a-login-hash','teacher'); INSERT INTO classes(id,stable_key,name) VALUES(1,'one','one'),(2,'two','two');")
  const insert = (id, format, status, attempts, pages, body = 'NULL') => `INSERT INTO documents(id,class_id,created_by,title,format,version,status,original_bytes,original_digest,size_bytes,body,page_count,attempts,created_at,updated_at) VALUES(${id},1,1,'自制课程','${format}',1,'${status}',X'0061FF',UNHEX(SHA2(X'0061FF',256)),3,${body},${pages},${attempts},UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`
  sql('old ready PDF, Word and scanned PDF', insert(1,'pdf','ready',1,2,"'旧PDF'") + ';' + insert(2,'docx','ready',1,'NULL',"'旧Word'") + ';' + insert(3,'pdf','needs_ocr',1,4))
  const snapshot = () => sql('old metadata/raw bytes snapshot', 'SELECT id,format,status,version,HEX(original_bytes),HEX(original_digest),size_bytes,HEX(body),page_count,attempts FROM documents WHERE id<=3 ORDER BY id')
  const before = snapshot()
  let ddl = readFileSync(candidate, 'utf8')
  if (mutate) { assert.ok(ddl.includes("status='needs_ocr' AND attempts=0")); ddl = ddl.replace("status='needs_ocr' AND attempts=0", "status='needs_ocr' AND attempts=1") }
  sql('candidate single atomic ALTER', ddl)
  assert.equal(snapshot(), before)
  sql('PNG/JPEG needs_ocr admission without fake parse attempt', insert(10,'png','needs_ocr',0,1) + ';' + insert(11,'jpeg','needs_ocr',0,1))
  const rejections = [
    ['image cannot become ready', "UPDATE documents SET status='ready',attempts=1,body='fabricated' WHERE id=10", 3819],
    ['image cannot become parse queued', "UPDATE documents SET status='queued',page_count=NULL WHERE id=10", 3819],
    ['image cannot consume parse attempt', 'UPDATE documents SET attempts=1 WHERE id=10', 3819],
    ['image page count is exactly one', 'UPDATE documents SET page_count=2 WHERE id=10', 3819],
    ['NULL cannot evade image page constraint', 'UPDATE documents SET page_count=NULL WHERE id=10', 3819],
    ['image cannot fabricate original body', "UPDATE documents SET body='OCR must use revisions' WHERE id=10", 3819],
    ['image original version immutable', 'UPDATE documents SET version=2 WHERE id=10', 3819],
    ['raw length still enforced', 'UPDATE documents SET size_bytes=2 WHERE id=10', 3819],
    ['old Word still cannot needs_ocr', "UPDATE documents SET status='needs_ocr',body=NULL,page_count=1 WHERE id=2", 3819],
    ['old PDF ready requires pages', 'UPDATE documents SET page_count=NULL WHERE id=1', 3819],
    ['old PDF cannot zero-attempt needs_ocr', 'UPDATE documents SET attempts=0 WHERE id=3', 3819],
    ['old Word ready requires body', 'UPDATE documents SET body=NULL WHERE id=2', 3819],
    ['original source cross-class FK', "INSERT INTO document_ocr_revisions VALUES(10,2,1,1,'ocr','probe',JSON_ARRAY(JSON_OBJECT('page',1)),UTC_TIMESTAMP(3))", 1452],
  ]
  for (const [label, text, code] of rejections) sql(label, text, code)
  sql('existing OCR composite FK accepts image original', "INSERT INTO document_ocr_revisions VALUES(10,1,1,1,'ocr','probe',JSON_ARRAY(JSON_OBJECT('page',1)),UTC_TIMESTAMP(3)); INSERT INTO document_ocr_heads VALUES(10,1,1,NULL)")
  assert.equal(snapshot(), before)
  assert.equal(sql('four original CHECK constraints retained', "SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name='documents' AND constraint_type='CHECK' AND enforced='YES'"), '4')
  assert.equal(sql('no parse jobs created', 'SELECT COUNT(*) FROM document_parse_jobs WHERE document_id IN(10,11)'), '0')
  report.negative_cases = rejections.length; report.legacy_snapshot_unchanged = true; report.exit_code = 0
} catch (error) { report.error = error.message.replaceAll(password, '<redacted>'); console.error(report.error) }
finally {
  report.cleanup = []
  for (const [args, absent] of [[['rm','-f',id],'No such container'], [['image','rm',image],'No such image']]) {
    const r = docker(args), ok = r.status === 0 || (r.stderr ?? '').includes(absent)
    report.cleanup.push({ resource: args.at(-1), ok }); if (!ok) report.exit_code = 1
  }
  if (existsSync(envfile)) unlinkSync(envfile)
  report.secret_file_removed = !existsSync(envfile)
  writeFileSync(directory + '/results.json', JSON.stringify({ ...report, results }, null, 2) + '\n')
}
process.exitCode = report.exit_code
