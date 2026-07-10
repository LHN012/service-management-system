import React from 'react'
import { CheckCircle2, CircleX, RefreshCw } from 'lucide-react'
import { getEnvironment } from '../api.js'

export function Environment({ refreshToken, onError }) {
  const [items, setItems] = React.useState([])
  const [loading, setLoading] = React.useState(true)
  const load = React.useCallback(async () => {
    setLoading(true)
    try { setItems(await getEnvironment()) } catch (error) { onError(error) } finally { setLoading(false) }
  }, [onError])
  React.useEffect(() => { load() }, [load, refreshToken])
  const ready = items.filter((item) => item.status === 'ready').length
  return (
    <section className="work-section page-section">
      <div className="section-heading section-heading--page"><div><h2>运行环境</h2><p>本机运行时、部署工具与权限状态</p></div><button className="button button--secondary" onClick={load} disabled={loading}><RefreshCw size={15} className={loading ? 'spin' : ''} />重新检测</button></div>
      <div className="environment-summary"><strong>{ready} / {items.length}</strong><span>项能力可用</span><div className="environment-progress"><i style={{ width: `${items.length ? (ready / items.length) * 100 : 0}%` }} /></div></div>
      <div className="environment-list">
        {items.map((item) => (
          <div className="environment-row" key={item.name}>
            <span className={`environment-row__icon environment-row__icon--${item.status}`}>{item.status === 'ready' ? <CheckCircle2 size={19} /> : <CircleX size={19} />}</span>
            <div><strong>{item.name}</strong><small>{item.required ? '核心依赖' : '按需使用'}</small></div>
            <code>{item.path || '未在 PATH 中发现'}</code>
            <span className={`environment-state environment-state--${item.status}`}>{item.status === 'ready' ? '可用' : '未安装'}</span>
          </div>
        ))}
      </div>
    </section>
  )
}
