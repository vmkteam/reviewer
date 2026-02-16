const dateOpts: Intl.DateTimeFormatOptions = {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
}

const dateTimeOpts: Intl.DateTimeFormatOptions = {
  ...dateOpts,
  hour: '2-digit',
  minute: '2-digit',
}

const reviewTypeLabels: Record<string, string> = {
  architecture: 'A',
  code: 'C',
  security: 'S',
  tests: 'T',
}

const reviewTypeFullNames: Record<string, string> = {
  architecture: 'Architecture',
  code: 'Code',
  security: 'Security',
  tests: 'Tests',
}

const severityOrder: Record<string, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
}

export function useFormat() {
  function timeAgo(dateStr: string): string {
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

  function formatDate(dateStr: string): string {
    return new Date(dateStr).toLocaleDateString('en-US', dateOpts)
  }

  function formatDateTime(dateStr: string): string {
    return new Date(dateStr).toLocaleString('en-US', dateTimeOpts)
  }

  function formatDuration(ms: number): string {
    const seconds = Math.floor(ms / 1000)
    if (seconds < 60) return `${seconds}s`
    const minutes = Math.floor(seconds / 60)
    const remainSeconds = seconds % 60
    return `${minutes}m ${remainSeconds}s`
  }

  function formatCost(usd: number): string {
    return `$${usd.toFixed(4)}`
  }

  function shortHash(hash: string): string {
    return hash.substring(0, 7)
  }

  function reviewTypeLabel(rt: string): string {
    return reviewTypeLabels[rt] ?? rt.charAt(0).toUpperCase()
  }

  function reviewTypeFullName(rt: string): string {
    return reviewTypeFullNames[rt] ?? rt
  }

  function compareSeverity(a: string, b: string): number {
    return (severityOrder[a] ?? 99) - (severityOrder[b] ?? 99)
  }

  function buildVcsMrURL(vcsURL: string, externalId: string): string {
    const isGitHub = vcsURL.includes('github.com')
    return isGitHub
      ? `${vcsURL}/pull/${externalId}`
      : `${vcsURL}/-/merge_requests/${externalId}`
  }

  function buildVcsCommitURL(vcsURL: string, commitHash: string): string {
    const isGitHub = vcsURL.includes('github.com')
    return isGitHub
      ? `${vcsURL}/commit/${commitHash}`
      : `${vcsURL}/-/commit/${commitHash}`
  }

  function buildVcsFileURL(vcsURL: string, commitHash: string, file: string, lines?: string): string {
    const isGitHub = vcsURL.includes('github.com')
    const base = isGitHub
      ? `${vcsURL}/blob/${commitHash}/${file}`
      : `${vcsURL}/-/blob/${commitHash}/${file}`

    if (!lines) return base

    const parts = lines.split('-')
    const start = parts[0]
    const end = parts[1]

    if (isGitHub) {
      return end ? `${base}#L${start}-L${end}` : `${base}#L${start}`
    }
    return end ? `${base}#L${start}-${end}` : `${base}#L${start}`
  }

  return {
    timeAgo,
    formatDate,
    formatDateTime,
    formatDuration,
    formatCost,
    shortHash,
    reviewTypeLabel,
    reviewTypeFullName,
    compareSeverity,
    buildVcsMrURL,
    buildVcsCommitURL,
    buildVcsFileURL,
  }
}
