import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import assert from 'node:assert/strict'
import { validateCorpus, sha256 } from './ocr-corpus.mjs'

const root = resolve(process.argv[2] ?? 'document-parser/testdata/ocr-v1')
const load = path => readFileSync(resolve(root, path))
const manifest = JSON.parse(load('manifest.json'))
assert.equal(sha256(readFileSync(new URL('./generate-ocr-fixtures.py', import.meta.url))), manifest.authoring.generator_sha256, 'Frozen generator must match its recorded source')
const result = validateCorpus(manifest, JSON.parse(load('corpus-lock.json')), load)
console.log('DI-SC-16/17 frozen inputs: '+JSON.stringify(result))
