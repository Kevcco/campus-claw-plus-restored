import assert from'node:assert/strict'
import{spawnSync}from'node:child_process'
import{readFileSync,writeFileSync,mkdirSync,readdirSync,unlinkSync,existsSync,copyFileSync}from'node:fs'
import{resolve}from'node:path'
import{randomBytes,createHash}from'node:crypto'
import{ocrResultFromTSV}from'./ocr-tsv.mjs'
import{microCER}from'./ocr-corpus.mjs'
import Ajv2020 from'ajv/dist/2020.js'
const rpcImage=process.argv[2]
const name='campus-ocr-revisions-'+randomBytes(6).toString('hex'),tag=name+':probe',dir=resolve('output',rpcImage?'ocr-rpc-pipeline':'ocr-revisions');mkdirSync(dir,{recursive:true})
const password=randomBytes(32).toString('base64url'),envfile=dir+'/'+name+'.env',results=[]
const network=name+'-net',ocrContainer=name+'-ocr',ocrEnv=dir+'/'+name+'-ocr.env',ocrToken=randomBytes(32).toString('base64url')
assert.ok(process.argv.length<=3);if(rpcImage)assert.match(rpcImage,/^sha256:[a-f0-9]{64}$/)
const report={exit_code:1,scope:'Isolated candidate revision DDL and actual Go transactions; production Target6 unchanged',run_id:name}
const sha=p=>createHash('sha256').update(readFileSync(p)).digest('hex')
function docker(args,input,timeout=180000){return spawnSync('docker',args,{input,encoding:'utf8',timeout,maxBuffer:8<<20,env:{...process.env,MYSQL_PWD:password,DOCUMENT_PARSER_TOKEN:ocrToken}})}
function run(label,args,input,timeout){const r=docker(args,input,timeout);results.push({label,exit_code:r.status});assert.equal(r.status,0,label+': '+(r.stderr??'').replaceAll(password,'<redacted>').slice(-1200));return r.stdout.trim()}
function sql(label,input,database='ocr_probe'){return run(label,['exec','-i','-e','MYSQL_PWD',name,'mysql','--protocol=TCP','-h127.0.0.1','-uroot','--batch','--raw','--skip-column-names','--default-character-set=utf8mb4',...(database?[database]:[])],input,30000)}
try{
 const fixtures=dir+'/fixtures';mkdirSync(fixtures,{recursive:true})
 const actual=JSON.parse(readFileSync('reports/changes/04E/ocr-pdf-development/positive-raw.json'))
 const source=ocrResultFromTSV(actual.pages.map(p=>p.tsv));writeFileSync(fixtures+'/real-source.json',JSON.stringify(source))
 copyFileSync('document-parser/testdata/ocr-v1/pdf/development-positive.pdf',fixtures+'/development-positive.pdf')
 report.real_ocr_fixture={engine_output_sha256:sha('reports/changes/04E/ocr-pdf-development/positive-raw.json'),source_sha256:sha(fixtures+'/real-source.json'),original_sha256:sha(fixtures+'/development-positive.pdf'),pages:source.pages.length,words:source.pages.reduce((n,p)=>n+p.words.length,0)}
 writeFileSync(envfile,'MYSQL_ROOT_PASSWORD='+password+'\n',{mode:0o600})
 run('build pinned MySQL probe',['build','-t',tag,'deploy/identity/mysql']);report.image_id=run('immutable MySQL image',['image','inspect','--format','{{.Id}}',tag])
 if(rpcImage)run('create exclusive internal OCR pipeline network',['network','create','--internal',network])
 run('start owned isolated MySQL',['run','-d','--name',name,'--network',rpcImage?network:'none',...(rpcImage?['--network-alias','mysql']:[]),'--memory','1g','--memory-swap','1g','--cpus','1','--mount','type=tmpfs,destination=/var/lib/mysql','--env-file',envfile,report.image_id])
 let ready=false;for(let i=0;i<120;i++){const r=docker(['exec','-e','MYSQL_PWD',name,'mysql','--protocol=TCP','-h127.0.0.1','-uroot','--batch','--skip-column-names','-e','SELECT 1'],undefined,2000);if(r.status===0&&r.stdout.trim()==='1'){ready=true;break}await new Promise(r=>setTimeout(r,500))}assert.ok(ready)
 report.mysql_version=sql('database version','SELECT VERSION()',null);assert.match(report.mysql_version,/^8\.4\.13/)
 sql('create isolated schema','CREATE DATABASE ocr_probe CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin',null)
 const old=readdirSync('backend/internal/migrations/sql').filter(n=>n.endsWith('.up.sql')).sort();assert.equal(old.length,6);for(const n of old)sql('baseline '+n,readFileSync('backend/internal/migrations/sql/'+n,'utf8'))
 sql('identity fixtures',"INSERT INTO users(id,username,password_hash,role) VALUES(1,'ocr_fixture_teacher','unusable','teacher');INSERT INTO classes(id,stable_key,name) VALUES(1,'ocr_1','班一'),(2,'ocr_2','班二');")
 const candidate='docs/contracts/ocr-draft/000007_ocr_revisions.up.sql';report.candidate_sql_sha256=sha(candidate);sql('candidate append-only revision DDL',readFileSync(candidate,'utf8'))
 const tc=JSON.parse(readFileSync('scripts/toolchain.json'));const child=docker(['run','--rm','--network','container:'+name,'--memory','512m','--cpus','1','-e','MYSQL_PWD','-e','GOTOOLCHAIN=local','--mount',`type=bind,source=${resolve('backend')},target=/src,readonly`,'--mount',`type=bind,source=${fixtures},target=/fixtures,readonly`,'-v','campus-claw-plus-go-cache:/root/.cache/go-build','-w','/src',tc.goImage,'go','test','-tags=ocrreview','-shuffle=on','-count=1','-timeout=60s','-v','./internal/ocrreview'],undefined,150000)
 const output=((child.stdout??'')+(child.stderr??'')).replaceAll(password,'<redacted>');writeFileSync(dir+'/mysql-tests.txt',output);assert.equal(child.status,0,'real revision transactions failed; see mysql-tests.txt')
 const groups=[...output.matchAll(/^--- PASS: (TestMySQL\w+) /gm)].map(x=>x[1]);assert.equal(groups.length,11);assert.ok(!output.includes('--- SKIP:'));report.groups=groups
 if(rpcImage){
  const rpcEvidence=dir+'/rpc-evidence';mkdirSync(rpcEvidence,{recursive:true})
  const engine=dir+'/engine',engineFiles=['server.py','runner.py','ocr_server.py','ocr_process.py','ocr_pdf.py','ocr_result.py'].sort();mkdirSync(engine,{recursive:true})
  assert.ok(readdirSync(engine).every(n=>engineFiles.includes(n)),'Unexpected data in OCR code-only mount')
  for(const file of engineFiles){copyFileSync('document-parser/'+file,engine+'/'+file);assert.equal(sha(engine+'/'+file),sha('document-parser/'+file))}
  writeFileSync(ocrEnv,'DOCUMENT_PARSER_TOKEN='+ocrToken+'\n',{mode:0o600})
  run('start immutable real OCR service',['run','-d','--name',ocrContainer,'--network',network,'--network-alias','document-ocr','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--memory','512m','--memory-swap','512m','--cpus','1','--pids-limit','32','--tmpfs','/tmp:rw,noexec,nosuid,size=64m','--env-file',ocrEnv,'--mount',`type=bind,source=${engine},target=/engine,readonly`,'--entrypoint','python',rpcImage,'-B','/engine/ocr_server.py'])
  run('actual OCR filesystem excludes corpus/gold/blind/fixtures',['exec',ocrContainer,'python','-c',`from pathlib import Path; assert sorted(p.name for p in Path('/engine').iterdir())==${JSON.stringify(engineFiles)}; assert not Path('/engine/testdata').exists(); assert not Path('/fixtures').exists(); assert not Path('/inputs').exists(); print('code-only; no corpus, gold, blind or fixture mount')`])
  let healthy=false;for(let i=0;i<40;i++){const r=docker(['exec',ocrContainer,'python','-c',"import os,urllib.request; r=urllib.request.Request('http://127.0.0.1:8092/health',headers={'Authorization':'Bearer '+os.environ['DOCUMENT_PARSER_TOKEN']}); print(urllib.request.urlopen(r,timeout=1).status)"],undefined,3000);if(r.status===0&&r.stdout.trim()==='200'){healthy=true;break}await new Promise(r=>setTimeout(r,250))}assert.ok(healthy,'real OCR server did not become healthy')
  const started=Date.now();const pipeline=docker(['run','--rm','--network',network,'--memory','512m','--cpus','1','-e','MYSQL_PWD','-e','DOCUMENT_PARSER_TOKEN','-e','GOTOOLCHAIN=local','--mount',`type=bind,source=${resolve('backend')},target=/src,readonly`,'--mount',`type=bind,source=${fixtures},target=/fixtures,readonly`,'--mount',`type=bind,source=${rpcEvidence},target=/evidence`,'-v','campus-claw-plus-go-cache:/root/.cache/go-build','-w','/src',tc.goImage,'go','test','-tags=ocrpipeline','-count=1','-timeout=90s','-v','./internal/integration/ocrpipeline'],undefined,150000)
  const text=((pipeline.stdout??'')+(pipeline.stderr??'')).replaceAll(password,'<redacted>').replaceAll(ocrToken,'<redacted>');writeFileSync(dir+'/pipeline-tests.txt',text);assert.equal(pipeline.status,0,'real OCR/SQL pipeline failed; see pipeline-tests.txt');assert.ok(text.includes('--- PASS: TestRealPDFOCRWorkerAndReviewedSearch'))
  const logs=run('collect actual safe OCR logs',['logs',ocrContainer]);writeFileSync(dir+'/ocr-server-logs.txt',logs+'\n');assert.ok(!logs.includes(password)&&!logs.includes(ocrToken));for(const line of logs.split('\n').filter(Boolean)){const entry=JSON.parse(line);assert.deepEqual(Object.keys(entry).sort(),['duration_ms','path','request_id','status']);assert.match(entry.request_id,/^[a-f0-9]{32}$/);assert.ok(['/health','/internal/ocr','unknown'].includes(entry.path))}
  const sourceResult=JSON.parse(readFileSync(rpcEvidence+'/source.json'));const schema=new Ajv2020({strict:true}).compile(JSON.parse(readFileSync('docs/contracts/ocr-draft/OCRResult.schema.json')));assert.equal(schema(sourceResult),true,JSON.stringify(schema.errors));const gold=JSON.parse(readFileSync('document-parser/testdata/ocr-v1/manifest.json')).pages.filter(p=>p.split==='development-positive');assert.equal(sourceResult.pages.length,gold.length)
  const chars=[...sourceResult.body];const quality=microCER(sourceResult.pages.map((p,i)=>({gold:gold[i].gold,actual:chars.slice(p.char_start,p.char_end).join('')})))
  report.real_rpc={exit_code:0,image_id:rpcImage,duration_ms:Date.now()-started,engine_mount_files:engineFiles,gold_available:false,blind_available:false,response_schema:true,response_sha256:sha(rpcEvidence+'/source.json'),response_development_cer:quality,source_files:Object.fromEntries(['document-parser/ocr_server.py','document-parser/ocr_process.py','document-parser/ocr_pdf.py','document-parser/ocr_result.py','backend/internal/integration/ocrpipeline/pipeline_test.go',...readdirSync('backend/internal/privateocr').map(n=>'backend/internal/privateocr/'+n)].map(p=>[p,sha(p)])),scope:'new real PDF -> private HTTP -> Tesseract -> Go worker -> MySQL source -> teacher review -> student keyword search; no production activation'}
 }
 sql('remove owned candidate rows','DELETE FROM document_ocr_jobs;DELETE FROM document_ocr_sources;DELETE FROM document_ocr_review_keys;DELETE FROM document_ocr_chunks;DELETE FROM document_ocr_heads;DELETE FROM document_ocr_revisions;')
 sql('candidate reverse DDL',readFileSync('docs/contracts/ocr-draft/000007_ocr_revisions.down.sql','utf8'));assert.equal(Number(sql('baseline tables preserved',"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='ocr_probe'")),19)
 report.source_files=Object.fromEntries(readdirSync('backend/internal/ocrreview').map(n=>['backend/internal/ocrreview/'+n,sha('backend/internal/ocrreview/'+n)]));report.exit_code=0
}catch(e){report.error=e.message.replaceAll(password,'<redacted>');console.error(report.error)}finally{
 report.cleanup=[];for(const[args,absent]of[...(rpcImage?[[['rm','-f',ocrContainer],'No such container']]:[]),[['rm','-f',name],'No such container'],[['image','rm',tag],'No such image'],...(rpcImage?[[['network','rm',network],'not found']]:[])]){const r=docker(args);const ok=r.status===0||(r.stderr??'').includes(absent);report.cleanup.push({resource:args.at(-1),ok});if(!ok)report.exit_code=1}
 if(existsSync(envfile))unlinkSync(envfile);if(existsSync(ocrEnv))unlinkSync(ocrEnv);report.secret_file_removed=!existsSync(envfile)&&!existsSync(ocrEnv);writeFileSync(dir+'/results.json',JSON.stringify({...report,results},null,2)+'\n')
}
process.exitCode=report.exit_code
