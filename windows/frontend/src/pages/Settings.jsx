import React from 'react'
import { Save } from 'lucide-react'
import { getSettings, saveSettings } from '../api.js'

export function Settings({ onError, onToast }) {
  const [settings, setSettings] = React.useState(null)
  const [saving, setSaving] = React.useState(false)
  React.useEffect(() => { getSettings().then(setSettings).catch(onError) }, [onError])
  if (!settings) return <div className="loading-block">正在读取设置...</div>
  const update = (field, value) => setSettings((current) => ({ ...current, [field]: Number(value) }))
  const save = async () => {
    setSaving(true)
    try { await saveSettings(settings); onToast('设置已保存') } catch (error) { onError(error) } finally { setSaving(false) }
  }
  return (
    <section className="settings-page">
      <div className="section-heading section-heading--page"><div><h2>系统设置</h2><p>扫描、停止超时和日志保留策略</p></div><button className="button button--primary" onClick={save} disabled={saving}><Save size={16} />{saving ? '保存中...' : '保存设置'}</button></div>
      <div className="settings-group">
        <h3>运行策略</h3>
        <SettingRow title="扫描间隔" description="定时重建项目、进程和端口状态。"><div className="number-control"><input type="number" min="10" value={settings.scanIntervalSeconds} onChange={(event) => update('scanIntervalSeconds', event.target.value)} /><span>秒</span></div></SettingRow>
        <SettingRow title="优雅停止超时" description="超过该时间后允许用户选择强制结束进程。"><div className="number-control"><input type="number" min="1" value={settings.stopTimeoutSeconds} onChange={(event) => update('stopTimeoutSeconds', event.target.value)} /><span>秒</span></div></SettingRow>
        <SettingRow title="日志保留" description="超过保留期的历史日志可由清理任务归档。"><div className="number-control"><input type="number" min="1" value={settings.logRetentionDays} onChange={(event) => update('logRetentionDays', event.target.value)} /><span>天</span></div></SettingRow>
      </div>
      <div className="settings-group">
        <h3>Agent</h3>
        <SettingRow title="运行模式" description="当前版本由桌面应用进程提供本地 Agent 能力。"><span className="read-only-value">Desktop Agent</span></SettingRow>
        <SettingRow title="远程访问" description="管理 API 仅在应用进程内可用，不开放网络端口。"><span className="read-only-value">已关闭</span></SettingRow>
      </div>
    </section>
  )
}

function SettingRow({ title, description, children }) {
  return <div className="setting-row"><div><strong>{title}</strong><p>{description}</p></div>{children}</div>
}
