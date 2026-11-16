// DI-SC-16/17: frozen evaluation inputs and independent scoring primitives.
// This module never invokes an OCR engine or manufactures recognition results.
import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'

export const thresholds = Object.freeze({ clear_micro_cer: .05, rotated_micro_cer: .10,
  bbox_iou: .5, bbox_match_rate: .95, page_accuracy: 1, negative_review_minimum: 9 })
export const sha256 = bytes => createHash('sha256').update(bytes).digest('hex')
export const normalizeText = text => text.normalize('NFC').replace(/\p{White_Space}+/gu, ' ').trim()

export function editDistance(left, right) {
  const a = [...normalizeText(left)], b = [...normalizeText(right)]
  let previous = Array.from({ length: b.length + 1 }, (_, i) => i)
  for (let i = 0; i < a.length; i++) {
    const current = [i + 1]
    for (let j = 0; j < b.length; j++) current.push(Math.min(
      current[j] + 1, previous[j + 1] + 1, previous[j] + (a[i] === b[j] ? 0 : 1)))
    previous = current
  }
  return previous[b.length]
}

function validBox(box) {
  assert.ok(Array.isArray(box) && box.length === 4 && box.every(n => Number.isFinite(n) && n >= 0 && n <= 1), 'Normalized finite bbox required')
  assert.ok(box[0] < box[2] && box[1] < box[3], 'Positive bbox area required')
}

export function boxIoU(a, b) {
  validBox(a); validBox(b)
  const overlap = Math.max(0, Math.min(a[2], b[2]) - Math.max(a[0], b[0])) *
    Math.max(0, Math.min(a[3], b[3]) - Math.max(a[1], b[1]))
  return overlap / ((a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - overlap)
}

export function microCER(pairs) {
  assert.ok(Array.isArray(pairs) && pairs.length > 0, 'Nonempty evaluation group required')
  let errors = 0, denominator = 0
  for (const { gold, actual } of pairs) {
    assert.equal(typeof gold, 'string'); assert.equal(typeof actual, 'string')
    const count = [...normalizeText(gold)].length
    assert.ok(count > 0, 'Gold denominator must not be empty')
    denominator += count; errors += editDistance(gold, actual)
  }
  return { errors, denominator, cer: errors / denominator, pages: pairs.length }
}

export function imageSize(bytes) {
  if (bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))) {
    assert.equal(bytes.toString('ascii', 12, 16), 'IHDR')
    return [bytes.readUInt32BE(16), bytes.readUInt32BE(20)]
  }
  assert.ok(bytes[0] === 255 && bytes[1] === 216, 'PNG or JPEG signature required')
  for (let p = 2; p + 4 < bytes.length;) {
    assert.equal(bytes[p++], 255, 'Malformed JPEG marker')
    while (bytes[p] === 255) p++
    const marker = bytes[p++]
    if (marker === 217 || marker === 218) break
    const length = bytes.readUInt16BE(p)
    assert.ok(length >= 2 && p + length <= bytes.length, 'Bounded JPEG segment required')
    if ([192, 193, 194].includes(marker)) return [bytes.readUInt16BE(p + 5), bytes.readUInt16BE(p + 3)]
    p += length
  }
  throw new Error('JPEG dimensions missing')
}

function safePath(path) {
  assert.ok(typeof path === 'string' && /^[a-zA-Z0-9_./-]+$/.test(path) &&
    !path.startsWith('/') && path.split('/').every(p => p && p !== '.' && p !== '..'), 'Corpus-relative path required')
}

