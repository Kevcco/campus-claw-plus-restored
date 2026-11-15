import { useEffect, useMemo, useRef, useState } from 'react'
import type { FormEvent, ReactNode } from 'react'
import { AuthError, createAuthClient } from '../auth/client'
import type { Principal } from '../auth/client'
import { Link, ErrorSummary, useCooldown } from '../components/ui'
import { checkCitation, locationName, retryable, trimSpace, validateTitle, validateUpload, UploadValidationError } from './contracts'
import type { Citation, Detail, Document, Page, Search, UploadPayload } from './contracts'
import '../materials/materials.css'
import './documents.css'

type Client = ReturnType<typeof createAuthClient>['documents']
const failure = (v: unknown) => v instanceof AuthError ? v : new AuthError(0, '资料响应异常，请检查连接后重试。')
const labels = { queued: '等待解析', processing: '正在解析', ready: '可检索', needs_ocr: '待文字识别', failed: '解析失败' }
const reasons: Record<string, string> = { corrupt: '文件损坏，请重新导出后发起新上传。', encrypted: '文件有密码保护，请先移除密码。', unsupported_content: '内容暂不支持，请导出为普通文本型 PDF 或 .docx。', resource_limit: '文件超过解析资源限制，请拆分或简化后重新上传。', parse_timeout: '解析超时。', worker_interrupted: '解析服务中断。', internal_failure: '解析暂未完成。' }
const date = (value: string) => new Date(value).toLocaleDateString('zh-CN')
export function MaterialTypes({ documents = false }: { documents?: boolean }) {
  return <nav className="document-tabs" aria-label="资料类型"><Link to="/app/materials" current={!documents}>文本资料</Link><Link to="/app/materials/documents" current={documents}>PDF与Word资料</Link></nav>
}
function Status({ value }: { value: Document['status'] }) { return <span className={'document-status document-status-' + value}>{labels[value]}</span> }
export function DocumentsList({ client, principal, openCitation }: { client: Client; principal: Principal; openCitation: (c: Citation) => void }) {
  const [query, setQuery] = useState(''), [activeQuery, setActiveQuery] = useState(''), [page, setPage] = useState(1)
  const [data, setData] = useState<Page | Search>(), [busy, setBusy] = useState(true), [error, setError] = useState<AuthError>()
  const request = useRef<AbortController>(), sequence = useRef(0), heading = useRef<HTMLHeadingElement>(null)
  const wait = useCooldown(error?.retryAt)
  async function load(target: number, q: string, quiet = false) {
    request.current?.abort(); const controller = new AbortController(); request.current = controller; const id = ++sequence.current
    setBusy(true); setError(undefined); if (!quiet) setData(undefined); setPage(target); setActiveQuery(q)
    try { const result = q ? await client.search(q, controller.signal) : await client.list(target, controller.signal); if (!controller.signal.aborted && id === sequence.current) setData(result) }
    catch (e) { if (!controller.signal.aborted && id === sequence.current) setError(failure(e)) }
    finally { if (!controller.signal.aborted && id === sequence.current) setBusy(false) }
  }
  useEffect(() => { void load(1, ''); return () => { sequence.current++; request.current?.abort() } }, [client])
  useEffect(() => {
    if (!data || 'mode' in data || busy || error || !data.items.some(d => d.status === 'queued' || d.status === 'processing')) return
    const timer = window.setTimeout(() => { if (document.visibilityState === 'visible') void load(page, '', true) }, 2000)
    const resume=()=>{if(document.visibilityState==='visible')void load(page,'',true)}
    document.addEventListener('visibilitychange',resume)
    return () => {window.clearTimeout(timer);document.removeEventListener('visibilitychange',resume)}
  }, [data, busy, error, page])
  function search(event: FormEvent) {
    event.preventDefault(); if (wait) return
    const q = trimSpace(query)
    if (!q || [...q].length > 2000 || /[\p{Cc}\uD800-\uDFFF]/u.test(q)) { setError(new AuthError(400, '请输入 1–2000 个有效字符的关键词。')); return }
    void load(1, q).then(() => heading.current?.focus())
  }
  return <div className="materials-page"><MaterialTypes documents />
    <div className="material-title-row"><div><h1>PDF与Word资料</h1><p className="muted">查找本班已解析资料，核对页码或逻辑块出处。</p></div>{principal.role === 'teacher' && <Link className="material-upload-link" to="/app/materials/documents/upload">上传资料</Link>}</div>
    <form className="material-search" onSubmit={search} noValidate><div className="material-search-field"><label htmlFor="document-query">关键词</label><input id="document-query" value={query} onChange={e => setQuery(e.target.value)} placeholder="按正文关键词搜索" /></div><div className="material-actions"><button disabled={wait > 0}>搜索</button><button type="button" disabled={wait > 0} onClick={() => { setQuery(''); void load(1, '') }}>清除</button></div></form>
    <ErrorSummary id="document-list-error" error={error} wait={wait} />
    <section className="material-results" aria-busy={busy}><h2 ref={heading} tabIndex={-1}>{activeQuery ? '检索结果' : '本班资料'}</h2>
      {busy && !data ? <p className="material-state" role="status">正在加载资料…</p> : error ? <div className="material-state"><button disabled={wait > 0} onClick={() => void load(page, activeQuery)}>重试加载</button></div> : !data?.items.length ? <div className="material-state"><p>{activeQuery ? '没有找到匹配资料' : '当前班级还没有PDF与Word资料'}</p><p className="muted">{activeQuery ? '请调整关键词，或清除查询查看列表。' : principal.role === 'teacher' ? '上传本班学习材料，解析后即可检索。' : '请等待任课教师提供已解析的资料。'}</p></div> : 'mode' in data ?
        <ul className="material-citations">{data.items.map(c => <li key={c.chunk_id}><Link to={'/app/materials/documents/' + c.document_id} onFollow={() => openCitation(c)}>{c.title}</Link><p className="muted">{locationName(c.location)} · 资料 ID：{c.document_id}</p><p className="material-quote">{c.quote}</p></li>)}</ul> :
        <><ul className="material-list">{data.items.map(d => <li key={d.id}><div className="material-record-title"><Link to={'/app/materials/documents/' + d.id}>{d.title}</Link><p className="material-id muted">{d.format === 'pdf' ? 'PDF' : 'Word'} · 资料 ID：{d.id}</p></div><Status value={d.status} /><span className="muted">{(d.size_bytes / 1024).toFixed(1)} KiB</span><time dateTime={d.created_at} className="muted">{date(d.created_at)}</time></li>)}</ul><div className="material-pagination" aria-label="资料分页"><button disabled={busy || page <= 1} onClick={() => void load(page - 1, '')}>上一页</button><span>第 {page} 页</span><button disabled={busy || page * 20 >= data.total || page >= 2147483647} onClick={() => void load(page + 1, '')}>下一页</button></div></>}
    </section><p className="muted help">当前支持文本型 PDF 和普通 .docx。扫描件会标为待文字识别；Word 使用逻辑块定位，不提供排版页码。</p>
  </div>
}

