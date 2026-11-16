import test from 'node:test'
import assert from 'node:assert/strict'
import {parseOCRPage,ocrResultFromTSV} from './ocr-tsv.mjs'
const header='level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext'
const rows=['1\t1\t0\t0\t0\t0\t0\t0\t1000\t500\t-1\t','5\t1\t1\t1\t1\t1\t100\t50\t100\t50\t95.5\t课程😀','5\t1\t1\t1\t1\t2\t250\t50\t100\t50\t98\té','5\t1\t1\t1\t2\t1\t100\t150\t100\t50\t90\t下一行']
const sample=()=>[header,...rows].join('\n')+'\n'
test('DI-SC-17 actual TSV shape yields original boxes and Unicode codepoint quotes across pages',()=>{
 const r=ocrResultFromTSV([sample(),sample()]);assert.equal(r.status,'needs_review');assert.equal(r.body,'课程😀 é\n下一行\n\n课程😀 é\n下一行');assert.equal(r.pages[0].words[0].char_end,3);assert.deepEqual(r.pages[0].words[0].bbox,[.1,.1,.2,.2]);assert.equal(r.pages[1].char_start,11)
 for(const p of r.pages)for(const w of p.words)assert.equal([...r.body].slice(w.char_start,w.char_end).join(''),w.text)
})
test('DI-SC-17 malformed, forged boxes, nonfinite confidence, unordered and empty TSV fail closed',()=>{
 for(const bad of [sample().replace(header,'bad'),sample().replace('100\t50\t100\t50\t95.5','990\t50\t100\t50\t95.5'),sample().replace('95.5','NaN'),sample().replace('95.5','101'),sample().replace('课程😀','bad\u0000'),[header,rows[0]].join('\n'),[header,rows[0],rows[1],rows[1]].join('\n'),sample().replace('1000\t500','20000000\t500'),sample().replace('5\t1\t1','5\t2\t1')])assert.throws(()=>parseOCRPage(bad))
 assert.throws(()=>ocrResultFromTSV([]));assert.throws(()=>ocrResultFromTSV(Array(101).fill(sample())))
})
test('DI-SC-18 low raw confidence remains a review result and never publishes ready',()=>{
 const r=ocrResultFromTSV([sample().replace('95.5','10')]);assert.equal(r.quality,'low_quality');assert.equal(r.status,'needs_review')
})
test('DI-SC-17 real engine whitespace word rows are layout rather than fabricated text',()=>{
 const space='5\t1\t1\t1\t3\t1\t100\t250\t100\t50\t95\t   '
 assert.equal(ocrResultFromTSV([sample()+space+'\n']).body,ocrResultFromTSV([sample()]).body)
})
