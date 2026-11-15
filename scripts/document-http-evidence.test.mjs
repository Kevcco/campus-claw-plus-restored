import test from 'node:test'
import assert from 'node:assert/strict'
import {validateDocumentHTTP} from './document-http-evidence.mjs'
const fixture=()=>({results:['DI public HTTPS main','DI stop actual parser only','DI public HTTPS outage','DI restore actual private parser','recreate isolated HTTPS path','DI public HTTPS persist'].map(label=>({label,exit_code:0,output:'--- PASS: TestDocumentHTTPProduction (1.00s)\nDI public HTTP stage: '+label.split(' ').at(-1)+' PASS\n2 real formats; 5 actors; exact originals and persisted codepoint citations; replay and authorization PASS'}))})
test('DI-SC-11..14 public HTTP needs all real ordered stages and production test result',()=>{assert.equal(validateDocumentHTTP(fixture()).real_formats,2)
 for(const change of [r=>r.results.splice(0,1),r=>r.results.reverse(),r=>r.results.push(r.results[0]),r=>r.results[0].exit_code=1,r=>r.results[0].output+='\n--- SKIP: TestDocumentHTTPProduction (1.00s)',r=>r.results[0].output='PASS']){const r=fixture();change(r);assert.throws(()=>validateDocumentHTTP(r))}
})