export function DocumentView({ client, principal, id, citation }: { client: Client; principal: Principal; id: string; citation?: Citation }) {
  const [data, setData] = useState<Detail>(), [busy, setBusy] = useState(true), [error, setError] = useState<AuthError>(), [actionError, setActionError] = useState<AuthError>()
  const [action, setAction] = useState(''), [notice, setNotice] = useState(''), [revision, setRevision] = useState(0)
  const requests = useRef(new Set<AbortController>()), urls = useRef(new Set<string>()), alive = useRef(false), lock = useRef(false)
  const wait = useCooldown(actionError?.retryAt)
  const chars=useMemo(()=>[...(data?.body??'')],[data?.body])
  useEffect(() => {
    alive.current = true; let timer = 0; const controller = new AbortController(); requests.current.add(controller); setData(undefined); setError(undefined); setBusy(true)
    async function load() {
      try { const d = await client.detail(id, controller.signal); if (citation) checkCitation(d, citation); if (!alive.current || controller.signal.aborted) return; setData(d); setBusy(false)
        if (citation) requestAnimationFrame(() => {
          if (!alive.current || controller.signal.aborted) return
          const source = document.getElementById('source-' + citation.location.unit)
          const target = source?.querySelector('mark') ?? source
          target?.focus({ preventScroll: true })
          target?.scrollIntoView({ block: 'start' })
        })
        if (d.document.status === 'queued' || d.document.status === 'processing') timer = window.setTimeout(() => { void load() }, 2000)
      } catch (e) { if (alive.current && !controller.signal.aborted) { setError(failure(e)); setBusy(false) } }
    }
    void load()
    return () => { alive.current = false; window.clearTimeout(timer); requests.current.forEach(c => c.abort()); requests.current.clear(); urls.current.forEach(u => URL.revokeObjectURL(u)); urls.current.clear(); lock.current = false }
  }, [client, id, citation, revision])
  async function perform(kind: 'download' | 'retry') {
    if (!data || lock.current || wait) return; lock.current = true; setAction(kind); setActionError(undefined)
    const controller = new AbortController(); requests.current.add(controller)
    try {
      if (kind === 'retry') { await client.retry(id, controller.signal); if (alive.current && !controller.signal.aborted) { setNotice('重试请求已受理，请查看最新解析状态。'); setRevision(n => n + 1) } }
      else { const blob = await client.download(id, controller.signal); if (!alive.current || controller.signal.aborted) return; const url = URL.createObjectURL(blob); urls.current.add(url); const link = document.createElement('a'); link.href = url; link.download = `document-${id}.${data.document.format}`; link.click(); window.setTimeout(() => { URL.revokeObjectURL(url); urls.current.delete(url) }, 0); setNotice('原文件已准备下载。') }
    } catch (e) { if (alive.current && !controller.signal.aborted) setActionError(failure(e)) }
    finally { requests.current.delete(controller); if (alive.current && !controller.signal.aborted) { lock.current = false; setAction('') } }
  }
  const d = data?.document
  return <div className="materials-page"><Link to="/app/materials/documents" className="material-back">← 返回PDF与Word资料</Link><h1>{d?.title ?? '资料详情'}</h1>
    {busy ? <p className="material-state" role="status">正在加载资料…</p> : error ? <div className="material-state"><ErrorSummary id="document-detail-error" error={error} wait={0} /><button onClick={() => setRevision(n => n + 1)}>重试加载</button></div> : d && data && <>
      <div className="document-metadata"><Status value={d.status} /><span>{d.format === 'pdf' ? 'PDF' : 'Word'} · {(d.size_bytes / 1024).toFixed(1)} KiB</span><span>已尝试 {d.attempts} / 3 次</span><span>资料 ID：{d.id}</span></div>
      {notice && <p role="status" className="material-saved">{notice}</p>}
      <div className="material-actions"><button disabled={!!action || wait > 0} onClick={() => void perform('download')}>{action === 'download' ? '正在准备…' : '下载原件'}</button>{principal.role === 'teacher' && retryable(d) && <button disabled={!!action || wait > 0} onClick={() => void perform('retry')}>{action === 'retry' ? '正在提交…' : '重试解析'}</button>}</div>
      <ErrorSummary id="document-action-error" error={actionError} wait={wait} />
      {d.status === 'queued' || d.status === 'processing' ? <p role="status" className="material-state">{d.status === 'queued' ? '资料已保存，等待解析。' : '正在解析正文，完成后会自动更新。'}</p> : d.status === 'needs_ocr' ? <div className="material-state"><h2>待文字识别</h2><p>这份 PDF 含扫描图片，当前不能检索正文。教师可下载原件；文字识别将在后续阶段提供。</p></div> : d.status === 'failed' ? <div className="material-state"><h2>解析失败</h2><p>{reasons[d.failure_reason ?? '']}{retryable(d) && '教师可以重试。'}</p>{d.attempts >= 3 && <p>已达到重试上限，请核对原件后发起新上传。</p>}</div> : <section className="document-sources"><h2>原文与出处</h2><p className="muted">{d.format === 'pdf' ? `原件共 ${d.page_count} 页，以下保留原页码。` : '以下按正文顺序显示段落和表格逻辑块，逻辑块不是Word排版页码。'}</p>{data.locations.map(p => {
        const selected = citation?.location.unit === p.unit ? citation.location : undefined
        const content: ReactNode = selected ? <>{chars.slice(p.char_start, selected.char_start).join('')}<mark tabIndex={-1}>{chars.slice(selected.char_start, selected.char_end).join('')}</mark>{chars.slice(selected.char_end, p.char_end).join('')}</> : chars.slice(p.char_start, p.char_end).join('')
        return <section key={p.unit} id={'source-' + p.unit} className="document-source" tabIndex={-1}><h3>{locationName(p)}</h3><p className="document-body">{content}</p></section>
      })}</section>}
    </>}
  </div>
}

