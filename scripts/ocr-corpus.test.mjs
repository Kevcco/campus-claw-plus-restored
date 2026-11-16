import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { normalizeText, editDistance, microCER, boxIoU, validateCorpus } from './ocr-corpus.mjs'

const root = fileURLToPath(new URL('../document-parser/testdata/ocr-v1/', import.meta.url))
const load = path => readFileSync(resolve(root, path))
const original = JSON.parse(load('manifest.json')), lock = JSON.parse(load('corpus-lock.json'))

test('DI-SC-16/17 fixed 50 blind, 10 negatives, independent development and 100 boxes', () => {
  assert.deepEqual(validateCorpus(original, lock, load), { pages: 64, blind: 50, negative: 10, development_positive: 4, boxes: 100, pdfs: 3, quality_measured: false })
})

test('DI-SC-16 sample loss, split leakage, digest changes and relaxed limits are rejected', () => {
  for (const mutate of [
    m => m.pages.pop(), m => { m.pages[0].split = 'development-positive' },
    m => { m.pages[1].gold = m.pages[0].gold }, m => { m.pages[0].sha256 = '0'.repeat(64) },
    m => { m.pages[0].file = '../outside.png' }, m => { m.thresholds.clear_micro_cer = .06 },
    m => { m.pages[50].expected_class = 'recognizable' }, m => { m.pdfs[0].pages = 49 },
    m => { m.pages[40].clockwise_rotation = 0 }, m => { m.pages[0].width++ }
  ]) {
    const m = structuredClone(original); mutate(m); assert.throws(() => validateCorpus(m, lock, load))
  }
  assert.throws(() => validateCorpus(original, { ...lock, quality_measured: true }, load))
  assert.throws(() => validateCorpus(original, lock, path => { const b = Buffer.from(load(path)); if (path === original.pages[0].file) b[100] ^= 1; return b }))
})

test('DI-SC-17 wrong coordinate frame, area, quote and PDF page binding cannot pass', () => {
  for (const mutate of [
    m => { m.coordinate_frame = 'corrected OCR canvas' },
    m => { m.pages[0].boxes[0].bbox = [0, 0, 0, 1] },
    m => { m.pages[0].boxes[0].bbox = [0, 0, 2, 1] },
    m => { m.pages[0].boxes[0].char_start++ },
    m => { m.pages[0].boxes[0].text = 'invented quote' },
    m => { m.pages[0].pdf_page = 2 }
  ]) { const m = structuredClone(original); mutate(m); assert.throws(() => validateCorpus(m, lock, load)) }
})

test('DI-SC-16 scoring retains case/punctuation and uses codepoints and micro denominators', () => {
  assert.equal(normalizeText('  e\u0301\tA\u00a0中\n'), 'é A 中')
  assert.equal(editDistance('甲乙丙', '甲丁丙'), 1)
  assert.equal(editDistance('abc', 'ac'), 1)
  assert.equal(editDistance('abc', 'abxc'), 1)
  assert.equal(editDistance('A!', 'a'), 2)
  assert.equal(editDistance('𠮷a', '𠮷b'), 1)
  assert.equal(editDistance('abc', ''), 3)
  assert.deepEqual(microCER([{ gold: 'abc', actual: 'axc' }, { gold: '1234567', actual: '1234567' }]), { errors: 1, denominator: 10, cer: .1, pages: 2 })
  assert.throws(() => microCER([])); assert.throws(() => microCER([{ gold: ' \n', actual: 'x' }]))
})

test('DI-SC-17 IoU handles overlap boundaries and rejects malformed boxes', () => {
  assert.equal(boxIoU([0, 0, .5, .5], [0, 0, .5, .5]), 1)
  assert.equal(boxIoU([0, 0, .5, .5], [.5, .5, 1, 1]), 0)
  assert.equal(boxIoU([0, 0, 1, 1], [0, 0, .5, 1]), .5)
  assert.throws(() => boxIoU([0, 0, 0, 1], [0, 0, 1, 1]))
  assert.throws(() => boxIoU([0, NaN, 1, 1], [0, 0, 1, 1]))
})
