export const mockProjects = [
  {
    code: 'billing', name: '计费中心', manageMode: 'external', status: 'running',
    backends: 3, runningBackends: 3, frontends: 1, runningFrontends: 1, ports: [8081, 8082, 8083],
    lastScanAt: new Date().toISOString(),
  },
  {
    code: 'portal', name: '运营门户', manageMode: 'internal', status: 'partial',
    backends: 2, runningBackends: 1, frontends: 1, runningFrontends: 1, ports: [80, 3100],
    lastScanAt: new Date(Date.now() - 45000).toISOString(),
  },
  {
    code: 'reporting', name: '报表服务', manageMode: 'external', status: 'stopped',
    backends: 1, runningBackends: 0, frontends: 0, runningFrontends: 0, ports: [],
    lastScanAt: new Date(Date.now() - 120000).toISOString(),
  },
]

export const mockEnvironment = [
  { name: 'PowerShell', status: 'ready', path: 'C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe', required: true },
  { name: 'Java', status: 'ready', path: 'D:\\runtime\\jdk-17\\bin\\java.exe', required: false },
  { name: 'Python', status: 'ready', path: 'D:\\runtime\\python\\python.exe', required: false },
  { name: 'Node', status: 'ready', path: 'D:\\runtime\\node\\node.exe', required: false },
  { name: 'Nginx', status: 'missing', path: '', required: false },
  { name: 'Tar', status: 'ready', path: 'C:\\Windows\\System32\\tar.exe', required: false },
  { name: '7-Zip', status: 'ready', path: 'C:\\Program Files\\7-Zip\\7z.exe', required: false },
]

export const mockProcesses = [
  { pid: 8140, name: 'java.exe', command: 'java -jar billing-api.jar --server.port=8081', cwd: 'D:\\apps\\billing', user: 'svc-app', ports: [8081] },
  { pid: 11328, name: 'java.exe', command: 'java -jar billing-worker.jar --server.port=8082', cwd: 'D:\\apps\\billing', user: 'svc-app', ports: [8082] },
  { pid: 12944, name: 'node.exe', command: 'node server.js --port 3100', cwd: 'D:\\apps\\portal', user: 'deploy', ports: [3100] },
  { pid: 4768, name: 'nginx.exe', command: 'nginx.exe -p D:\\tools\\nginx', cwd: 'D:\\tools\\nginx', user: 'SYSTEM', ports: [80] },
]

export const mockRecent = [
  { time: new Date(Date.now() - 60000).toISOString(), action: 'scan', target: '全部项目', result: 'success' },
  { time: new Date(Date.now() - 420000).toISOString(), action: 'restart', target: 'portal-api', result: 'success' },
  { time: new Date(Date.now() - 1200000).toISOString(), action: 'deploy', target: 'billing', result: 'success' },
  { time: new Date(Date.now() - 4200000).toISOString(), action: 'stop', target: 'reporting', result: 'success' },
]

export const mockProjectDetails = {
  billing: {
    code: 'billing', name: '计费中心', manageMode: 'external', description: '账单、对账与支付回调服务',
    backends: [
      { name: 'api', runtime: 'java', workDir: 'D:\\apps\\billing', startCommand: 'java -jar billing-api.jar --server.port=8081', stopMode: 'graceful', expectedPorts: [8081], healthCheck: 'http://127.0.0.1:8081/actuator/health', match: { commandContains: 'billing-api.jar', cwd: 'D:\\apps\\billing' } },
      { name: 'worker', runtime: 'java', workDir: 'D:\\apps\\billing', startCommand: 'java -jar billing-worker.jar --server.port=8082', stopMode: 'graceful', expectedPorts: [8082], match: { commandContains: 'billing-worker.jar', cwd: 'D:\\apps\\billing' } },
    ],
    frontends: [{ name: 'web', nginxMode: 'shared', rootDir: 'D:\\apps\\billing\\web', nginxConf: 'D:\\tools\\nginx\\conf\\billing.conf', reloadCommand: 'nginx.exe -s reload', expectedPorts: [80] }],
    deployRules: [
      { name: 'api-jar', source: 'deploy-files/billing-api.jar', targetDir: 'D:\\apps\\billing', targetName: 'billing-api.jar', type: 'file', backup: true },
      { name: 'web-package', source: 'deploy-files/web-dist.zip', targetDir: 'D:\\apps\\billing\\web', type: 'archive', archiveFormat: 'auto', stripTopLevel: 'auto', replaceMode: 'entries', backup: true },
    ],
  },
}

export const mockDashboard = {
  agent: { status: 'running', mode: 'desktop-agent', startedAt: new Date(Date.now() - 86400000).toISOString(), lastScan: new Date().toISOString() },
  projects: mockProjects,
  environment: mockEnvironment,
  recent: mockRecent,
  processCount: mockProcesses.length,
  portCount: mockProcesses.reduce((total, process) => total + process.ports.length, 0),
  dataRoot: 'C:\\ProgramData\\ServiceManagementSystem',
}

export const emptyProject = () => ({
  code: '', name: '', manageMode: 'external', description: '', backends: [], frontends: [], deployRules: [],
})
