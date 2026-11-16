// DI-SC-16/17/19: only development PDFs enter the recognizer; gold stays on host.
import assert from 'node:assert/strict'
import {readFileSync,writeFileSync,mkdirSync,copyFileSync} from 'node:fs'
import {resolve} from 'node:path'
import {spawnSync} from 'node:child_process'
import {createHash,randomBytes} from 'node:crypto'
import {microCER} from './ocr-corpus.mjs'
import {ocrResultFromTSV} from './ocr-tsv.mjs'
const image=process.argv[2];assert.match(image??'',/^sha256:[0-9a-f]{64}$/)
const root='document-parser/testdata/ocr-v1',manifest=JSON.parse(readFileSync(root+'/manifest.json'))
const name='campus-ocr-pdf-'+randomBytes(6).toString('hex'),directory=resolve('output',name);mkdirSync(directory,{recursive:true})
const rows=[];let success=false
try{
 for(const pdf of manifest.pdfs.filter(p=>p.split.startsWith('development-'))){
  const bytes=readFileSync(root+'/'+pdf.file);assert.equal(createHash('sha256').update(bytes).digest('hex'),pdf.sha256)
  copyFileSync(root+'/'+pdf.file,directory+'/input.pdf')
  const child=spawnSync('docker',['run','--rm','--name',name,'--network','none','--read-only','--user','65532:65532','--cap-drop','ALL','--security-opt','no-new-privileges','--memory','512m','--memory-swap','512m','--cpus','1','--pids-limit','32','--tmpfs','/tmp:rw,noexec,nosuid,size=64m','--mount',`type=bind,source=${directory}/input.pdf,target=/inputs/input.pdf,readonly`,'--mount',`type=bind,source=${resolve('document-parser/ocr_pdf.py')},target=/driver/ocr_pdf.py,readonly`,'--entrypoint','python',image,'-B','/driver/ocr_pdf.py','/inputs/input.pdf'],{encoding:'utf8',timeout:185000,maxBuffer:16<<20})
  writeFileSync(directory+'/'+pdf.split+'-stdout.json',child.stdout??'');writeFileSync(directory+'/'+pdf.split+'-stderr.txt',child.stderr??'');assert.equal(child.status,0,'Raster/OCR subprocess must succeed')
  const raw=JSON.parse(child.stdout),gold=manifest.pages.filter(p=>p.split===pdf.split);assert.equal(raw.pages.length,gold.length)
  const pages=raw.pages.map((p,i)=>{let adapted=null,empty=false;try{adapted=ocrResultFromTSV([p.tsv])}catch(e){assert.match(e.message,/Empty OCR page/);empty=true}return {page:p.page,id:gold[i].id,width:p.width,height:p.height,...microCER([{gold:gold[i].gold,actual:p.text}]),elapsed_ms:p.elapsed_ms,peak_bytes:p.peak_bytes,word_anchors:adapted?.pages[0].words.length??0,empty_rejected:empty,provisional_attention:empty||adapted.quality==='low_quality'}})
  rows.push({pdf:pdf.file,sha256:pdf.sha256,split:pdf.split,elapsed_ms:raw.elapsed_ms,pypdfium2:raw.pypdfium2,pdfium:raw.pdfium,pages,micro:microCER(raw.pages.map((p,i)=>({gold:gold[i].gold,actual:p.text})))})
 }
 assert.equal(rows.length,2);success=true
}finally{
 const cleanup=spawnSync('docker',['rm','-f',name],{encoding:'utf8',timeout:30000});const cleaned=cleanup.status===0||(cleanup.stderr??'').includes('No such container')
 const summary={image_id:image,rows,exit_code:success&&cleaned?0:1,cleanup:cleaned,blind_ocr_executed:false,gold_bbox_evaluated:false,production_acceptance:false}
 writeFileSync(directory+'/summary.json',JSON.stringify(summary,null,2)+'\n');console.log(JSON.stringify({directory,...summary}));process.exitCode=summary.exit_code
}
