import {
  Activity, Boxes, FileClock, Gauge, HardDrive, ListRestart, Logs, RefreshCw,
  Settings, ShieldCheck, TerminalSquare,
} from 'lucide-react'
import { Status } from './Status.jsx'

const navigation = [
  { id: 'overview', label: '总览', icon: Gauge },
  { id: 'projects', label: '项目管理', icon: Boxes },
  { id: 'processes', label: '进程与端口', icon: Activity },
  { id: 'deployment', label: '部署迁移', icon: ListRestart },
  { id: 'environment', label: '环境检测', icon: ShieldCheck },
  { id: 'logs', label: '日志审计', icon: Logs },
  { id: 'settings', label: '设置', icon: Settings },
]

const pageTitles = Object.fromEntries(navigation.map((item) => [item.id, item.label]))

export function Shell({ page, onNavigate, dashboard, loading, onRefresh, children }) {
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand__mark"><TerminalSquare size={18} strokeWidth={2.2} /></span>
          <span className="brand__copy"><strong>Service Manager</strong><small>Windows</small></span>
        </div>
        <nav className="sidebar__nav" aria-label="主导航">
          {navigation.map(({ id, label, icon: Icon }) => (
            <button key={id} className={`nav-item${page === id ? ' nav-item--active' : ''}`} onClick={() => onNavigate(id)} title={label}>
              <Icon size={17} strokeWidth={1.9} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
        <div className="sidebar__footer">
          <div className="agent-state">
            <Status value={dashboard?.agent?.status ?? 'unknown'} compact />
            <span><strong>Desktop Agent</strong><small>{dashboard?.agent?.mode ?? '正在连接'}</small></span>
          </div>
          <div className="data-root" title={dashboard?.dataRoot ?? ''}>
            <HardDrive size={14} />
            <span>{dashboard?.dataRoot ?? 'ProgramData'}</span>
          </div>
        </div>
      </aside>
      <main className="main-area">
        <header className="topbar">
          <div>
            <h1>{pageTitles[page]}</h1>
            <p>{page === 'overview' ? '本机项目、进程与部署状态' : 'Service Management System'}</p>
          </div>
          <div className="topbar__actions">
            <span className="scan-time"><FileClock size={14} />{formatScanTime(dashboard?.agent?.lastScan)}</span>
            <button className="icon-button" onClick={onRefresh} disabled={loading} title="立即扫描" aria-label="立即扫描">
              <RefreshCw size={17} className={loading ? 'spin' : ''} />
            </button>
          </div>
        </header>
        <div className="page-content">{children}</div>
      </main>
    </div>
  )
}

function formatScanTime(value) {
  if (!value) return '尚未扫描'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '尚未扫描'
  return `最近扫描 ${date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}`
}
