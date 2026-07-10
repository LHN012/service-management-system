import React from 'react'
import { AlertTriangle, Check, X } from 'lucide-react'

export function Dialog({ title, description, children, onClose, footer, size = 'medium' }) {
  return (
    <div className="dialog-layer" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose?.()}>
      <section className={`dialog dialog--${size}`} role="dialog" aria-modal="true" aria-label={title}>
        <header className="dialog__header">
          <div><h2>{title}</h2>{description ? <p>{description}</p> : null}</div>
          <button className="icon-button" onClick={onClose} title="关闭" aria-label="关闭"><X size={18} /></button>
        </header>
        <div className="dialog__body">{children}</div>
        {footer ? <footer className="dialog__footer">{footer}</footer> : null}
      </section>
    </div>
  )
}

export function ConfirmDialog({ action, onClose, onConfirm, busy }) {
  const [step, setStep] = React.useState(1)
  const destructive = action?.type === 'stop' || action?.type === 'restart' || action?.type === 'delete'
  const needsDouble = action?.wholeProject && destructive
  const labels = { stop: '停止', restart: '重启', delete: '删除', deploy: '执行迁移' }
  const proceed = () => {
    if (needsDouble && step === 1) setStep(2)
    else onConfirm()
  }
  return (
    <Dialog
      title={`${labels[action?.type] ?? '确认'} ${action?.name ?? ''}`}
      description={needsDouble ? `高风险操作 · 第 ${step} 次确认，共 2 次` : '请确认本次操作的影响范围'}
      onClose={onClose}
      footer={<><button className="button button--secondary" onClick={onClose} disabled={busy}>取消</button><button className={`button ${destructive ? 'button--danger' : 'button--primary'}`} onClick={proceed} disabled={busy}>{busy ? '处理中...' : step === 1 && needsDouble ? '继续确认' : '确认执行'}</button></>}
    >
      <div className="confirm-impact">
        <AlertTriangle size={22} />
        <div>
          <strong>{action?.impact ?? '该操作将修改当前运行状态。'}</strong>
          <p>{action?.detail ?? '所有结果都会写入操作日志。'}</p>
        </div>
      </div>
      {step === 2 ? <label className="confirm-check"><Check size={16} />已确认项目、进程和端口影响范围</label> : null}
    </Dialog>
  )
}

export function Toast({ toast, onClose }) {
  React.useEffect(() => {
    if (!toast) return undefined
    const timer = window.setTimeout(onClose, 3600)
    return () => window.clearTimeout(timer)
  }, [toast, onClose])
  if (!toast) return null
  return <div className={`toast toast--${toast.type ?? 'success'}`}><span>{toast.message}</span><button onClick={onClose} aria-label="关闭通知"><X size={15} /></button></div>
}
