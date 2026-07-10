import React from 'react'
import { RefreshCw, Search } from 'lucide-react'
import { listProcesses } from '../api.js'
import { PortList } from '../components/ProjectTable.jsx'
import { SkeletonRows } from '../components/Status.jsx'

export function Processes({ refreshToken, onError }) {
  const [processes, setProcesses] = React.useState([])
  const [query, setQuery] = React.useState('')
  const [loading, setLoading] = React.useState(true)

  const load = React.useCallback(async () => {
    setLoading(true)
    try { setProcesses(await listProcesses()) } catch (error) { onError(error) } finally { setLoading(false) }
  }, [onError])

  React.useEffect(() => { load() }, [load, refreshToken])

  const normalized = query.trim().toLowerCase()
  const filtered = normalized ? processes.filter((process) => [process.name, process.command, process.cwd, String(process.pid), ...(process.ports ?? []).map(String)].some((value) => value?.toLowerCase().includes(normalized))) : processes

  return (
    <section className="work-section page-section">
      <div className="section-heading section-heading--page">
        <div><h2>进程与端口</h2><p>已识别的 Java、Python、Node、Nginx 和监听端口</p></div>
        <div className="heading-actions">
          <label className="search-field"><Search size={15} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索 PID、端口或命令" /></label>
          <button className="button button--secondary" onClick={load} disabled={loading}><RefreshCw size={15} className={loading ? 'spin' : ''} />刷新</button>
        </div>
      </div>
      <div className="table-wrap">
        <table className="data-table process-table">
          <thead><tr><th>PID</th><th>进程</th><th>用户</th><th>监听端口</th><th>工作目录</th><th>命令行</th></tr></thead>
          <tbody>
            {loading ? <SkeletonRows rows={5} columns={6} /> : null}
            {!loading && filtered.length === 0 ? <tr><td colSpan="6"><div className="empty-table">没有符合条件的进程。</div></td></tr> : null}
            {!loading ? filtered.map((process) => (
              <tr key={process.pid}>
                <td><code className="pid">{process.pid}</code></td>
                <td><strong>{process.name}</strong></td>
                <td>{process.user || '-'}</td>
                <td><PortList ports={process.ports} /></td>
                <td><span className="truncate-cell" title={process.cwd}>{process.cwd || '-'}</span></td>
                <td><code className="command-cell" title={process.command}>{process.command}</code></td>
              </tr>
            )) : null}
          </tbody>
        </table>
      </div>
      <div className="table-footer">显示 {filtered.length} / {processes.length} 个进程</div>
    </section>
  )
}
