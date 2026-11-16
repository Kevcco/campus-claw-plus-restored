import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {fileURLToPath} from 'node:url'
import Ajv2020 from 'ajv/dist/2020.js'
import SwaggerParser from '@apidevtools/swagger-parser'
import {validateOCRResult,validateReview} from './ocr-contract-semantics.mjs'
const root=new URL('../docs/contracts/ocr-draft/',import.meta.url)
const names=['OCRResult','OCRReview','OCRReviewReceipt']
const schemas=names.map(n=>JSON.parse(readFileSync(new URL(n+'.schema.json',root))))
const api=await SwaggerParser.validate(fileURLToPath(new URL('openapi.json',root)),{resolve:{http:false,localSchemas:{order:1,canRead:f=>schemas.some(s=>s.$id===f.url),read:f=>JSON.stringify(schemas.find(s=>s.$id===f.url))}},dereference:{circular:false}})
const ajv=new Ajv2020({allErrors:true,strict:true});schemas.forEach(s=>ajv.addSchema(s))
const direct=schemas.map(s=>ajv.getSchema(s.$id)),via=names.map(n=>new Ajv2020({strict:true}).compile(api.components.schemas[n]))
const valid={status:'needs_review',engine:'tesseract-5.5.4-fast-8741641',body:'课程材料',quality:'acceptable_candidate',pages:[{page:1,width:1000,height:1000,rotation_applied:0,source:'ocr',char_start:0,char_end:4,words:[{text:'课程材料',char_start:0,char_end:4,bbox:[.1,.1,.9,.2],engine_confidence:95}]}]}
const review={base_revision:2,acknowledged:true,pages:[{page:1,text:'教师核对的文字'}]},context={role:'teacher',sameClass:true,latestDraftRevision:2,pageCount:1}
function accepted(i,value){for(const v of [direct[i],via[i]])assert.equal(v(value),true,JSON.stringify(v.errors))}
function rejected(i,value){for(const v of [direct[i],via[i]])assert.equal(v(value),false)}
test('DI-SC-16/17 private draft single-source 2020-12 and OpenAPI paths accept only closed bounded data',()=>{
 accepted(0,valid);validateOCRResult(valid);accepted(0,{status:'failed',reason:'language_missing'})
 for(const mutate of [x=>x.status='ready',x=>x.class_id='1',x=>x.engine='unlocked',x=>x.pages[0].rotation_applied=45,x=>x.pages[0].words[0].bbox[2]=1.1,x=>x.pages[0].words[0].engine_confidence=101,x=>x.pages[0].page=101]){const x=structuredClone(valid);mutate(x);rejected(0,x)}
})
test('DI-SC-17 semantic pixel/range/quote/order/bbox guards reject structurally valid false citations',()=>{
 for(const mutate of [x=>x.pages[0].width=20000000,x=>x.pages[0].page=2,x=>x.pages[0].words[0].bbox=[.2,.1,.1,.2],x=>x.pages[0].words[0].text='伪造出处',x=>x.pages[0].words[0].char_end=3,x=>x.pages[0].char_end=3]){const x=structuredClone(valid);mutate(x);accepted(0,x);assert.throws(()=>validateOCRResult(x))}
})
test('DI-SC-18 review requires explicit acknowledgement, bounded content and server authority/CAS',()=>{
 accepted(1,review);validateReview(review,context);accepted(2,{document_id:'123',revision:3,status:'ready'})
 for(const mutate of [x=>x.acknowledged=false,x=>x.base_revision=0,x=>x.role='teacher',x=>x.pages[0].text='']){const x=structuredClone(review);mutate(x);rejected(1,x)}
 for(const c of [{...context,role:'student'},{...context,sameClass:false},{...context,latestDraftRevision:3},{...context,pageCount:2}])assert.throws(()=>validateReview(review,c))
 assert.throws(()=>validateReview({...review,pages:[{page:2,text:'text'}]},context))
})
test('DI-SC-25 candidate is explicitly draft and cannot activate current endpoints/config/migration',()=>{
 const limits=JSON.parse(readFileSync(new URL('limits.json',root)))
 assert.equal(api['x-contract-status'],'draft');assert.equal(api['x-runtime-activation'],false)
 assert.equal(limits.runtime_activation,false);assert.equal(limits.migration_frozen,false);assert.equal(limits.public_activation,false)
 assert.equal(limits.input_bytes,10485760);assert.equal(limits.page_pixels,20000000);assert.equal(limits.page_ms,15000);assert.equal(limits.task_ms,180000)
 const register=JSON.parse(readFileSync('docs/contracts/endpoint-register.json'));assert.equal(register.operations.filter(o=>o.status==='current').length,28)
 assert.ok(register.operations.every(o=>!String(o.path).includes('ocr-review')))
})
