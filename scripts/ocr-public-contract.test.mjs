import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {createHash} from 'node:crypto'
import {loadOCRPublic,revisionSemantics,statusSemantics,citationSemantics}from'./ocr-public-contract.mjs'
const {api,direct,via,response}=await loadOCRPublic()
const status={document_id:'91',title:'扫描',format:'pdf',original_version:1,original_status:'needs_ocr',page_count:1,status:'ready',latest_revision:2,published_revision:2,attempts:1,failure_reason:null,can_start:false,can_retry:false}
const revision={document_id:'91',original_version:1,revision:2,kind:'review',body:'课程😀',pages:[{page:1,text:'课程😀'}]}
const citation={document_id:'91',title:'扫描',revision:2,page:1,char_start:2,char_end:3,quote:'😀',url:'/api/v1/documents/91/ocr-revisions/2'}
const both=(name,value,expected)=>{for(const check of[direct[name],via[name]])assert.equal(check(value),expected,JSON.stringify(check.errors))}

test('DI-SC-18 public revision/status/citation single-source and OpenAPI agree on closed bounded resources',()=>{
 for(const[name,value]of Object.entries({Status:status,Revision:revision,Citation:citation,Accepted:{document_id:'91',status:'accepted'},Search:{items:[citation],mode:'keyword'}})){both(name,value,true);both(name,{...value,class_id:'2'},false)}
 revisionSemantics(revision);statusSemantics(status);citationSemantics(citation,revision)
 both('Revision',{...revision,original_version:2},false);both('Revision',{...revision,kind:'ocr'},false)
 both('Status',{...status,published_revision:null},false);both('Citation',{...citation,revision:1},false)
})
test('DI-SC-17/18 semantic Unicode byte budgets and immutable URLs reject valid-schema false claims',()=>{
 const wrong={...citation,char_start:1};both('Citation',wrong,true);assert.throws(()=>citationSemantics(wrong,revision))
 assert.throws(()=>citationSemantics({...citation,url:'/api/v1/documents/91'},revision))
 const huge={...revision,body:'中'.repeat(700000),pages:[{page:1,text:'中'.repeat(700000)}]};both('Revision',huge,true);assert.throws(()=>revisionSemantics(huge))
 assert.throws(()=>revisionSemantics({...revision,pages:[{page:2,text:revision.body}]}))
 assert.throws(()=>statusSemantics({...status,published_revision:3}))
 assert.throws(()=>statusSemantics({...status,status:'failed',published_revision:null,attempts:3,can_retry:true,failure_reason:'ocr_timeout'}))
})
test('DI-SC-19 exact seven operations freeze request/response budgets without production activation',()=>{
 const p=JSON.parse(readFileSync('docs/contracts/ocr-public/profile.json'))
 assert.equal(p.status,'frozen-component');assert.equal(p.runtime_activation,false);assert.equal(api['x-runtime-activation'],false)
 assert.deepEqual(Object.entries(api.paths).flatMap(([path,item])=>Object.keys(item).filter(m=>['get','post'].includes(m)).map(m=>({method:m.toUpperCase(),path,operationId:item[m].operationId}))),p.operations)
 assert.equal(p.operations.length,7);assert.deepEqual(api.security,[{bearerAuth:[]}]);assert.deepEqual(api.servers,[{url:'/'}])
 assert.equal(p.query_codepoints,2000);assert.equal(p.chunk_codepoints-p.overlap_codepoints,2097);assert.equal(p.review_request_bytes,12648448)
 assert.equal(api.paths['/api/v1/documents/{id}/ocr-reviews'].post.requestBody['x-max-bytes'],p.review_request_bytes)
 const register=JSON.parse(readFileSync('docs/contracts/endpoint-register.json'));assert.equal(register.operations.filter(x=>x.status==='current').length,28);assert.ok(register.operations.every(x=>!x.path.includes('/ocr-')))
})
test('DI-SC-18/19 exact public error envelope, 201/202 Locations and no-store apply',()=>{
 const path='/api/v1/documents/{id}/ocr-reviews',error={error:{code:'CONFLICT',message:'状态冲突',fields:[],details:null},request_id:'1'.repeat(32)}
 assert.equal(response(path,'post',409)(error),true);assert.equal(response(path,'post',404)(error),false)
 assert.equal(response(path,'post',201)({document_id:'91',revision:2,status:'ready'}),true)
 for(const item of Object.values(api.paths))for(const method of['get','post'])if(item[method])for(const r of Object.values(item[method].responses)){assert.equal(r.headers['Cache-Control'].schema.const,'no-store');assert.equal(r.headers['X-Request-ID']['x-required'],true)}
 for(const path of['/api/v1/documents/{id}/ocr-jobs','/api/v1/documents/{id}/ocr-retry']){assert.equal(api.paths[path].post['x-empty-body'],true);assert.equal(api.paths[path].post.responses[202].headers.Location['x-required'],true)}
})

test('DI-SC-25 frozen public component binds only explicit local single-source files',()=>{
 const files=['docs/contracts/ocr-public/openapi.json','docs/contracts/ocr-public/OCRPublic.schema.json','docs/contracts/ocr-public/profile.json','docs/contracts/ocr-draft/OCRResult.schema.json','docs/contracts/ocr-draft/OCRReview.schema.json','docs/contracts/ocr-draft/OCRReviewReceipt.schema.json','docs/contracts/schemas/ID.schema.json','docs/contracts/schemas/Error.schema.json'].sort()
 const lock=JSON.parse(readFileSync('docs/contracts/ocr-public/contract-lock.json'));assert.equal(lock.status,'frozen-component');assert.deepEqual(lock.files.map(x=>x.path),files)
 for(const file of lock.files)assert.equal(createHash('sha256').update(readFileSync(file.path)).digest('hex'),file.sha256,'Frozen OCR source changed without explicit contract update')
})