export function validateCorpus(manifest, lock, load) {
  assert.equal(manifest.version, 1); assert.equal(lock.version, 1)
  assert.equal(lock.quality_measured, false, 'Input freeze is not an OCR quality result')
  assert.deepEqual(manifest.thresholds, thresholds, 'Approved OCR thresholds must not drift')
  assert.equal(manifest.coordinate_frame, 'original input raster before OCR orientation correction; top-left normalized xyxy')
  assert.equal(manifest.pages.length, 64)
  const seen = new Set(), texts = new Set(), imageHashes = new Set(), boxes = new Set()
  const groups = {}, splits = {}, angles = {}, pdfPages = new Map()
  for (const p of manifest.pages) {
    assert.match(p.id, /^(zh|en|rot|negative|dev)-\d{2}$/)
    assert.ok(!seen.has(p.id), 'Duplicate page ID'); seen.add(p.id)
    assert.ok(['zh', 'en'].includes(p.language))
    assert.ok(['blind', 'development-negative', 'development-positive'].includes(p.split))
    assert.equal(p.split, ['clear_zh', 'clear_en', 'rotated'].includes(p.group) ? 'blind' : p.group)
    if (p.group === 'clear_zh') assert.equal(p.language, 'zh')
    if (p.group === 'clear_en') assert.equal(p.language, 'en')
    if (p.group.startsWith('clear_')) assert.equal(p.clockwise_rotation, 0)
    assert.ok([0, 90, 180, 270].includes(p.clockwise_rotation))
    if (p.group === 'rotated') { assert.notEqual(p.clockwise_rotation, 0); angles[p.clockwise_rotation] = (angles[p.clockwise_rotation] ?? 0) + 1 }
    groups[p.group] = (groups[p.group] ?? 0) + 1; splits[p.split] = (splits[p.split] ?? 0) + 1
    safePath(p.file); assert.ok(p.file.startsWith(p.split+'/'))
    const bytes = load(p.file)
    assert.match(p.sha256, /^[a-f0-9]{64}$/); assert.equal(sha256(bytes), p.sha256, 'Raster bytes must match frozen SHA')
    assert.equal(bytes.length, p.bytes); assert.ok(p.bytes > 0 && p.bytes <= 10 * 1024 * 1024)
    assert.ok(!imageHashes.has(p.sha256), 'Splits must not reuse input bytes'); imageHashes.add(p.sha256)
    assert.deepEqual(imageSize(bytes), [p.width, p.height])
    assert.ok(p.width * p.height <= 20_000_000)
    assert.equal(p.dpi, 200)
    assert.equal(typeof p.gold, 'string'); const text = normalizeText(p.gold)
    assert.ok([...text].length >= 100 && !texts.has(text), 'Nonempty disjoint gold text required'); texts.add(text)
    assert.equal(p.boxes.length, p.split === 'blind' ? 2 : 0)
    for (const b of p.boxes) {
      assert.ok(!boxes.has(b.id)); boxes.add(b.id); validBox(b.bbox)
      assert.ok(Number.isSafeInteger(b.char_start) && Number.isSafeInteger(b.char_end) && b.char_start >= 0 && b.char_start < b.char_end && b.char_end <= [...p.gold].length)
      assert.equal([...p.gold].slice(b.char_start, b.char_end).join(''), b.text, 'Gold bbox span must quote its exact source')
      assert.ok(normalizeText(b.text).length > 0)
    }
    if (p.split === 'development-negative') {
      assert.equal(p.expected_class, 'needs_review_or_failed'); assert.ok(['blur', 'occlusion'].includes(p.degradation?.kind))
    } else { assert.equal(p.expected_class, 'recognizable'); assert.equal(p.degradation, null) }
    safePath(p.pdf_file); assert.ok(Number.isSafeInteger(p.pdf_page) && p.pdf_page > 0)
    const key = p.pdf_file+':'+p.pdf_page; assert.ok(!pdfPages.has(key)); pdfPages.set(key, p)
    assert.equal(lock.sha256[p.file], p.sha256)
  }
  assert.deepEqual(groups, { clear_zh: 20, clear_en: 20, rotated: 10, 'development-negative': 10, 'development-positive': 4 })
  assert.deepEqual(splits, { blind: 50, 'development-negative': 10, 'development-positive': 4 })
  assert.deepEqual(angles, { 90: 4, 180: 3, 270: 3 }); assert.equal(boxes.size, 100)
  assert.equal(manifest.pdfs.length, 3)
  const pdfSplits = new Set()
  for (const pdf of manifest.pdfs) {
    safePath(pdf.file); assert.ok(!pdfSplits.has(pdf.split)); pdfSplits.add(pdf.split)
    assert.equal(pdf.pages, splits[pdf.split]); assert.equal(pdf.text_layer, false)
    const bytes = load(pdf.file)
    assert.equal(bytes.subarray(0, 5).toString(), '%PDF-'); assert.equal(sha256(bytes), pdf.sha256)
    assert.equal(bytes.length, pdf.bytes); assert.ok(pdf.bytes <= 10 * 1024 * 1024)
    assert.equal((bytes.toString('latin1').match(/\/Type\s*\/Page\b/g) ?? []).length, pdf.pages)
    for (let n = 1; n <= pdf.pages; n++) assert.equal(pdfPages.get(pdf.file+':'+n)?.split, pdf.split)
    assert.equal(lock.sha256[pdf.file], pdf.sha256)
  }
  for (const [path, hash] of Object.entries(lock.sha256)) { safePath(path); assert.equal(sha256(load(path)), hash, 'Frozen file drift: '+path) }
  return { pages: 64, blind: 50, negative: 10, development_positive: 4, boxes: 100, pdfs: 3, quality_measured: false }
}
