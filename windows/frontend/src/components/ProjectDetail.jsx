import React from 'react'
import { Pencil, Play, RotateCcw, Square, Trash2 } from 'lucide-react'
import { Dialog } from './Dialog.jsx'
import { Status } from './Status.jsx'

export function ProjectDetail({ detail, onClose, onEdit, onOperate, onDelete }) {
  const [tab, setTab] = React.useState('runtime')
  const { project, runtime } = detail
  const runtimeByName = new Map((runtime?.processes ?? []).map((item) => [item.name, item]))
  return (
    <Dialog
      title={project.name}
      description={`${project.code} · ${project.manageMode === 'internal' ? '内部管理' : '外部管理'}`}
      onClose={onClose}
      size="large"
      footer={<><button className="button button--danger-quiet" onClick={() => onDelete(project)}><Trash2 size={15} />删除</button><span className="footer-spacer" /><button className="button button--secondary" onClick={() => onEdit(project.code)}><Pencil size={15} />编辑</button><button className="button button--secondary" onClick={() => onOperate('restart', project)}><RotateCcw size={15} />重启</button>{overallStatus(runtime) === 'stopped' ? <button className="button button--primary" onClick={() => onOperate('start', project)}><Play size={15} />启动</button> : <button className="button button--danger" onClick={() => onOperate('stop', project)}><Square size={14} />停止</button>}</>}
    >
      <div className="detail-summary"><div><span>项目状态</span><Status value={overallStatus(runtime)} /></div><div><span>后端</span><strong>{project.backends.length}</strong></div><div><span>前端</span><strong>{project.frontends.length}</strong></div><div><span>部署规则</span><strong>{project.deployRules.length}</strong></div></div>
      <div className="detail-tabs">{[['runtime', '运行状态'], ['backends', '后端配置'], ['frontends', '前端 Nginx'], ['deploy', '部署规则']].map(([id, label]) => <button key={id} className={tab === id ? 'active' : ''} onClick={() => setTab(id)}>{label}</button>)}</div>
      {tab === 'runtime' ? <RuntimeList project={project} runtimeByName={runtimeByName} /> : null}
      {tab === 'backends' ? <ConfigTable rows={project.backends} columns={[['name', '名称'], ['runtime', '环境'], ['workDir', '工作目录'], ['expectedPorts', '端口']]} /> : null}
      {tab === 'frontends' ? <ConfigTable rows={project.frontends} columns={[['name', '名称'], ['nginxMode', '模式'], ['rootDir', '站点目录'], ['expectedPorts', '端口']]} /> : null}
      {tab === 'deploy' ? <ConfigTable rows={project.deployRules} columns={[['name', '规则'], ['type', '类型'], ['source', '源文件'], ['targetDir', '目标目录']]} /> : null}
    </Dialog>
  )
}

function RuntimeList({ project, runtimeByName }) {
  const components = [...project.backends.map((item) => ({ ...item, type: '后端' })), ...project.frontends.map((item) => ({ ...item, type: '前端' }))]
  return <div className="runtime-list">{components.length ? components.map((component) => { const runtime = runtimeByName.get(component.name); return <div key={`${component.type}-${component.name}`}><Status value={runtime?.status ?? 'stopped'} /><strong>{component.name}</strong><span>{component.type}</span><code>{runtime?.pid ? `PID ${runtime.pid}` : '-'}</code><code>{runtime?.ports?.length ? runtime.ports.join(', ') : '-'}</code></div> }) : <div className="empty-config">项目没有配置进程。</div>}</div>
}

function ConfigTable({ rows, columns }) {
  return <div className="table-wrap detail-table"><table className="data-table"><thead><tr>{columns.map(([_key, label]) => <th key={label}>{label}</th>)}</tr></thead><tbody>{rows.length ? rows.map((row, index) => <tr key={`${row.name}-${index}`}>{columns.map(([key]) => <td key={key}>{Array.isArray(row[key]) ? row[key].join(', ') || '-' : <span className="truncate-cell" title={String(row[key] ?? '')}>{String(row[key] ?? '-')}</span>}</td>)}</tr>) : <tr><td colSpan={columns.length}><div className="empty-table">暂无配置。</div></td></tr>}</tbody></table></div>
}

function overallStatus(runtime) {
  const processes = runtime?.processes ?? []
  if (!processes.length) return 'stopped'
  const running = processes.filter((item) => item.status === 'running').length
  if (running === processes.length) return 'running'
  if (running > 0) return 'partial'
  return processes.some((item) => item.status === 'failed') ? 'failed' : 'stopped'
}
