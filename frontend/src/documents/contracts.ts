import { bounded, shape, timestamp, validId } from '../shared/values.ts'
import { trimSpace, validateTitle, UploadValidationError } from '../materials/contracts.ts'
export { trimSpace, validateTitle, UploadValidationError }
export type Document = { id: string; title: string; format: 'pdf' | 'docx'; version: 1; status: 'queued' | 'processing' | 'ready' | 'needs_ocr' | 'failed'; size_bytes: number; attempts: number; page_count: number | null; failure_reason: string | null; created_at: string; updated_at: string }
export type Location = { kind: 'pdf_page' | 'docx_paragraph' | 'docx_table_cell'; unit: number; char_start: number; char_end: number }
export type Detail = { document: Document; body: string | null; locations: Location[] }
export type Page = { items: Document[]; page: number; page_size: number; total: number }
export type Citation = { document_id: string; chunk_id: string; document_version: 1; title: string; location: Location; quote: string; url: string }
export type Search = { mode: 'keyword'; items: Citation[] }
export type UploadPayload = { title: string; file: File }
const integer = (v: unknown, min: number, max: number): v is number => typeof v === 'number' && Number.isInteger(v) && v >= min && v <= max
const text = (v: unknown, min: number, max: number): v is string => bounded(v, min, max) && !/[\p{Cc}\uD800-\uDFFF]/u.test(v)
const bad = () => { throw new Error('Invalid document response') }
const reasons = ['corrupt', 'encrypted', 'unsupported_content', 'resource_limit', 'parse_timeout', 'worker_interrupted', 'internal_failure']
export const retryable = (d: Document) => d.status === 'failed' && d.attempts < 3 && ['parse_timeout', 'worker_interrupted', 'internal_failure'].includes(d.failure_reason ?? '')
export const locationName = (p: Location) => p.kind === 'pdf_page' ? `第 ${p.unit} 页` : p.kind === 'docx_table_cell' ? `表格内第 ${p.unit} 逻辑块` : `第 ${p.unit} 逻辑块`
export function documentRoute(path: string): { kind: 'list' | 'upload' | 'detail'; id?: string } | undefined {
  if (path === '/app/materials/documents') return { kind: 'list' }
  if (path === '/app/materials/documents/upload') return { kind: 'upload' }
  const id = path.replace(/^\/app\/materials\/documents\//, '')
  if (path.startsWith('/app/materials/documents/') && validId(id)) return { kind: 'detail', id }
}
export function parseDocument(v: unknown): Document {
  if (!shape(v, ['id', 'title', 'format', 'version', 'status', 'size_bytes', 'attempts', 'page_count', 'failure_reason', 'created_at', 'updated_at']) ||
    !validId(v.id) || !text(v.title, 1, 200) || trimSpace(v.title) !== v.title || !['pdf', 'docx'].includes(v.format as string) || v.version !== 1 ||
    !['queued', 'processing', 'ready', 'needs_ocr', 'failed'].includes(v.status as string) || !integer(v.size_bytes, 1, 10485760) || !integer(v.attempts, 0, 3) ||
    !timestamp(v.created_at) || !timestamp(v.updated_at) || v.updated_at < v.created_at ||
    (v.status === 'queued' ? v.attempts > 2 : v.attempts < 1) ||
    (v.status === 'failed' ? !reasons.includes(v.failure_reason as string) : v.failure_reason !== null) ||
    (v.format === 'pdf' && (v.status === 'ready' || v.status === 'needs_ocr') ? !integer(v.page_count, 1, 200) : v.page_count !== null) ||
    (v.format === 'docx' && v.status === 'needs_ocr')) return bad()
  return v as Document
}
export function parseLocation(v: unknown): Location {
  if (!shape(v, ['kind', 'unit', 'char_start', 'char_end']) || !['pdf_page', 'docx_paragraph', 'docx_table_cell'].includes(v.kind as string) ||
    !integer(v.unit, 1, v.kind === 'pdf_page' ? 200 : 20000) || !integer(v.char_start, 0, 2097151) || !integer(v.char_end, 1, 2097152) || v.char_end <= v.char_start) return bad()
  return v as Location
}
export function parseDetail(v: unknown, id: string): Detail {
  if (!shape(v, ['document', 'body', 'locations']) || !Array.isArray(v.locations) || v.locations.length > 20000) return bad()
  const d = parseDocument(v.document), locations = v.locations.map(parseLocation)
  if (d.id !== id) return bad()
  if (d.status !== 'ready') { if (v.body !== null || locations.length) return bad(); return { document: d, body: null, locations } }
  if (typeof v.body !== 'string' || !v.body.length || new TextEncoder().encode(v.body).length > 2097152 || /[\p{Cc}\uD800-\uDFFF]/u.test(v.body.replaceAll('\t', '').replaceAll('\n', '')) || !locations.length) return bad()
  const chars = [...v.body]; let end = 0, unit = 0
  for (const [i, p] of locations.entries()) {
    if (p.char_start !== end + (i ? 2 : 0) || (i && chars.slice(end, end + 2).join('') !== '\n\n') || p.char_end > chars.length || p.unit <= unit || !trimSpace(chars.slice(p.char_start, p.char_end).join('')) ||
      (d.format === 'pdf' ? p.kind !== 'pdf_page' || p.unit > d.page_count! : p.kind === 'pdf_page')) return bad()
    end = p.char_end; unit = p.unit
  }
  if (end !== chars.length) return bad()
  return { document: d, body: v.body, locations }
}
export function parsePage(v: unknown, page: number): Page {
  if (!shape(v, ['items', 'page', 'page_size', 'total']) || v.page !== page || v.page_size !== 20 || !integer(v.total, 0, Number.MAX_SAFE_INTEGER) || !Array.isArray(v.items) || v.items.length > 20) return bad()
  const items = v.items.map(parseDocument)
  if (items.length !== Math.min(20, Math.max(0, v.total - (page - 1) * 20)) || items.some((d, i) => i > 0 && BigInt(d.id) >= BigInt(items[i - 1].id))) return bad()
  return { items, page, page_size: 20, total: v.total }
}
export function parseSearch(v: unknown): Search {
  if (!shape(v, ['mode', 'items']) || v.mode !== 'keyword' || !Array.isArray(v.items) || v.items.length > 4) return bad()
  const seen = new Set<string>()
  const items = v.items.map(c => {
    if (!shape(c, ['document_id', 'chunk_id', 'document_version', 'title', 'location', 'quote', 'url']) || !validId(c.document_id) || !validId(c.chunk_id) || c.document_version !== 1 || !text(c.title, 1, 200) || trimSpace(c.title) !== c.title || !bounded(c.quote, 1, 2000) || /[\p{Cc}\uD800-\uDFFF]/u.test(c.quote.replaceAll('\t', '').replaceAll('\n', '')) || c.url !== `/api/v1/documents/${c.document_id}` || seen.has(c.chunk_id)) return bad()
    const location = parseLocation(c.location)
    if ([...c.quote].length !== location.char_end - location.char_start) return bad()
    seen.add(c.chunk_id); return { ...c, location } as Citation
  })
  return { mode: 'keyword', items }
}
export function checkCitation(d: Detail, c: Citation) {
  if (d.document.status !== 'ready' || !d.body || d.document.id !== c.document_id || d.document.version !== c.document_version || d.document.title !== c.title || !d.locations.some(p => p.kind === c.location.kind && p.unit === c.location.unit && p.char_start <= c.location.char_start && p.char_end >= c.location.char_end) || [...d.body].slice(c.location.char_start, c.location.char_end).join('') !== c.quote) return bad()
}
export async function validateUpload(p: UploadPayload): Promise<UploadPayload> {
  const title = validateTitle(p.title)
  if (!(p.file instanceof File) || p.file.size < 1 || p.file.size > 10485760 || !/\.(pdf|docx)$/i.test(p.file.name)) throw new UploadValidationError('file', '请选择 1 字节至 10 MiB 的 PDF 或 .docx 文件，不支持 .doc、宏文件或图片。')
  const signature = new Uint8Array(await p.file.slice(0, 5).arrayBuffer())
  if (/\.pdf$/i.test(p.file.name) ? new TextDecoder().decode(signature) !== '%PDF-' : signature.length < 4 || signature.slice(0, 4).join(',') !== '80,75,3,4') throw new UploadValidationError('file', '文件扩展名与内容不符，请选择有效 PDF 或 .docx 文件。')
  return { title, file: p.file }
}
