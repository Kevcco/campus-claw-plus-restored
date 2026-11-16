import assert from 'node:assert/strict'
import{readFileSync}from'node:fs'
import{resolve}from'node:path'
import{spawnSync}from'node:child_process'
const toolchain=JSON.parse(readFileSync('scripts/toolchain.json'))
const child=spawnSync('docker',['run','--rm','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--memory','128m','--cpus','1','--pids-limit','16','--tmpfs','/tmp:rw,noexec,nosuid,size=64m','--mount',`type=bind,source=${resolve('document-parser')},target=/tests,readonly`,'--workdir','/tests','--entrypoint','python',toolchain.documentPythonImage,'-B','-m','unittest','test_ocr_pdf.py','test_ocr_result.py','test_ocr_server.py'],{encoding:'utf8',timeout:30000})
process.stdout.write(child.stdout??'');process.stderr.write(child.stderr??'');assert.equal(child.status,0,'OCR raster bound tests failed')
