function escapeHtml(str: string): string {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

function isGitHub(trackerURL: string): boolean {
  return trackerURL.includes('github.com')
}

/** Build full task URL from tracker base URL and task ID. */
export function buildTaskURL(trackerURL: string, taskId: string): string {
  const base = trackerURL.replace(/\/+$/, '')
  if (isGitHub(trackerURL)) {
    return `${base}/issues/${taskId}`
  }
  return `${base}/issue/${taskId}`
}

/** Get regex pattern for task IDs based on tracker type. */
export function getTaskPattern(trackerURL: string): RegExp {
  if (isGitHub(trackerURL)) {
    return /#(\d+)\b/g
  }
  return /\b([A-Z]{2,}-\d+)\b/g
}

/** Replace task IDs in plain text with clickable links. Text is HTML-escaped first. */
export function linkifyTaskIds(text: string, trackerURL: string | null): string {
  if (!trackerURL || !text) return escapeHtml(text ?? '')
  const escaped = escapeHtml(text)
  if (isGitHub(trackerURL)) {
    return escaped.replace(/#(\d+)\b/g, (_match, num) => {
      const url = buildTaskURL(trackerURL, num)
      return `<a href="${url}" target="_blank" class="task-link" onclick="event.stopPropagation()">#${num}</a>`
    })
  }
  return escaped.replace(/\b([A-Z]{2,}-\d+)\b/g, (_match, id) => {
    const url = buildTaskURL(trackerURL, id)
    return `<a href="${url}" target="_blank" class="task-link" onclick="event.stopPropagation()">${id}</a>`
  })
}
