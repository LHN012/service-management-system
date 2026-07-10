import React from 'react'
import { Archive, Box, FileCog, Plus, Save, Server, Trash2 } from 'lucide-react'
import { Dialog } from './Dialog.jsx'
import { emptyProject } from '../mockData.js'

const tabs = [
  { id: 'basic', label: '基础信息', icon: FileCog },
  { id: 'backends', label: '后端', icon: Server },
  { id: 'frontends', label: '前端 Nginx', icon: Box },
  { id: 'deploy', label: '部署规则', icon: Archive },
]

export function ProjectEditor({ initialProject, onClose, onSave, busy }) {
  const [activeTab, setActiveTab] = React.useState('basic')
  const [project, setProject] = React.useState(() => structuredClone(initialProject ?? emptyProject()))
  const [error, setError] = React.useState('')
  const editing = Boolean(initialProject?.code)

  const update = (field, value) => setProject((current) => ({ ...current, [field]: value }))
  const updateItem = (collection, index, field, value) => setProject((current) => ({
    ...current,
    [collection]: current[collection].map((item, itemIndex) => itemIndex === index ? { ...item, [field]: value } : item),
  }))
  const removeItem = (collection, index) => setProject((current) => ({ ...current, [collection]: current[collection].filter((_item, itemIndex) => itemIndex !== index) }))

  const submit = () => {
    if (!project.code.trim() || !project.name.trim()) {
      setError('项目代码和名称不能为空。')
      setActiveTab('basic')
      return
    }
    if (!/^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(project.code)) {
      setError('项目代码只能包含字母、数字、点、下划线和连字符。')
      setActiveTab('basic')
      return
    }
    setError('')
    onSave(project)
  }

  return (
    <Dialog
      title={editing ? `编辑 ${initialProject.name}` : '新建项目'}
      description="配置项目、进程和部署规则"
      onClose={onClose}
      size="large"
      footer={<><button className="button button--secondary" onClick={onClose} disabled={busy}>取消</button><button className="button button--primary" onClick={submit} disabled={busy}><Save size={16} />{busy ? '保存中...' : '保存项目'}</button></>}
    >
      <div className="editor-layout">
        <nav className="editor-tabs" aria-label="项目配置分类">
          {tabs.map(({ id, label, icon: Icon }) => (
            <button key={id} className={activeTab === id ? 'active' : ''} onClick={() => setActiveTab(id)}><Icon size={16} />{label}<span>{tabCount(id, project)}</span></button>
          ))}
        </nav>
        <div className="editor-content">
          {error ? <div className="form-error">{error}</div> : null}
          {activeTab === 'basic' ? <BasicFields project={project} update={update} editing={editing} /> : null}
          {activeTab === 'backends' ? (
            <CollectionEditor
              title="后端进程" description="Java、Python、Node 或其他命令行服务"
              items={project.backends} onAdd={() => update('backends', [...project.backends, newBackend(project.backends.length)])}
              onRemove={(index) => removeItem('backends', index)}
              renderItem={(item, index) => <BackendFields item={item} onChange={(field, value) => updateItem('backends', index, field, value)} />}
            />
          ) : null}
          {activeTab === 'frontends' ? (
            <CollectionEditor
              title="前端 Nginx" description="支持共享实例和项目独占实例"
              items={project.frontends} onAdd={() => update('frontends', [...project.frontends, newFrontend(project.frontends.length)])}
              onRemove={(index) => removeItem('frontends', index)}
              renderItem={(item, index) => <FrontendFields item={item} onChange={(field, value) => updateItem('frontends', index, field, value)} />}
            />
          ) : null}
          {activeTab === 'deploy' ? (
            <CollectionEditor
              title="部署规则" description="统一投放区中的文件、目录和压缩包"
              items={project.deployRules} onAdd={() => update('deployRules', [...project.deployRules, newRule(project.deployRules.length)])}
              onRemove={(index) => removeItem('deployRules', index)}
              renderItem={(item, index) => <DeployFields item={item} onChange={(field, value) => updateItem('deployRules', index, field, value)} />}
            />
          ) : null}
        </div>
      </div>
    </Dialog>
  )
}

function BasicFields({ project, update, editing }) {
  return (
    <div className="form-section">
      <div className="form-section__heading"><h3>基础信息</h3><p>项目代码保存后不可修改。</p></div>
      <div className="field-grid field-grid--two">
        <Field label="项目代码" required><input value={project.code} onChange={(event) => update('code', event.target.value)} disabled={editing} placeholder="demo" /></Field>
        <Field label="项目名称" required><input value={project.name} onChange={(event) => update('name', event.target.value)} placeholder="示例项目" /></Field>
        <Field label="管理模式"><select value={project.manageMode} onChange={(event) => update('manageMode', event.target.value)}><option value="external">外部管理</option><option value="internal">内部管理</option></select></Field>
        <Field label="描述" wide><input value={project.description ?? ''} onChange={(event) => update('description', event.target.value)} placeholder="项目用途或负责人" /></Field>
      </div>
    </div>
  )
}

function CollectionEditor({ title, description, items, onAdd, onRemove, renderItem }) {
  return (
    <div className="form-section">
      <div className="form-section__heading form-section__heading--action"><div><h3>{title}</h3><p>{description}</p></div><button className="button button--secondary button--small" onClick={onAdd}><Plus size={15} />添加</button></div>
      <div className="config-list">
        {items.length === 0 ? <div className="empty-config">尚未配置{title}。</div> : null}
        {items.map((item, index) => (
          <section className="config-item" key={`${item.name}-${index}`}>
            <header><span>{index + 1}</span><strong>{item.name || `未命名${title}`}</strong><button className="icon-button" onClick={() => onRemove(index)} title="删除"><Trash2 size={15} /></button></header>
            {renderItem(item, index)}
          </section>
        ))}
      </div>
    </div>
  )
}

