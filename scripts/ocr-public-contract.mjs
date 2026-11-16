import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {fileURLToPath} from 'node:url'
import SwaggerParser from '@apidevtools/swagger-parser'
import Ajv2020 from 'ajv/dist/2020.js'

const directory=new URL('../docs/contracts/',import.meta.url)
const files=['ocr-public/openapi.json','ocr-public/OCRPublic.schema.json','ocr-draft/OCRResult.schema.json','ocr-draft/OCRReview.schema.json','ocr-draft/OCRReviewReceipt.schema.json','schemas/ID.schema.json','schemas/Error.schema.json']
const canonical=p=>process.platform==='win32'?p.replaceAll('\\','/').replace(/^[A-Z]:/,v=>v.toLowerCase()):p
const strip=value=>Array.isArray(value)?value.map(strip):value&&typeof value==='object'?Object.fromEntries(Object.entries(value).filter(([k])=>!['$id','$schema'].includes(k)).map(([k,v])=>[k,strip(v)])):value

export async function loadOCRPublic(){
 const sources=files.map(p=>({path:fileURLToPath(new URL(p,directory)),value:JSON.parse(readFileSync(new URL(p,directory)))}))
 const allowed=new Map(sources.flatMap(({path,value})=>[[canonical(path),value],...(value.$id?[[value.$id,value]]:[])]))
 const api=await SwaggerParser.validate(sources[0].path,{resolve:{file:false,http:false,localOCR:{order:1,canRead:f=>allowed.has(canonical(f.url)),read:f=>JSON.stringify(allowed.get(canonical(f.url)))}},dereference:{circular:false}})
 const ajv=new Ajv2020({strict:true,strictRequired:false,allErrors:true});for(const {value} of sources.slice(1))ajv.addSchema(value)
 const root=sources[1].value.$id
 const direct=Object.fromEntries(['Status','Revision','Citation','Accepted','Search'].map(n=>[n,ajv.getSchema(root+'#/$defs/'+n)]))
 const via=Object.fromEntries(Object.keys(direct).map(n=>[n,new Ajv2020({strict:true,strictRequired:false}).compile(strip(api.components.schemas[n]))]))
 return{api,direct,via,response:(path,method,status)=>new Ajv2020({strict:true,strictRequired:false}).compile(strip(api.paths[path][method].responses[status].content['application/json'].schema))}
}

export function revisionSemantics(value){
 assert.equal(value.body,value.pages.map(p=>p.text).join('\n\n'))
 assert.ok(Buffer.byteLength(value.body)<=2097152)
 value.pages.forEach((p,i)=>assert.equal(p.page,i+1))
}
export function statusSemantics(value){
 if(value.published_revision!==null){assert.ok(value.latest_revision>=value.published_revision);assert.equal(value.status,'ready')}
 if(value.status==='needs_review'){assert.ok(value.latest_revision>=1);assert.equal(value.published_revision,null)}
 if(value.can_start){assert.equal(value.status,'idle');assert.equal(value.original_status,'needs_ocr')}
 if(value.can_retry){assert.equal(value.status,'failed');assert.ok(value.attempts<3);assert.ok(['ocr_timeout','worker_interrupted','internal_failure'].includes(value.failure_reason))}
}
export function citationSemantics(value,revision){
 assert.equal(value.char_end-value.char_start,[...value.quote].length)
 assert.equal(value.url,`/api/v1/documents/${value.document_id}/ocr-revisions/${value.revision}`)
 if(revision){assert.equal(value.document_id,revision.document_id);assert.equal(value.revision,revision.revision);assert.equal([...revision.body].slice(value.char_start,value.char_end).join(''),value.quote)}
}
