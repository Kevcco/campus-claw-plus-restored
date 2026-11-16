import test from 'node:test'
import assert from 'node:assert/strict'
import { independentChecks, checkedProcess } from './independent-checks.mjs'

test('DI-SC-14 all isolated checks start before any completes; all results required', async () => {
  const started = [], release = []
  const run = independentChecks([0, 1, 2].map(i => async () => { started.push(i); await new Promise(r => release.push(r)); return i }))
  await new Promise(r => setImmediate(r))
  assert.deepEqual(started, [0, 1, 2])
  release.forEach(r => r())
  assert.deepEqual(await run, [0, 1, 2])
  await assert.rejects(independentChecks([async () => 0]), /Exactly three/)
})

test('DI-SC-14 real child failure does not return before siblings finish and propagates', async () => {
  const completed = []
  const run = independentChecks([0, 1, 2].map(i => async () => {
    const child = await checkedProcess(process.execPath, ['-e', `setTimeout(()=>{console.log('done');process.exit(${i === 0 ? 7 : 0})},${i * 100})`], 5000)
    completed.push(i); assert.equal(child.status, 0); return child.stdout
  }))
  await assert.rejects(run, AggregateError)
  assert.deepEqual(completed.sort(), [0, 1, 2])
})

test('DI-SC-14 synchronous failure, timeout and missing executable never turn into success', async () => {
  await assert.rejects(independentChecks([() => { throw Error('sync') }, async () => 0, async () => 0]), AggregateError)
  const timed = await checkedProcess(process.execPath, ['-e', 'setTimeout(()=>{},10000)'], 50)
  assert.notEqual(timed.status, 0); assert.equal(timed.error.killed, true)
  const missing = await checkedProcess('campus-deliberately-absent-executable', [], 1000)
  assert.notEqual(missing.status, 0)
})