function BackendFields({ item, onChange }) {
  return (
    <div className="field-grid field-grid--two compact-fields">
      <Field label="名称"><input value={item.name} onChange={(event) => onChange('name', event.target.value)} /></Field>
      <Field label="运行环境"><select value={item.runtime ?? 'java'} onChange={(event) => onChange('runtime', event.target.value)}><option value="java">Java</option><option value="python">Python</option><option value="node">Node</option><option value="other">其他</option></select></Field>
      <Field label="工作目录" wide><input value={item.workDir} onChange={(event) => { onChange('workDir', event.target.value); onChange('match', { ...(item.match ?? {}), cwd: event.target.value }) }} placeholder="D:\apps\demo" /></Field>
      <Field label="启动命令" wide><input value={item.startCommand} onChange={(event) => onChange('startCommand', event.target.value)} placeholder="java -jar demo.jar" /></Field>
      <Field label="命令匹配"><input value={item.match?.commandContains ?? ''} onChange={(event) => onChange('match', { ...(item.match ?? {}), commandContains: event.target.value })} placeholder="demo.jar" /></Field>
      <Field label="预期端口"><input value={(item.expectedPorts ?? []).join(', ')} onChange={(event) => onChange('expectedPorts', parsePortList(event.target.value))} placeholder="8080" /></Field>
      <Field label="健康检查" wide><input value={item.healthCheck ?? ''} onChange={(event) => onChange('healthCheck', event.target.value)} placeholder="http://127.0.0.1:8080/health" /></Field>
    </div>
  )
}

function FrontendFields({ item, onChange }) {
  return (
    <div className="field-grid field-grid--two compact-fields">
      <Field label="名称"><input value={item.name} onChange={(event) => onChange('name', event.target.value)} /></Field>
      <Field label="Nginx 模式"><select value={item.nginxMode} onChange={(event) => onChange('nginxMode', event.target.value)}><option value="shared">共享实例</option><option value="dedicated">独占实例</option></select></Field>
      <Field label="站点目录" wide><input value={item.rootDir} onChange={(event) => onChange('rootDir', event.target.value)} /></Field>
      <Field label="配置文件" wide><input value={item.nginxConf} onChange={(event) => onChange('nginxConf', event.target.value)} /></Field>
      <Field label="Reload 命令"><input value={item.reloadCommand ?? ''} onChange={(event) => onChange('reloadCommand', event.target.value)} /></Field>
      <Field label="预期端口"><input value={(item.expectedPorts ?? []).join(', ')} onChange={(event) => onChange('expectedPorts', parsePortList(event.target.value))} /></Field>
    </div>
  )
}

function DeployFields({ item, onChange }) {
  return (
    <div className="field-grid field-grid--two compact-fields">
      <Field label="规则名称"><input value={item.name} onChange={(event) => onChange('name', event.target.value)} /></Field>
      <Field label="类型"><select value={item.type} onChange={(event) => onChange('type', event.target.value)}><option value="file">文件</option><option value="directory">目录</option><option value="archive">压缩包</option></select></Field>
      <Field label="投放区源路径" wide><input value={item.source} onChange={(event) => onChange('source', event.target.value)} placeholder="deploy-files/demo.jar" /></Field>
      <Field label="目标目录" wide><input value={item.targetDir} onChange={(event) => onChange('targetDir', event.target.value)} placeholder="D:\apps\demo" /></Field>
      {item.type === 'file' ? <Field label="目标文件名"><input value={item.targetName ?? ''} onChange={(event) => onChange('targetName', event.target.value)} /></Field> : null}
      {item.type === 'archive' ? <Field label="外层目录处理"><select value={item.stripTopLevel ?? 'auto'} onChange={(event) => onChange('stripTopLevel', event.target.value)}><option value="auto">自动识别</option><option value="always">始终去除</option><option value="never">始终保留</option></select></Field> : null}
      <Field label="覆盖前备份"><span className="toggle-row"><input type="checkbox" checked={item.backup} onChange={(event) => onChange('backup', event.target.checked)} /><span className="toggle" />启用</span></Field>
    </div>
  )
}

function Field({ label, required, wide, children }) {
  return <label className={`field${wide ? ' field--wide' : ''}`}><span>{label}{required ? <b>*</b> : null}</span>{children}</label>
}

function tabCount(tab, project) {
  if (tab === 'backends') return project.backends.length
  if (tab === 'frontends') return project.frontends.length
  if (tab === 'deploy') return project.deployRules.length
  return ''
}

function parsePortList(value) {
  return value.split(',').map((item) => Number.parseInt(item.trim(), 10)).filter((port) => Number.isInteger(port) && port > 0 && port < 65536)
}

function newBackend(index) {
  return { name: `backend-${index + 1}`, runtime: 'java', workDir: '', startCommand: '', stopMode: 'graceful', expectedPorts: [], healthCheck: '', match: { commandContains: '', cwd: '' } }
}

function newFrontend(index) {
  return { name: `frontend-${index + 1}`, nginxMode: 'shared', rootDir: '', nginxConf: '', reloadCommand: 'nginx.exe -s reload', expectedPorts: [80] }
}

function newRule(index) {
  return { name: `rule-${index + 1}`, source: 'deploy-files/', targetDir: '', type: 'file', targetName: '', archiveFormat: 'auto', stripTopLevel: 'auto', replaceMode: 'entries', backup: true }
}