export function DocumentUpload({ client, principal, completed }: { client: Client; principal: Principal; completed: (d: Document) => void }) {
  const [title, setTitle] = useState(''), [file, setFile] = useState<File>(), [error, setError] = useState<AuthError>(), [busy, setBusy] = useState(false), [uncertain, setUncertain] = useState(false), [confirmNew, setConfirmNew] = useState(false), [hint, setHint] = useState('')
  const attempt = useRef<{ payload: UploadPayload; key: string }>(), input = useRef<HTMLInputElement>(null), alive = useRef(false), sending = useRef(false), request = useRef<AbortController>()
  const wait = useCooldown(error?.retryAt)
  useEffect(() => { alive.current = true; return () => { alive.current = false; request.current?.abort(); attempt.current = undefined; if (input.current) input.current.value = '' } }, [])
  async function submit(event: FormEvent) {
    event.preventDefault(); if (sending.current || wait || principal.role !== 'teacher' || error?.status === 409) return
    sending.current = true; setBusy(true); setError(undefined); const controller = new AbortController(); request.current = controller
    try {
      if (!attempt.current) { if (!file) throw new UploadValidationError('file', '请选择 PDF 或 .docx 文件。'); const payload = await validateUpload({ title, file }); if (!alive.current || controller.signal.aborted) return; attempt.current = { payload, key: crypto.randomUUID() } }
      const result = await client.upload(attempt.current.payload, attempt.current.key, controller.signal)
      if (alive.current && !controller.signal.aborted) { attempt.current = undefined; completed(result) }
    } catch (e) { if (alive.current && !controller.signal.aborted) { const value = e instanceof UploadValidationError ? new AuthError(400, e.message, '', 0, [e.field]) : failure(e); setError(value); setUncertain(!!attempt.current && [0, 401, 503, 504].includes(value.status)) } }
    finally { if (alive.current && !controller.signal.aborted) { sending.current = false; setBusy(false) } }
  }
  if (principal.role !== 'teacher') return <div className="materials-page"><h1>无权上传资料</h1><p>只有当前班级教师可以上传资料。</p><Link to="/app/materials/documents">返回资料列表</Link></div>
  return <div className="materials-page"><Link to="/app/materials/documents" className="material-back">← 返回PDF与Word资料</Link><h1>上传PDF与Word资料</h1><p className="muted intro">资料将保留原件并异步解析。受理成功后可查看真实解析状态。</p>
    <form className="material-upload-form" onSubmit={submit} noValidate aria-busy={busy}>
      <label htmlFor="document-title">资料标题 <span className="muted">（必填，最多200个字符）</span></label><input id="document-title" required value={title} disabled={busy || !!attempt.current} onChange={e => { setTitle(e.target.value); setHint('') }} onBlur={() => { try { validateTitle(title); setHint('') } catch { setHint('请输入 1–200 个字符的有效标题。') } }} aria-invalid={!!hint || error?.fields.includes('title')} aria-describedby={hint ? 'document-title-hint' : error?.fields.includes('title') ? 'document-upload-error' : undefined} />
      {hint && <p id="document-title-hint" className="error-message help">{hint}</p>}
      <label htmlFor="document-file">资料文件 <span className="muted">（必填，最大10 MiB）</span></label><input ref={input} id="document-file" type="file" required accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document" disabled={busy || !!attempt.current} onChange={e => setFile(e.target.files?.[0])} aria-invalid={error?.fields.includes('file')} aria-describedby="document-file-help document-upload-error" />
      <p id="document-file-help" className="help muted">支持文本型 PDF 和普通 .docx；旧 .doc 请先另存为 .docx。不支持宏、图片或密码保护；扫描页会进入待文字识别。</p>
      <ErrorSummary id="document-upload-error" error={error} wait={wait} />
      {uncertain && <p className="material-saved" role="status">上传结果尚未确认。请使用相同内容确认结果，或先回列表核对。</p>}
      <div className="material-actions"><button className="primary" disabled={busy || wait > 0 || error?.status === 409}>{busy ? '正在提交…' : uncertain ? '确认上传结果' : attempt.current ? '重试上传' : '上传资料'}</button>{attempt.current && <button type="button" disabled={busy} onClick={() => setConfirmNew(true)}>更换内容，发起新上传</button>}</div>
      {confirmNew && <section className="section" aria-label="确认新上传"><h2>确认发起新上传</h2><p>旧请求可能已保存。请先在列表核对，新上传会使用新的上传标识。</p><div className="material-actions"><button type="button" onClick={() => { attempt.current = undefined; setTitle(''); setFile(undefined); if (input.current) input.current.value = ''; setError(undefined); setUncertain(false); setConfirmNew(false) }}>确认新上传</button><button type="button" onClick={() => setConfirmNew(false)}>取消</button></div></section>}
    </form><p className="help muted material-notice">刷新、离开页面或身份变化会清除文件与上传确认信息。取消等待不会撤销已保存的资料，结果不确定时请先核对列表。</p>
  </div>
}
