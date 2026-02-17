/**
 * Extracts a project title from a VCS URL.
 * For valid URLs: takes the last 2 path segments (org/repo), stripping .git suffix.
 * For invalid URLs: strips the protocol prefix.
 */
export function extractTitleFromVcsURL(url: string): string {
  try {
    const path = new URL(url).pathname.replace(/^\/|\/+$|\.git$/g, '')
    const parts = path.split('/')
    if (parts.length >= 2) {
      return parts.slice(-2).join('/')
    }
    return path
  } catch {
    return url.replace(/^https?:\/\//, '')
  }
}
