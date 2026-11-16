import assert from'node:assert/strict'
import{spawnSync}from'node:child_process'
import{readFileSync,writeFileSync,mkdirSync,readdirSync,unlinkSync,existsSync}from'node:fs'
import{resolve}from'node:path'
import{randomBytes,createHash}from'node:crypto'
const name='campus-ocr-revisions-'+randomBytes(6).toString('hex'),tag=name+':probe',dir=resolve('output/ocr-revisions');mkdirSync(dir,{recursive:true})
const password=randomBytes(32).toString('base64url'),envfile=dir+'/'+name+'.env',results=[]
const report={exit_code:1,scope:'Isolated candidate revision DDL and actual Go transactions; production Target6 unchanged',run_id:name}
const sha=p=>createHash('sha256').update(readFileSync(p)).digest('hex')
function docker(args,input,timeout=180000){return spawnSync('docker',args,{input,encoding:'utf8',timeout,maxBuffer:8<<20,env:{...process.env,MYSQL_PWD:password}})}
function run(label,args,input,timeout){const r=docker(args,input,timeout);results.push({label,exit_code:r.status});assert.equal(r.status,0,label+': '+(r.stderr??'').replaceAll(password,'<redacted>').slice(-1200));return r.stdout.trim()}
function sql(label,input,database='ocr_probe'){return run(label,['exec','-i','-e','MYSQL_PWD',name,'mysql','--protocol=TCP','-h127.0.0.1','-uroot','--batch','--raw','--skip-column-names','--default-character-set=utf8mb4',...(database?[database]:[])],input,30000)}
try{
 writeFileSync(envfile,'MYSQL_ROOT_PASSWORD='+password+'\n',{mode:0o600})
 run('build pinned MySQL probe',['build','-t',tag,'deploy/identity/mysql']);report.image_id=run('immutable MySQL image',['image','inspect','--format','{{.Id}}',tag])
 run('start owned isolated MySQL',['run','-d','--name',name,'--network','none','--memory','1g','--memory-swap','1g','--cpus','1','--mount','type=tmpfs,destination=/var/lib/mysql','--env-file',envfile,report.image_id])
 let ready=false;for(let i=0;i<120;i++){const r=docker(['exec','-e','MYSQL_PWD',name,'mysql','--protocol=TCP','-h127.0.0.1','-uroot','--batch','--skip-column-names','-e','SELECT 1'],undefined,2000);if(r.status===0&&r.stdout.trim()==='1'){ready=true;break}await new Promise(r=>setTimeout(r,500))}assert.ok(ready)
 report.mysql_version=sql('database version','SELECT VERSION()',null);assert.match(report.mysql_version,/^8\.4\.13/)
 sql('create isolated schema','CREATE DATABASE ocr_probe CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin',null)
 const old=readdirSync('backend/internal/migrations/sql').filter(n=>n.endsWith('.up.sql')).sort();assert.equal(old.length,6);for(const n of old)sql('baseline '+n,readFileSync('backend/internal/migrations/sql/'+n,'utf8'))
 sql('identity fixtures',"INSERT INTO users(id,username,password_hash,role) VALUES(1,'ocr_fixture_teacher','unusable','teacher');INSERT INTO classes(id,stable_key,name) VALUES(1,'ocr_1','班一'),(2,'ocr_2','班二');")
 const candidate='docs/contracts/ocr-draft/000007_ocr_revisions.up.sql';report.candidate_sql_sha256=sha(candidate);sql('candidate append-only revision DDL',readFileSync(candidate,'utf8'))
 const tc=JSON.parse(readFileSync('scripts/toolchain.json'));const child=docker(['run','--rm','--network','container:'+name,'--memory','512m','--cpus','1','-e','MYSQL_PWD','-e','GOTOOLCHAIN=local','--mount',`type=bind,source=${resolve('backend')},target=/src,readonly`,'-v','campus-claw-plus-go-cache:/root/.cache/go-build','-w','/src',tc.goImage,'go','test','-tags=ocrreview','-shuffle=on','-count=1','-timeout=60s','-v','./internal/ocrreview'],undefined,150000)
 const output=((child.stdout??'')+(child.stderr??'')).replaceAll(password,'<redacted>');writeFileSync(dir+'/mysql-tests.txt',output);assert.equal(child.status,0,'real revision transactions failed; see mysql-tests.txt')
 const groups=[...output.matchAll(/^--- PASS: (TestMySQL\w+) /gm)].map(x=>x[1]);assert.equal(groups.length,5);assert.ok(!output.includes('--- SKIP:'));report.groups=groups
 sql('remove owned candidate rows','DELETE FROM document_ocr_review_keys;DELETE FROM document_ocr_chunks;DELETE FROM document_ocr_heads;DELETE FROM document_ocr_revisions;')
 sql('candidate reverse DDL',readFileSync('docs/contracts/ocr-draft/000007_ocr_revisions.down.sql','utf8'));assert.equal(Number(sql('baseline tables preserved',"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='ocr_probe'")),19)
 report.source_files=Object.fromEntries(readdirSync('backend/internal/ocrreview').map(n=>['backend/internal/ocrreview/'+n,sha('backend/internal/ocrreview/'+n)]));report.exit_code=0
}catch(e){report.error=e.message.replaceAll(password,'<redacted>');console.error(report.error)}finally{
 report.cleanup=[];for(const[args,absent]of[[['rm','-f',name],'No such container'],[['image','rm',tag],'No such image']]){const r=docker(args);const ok=r.status===0||(r.stderr??'').includes(absent);report.cleanup.push({resource:args.at(-1),ok});if(!ok)report.exit_code=1}
 if(existsSync(envfile))unlinkSync(envfile);report.secret_file_removed=!existsSync(envfile);writeFileSync(dir+'/results.json',JSON.stringify({...report,results},null,2)+'\n')
}
process.exitCode=report.exit_code
