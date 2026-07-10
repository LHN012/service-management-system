import { ArrowRight, Boxes, CircleDot, Clock3, Network, ServerCog } from 'lucide-react'
import { ProjectTable } from '../components/ProjectTable.jsx'
import { Status } from '../components/Status.jsx'

export function Overview({ dashboard, loading, onNavigate, onOpenProject, onEditProject, onOperate }) {
  const projects = dashboard?.projects ?? []
  const running = projects.filter((project) => project.status === 'running').length
  const attention = projects.filter((project) => ['partial', 'failed', 'unknown'].includes(project.status)).length
  return (
    <div className="overview-page">
      <section className="metric-strip" aria-label="运行概览">
        <Metric icon={Boxes} label="项目" value={projects.length} detail={`${running} 个运行中`} />
        <Metric icon={ServerCog} label="已识别进程" value={dashboard?.processCount ?? 0} detail="Java / Python / Node / Nginx" />
        <Metric icon={Network} label="监听端口" value={dashboard?.portCount ?? 0} detail="本机 TCP 监听" />
        <Metric icon={CircleDot} label="需要处理" value={attention} detail={attention ? '存在部分运行或未知状态' : '当前没有状态告警'} tone={attention ? 'warning' : 'success'} />
      </section>

      <div className="overview-grid">
        <section className="work-section">
          <div className="section-heading">
            <div><h2>项目运行状态</h2><p>按项目查看后端、前端与监听端口</p></div>
            <button className="text-button" onClick={() => onNavigate('projects')}>全部项目 <ArrowRight size={15} /></button>
          </div>
          <ProjectTable projects={projects} loading={loading} onOpen={onOpenProject} onEdit={onEditProject} onOperate={onOperate} />
        </section>
        <aside className="activity-rail">
          <div className="section-heading"><div><h2>最近操作</h2><p>本机审计记录</p></div></div>
          <div className="activity-list">
            {(dashboard?.recent ?? []).length === 0 ? <p className="empty-copy">暂无操作记录</p> : null}
            {(dashboard?.recent ?? []).map((item, index) => (
              <div className="activity-item" key={`${item.time}-${index}`}>
                <span className="activity-item__line" />
                <span className={`activity-item__mark activity-item__mark--${item.result}`} />
                <div>
                  <div className="activity-item__top"><strong>{actionLabel(item.action)}</strong><Status value={item.result === 'success' ? 'running' : 'failed'} compact /></div>
                  <p>{item.target || '系统'}</p>
                  <time><Clock3 size={12} />{relativeTime(item.time)}</time>
                </div>
              </div>
            ))}
          </div>
        </aside>
      </div>
    </div>
  )
}

function Metric({ icon: Icon, label, value, detail, tone = 'default' }) {
  return (
    <div className={`metric metric--${tone}`}>
      <span className="metric__icon"><Icon size={18} /></span>
      <div><span>{label}</span><strong>{value}</strong><small>{detail}</small></div>
    </div>
  )
}

function actionLabel(action) {
  return ({ scan: '状态扫描', restart: '重启项目', deploy: '部署迁移', stop: '停止项目', start: '启动项目', save_project: '保存项目' })[action] ?? action
}

function relativeTime(value) {
  const difference = Date.now() - new Date(value).getTime()
  if (!Number.isFinite(difference)) return '-'
  if (difference < 60000) return '刚刚'
  if (difference < 3600000) return `${Math.floor(difference / 60000)} 分钟前`
  if (difference < 86400000) return `${Math.floor(difference / 3600000)} 小时前`
  return new Date(value).toLocaleDateString('zh-CN')
}
