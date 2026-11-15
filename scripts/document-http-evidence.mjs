import assert from 'node:assert/strict'
export function validateDocumentHTTP(report){
  let previous=-1
  for(const stage of ['main','outage','persist']){
    const rows=report.results.filter(r=>r.label==='DI public HTTPS '+stage)
    assert.equal(rows.length,1,'Exactly one public HTTPS '+stage)
    const r=rows[0],index=report.results.indexOf(r);assert.ok(index>previous);previous=index
    assert.equal(r.exit_code,0);assert.ok(!/^\s*--- (?:FAIL|SKIP):/m.test(r.output))
    assert.equal((r.output.match(/^--- PASS: TestDocumentHTTPProduction \(\d+(?:\.\d+)?s\)$/gm)??[]).length,1)
    assert.ok(r.output.includes('DI public HTTP stage: '+stage+' PASS'))
  }
  const index=label=>{const rows=report.results.filter(r=>r.label===label);assert.equal(rows.length,1);assert.equal(rows[0].exit_code,0);return report.results.indexOf(rows[0])}
  assert.ok(index('DI stop actual parser only')<index('DI public HTTPS outage'))
  assert.ok(index('DI public HTTPS outage')<index('DI restore actual private parser'))
  assert.ok(index('recreate isolated HTTPS path')<index('DI public HTTPS persist'))
  const main=report.results.find(r=>r.label==='DI public HTTPS main').output
  assert.ok(main.includes('2 real formats; 5 actors; exact originals and persisted codepoint citations; replay and authorization PASS'))
  return {stages:['main','outage','persist'],real_formats:2,actors:5,default_https:true,real_parser:true,exact_originals:true,persistent_quotes:true,actual_parser_outage:true,container_recreation:true}
}
