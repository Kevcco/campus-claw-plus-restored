import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { createHash, randomBytes } from 'node:crypto'
import { mkdirSync, readFileSync, realpathSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { independentChecks, checkedProcess } from './independent-checks.mjs'
import { databaseArgs, scanner } from './verify-image-audits.mjs'

// DI-SC-02..06/14: offline core + private HTTP component, no default service/config/SQL activation.
const scan = process.argv.length === 3 && process.argv[2] === '--scan'
if (process.argv.length > 2 && !scan) throw new Error('Expected no arguments or --scan')
const id = 'campus-di-' + randomBytes(6).toString('hex')
const tag = id + ':check'
const container = id + '-run'
const directory = resolve('output/document-parser', scan ? 'scan' : 'check')
mkdirSync(directory, { recursive: true })
const sha = value => createHash('sha256').update(value).digest('hex')
const git = args => spawnSync('git', args, { encoding: 'utf8' }).stdout.trim()
const inputs = ['core.py', 'worker.py', 'runner.py', 'server.py', 'health.py', 'test_rpc.py', 'test_health.py', 'requirements.lock', 'Dockerfile', '.dockerignore']
const clientInputs = ['backend/internal/documentparser/client.go', 'backend/internal/documentparser/client_test.go', 'scripts/check-document-parser-client.mjs', 'scripts/check-document-parser.mjs', 'scripts/independent-checks.mjs']
const fingerprint = () => sha([...inputs.map(p => 'document-parser/' + p), ...clientInputs].map(p => p + ':' + sha(readFileSync(p))).join('\n'))
const source = { commit: git(['rev-parse', 'HEAD']), dirty: git(['status', '--porcelain']) !== '', fingerprint: fingerprint() }
const toolchain = JSON.parse(readFileSync('scripts/toolchain.json'))
assert.ok(readFileSync('document-parser/Dockerfile', 'utf8').split('\n').filter(l => l.startsWith('FROM '))
  .every(l => l.split(' ')[1] === toolchain.documentPythonImage), 'Python image must match its pinned register')
assert.equal(readFileSync('document-parser/requirements.lock', 'utf8').trim(),
  `pypdf==${toolchain.documentPypdfVersion} --hash=sha256:${toolchain.documentPypdfWheelSha256}`)
const results = []
let report = { time: new Date().toISOString(), run_id: id, scope: '04E core/private HTTP/client/probe component tests; five-service production wiring is validated separately', source, exit_code: 1 }
async function run(label, args, timeout = 180000) {
  const child = await checkedProcess('docker', args, timeout)
  results.push({ label, duration_ms: child.duration_ms, exit_code: child.status, error: child.error?.message,
    stdout: child.stdout ?? '', stderr: child.stderr ?? '' })
  console.log(`Document parser ${label}: exit ${child.status}`)
  if (child.status !== 0) throw new Error(`${label} failed; see output/document-parser/${scan ? 'scan' : 'check'}/results.json`)
  return child.stdout
}
try {
  await run('build pinned candidate', ['build', '-t', tag, 'document-parser'])
  const imageId = (await run('record immutable image', ['image', 'inspect', '--format', '{{.Id}}', tag])).trim()
  assert.match(imageId, /^sha256:[a-f0-9]{64}$/)
  report.image_id = imageId
  // In scan mode test the exact same immutable image before exporting it;
  // separate builds may carry distinct provenance/manifest IDs.
  {
    const corpus = JSON.parse(readFileSync('document-parser/testdata/acceptance-v1/corpus-lock.json'))
    for (const [file, hash] of Object.entries(corpus.sha256)) {
      assert.equal(sha(readFileSync('document-parser/testdata/acceptance-v1/' + file)), hash, 'Frozen corpus drift: ' + file)
    }
    const componentStarted = performance.now()
    await independentChecks([async () => {
    const raw = await run('actual bounded acceptance', ['run', '--rm', '--name', container + '-core',
      '--network', 'none', '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges',
      '--memory', '512m', '--memory-swap', '512m', '--cpus', '1', '--pids-limit', '32',
      '--tmpfs', '/tmp:rw,noexec,nosuid,size=64m',
      '--mount', `type=bind,source=${resolve('document-parser')},target=/tests,readonly`,
      '--entrypoint', 'python', imageId, '-B', '-I', '/tests/test_core.py'], 180000)
    writeFileSync(resolve(directory, 'tests.json'), raw)
    const tests = JSON.parse(raw)
    assert.equal(tests.exit_code, 0)
    assert.equal(tests.acceptance.length, 40)
    assert.equal(tests.negative.length, 25)
    assert.deepEqual(tests.unexecuted_negative, [])
    assert.ok(tests.acceptance.every(c => c.cer <= .005 && c.locations > 0 && c.critical_phrases > 0 && c.ms <= 30000))
    report.tests = { groups: tests.groups, clear_documents: 40, negative_classes: 25,
      max_cer: Math.max(...tests.acceptance.map(c => c.cer)),
      max_document_ms: Math.max(...tests.acceptance.map(c => c.ms)), raw_sha256: sha(raw) }
    report.isolation = tests.isolation
    }, async () => {
    const rpcRaw = await run('private HTTP adapter real corpus and boundary tests', ['run', '--rm', '--name', container + '-http',
      '--network', 'none', '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges',
      '--memory', '512m', '--memory-swap', '512m', '--cpus', '1', '--pids-limit', '32',
      '--tmpfs', '/tmp:rw,noexec,nosuid,size=64m',
      '--mount', `type=bind,source=${resolve('document-parser')},target=/tests,readonly`,
      '--entrypoint', 'python', imageId, '-B', '-I', '/tests/test_rpc.py'], 180000)
    const rpc = JSON.parse(rpcRaw)
    assert.equal(rpc.exit_code, 0)
    assert.equal(rpc.groups, 7)
    assert.equal(rpc.safe_logs, true)
    assert.equal(rpc.documents.length, 40)
    report.private_http = rpc
    const healthRaw = await run('actual authenticated health and startup process tests', ['run', '--rm', '--name', container + '-health',
      '--network', 'none', '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges',
      '--memory', '512m', '--memory-swap', '512m', '--cpus', '1', '--pids-limit', '32',
      '--tmpfs', '/tmp:rw,noexec,nosuid,size=64m', '--mount', `type=bind,source=${resolve('document-parser')},target=/tests,readonly`,
      '--entrypoint', 'python', imageId, '-B', '-I', '/tests/test_health.py'], 30000)
    report.private_health = JSON.parse(healthRaw)
    assert.equal(report.private_health.exit_code,0); assert.equal(report.private_health.groups,3); assert.equal(report.private_health.safe_logs,true)
    }, async () => {
    const client = await checkedProcess(process.execPath, ['scripts/check-document-parser-client.mjs', '--image', imageId], 240000)
    assert.equal(client.status, 0, 'Real Go/private HTTP interoperability failed: ' + (client.stdout ?? '') + (client.stderr ?? ''))
    report.private_client = JSON.parse(client.stdout)
    assert.equal(report.private_client.actual.documents.length, 40)
    assert.equal(report.private_client.secret_file_removed, true)
    }])
    report.component_schedule = { isolated_groups: 3, all_settled_before_cleanup: true, duration_ms: Math.round(performance.now() - componentStarted) }
  }
  if (scan) {
    const workspace = realpathSync('.')
    const workspaceId = sha(process.platform === 'win32' ? workspace.toLowerCase() : workspace).slice(0, 16)
    const cache = `campus-trivy-db-${workspaceId}`
    await run('export candidate without scanner socket', ['image', 'save', '-o', resolve(directory, 'image.tar'), imageId])
    report.image_export_sha256 = sha(readFileSync(resolve(directory, 'image.tar')))
    try {
      await run('Trivy candidate HIGH/CRITICAL', ['run', '--rm', '--name', container,
        '--mount', `type=bind,source=${directory},target=/reports`, '-v', `${cache}:/root/.cache/trivy`, scanner,
        'image', '--input', '/reports/image.tar', ...databaseArgs, '--cache-backend', 'memory',
        '--scanners', 'vuln', '--severity', 'HIGH,CRITICAL', '--exit-code', '1', '--list-all-pkgs',
        '--no-progress', '--timeout', '3m', '--format', 'json', '--output', '/reports/parser.json'], 210000)
    } finally {
      try { report.raw_sha256 = sha(readFileSync(resolve(directory, 'parser.json'))) } catch {}
    }
    const raw = JSON.parse(readFileSync(resolve(directory, 'parser.json')))
    assert.ok(raw.Metadata?.OS?.Family && raw.Results?.some(r => r.Class === 'os-pkgs'), 'OS packages must be analyzed')
    assert.ok(raw.Results.some(r => r.Packages?.some(p => p.Name === 'pypdf' && p.Version === toolchain.documentPypdfVersion)), 'Real pinned pypdf must be recognized')
    assert.equal(raw.Results.flatMap(r => r.Vulnerabilities ?? []).length, 0, 'High/critical finding blocks candidate')
    report.scanner = scanner
    report.findings = { HIGH: 0, CRITICAL: 0 }
    report.database = JSON.parse(await run('record actual vulnerability database', ['run', '--rm', '--name', container,
      '--network', 'none', '-v', `${cache}:/root/.cache/trivy`, '--entrypoint', 'cat', scanner,
      '/root/.cache/trivy/db/metadata.json']))
  }
  assert.equal(fingerprint(), source.fingerprint, 'Parser source changed during verification')
  report.exit_code = 0
} catch (error) {
  report.error = error.message
  if (error instanceof AggregateError) report.component_errors = error.errors.map(e => e.message)
  console.error(error.message)
} finally {
  report.cleanup = []
  for (const [args, absent] of [...[container, container + '-core', container + '-http', container + '-health'].map(name => [['rm', '-f', name], 'No such container']), [['image', 'rm', tag], 'No such image']]) {
    const child = spawnSync('docker', args, { encoding: 'utf8', timeout: 30000 })
    const ok = child.status === 0 || (child.stderr ?? '').includes(absent)
    report.cleanup.push({ resource: args.at(-1), exit_code: child.status, ok })
    if (!ok) report.exit_code = 1
  }
  // An exported image is a local artifact, not a published attachment. Keep
  // only its hash and scanner raw report; remove this own exact file safely.
  if (scan) {
    const { unlinkSync, existsSync } = await import('node:fs')
    const file = resolve(directory, 'image.tar')
    if (existsSync(file)) unlinkSync(file)
  }
  report.finished_at = new Date().toISOString()
  writeFileSync(resolve(directory, 'results.json'), JSON.stringify({ ...report, results }, null, 2) + '\n')
}
process.exitCode = report.exit_code
