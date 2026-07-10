import React from 'react'
import { getDashboard, getProject, operate, saveProject, deleteProject, scanNow } from './api.js'
import { ConfirmDialog, Toast } from './components/Dialog.jsx'
import { ProjectDetail } from './components/ProjectDetail.jsx'
import { ProjectEditor } from './components/ProjectEditor.jsx'
import { Shell } from './components/Shell.jsx'
import { Deployment } from './pages/Deployment.jsx'
import { Environment } from './pages/Environment.jsx'
import { Logs } from './pages/Logs.jsx'
import { Overview } from './pages/Overview.jsx'
import { Processes } from './pages/Processes.jsx'
import { Projects } from './pages/Projects.jsx'
import { Settings } from './pages/Settings.jsx'

export default function App() {
  const [page, setPage] = React.useState('overview')
  const [dashboard, setDashboard] = React.useState(null)
  const [loading, setLoading] = React.useState(true)
  const [refreshToken, setRefreshToken] = React.useState(0)
  const [toast, setToast] = React.useState(null)
  const [detail, setDetail] = React.useState(null)
  const [editor, setEditor] = React.useState(null)
  const [confirm, setConfirm] = React.useState(null)
  const [busy, setBusy] = React.useState(false)

  const showError = React.useCallback((error) => setToast({ type: 'error', message: error?.message ?? String(error) }), [])
  const showSuccess = React.useCallback((message) => setToast({ type: 'success', message }), [])

  const load = React.useCallback(async (scan = false) => {
    setLoading(true)
    try {
      const result = scan ? await scanNow() : await getDashboard()
      setDashboard(result)
      setRefreshToken((value) => value + 1)
    } catch (error) { showError(error) } finally { setLoading(false) }
  }, [showError])

  React.useEffect(() => { load(false) }, [load])

  const openProject = async (code) => {
    setBusy(true)
    try { setDetail(await getProject(code)) } catch (error) { showError(error) } finally { setBusy(false) }
  }

  const editProject = async (code) => {
    if (!code) { setEditor({ project: null }); return }
    setBusy(true)
    try {
      const result = await getProject(code)
      setDetail(null)
      setEditor({ project: result.project })
    } catch (error) { showError(error) } finally { setBusy(false) }
  }

  const persistProject = async (project) => {
    setBusy(true)
    try {
      await saveProject(project)
      setEditor(null)
      showSuccess(`项目 ${project.name} 已保存`)
      await load(false)
    } catch (error) { showError(error) } finally { setBusy(false) }
  }

  const requestOperation = (type, project) => {
    if (type === 'start') { executeOperation(type, project); return }
    setConfirm({
      type, project, name: project.name, wholeProject: true,
      impact: type === 'stop' ? `将停止 ${project.backends} 个后端和 ${project.frontends} 个前端配置。` : `将先停止再启动 ${project.name}。`,
      detail: `已识别端口：${project.ports?.join(', ') || '无'}。共享 Nginx 不会被直接关闭。`,
    })
  }

  const executeOperation = async (type, project) => {
    setBusy(true)
    try {
      await operate(type, project.code)
      setConfirm(null)
      setDetail(null)
      showSuccess(`${project.name} ${type === 'stop' ? '已停止' : type === 'start' ? '已启动' : '已重启'}`)
      await load(false)
    } catch (error) { showError(error) } finally { setBusy(false) }
  }

  const requestDelete = (project) => {
    setConfirm({ type: 'delete', project, name: project.name, wholeProject: true, impact: '将删除项目管理配置、投放文件、项目日志和备份。', detail: '外部应用目录不会被删除。' })
  }

  const executeConfirm = async () => {
    if (confirm.type !== 'delete') { await executeOperation(confirm.type, confirm.project); return }
    setBusy(true)
    try {
      await deleteProject(confirm.project.code)
      setConfirm(null)
      setDetail(null)
      showSuccess(`项目 ${confirm.project.name} 已删除`)
      await load(false)
    } catch (error) { showError(error) } finally { setBusy(false) }
  }

  const pageProps = { dashboard, projects: dashboard?.projects ?? [], loading, refreshToken, onError: showError, onToast: showSuccess }
  return (
    <Shell page={page} onNavigate={setPage} dashboard={dashboard} loading={loading} onRefresh={() => load(true)}>
      {page === 'overview' ? <Overview {...pageProps} onNavigate={setPage} onOpenProject={openProject} onEditProject={editProject} onOperate={requestOperation} /> : null}
      {page === 'projects' ? <Projects {...pageProps} onNew={() => editProject()} onOpen={openProject} onEdit={editProject} onOperate={requestOperation} /> : null}
      {page === 'processes' ? <Processes {...pageProps} /> : null}
      {page === 'deployment' ? <Deployment {...pageProps} /> : null}
      {page === 'environment' ? <Environment {...pageProps} /> : null}
      {page === 'logs' ? <Logs {...pageProps} /> : null}
      {page === 'settings' ? <Settings {...pageProps} /> : null}
      {detail ? <ProjectDetail detail={detail} onClose={() => setDetail(null)} onEdit={editProject} onOperate={requestOperation} onDelete={requestDelete} /> : null}
      {editor ? <ProjectEditor initialProject={editor.project} onClose={() => setEditor(null)} onSave={persistProject} busy={busy} /> : null}
      {confirm ? <ConfirmDialog action={confirm} onClose={() => setConfirm(null)} onConfirm={executeConfirm} busy={busy} /> : null}
      <Toast toast={toast} onClose={() => setToast(null)} />
    </Shell>
  )
}
