const AUTH_KEY = 'vt_auth_key'

export function getAuthKey(): string | null {
  return localStorage.getItem(AUTH_KEY)
}

export function setAuthKey(key: string | null) {
  if (key) {
    localStorage.setItem(AUTH_KEY, key)
  } else {
    localStorage.removeItem(AUTH_KEY)
  }
}
