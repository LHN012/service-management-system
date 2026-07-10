import { Eye, MoreHorizontal, Pencil, Play, RotateCcw, Square } from 'lucide-react'
import { SkeletonRows, Status } from './Status.jsx'

export function ProjectTable({ projects, loading, onOpen, onEdit, onOperate, showHeader = true }) {
  return (
    <div className="table-wrap">
      <table className="data-table project-table">
        {showHeader ? (
          <thead>
            <tr>
              <th>状态</th><th>项目</th><th>模式</th><th>后端</th><th>前端</th><th>端口</th><th className="align-right">操作</th>
            </tr>
          </thead>
        ) : null}
        <tbody>
          {loading ? <SkeletonRows rows={4} columns={7} /> : null}
          {!loading && projects.length === 0 ? (
            <tr><td colSpan="7"><div className="empty-table">暂无项目，使用“新建项目”添加第一项配置。</div></td></tr>
          ) : null}
          {!loading ? projects.map((project) => (
            <tr key={project.code}>
              <td><Status value={project.status} /></td>
              <td>
                <button className="project-link" onClick={() => onOpen(project.code)}>
                  <strong>{project.name}</strong><small>{project.code}</small>
                </button>
              </td>
              <td><span className="plain-tag">{project.manageMode === 'internal' ? '内部' : '外部'}</span></td>
              <td><strong>{project.runningBackends}</strong><span className="muted"> / {project.backends}</span></td>
              <td><strong>{project.runningFrontends}</strong><span className="muted"> / {project.frontends}</span></td>
              <td><PortList ports={project.ports} /></td>
              <td>
                <div className="row-actions">
                  {project.status === 'stopped' ? (
                    <button className="icon-button icon-button--table" onClick={() => onOperate('start', project)} title="启动"><Play size={15} /></button>
                  ) : (
                    <button className="icon-button icon-button--table" onClick={() => onOperate('stop', project)} title="停止"><Square size={14} /></button>
                  )}
                  <button className="icon-button icon-button--table" onClick={() => onOperate('restart', project)} title="重启"><RotateCcw size={15} /></button>
                  <button className="icon-button icon-button--table" onClick={() => onEdit(project.code)} title="编辑"><Pencil size={15} /></button>
                  <button className="icon-button icon-button--table" onClick={() => onOpen(project.code)} title="详情"><Eye size={15} /></button>
                  <button className="icon-button icon-button--table action-overflow" title="更多"><MoreHorizontal size={16} /></button>
                </div>
              </td>
            </tr>
          )) : null}
        </tbody>
      </table>
    </div>
  )
}

export function PortList({ ports = [] }) {
  if (!ports.length) return <span className="muted">-</span>
  return <span className="port-list">{ports.map((port) => <code key={port}>{port}</code>)}</span>
}
