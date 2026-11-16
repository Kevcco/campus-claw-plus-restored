import test from 'node:test'
import assert from 'node:assert/strict'
import { createAuthClient, AuthError } from '../src/auth/client.ts'
import { parseDocument, parseDetail, parseSearch, checkCitation, validateUpload, documentRoute } from '../src/documents/contracts.ts'
const document=(extra={})=>({id:'9223372036854775807',title:'课程𠮷😀',format:'pdf',version:1,status:'ready',size_bytes:10,attempts:1,page_count:2,failure_reason:null,created_at:'2026-11-15T00:00:00.000Z',updated_at:'2026-11-15T00:00:00.100Z',...extra})
const detail=()=>({document:document(),body:'甲😀乙\n\nneedle%_𠮷',locations:[{kind:'pdf_page',unit:1,char_start:0,char_end:3},{kind:'pdf_page',unit:2,char_start:5,char_end:14}]})
const citation=()=>({document_id:document().id,chunk_id:'2',document_version:1,title:document().title,location:detail().locations[1],quote:'needle%_𠮷',url:'/api/v1/documents/'+document().id})
const principal={user_id:'1',class_id:'2',role:'teacher',session_id:'b'.repeat(32),session_version:1,expires_at:'2030-01-01T00:00:00.000Z'}
const json=(v,status=200,headers={})=>Response.json(v,{status,headers:{'X-Request-ID':'a'.repeat(32),'Cache-Control':'no-store',...headers}})
const denied=status=>json({error:{code:({401:'UNAUTHENTICATED',409:'CONFLICT',503:'DEPENDENCY_UNAVAILABLE'})[status],message:'private remote text',fields:[],details:null},request_id:'a'.repeat(32)},status)
function fixture(responses,role='teacher'){
 const calls=[],queue=[json({access_token:'synthetic-document-access',token_type:'Bearer',expires_in:900,principal:{...principal,role}}),json({csrf_token:'c'.repeat(32)}),...responses]
 const client=createAuthClient(async(path,init)=>{calls.push({path,...init});const next=queue.shift();assert.ok(next,'unexpected request');return typeof next==='function'?next(path,init):next})
 return{client,calls,login:()=>client.login('fixture','fixture')}
}
test('DI-SC-02/05 Document states, UTF codepoints, exact milliseconds and nullable fields are closed',()=>{
 assert.equal(parseDocument(document()).id,'9223372036854775807');checkCitation(parseDetail(detail(),document().id),citation())
 const word=detail();word.document.format='docx';word.document.page_count=null;word.locations[0].kind='docx_paragraph';word.locations[1].kind='docx_table_cell';assert.equal(parseDetail(word,document().id).document.page_count,null)
 for(const mutate of [d=>d.id=1,d=>d.attempts=null,d=>d.page_count=null,d=>d.created_at='2026-11-15T00:00:00Z',d=>d.updated_at='2026-11-15T00:00:00.1Z',d=>d.failure_reason='corrupt',d=>d.unexpected=true]){const d=document();mutate(d);assert.throws(()=>parseDocument(d))}
 for(const mutate of [d=>d.locations[0].char_end=4,d=>d.locations[1].char_start=4,d=>d.locations[1].unit=1,d=>d.document.page_count=1,d=>d.document.status='queued',d=>d.body='甲\r\n乙']){const d=detail();mutate(d);assert.throws(()=>parseDetail(d,document().id))}
 const c=citation();c.quote='needle%_X';assert.throws(()=>checkCitation(detail(),c));c.url='https://other.invalid/private';assert.throws(()=>parseSearch({mode:'keyword',items:[c]}))
})
test('DI-SC-02 binary selection validates extension/signature, size and title without decoding PDF as text',async()=>{
 const p={title:'\u0085课程 \u0085',file:new File(['%PDF-fixture'],'x.PDF')};assert.equal((await validateUpload(p)).title,'课程')
 await validateUpload({title:'word',file:new File([new Uint8Array([80,75,3,4,0])],'x.docx')})
 for(const [name,data]of [['x.doc','%PDF-'],['x.docm','PK\x03\x04'],['x.docx','%PDF-'],['x.pdf','fake'],['x.pdf','']])await assert.rejects(validateUpload({title:'a',file:new File([data],name)}))
 await assert.rejects(validateUpload({...p,title:'题'.repeat(201)}));await assert.rejects(validateUpload({...p,file:new File([new Uint8Array(10485761)],'x.pdf')}))
})
test('DI-SC-11/12 six fixed operations preserve bearer isolation, literal search and binary headers',async()=>{
 const headers={'Content-Type':'application/pdf','Content-Disposition':`attachment; filename="document-${document().id}.pdf"`,'X-Content-Type-Options':'nosniff','Cache-Control':'no-store'}
 const accepted=document({status:'queued',attempts:0,page_count:null,updated_at:document().created_at})
 const f=fixture([json({items:[document()],page:1,page_size:20,total:1}),json({mode:'keyword',items:[citation()]}),json(detail()),new Response('%PDF-file',{headers}),json(accepted,202),json(accepted,202)])
 await f.login();await f.client.documents.list();await f.client.documents.search(' \u0085needle%_𠮷 ');await f.client.documents.detail(document().id)
 const blob=await f.client.documents.download(document().id);assert.equal(await blob.text(),'%PDF-file')
 await f.client.documents.upload({title:'课程',file:new File(['%PDF-fixture'],'x.pdf')},'public_document_key_01');await f.client.documents.retry(document().id)
 assert.equal(f.calls[2].path,'/api/v1/documents?page=1&page_size=20');assert.equal(new URL(f.calls[3].path,'https://campus.invalid').searchParams.get('q'),'needle%_𠮷')
 for(const request of f.calls.slice(2)){assert.equal(request.headers.Authorization,'Bearer synthetic-document-access');assert.equal(request.redirect,'error');assert.equal(request.cache,'no-store')}
 assert.equal(f.client.access_token,undefined);assert.equal(f.calls[6].headers['Idempotency-Key'],'public_document_key_01')
})
test('DI-SC-07/11 writes never auto-replay, explicit same-key retry remains possible and binary failures are rejected',async()=>{
 const f=fixture([denied(503),json(document({status:'queued',attempts:0,page_count:null,updated_at:document().created_at}),202)]);await f.login()
 const p={title:'课程',file:new File(['%PDF-fixture'],'x.pdf')};await assert.rejects(f.client.documents.upload(p,'same_document_key_01'),AuthError);assert.equal(f.calls.length,3)
 await f.client.documents.upload(p,'same_document_key_01');assert.equal(f.calls[2].headers['Idempotency-Key'],f.calls[3].headers['Idempotency-Key'])
 const bad=fixture([new Response('%PDF-file',{headers:{'Content-Type':'text/html','Content-Disposition':'inline'}})]);await bad.login();await assert.rejects(bad.client.documents.download(document().id),AuthError)
})
test('DI-SC-11 invalid routes/parameters and student writes fail before business requests; context reset rejects late data',async()=>{
 for(const path of ['/app/materials/documents/0','/app/materials/documents/01','/app/materials/documents/9223372036854775808','/app/materials/documents/1/extra','/missing'])assert.equal(documentRoute(path),undefined)
 assert.deepEqual(documentRoute('/app/materials/documents/'+document().id),{kind:'detail',id:document().id})
 const student=fixture([],'student');await student.login();await assert.rejects(student.client.documents.upload({title:'课程',file:new File(['%PDF-'],'x.pdf')},'student_document_key'),AuthError);assert.equal(student.calls.length,2)
 const f=fixture([]);await f.login();for(const q of ['', '题'.repeat(2001),'a\x00','\ud800'])await assert.rejects(f.client.documents.search(q),AuthError);assert.throws(()=>f.client.documents.detail('https://other.invalid'));assert.equal(f.calls.length,2)
 let resolve;const late=fixture([()=>new Promise(r=>{resolve=r})]);await late.login();const pending=late.client.documents.detail(document().id);late.client.clear();resolve(json(detail()));await assert.rejects(pending,AuthError)
})
