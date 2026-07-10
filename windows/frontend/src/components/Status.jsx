const labels = {
  running: '运行中', stopped: '已停止', partial: '部分运行', failed: '失败', unknown: '待确认',
  ready: '可用', missing: '未安装', skipped: '已跳过', superseded: '已处理',
}

export function Status({ value, compact = false }) {
  const normalized = value || 'unknown'
  return (
    <span className={`status status--${normalized}${compact ? ' status--compact' : ''}`}>
      <span className="status__dot" aria-hidden="true" />
      {compact ? null : <span>{labels[normalized] ?? normalized}</span>}
    </span>
  )
}

export function SkeletonRows({ rows = 4, columns = 5 }) {
  return Array.from({ length: rows }, (_, row) => (
    <tr key={row} className="skeleton-row">
      {Array.from({ length: columns }, (_item, column) => (
        <td key={column}><span className="skeleton-line" /></td>
      ))}
    </tr>
  ))
}
