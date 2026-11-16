import assert from 'node:assert/strict'
export function validateOCRResult(value) {
  if (value.status === 'failed') return
  assert.equal(value.status,'needs_review')
  assert.ok(Buffer.byteLength(value.body,'utf8')<=2097152)
  const chars=[...value.body]; let last=0,count=0
  value.pages.forEach((p,i)=>{
    assert.equal(p.page,i+1);assert.ok(p.width*p.height<=20000000)
    assert.ok(p.char_start>=last && p.char_end>p.char_start && p.char_end<=chars.length);last=p.char_end
    let end=p.char_start
    for(const w of p.words){const [x1,y1,x2,y2]=w.bbox;assert.ok(x1<x2&&y1<y2);assert.ok(w.char_start>=end&&w.char_end>w.char_start&&w.char_end<=p.char_end);assert.equal(chars.slice(w.char_start,w.char_end).join(''),w.text);end=w.char_end;count++}
    if(p.source==='ocr')assert.ok(p.words.length>0,'OCR page must have source word anchors')
  })
  assert.equal(value.pages[0].char_start,0);assert.equal(last,chars.length);assert.ok(count<=100000)
}
export function validateReview(value, context) {
  assert.equal(context.role,'teacher');assert.equal(context.sameClass,true)
  assert.equal(value.base_revision,context.latestDraftRevision,'Stale draft must conflict')
  assert.equal(value.acknowledged,true);assert.equal(value.pages.length,context.pageCount)
  value.pages.forEach((p,i)=>assert.equal(p.page,i+1))
  assert.ok(Buffer.byteLength(value.pages.map(p=>p.text).join('\n\n'),'utf8')<=2097152)
}
