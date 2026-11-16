import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {fileURLToPath} from 'node:url'
import Ajv2020 from 'ajv/dist/2020.js'
import SwaggerParser from '@apidevtools/swagger-parser'

const root=new URL('../docs/contracts/ocr-draft/',import.meta.url)
const read=name=>JSON.parse(readFileSync(new URL(name,root)))
const result=read('OCRResult.schema.json'),source=read('private-openapi.json')
const canonical=value=>process.platform==='win32'?value.replaceAll('\\','/').replace(/^[A-Z]:/,v=>v.toLowerCase()):value
const allowed=[fileURLToPath(new URL('private-openapi.json',root)),fileURLToPath(new URL('OCRResult.schema.json',root)),result.$id].map(canonical)
const api=await SwaggerParser.validate(fileURLToPath(new URL('private-openapi.json',root)),{resolve:{file:false,http:false,ocr:{order:1,canRead:f=>allowed.includes(canonical(f.url)),read:f=>JSON.stringify(f.url.endsWith('private-openapi.json')?source:result)}},dereference:{circular:false}})
const strip=value=>Array.isArray(value)?value.map(strip):value&&typeof value==='object'?Object.fromEntries(Object.entries(value).filter(([k])=>!['$id','$schema'].includes(k)).map(([k,v])=>[k,strip(v)])):value
const direct=new Ajv2020({strict:true}).compile(result)

test('DI-SC-16/19 private OCR exact fixed origin, canonical auth and three bounded binary media',()=>{
 const profile=read('private-rpc.json')
 assert.equal(api['x-runtime-activation'],false);assert.equal(api['x-contract-status'],'candidate-component')
 assert.deepEqual(api.servers,[{url:profile.origin}]);assert.deepEqual(api.security,[{OCRBearer:[]}])
 assert.deepEqual(Object.keys(api.paths),['/health','/internal/ocr'])
 assert.deepEqual(Object.keys(api.paths['/internal/ocr'].post.requestBody.content),profile.implemented_media)
 assert.equal(api.paths['/internal/ocr'].post.requestBody.required,true)
 for(const op of [api.paths['/health'].get,api.paths['/internal/ocr'].post]){assert.equal(op.servers,undefined);assert.equal(op.security,undefined);assert.equal(op.parameters,undefined)}
 for(const media of Object.values(api.paths['/internal/ocr'].post.requestBody.content)){assert.deepEqual(media.schema,{type:'string',format:'binary'});assert.equal(media['x-max-bytes'],10485760)}
 assert.deepEqual(profile.planned_media,[]);assert.equal(profile.image_policy.single_frame,true);assert.equal(profile.image_policy.page_ms,15000)
})

test('DI-SC-17/19 exact HTTP status and single-source success/failure data agree',()=>{
 const responses=api.paths['/internal/ocr'].post.responses
 assert.deepEqual(Object.keys(responses),['200','400','401','404','405','422','503','504'])
 const success=JSON.parse(readFileSync('reports/changes/04E/ocr-rpc/actual-source.json'))
 const check=status=>new Ajv2020({strict:true}).compile(strip(responses[status].content['application/json'].schema))
 assert.equal(direct(success),true);assert.equal(check(200)(success),true)
 for(const [status,reasons]of Object.entries({400:['corrupt'],401:['internal_failure'],404:['unsupported_content'],405:['unsupported_content'],422:['corrupt','encrypted','unsupported_content','resource_limit'],503:['internal_failure','worker_interrupted','language_missing'],504:['ocr_timeout']})){
  for(const reason of reasons){const failed={status:'failed',reason};assert.equal(direct(failed),true);assert.equal(check(status)(failed),true);assert.equal(check(200)(failed),false);assert.equal(check(status)({...failed,body:'partial text'}),false)}
  assert.equal(check(status)(success),false)
 }
 assert.equal(check(503)({status:'failed',reason:'resource_limit'}),false)
 for(const response of Object.values(responses)){assert.equal(response.headers['Cache-Control'].schema.const,'no-store');assert.equal(response.headers['X-Request-ID'].schema.pattern,'^[a-f0-9]{32}$')}
 const healthy=new Ajv2020({strict:true}).compile(api.paths['/health'].get.responses[200].content['application/json'].schema)
 assert.equal(healthy({status:'ok'}),true);assert.equal(healthy({status:'ok',dependencies:{}}),false)
})
