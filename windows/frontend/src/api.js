import { emptyProject, mockDashboard, mockEnvironment, mockProcesses, mockProjectDetails, mockProjects } from './mockData.js'

const delay = (value, ms = 180) => new Promise((resolve) => window.setTimeout(() => resolve(value), ms))

const backend = () => window.go?.main?.App
let dashboardState = structuredClone(mockDashboard)
let projectState = structuredClone(mockProjectDetails)
let settingsState = { scanIntervalSeconds: 60, stopTimeoutSeconds: 15, logRetentionDays: 30 }

async function call(name, ...args) {
  const service = backend()
  if (!service?.[name]) return null
  return service[name](...args)
}

export async function getDashboard() {
  return (await call('GetDashboard')) ?? delay(structuredClone(dashboardState))
}

export async function scanNow() {
  const live = await call('Scan')
  if (live) return live
  dashboardState.agent.lastScan = new Date().toISOString()
  dashboardState.recent.unshift({ time: new Date().toISOString(), action: 'scan', target: '全部项目', result: 'success' })
  return delay(structuredClone(dashboardState), 420)
}

export async function getProject(code) {
  const live = await call('GetProject', code)
  if (live) return live
  const project = projectState[code] ?? { ...emptyProject(), ...mockProjects.find((item) => item.code === code) }
  return delay({ project: structuredClone(project), runtime: { projectCode: code, lastScanAt: new Date().toISOString(), processes: [] } })
}

export async function saveProject(project) {
  const live = await call('SaveProject', project)
  if (live) return live
  projectState[project.code] = structuredClone(project)
  const summary = {
    code: project.code, name: project.name, manageMode: project.manageMode, status: 'stopped',
    backends: project.backends?.length ?? 0, runningBackends: 0, frontends: project.frontends?.length ?? 0,
    runningFrontends: 0, ports: [], lastScanAt: new Date().toISOString(),
  }
  const existing = dashboardState.projects.findIndex((item) => item.code === project.code)
  if (existing >= 0) dashboardState.projects[existing] = summary
  else dashboardState.projects.push(summary)
  return delay({ project: structuredClone(project), runtime: { projectCode: project.code, processes: [] } }, 320)
}

export async function deleteProject(code) {
  const live = await call('DeleteProject', code)
  if (live !== null) return live
  dashboardState.projects = dashboardState.projects.filter((item) => item.code !== code)
  delete projectState[code]
  return delay(true)
}

export async function operate(action, target, force = false) {
  const names = { start: 'StartTarget', stop: 'StopTarget', restart: 'RestartTarget' }
  const live = await call(names[action], target, force)
  if (live) return live
  const projectCode = target.split('-')[0]
  const summary = dashboardState.projects.find((item) => item.code === projectCode)
  if (summary) {
    summary.status = action === 'stop' ? 'stopped' : 'running'
    summary.runningBackends = action === 'stop' ? 0 : summary.backends
    summary.runningFrontends = action === 'stop' ? 0 : summary.frontends
    summary.ports = action === 'stop' ? [] : structuredClone(mockProjects.find((item) => item.code === projectCode)?.ports ?? [])
  }
  dashboardState.recent.unshift({ time: new Date().toISOString(), action, target, result: 'success' })
  return delay({ results: [{ name: target, type: 'project', action, status: action === 'stop' ? 'stopped' : 'running', message: '操作已完成' }], runtime: { projectCode, lastScanAt: new Date().toISOString(), processes: [] } }, 520)
}

export async function listProcesses() {
  return (await call('ListProcesses')) ?? delay(structuredClone(mockProcesses))
}

export async function getEnvironment() {
  return (await call('GetEnvironment')) ?? delay(structuredClone(mockEnvironment))
}

export async function getSettings() {
  return (await call('GetSettings')) ?? delay(structuredClone(settingsState))
}

export async function saveSettings(settings) {
  const live = await call('SaveSettings', settings)
  if (live !== null) return live
  settingsState = structuredClone(settings)
  return delay(true)
}

export async function readLog(kind, project = '', limit = 500) {
  const live = await call('ReadLog', kind, project, limit)
  if (live) return live
  return delay([
    '2026-07-10T15:40:03+08:00 INFO  desktop-agent started',
    '2026-07-10T15:40:04+08:00 INFO  process scan completed projects=3 processes=4',
    '2026-07-10T15:47:12+08:00 INFO  project=portal action=restart result=success',
    '2026-07-10T15:52:28+08:00 INFO  project=billing action=deploy rule=api-jar result=success',
  ])
}

export async function prepareDeploy(projectCode, ruleName) {
  const live = await call('PrepareDeploy', projectCode, ruleName)
  if (live) return live
  const project = projectState[projectCode] ?? mockProjectDetails.billing
  const rule = project.deployRules?.find((item) => item.name === ruleName)
  if (!rule) throw new Error('未找到部署规则')
  const name = rule.targetName || rule.source.split('/').pop().replace(/\.zip$/i, '')
  const sourcePath = `C:\\ProgramData\\ServiceManagementSystem\\data\\projects\\${projectCode}\\${rule.source.replaceAll('/', '\\')}`
  return delay({
    id: `mock-${Date.now()}`, projectCode, rule: ruleName, sourcePath,
    contentRoot: rule.type === 'archive' ? `C:\\ProgramData\\ServiceManagementSystem\\tmp\\deploy\\${projectCode}\\extracted` : sourcePath,
    defaultBackup: rule.backup, changes: [{ action: 'replace', source: rule.source, target: `${rule.targetDir}\\${name}` }],
  })
}

export async function applyDeploy(id, backup) {
  const live = await call('ApplyDeploy', id, backup)
  if (live) return live
  return delay({ rule: id, changes: 1, backupPaths: backup ? ['C:\\ProgramData\\ServiceManagementSystem\\data\\backups\\latest.zip'] : [] }, 620)
}

export async function cancelDeploy(id) {
  const live = await call('CancelDeploy', id)
  return live ?? true
}
