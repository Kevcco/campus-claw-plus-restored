import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, existsSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { runRuntimeBatch } from './runtime-batch-runner.mjs'

const step = (name, code, args = [], timeoutMs = 5000) => ({ name, command: process.execPath,
  args: ['--input-type=module', '-e', code, ...args], timeoutMs })
const quiet = { stdio: 'ignore' }

test('DI-SC-14 real runtime children wait for images, then must overlap to pass', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'campus-batch-'))
  try {
    const ready = join(dir, 'images'), left = join(dir, 'left'), right = join(dir, 'right')
    const setup = step('images', "import{writeFileSync}from'node:fs';writeFileSync(process.argv[1],'ready')", [ready])
    const child = "import{writeFileSync,existsSync}from'node:fs';const[r,own,other]=process.argv.slice(1);if(!existsSync(r))process.exit(12);writeFileSync(own,'started');const timer=setInterval(()=>{if(existsSync(other)){clearInterval(timer);process.exit(0)}},5);setTimeout(()=>process.exit(13),2000).unref()"
    const r = await runRuntimeBatch([setup, step('https', child, [ready,left,right]), step('sql', child, [ready,right,left])], quiet)
    assert.equal(r.exitCode, 0); assert.deepEqual(r.notRun, [])
    assert.deepEqual(r.results.map(x => [x.name,x.exitCode]), [['images',0],['https',0],['sql',0]])
  } finally { rmSync(dir, { recursive: true, force: true }) }
})

test('DI-SC-14 failed prerequisite blocks both; failed sibling still waits for real cleanup', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'campus-batch-'))
  try {
    const marker = join(dir, 'cleanup'), write = step('sql', "import{writeFileSync}from'node:fs';setTimeout(()=>writeFileSync(process.argv[1],'cleaned'),150)", [marker])
    const r = await runRuntimeBatch([step('images','process.exit(7)'), step('https','process.exit(0)'), write], quiet)
    assert.equal(r.exitCode, 7); assert.deepEqual(r.notRun, ['https','sql']); assert.equal(existsSync(marker), false)
    const siblings = await runRuntimeBatch([step('images',''), step('https','process.exit(9)'), write], quiet)
    assert.equal(siblings.exitCode, 9); assert.deepEqual(siblings.notRun, [])
    assert.equal(siblings.results[2].exitCode, 0); assert.equal(existsSync(marker), true)
    await assert.rejects(runRuntimeBatch([]), /Exactly one/)
  } finally { rmSync(dir, { recursive: true, force: true }) }
})

test('DI-SC-14 actual timeout and cancellation remain failures; both children terminate', async () => {
  const timed = await runRuntimeBatch([step('images',''), step('https','setTimeout(()=>{},20000)',[],100), step('sql','')], quiet)
  assert.equal(timed.exitCode, 124); assert.equal(timed.results[1].reason, 'timeout'); assert.equal(timed.results[2].exitCode, 0)
  const controller = new AbortController(); let started = 0
  const cancelled = await runRuntimeBatch([step('images',''), step('https','setTimeout(()=>{},20000)'), step('sql','setTimeout(()=>{},20000)')], {
    ...quiet, signal: controller.signal, onStart: name => { if(name!=='images' && ++started===2) controller.abort() },
  })
  assert.equal(cancelled.exitCode, 130)
  assert.deepEqual(cancelled.results.slice(1).map(x=>x.reason), ['cancelled','cancelled'])
})
