import assert from 'node:assert/strict'
import {readFileSync,writeFileSync,mkdirSync,copyFileSync,readdirSync,existsSync,unlinkSync}from'node:fs'
import {resolve}from'node:path'
import {spawnSync}from'node:child_process'
import {randomBytes,createHash}from'node:crypto'
import {loadOCRPublic,revisionSemantics,statusSemantics,citationSemantics}from'./ocr-public-contract.mjs'

const id='campus-ocr-public-'+randomBytes(6).toString('hex'),dir=resolve('output/ocr-public'),tag=id+':probe'
mkdirSync(dir+'/fixtures',{recursive:true});mkdirSync(dir+'/evidence',{recursive:true})
const password=randomBytes(32).toString('base64url'),envfile=dir+'/'+id+'.env',toolchain=JSON.parse(readFileSync('scripts/toolchain.json')),report={run_id:id,exit_code:1,scope:'real identity service sessions, trusted HTTPS, MySQL7 and explicit OCR HTTP component; not production activation',results:[]}
const hash=p=>createHash('sha256').update(readFileSync(p)).digest('hex')
const docker=(args,input,timeout=180000)=>spawnSync('docker',args,{input,encoding:'utf8',timeout,maxBuffer:32<<20,env:{...process.env,MYSQL_PWD:password}})
function run(label,args,input,timeout){const r=docker(args,input,timeout);report.results.push({label,exit_code:r.status});assert.equal(r.status,0,label+': '+(r.stderr??'').replaceAll(password,'<redacted>').slice(-1200));return r.stdout}
try{
 copyFileSync('document-parser/testdata/ocr-v1/pdf/development-positive.pdf',dir+'/fixtures/development-positive.pdf');copyFileSync('reports/changes/04E/ocr-rpc/actual-source.json',dir+'/fixtures/actual-source.json')
 writeFileSync(envfile,'MYSQL_ROOT_PASSWORD='+password+'\n',{mode:0o600});run('build pinned MySQL',['build','-t',tag,'deploy/identity/mysql']);report.image_id=run('exact MySQL image',['image','inspect','--format','{{.Id}}',tag]).trim();run('start owned MySQL',['run','-d','--name',id,'--network','none','--memory','1g','--memory-swap','1g','--cpus','1','--mount','type=tmpfs,destination=/var/lib/mysql','--env-file',envfile,report.image_id])
 let ready=false;for(let i=0;i<120;i++){const r=docker(['exec','-e','MYSQL_PWD',id,'mysql','--protocol=TCP','-h127.0.0.1','-uroot','--batch','--skip-column-names','-e','SELECT 1'],undefined,2000);if(r.status===0&&r.stdout.trim()==='1'){ready=true;break}await new Promise(r=>setTimeout(r,500))}assert.ok(ready)
 report.mysql_version=run('actual MySQL version',['exec','-e','MYSQL_PWD',id,'mysql','--protocol=TCP','-h127.0.0.1','-uroot','--batch','--skip-column-names','-e','SELECT VERSION()']).trim();assert.match(report.mysql_version,/^8\.4\.13/)
 const probe=docker(['run','--rm','--network','container:'+id,'--memory','512m','--cpus','1','-e','MYSQL_PWD','-e','GOTOOLCHAIN=local','--mount',`type=bind,source=${resolve('backend')},target=/src,readonly`,'--mount',`type=bind,source=${dir+'/fixtures'},target=/fixtures,readonly`,'--mount',`type=bind,source=${dir+'/evidence'},target=/evidence`,'-v','campus-claw-plus-go-cache:/root/.cache/go-build','-w','/src',toolchain.goImage,'go','test','-tags=ocrpublic','-count=1','-timeout=120s','-v','./internal/integration/ocrpublic'],undefined,150000)
 const text=((probe.stdout??'')+(probe.stderr??'')).replaceAll(password,'<redacted>');writeFileSync(dir+'/tests.txt',text);assert.equal(probe.status,0,'Actual OCR HTTPS/MySQL tests failed; see tests.txt');const groups=[...text.matchAll(/^--- PASS: (TestOCRPublic\w+) /gm)].map(x=>x[1]);assert.equal(groups.length,4);assert.ok(!text.includes('--- SKIP:'));report.groups=groups
 const {response}=await loadOCRPublic();let checked=0;const revisions=new Map()
 const files=groups.map(g=>g+'.json');for(const file of files){const rows=JSON.parse(readFileSync(dir+'/evidence/'+file));for(const row of rows){const check=response(row.Path,row.Method.toLowerCase(),row.Code);assert.equal(check(row.Body),true,file+': '+JSON.stringify(check.errors));checked++;if(row.Code===200&&row.Path.includes('/ocr-revisions/')){revisionSemantics(row.Body);revisions.set(row.Body.document_id+':'+row.Body.revision,row.Body)}if(row.Code===200&&row.Path.endsWith('/ocr'))statusSemantics(row.Body);if(row.Code===200&&row.Path==='/api/v1/ocr-search')for(const c of row.Body.items)citationSemantics(c)}}
 report.http_schema_responses=checked;report.sources=['backend/internal/httpserver/ocr.go','backend/internal/httpserver/request.go','backend/internal/httpserver/documents.go','backend/internal/httpserver/materials.go','backend/internal/httpserver/ocr_test.go','backend/internal/ocrreview/status.go','backend/internal/ocrreview/store.go','backend/internal/integration/ocrpublic/http_test.go','docs/contracts/ocr-public/openapi.json','docs/contracts/ocr-public/OCRPublic.schema.json','docs/contracts/ocr-public/profile.json','docs/contracts/ocr-public/contract-lock.json','scripts/check-ocr-public.mjs'].map(path=>({path,sha256:hash(path)}));report.evidence=files.map(path=>({path,sha256:hash(dir+'/evidence/'+path)}));report.exit_code=0
}catch(e){report.error=e.message.replaceAll(password,'<redacted>');console.error(report.error)}finally{
 report.cleanup=[];for(const[args,absent]of[[['rm','-f',id],'No such container'],[['image','rm',tag],'No such image']]){const r=docker(args);const ok=r.status===0||(r.stderr??'').includes(absent);report.cleanup.push({resource:args.at(-1),ok});if(!ok)report.exit_code=1}
 if(existsSync(envfile))unlinkSync(envfile);report.secret_file_removed=!existsSync(envfile);writeFileSync(dir+'/results.json',JSON.stringify(report,null,2)+'\n')
}
process.exitCode=report.exit_code
