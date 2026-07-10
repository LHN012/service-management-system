import React from 'react'
import { Copy, RefreshCw } from 'lucide-react'
import { readLog } from '../api.js'

export function Logs({ projects, refreshToken, onError, onToast }) {
  const [kind, setKind] = React.useState('audit')
  const [project, setProject] = React.useState(projects[0]?.code ?? '')
  const [lines, setLines] = React.useState([])
  const [loading, setLoading] = React.useState(true)
  const load = React.useCallback(async () => {
    setLoading(true)
    try { setLines(await readLog(kind, project, 500)) } catch (error) { onError(error) } finally { setLoading(false) }
  }, [kind, project, onError])
  React.useEffect(() => { load() }, [load, refreshToken])
  const copy = async () => {
    await navigator.clipboard.writeText(lines.join('\n'))
    onToast('日志已复制到剪贴板')
  }
  return (
    <section className="work-section page-section log-page">
      <div className="section-heading section-heading--page"><div><h2>日志与审计</h2><p>查看系统、操作和项目日志</p></div><div className="heading-actions"><button className="button button--secondary" onClick={copy}><Copy size={15} />复制</button><button className="button button--secondary" onClick={load}><RefreshCw size={15} className={loading ? 'spin' : ''} />刷新</button></div></div>
      <div className="log-toolbar">
        <div className="segmented" role="tablist">{[['audit', '操作日志'], ['system', '系统日志'], ['project', '项目日志']].map(([value, label]) => <button key={value} className={kind === value ? 'active' : ''} onClick={() => setKind(value)}>{label}</button>)}</div>
        {kind === 'project' ? <select value={project} onChange={(event) => setProject(event.target.value)}>{projects.map((item) => <option key={item.code} value={item.code}>{item.name}</option>)}</select> : null}
        <span>{lines.length} 行</span>
      </div>
      <pre className="log-viewer" aria-live="polite">{loading ? '正在读取日志...' : lines.length ? lines.map((line, index) => <code key={index}><i>{String(index + 1).padStart(3, '0')}</i>{line}</code>) : '暂无日志记录。'}</pre>
    </section>
  )
}
