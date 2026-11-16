import assert from 'node:assert/strict'
import { validateOCRResult } from './ocr-contract-semantics.mjs'

// DI-SC-17: Tesseract TSV coordinates refer to its input raster. No pre-rotation
// occurs here; a future raster/pre-rotation adapter must map its boxes explicitly.
const header = 'level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext'
const integer = value => { assert.match(value,/^(0|[1-9][0-9]*)$/); const n=Number(value);assert.ok(Number.isSafeInteger(n));return n }
export function parseOCRPage(tsv) {
  assert.equal(typeof tsv,'string');assert.ok(Buffer.byteLength(tsv)<=8388608)
  const lines=tsv.replaceAll('\r\n','\n').split('\n');assert.equal(lines.shift(),header)
  let width,height,previous=null;const words=[]
  for(const line of lines){
    if(!line)continue
    const f=line.split('\t');assert.equal(f.length,12,'Closed TSV row required')
    const [level,page,block,paragraph,row,word,x,y,w,h]=f.slice(0,10).map(integer)
    assert.ok(level>=1&&level<=5);assert.equal(page,1,'Each input must be a single raster page')
    if(level===1){assert.equal(width,undefined,'Duplicate page header');assert.equal(x,0);assert.equal(y,0);assert.ok(w>0&&h>0&&w*h<=20000000);width=w;height=h;continue}
    assert.ok(width!==undefined,'Page dimensions required before words')
    if(level!==5)continue
    assert.ok(block>0&&paragraph>0&&row>0&&word>0)
    assert.ok(w>0&&h>0&&x+w<=width&&y+h<=height,'Word box outside input raster')
    assert.match(f[10],/^(?:[0-9]+)(?:\.[0-9]+)?$/);const confidence=Number(f[10]);assert.ok(confidence<=100)
    assert.ok(!/[\p{Cc}\p{Cs}]/u.test(f[11]),'Control characters forbidden')
    const text=f[11].normalize('NFC').trim();if(!text)continue
    assert.ok([...text].length<=2000)
    const key=[block,paragraph,row,word];if(previous){let cmp=0;for(let i=0;i<4&&!cmp;i++)cmp=Math.sign(key[i]-previous[i]);assert.ok(cmp>0,'Duplicate or unordered word identifier')};previous=key
    words.push({text,bbox:[x/width,y/height,(x+w)/width,(y+h)/height],engine_confidence:confidence,line:[block,paragraph,row].join(':')});assert.ok(words.length<=20000)
  }
  assert.ok(width!==undefined,'Missing page dimensions');assert.ok(words.length>0,'Empty OCR page requires explicit failure/review policy')
  return {width,height,words}
}

export function ocrResultFromTSV(pages) {
  assert.ok(Array.isArray(pages)&&pages.length>0&&pages.length<=100)
  let body='',offset=0;const resultPages=[];const scores=[]
  for(const tsv of pages){
    const page=parseOCRPage(tsv),start=offset;let lastLine=null;const words=[]
    if(resultPages.length){body+='\n\n';offset+=2}
    const pageStart=offset
    for(const word of page.words){
      if(lastLine!==null){const separator=lastLine===word.line?' ':'\n';body+=separator;offset++}
      const char_start=offset;body+=word.text;offset+=[...word.text].length
      const {line,...data}=word;words.push({...data,char_start,char_end:offset});scores.push(word.engine_confidence);lastLine=line
    }
    assert.ok(offset>start);resultPages.push({page:resultPages.length+1,width:page.width,height:page.height,rotation_applied:0,source:'ocr',char_start:pageStart,char_end:offset,words})
    assert.ok(Buffer.byteLength(body)<=2097152)
  }
  const mean=scores.reduce((a,b)=>a+b,0)/scores.length,low=scores.filter(s=>s<50).length/scores.length
  const result={status:'needs_review',engine:'tesseract-5.5.4-fast-8741641',body,pages:resultPages,quality:mean<80||low>.2?'low_quality':'acceptable_candidate'}
  validateOCRResult(result);assert.ok(Buffer.byteLength(JSON.stringify(result))<=8388608);return result
}
