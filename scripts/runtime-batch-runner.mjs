import { runSequence, runStage } from './gate-runner.mjs'

// Images are a prerequisite. The following HTTPS and SQL suites own separate
// databases, networks, ports and evidence directories. Await BOTH before the
// caller validates evidence or removes retained images, including on failure.
export async function runRuntimeBatch(steps, options = {}) {
  if (!Array.isArray(steps) || steps.length !== 3) throw new Error('Exactly one image prerequisite and two isolated runtime suites required')
  const first = await runSequence(steps.slice(0, 1), options)
  if (first.exitCode !== 0) return { ...first, notRun: steps.slice(1).map(s => s.name) }
  const settled = await Promise.allSettled(steps.slice(1).map(step => runStage(step, options)))
  const siblings = settled.map((r, i) => r.status === 'fulfilled' ? r.value : {
    name: steps[i + 1].name, exitCode: 1, reason: 'runner_error', signal: null,
  })
  return { exitCode: siblings.find(r => r.exitCode !== 0)?.exitCode ?? 0,
    results: [...first.results, ...siblings], notRun: [] }
}
