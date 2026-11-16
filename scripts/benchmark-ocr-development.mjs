// DI-SC-16/19 selection experiment only; explicit image ID, never production activation.
import assert from 'node:assert/strict'
import { mkdirSync, readFileSync, writeFileSync, copyFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { spawnSync } from 'node:child_process'
import { createHash, randomBytes } from 'node:crypto'
const [image, corpusArg, driverArg, scoreModuleArg] = process.argv.slice(2)
assert.match(image ?? '', /^sha256:[a-f0-9]{64}$/)
const root = resolve(corpusArg ?? 'document-parser/testdata/ocr-v1')
const { normalizeText, microCER } = await import(scoreModuleArg ? pathToFileURL(resolve(scoreModuleArg)).href : './ocr-corpus.mjs')
const driver = resolve(driverArg ?? 'scripts/ocr-development-driver.py')
const manifest = JSON.parse(readFileSync(root+'/manifest.json'))
const samples = manifest.pages.filter(p => p.split.startsWith('development-'))
assert.equal(samples.length, 14)
const id = 'campus-ocr-dev-' + randomBytes(6).toString('hex'), dir = resolve('output',id)
mkdirSync(dir+'/inputs',{recursive:true})
const mapping = samples.map((p,i)=>{
  const file = 'page-'+String(i+1).padStart(2,'0')+(p.file.endsWith('.png')?'.png':'.jpg')
  const bytes=readFileSync(root+'/'+p.file);assert.equal(createHash('sha256').update(bytes).digest('hex'),p.sha256)
  copyFileSync(root+'/'+p.file,dir+'/inputs/'+file);return {file,id:p.id,sha256:p.sha256,split:p.split}
})
writeFileSync(dir+'/mapping.json',JSON.stringify(mapping,null,2)+'\n')
const args=['run','--rm','--name',id,'--network','none','--user','65532:65532','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--memory','512m','--memory-swap','512m','--cpus','1','--pids-limit','32','--tmpfs','/tmp:rw,noexec,nosuid,size=64m','--mount',`type=bind,source=${dir}/inputs,target=/inputs,readonly`,'--mount',`type=bind,source=${driver},target=/driver.py,readonly`,'--entrypoint','python',image,'-B','-I','/driver.py']
let summary={image_id:image,blind_ocr_executed:false,scope:'14 development pages only; not B acceptance',exit_code:1}
try {
  const run=spawnSync('docker',args,{encoding:'utf8',timeout:240000,maxBuffer:16<<20})
  writeFileSync(dir+'/raw-stdout.json',run.stdout??'');writeFileSync(dir+'/driver-stderr.txt',run.stderr??'')
  assert.equal(run.status,0,'Development experiment driver failed')
  const raw=JSON.parse(run.stdout);assert.equal(raw.rows.length,14);assert.equal(raw.uid,65532);assert.equal(raw.blind_inputs,0);assert.equal(raw.tmp_output_files_after,0)
  const rows=raw.rows.map((r,i)=>{
    const p=samples[i];assert.equal(r.file,mapping[i].file);assert.equal(r.sha256,p.sha256)
    const words=(r.tsv??'').trim().split('\n').slice(1).map(line=>line.split('\t')).filter(f=>f[0]==='5' && f.length>=12 && f[11].trim()).map(f=>({confidence:Number(f[10]),text:f.slice(11).join('\t')}))
    assert.ok(words.every(w=>Number.isFinite(w.confidence)&&w.confidence>=0&&w.confidence<=100),'Malformed engine confidence must not pass diagnostics')
    const mean=words.length?words.reduce((s,w)=>s+w.confidence,0)/words.length:0
    const low=words.length?words.filter(w=>w.confidence<50).length/words.length:1
    // Provisional developer-only diagnostic, not calibrated or a ready decision.
    const needsAttention=r.exit_code!==0 || !normalizeText(r.text??'') || mean<80 || low>.2
    return {id:p.id,split:p.split,language:p.language,exit_code:r.exit_code,elapsed_ms:r.elapsed_ms,peak_bytes:r.container_peak_bytes_so_far,...microCER([{gold:p.gold,actual:r.text??''}]),words:words.length,mean_confidence:mean,low_confidence_fraction:low,provisional_attention:needsAttention}
  })
  const positives=samples.map((p,i)=>({p,i})).filter(x=>x.p.split==='development-positive')
  summary={...summary,exit_code:0,parameters:raw.parameters,rows,positive_micro:microCER(positives.map(({p,i})=>({gold:p.gold,actual:raw.rows[i].text??''}))),negative_attention:rows.filter(r=>r.split==='development-negative'&&r.provisional_attention).length,negative_denominator:10,quality_acceptance_passed:false}
}finally{
  const cleanup=spawnSync('docker',['rm','-f',id],{encoding:'utf8',timeout:30000})
  summary.cleanup=cleanup.status===0||(cleanup.stderr??'').includes('No such container')
  if (!summary.cleanup) summary.exit_code=1
  writeFileSync(dir+'/summary.json',JSON.stringify(summary,null,2)+'\n');console.log(JSON.stringify({directory:dir,...summary}))
}
process.exitCode=summary.exit_code
