import React from 'react'
import { Archive, CheckCircle2, Eye, FolderInput, ShieldCheck } from 'lucide-react'
import { applyDeploy, cancelDeploy, getProject, prepareDeploy } from '../api.js'
import { Dialog } from '../components/Dialog.jsx'

export function Deployment({ projects, onToast, onError }) {
  const [projectCode, setProjectCode] = React.useState(projects[0]?.code ?? '')
  const [project, setProject] = React.useState(null)
  const [loading, setLoading] = React.useState(false)
  const [preview, setPreview] = React.useState(null)
  const [backup, setBackup] = React.useState(true)
  const [applying, setApplying] = React.useState(false)

  React.useEffect(() => {
    if (!projectCode && projects[0]) setProjectCode(projects[0].code)
  }, [projectCode, projects])

  React.useEffect(() => {
    if (!projectCode) { setProject(null); return undefined }
    let active = true
    setLoading(true)
    getProject(projectCode).then((detail) => { if (active) setProject(detail.project) }).catch(onError).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [projectCode, onError])

  const openPreview = async (rule) => {
    setLoading(true)
    try {
      const result = await prepareDeploy(projectCode, rule.name)
      setPreview(result)
      setBackup(result.defaultBackup)
    } catch (error) { onError(error) } finally { setLoading(false) }
  }

  const closePreview = async () => {
    if (preview?.id) await cancelDeploy(preview.id)
    setPreview(null)
  }

  const execute = async () => {
    setApplying(true)
    try {
      const result = await applyDeploy(preview.id, backup)
      onToast(`规则 ${preview.rule} 已完成，共替换 ${result.changes} 项`)
      setPreview(null)
    } catch (error) { onError(error) } finally { setApplying(false) }
  }

  return (
    <section className="work-section page-section">
      <div className="section-heading section-heading--page">
        <div><h2>部署迁移</h2><p>从项目统一投放区预览、备份并覆盖目标内容</p></div>
        <label className="select-field"><span>项目</span><select value={projectCode} onChange={(event) => setProjectCode(event.target.value)}>{projects.map((item) => <option key={item.code} value={item.code}>{item.name} ({item.code})</option>)}</select></label>
      </div>
      <div className="deploy-path-band"><FolderInput size={17} /><div><span>统一投放区</span><code>{projectCode ? `data\\projects\\${projectCode}\\deploy-files` : '-'}</code></div></div>
      <div className="deploy-list">
        {loading ? <div className="loading-block">正在读取部署规则...</div> : null}
        {!loading && !project?.deployRules?.length ? <div className="empty-config">当前项目没有部署规则。</div> : null}
        {!loading ? project?.deployRules?.map((rule) => (
          <article className="deploy-row" key={rule.name}>
            <span className="deploy-row__icon"><Archive size={18} /></span>
            <div className="deploy-row__name"><strong>{rule.name}</strong><small>{rule.type}</small></div>
            <div><span>源文件</span><code>{rule.source}</code></div>
            <div><span>目标目录</span><code>{rule.targetDir}</code></div>
            <div className="deploy-row__backup"><ShieldCheck size={14} />{rule.backup ? '默认备份' : '不备份'}</div>
            <button className="button button--secondary button--small" onClick={() => openPreview(rule)}><Eye size={15} />预览</button>
          </article>
        )) : null}
      </div>
      {preview ? (
        <Dialog title={`迁移预览 · ${preview.rule}`} description="确认实际内容根目录和覆盖范围" onClose={closePreview} size="large" footer={<><button className="button button--secondary" onClick={closePreview} disabled={applying}>取消</button><button className="button button--primary" onClick={execute} disabled={applying}><CheckCircle2 size={16} />{applying ? '正在迁移...' : '确认执行'}</button></>}>
          <div className="preview-paths"><label>源文件<code>{preview.sourcePath}</code></label><label>实际内容根目录<code>{preview.contentRoot}</code></label></div>
          <label className="backup-choice"><input type="checkbox" checked={backup} onChange={(event) => setBackup(event.target.checked)} /><span className="toggle" /><div><strong>覆盖前创建备份</strong><small>失败时使用本次快照恢复已经修改的条目</small></div></label>
          <div className="change-list"><header><span>动作</span><span>目标路径</span></header>{preview.changes.map((change, index) => <div key={`${change.target}-${index}`}><span className={`change-action change-action--${change.action}`}>{change.action === 'replace' ? '替换' : '新增'}</span><code>{change.target}</code></div>)}</div>
        </Dialog>
      ) : null}
    </section>
  )
}
