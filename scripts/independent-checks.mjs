import { execFile } from 'node:child_process'

// Independent, isolated checks may overlap, but cleanup and success wait for ALL.
export async function independentChecks(checks) {
  if (!Array.isArray(checks) || checks.length !== 3 || checks.some(f => typeof f !== 'function')) throw new Error('Exactly three isolated component checks required')
  const results = await Promise.allSettled(checks.map(check => Promise.resolve().then(check)))
  const failures = results.filter(result => result.status === 'rejected')
  if (failures.length) throw new AggregateError(failures.map(result => result.reason), 'Independent component check failed')
  return results.map(result => result.value)
}

export function checkedProcess(command, args, timeout) {
  const started = performance.now()
  return new Promise(resolve => execFile(command, args, { encoding: 'utf8', timeout, maxBuffer: 16 * 1024 * 1024 }, (error, stdout, stderr) => resolve({
    status: error ? (Number.isInteger(error.code) ? error.code : null) : 0,
    error: error ? { message: error.message, killed: error.killed } : undefined,
    stdout: stdout ?? '', stderr: stderr ?? '', duration_ms: Math.round(performance.now() - started),
  })))
}
