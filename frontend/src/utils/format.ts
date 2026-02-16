export function timeAgo(dateStr: string): string {
  const now = Date.now()
  const date = new Date(dateStr).getTime()
  const diff = now - date

  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (days > 30) return formatDate(dateStr)
  if (days > 0) return `${days}d ago`
  if (hours > 0) return `${hours}h ago`
  if (minutes > 0) return `${minutes}m ago`
  return 'just now'
}

export function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

export function formatDateTime(dateStr: string): string {
  return new Date(dateStr).toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  const remainSeconds = seconds % 60
  return `${minutes}m ${remainSeconds}s`
}

export function formatCost(usd: number): string {
  return `$${usd.toFixed(4)}`
}

export function shortHash(hash: string): string {
  return hash.substring(0, 7)
}

export function reviewTypeLabel(rt: string): string {
  const map: Record<string, string> = {
    architecture: 'A',
    code: 'C',
    security: 'S',
    tests: 'T',
  }
  return map[rt] ?? rt.charAt(0).toUpperCase()
}

export function reviewTypeFullName(rt: string): string {
  const map: Record<string, string> = {
    architecture: 'Architecture',
    code: 'Code',
    security: 'Security',
    tests: 'Tests',
  }
  return map[rt] ?? rt
}

const severityOrder: Record<string, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
}

export function compareSeverity(a: string, b: string): number {
  return (severityOrder[a] ?? 99) - (severityOrder[b] ?? 99)
}
