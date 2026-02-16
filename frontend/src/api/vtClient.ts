let rpcId = 0

const AUTH_KEY = 'vt_auth_key'

export class RpcError extends Error {
  code: number
  data?: unknown

  constructor(code: number, message: string, data?: unknown) {
    super(message)
    this.code = code
    this.data = data
  }
}

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

export async function send<T>(method: string, params?: Record<string, unknown>): Promise<T> {
  const id = ++rpcId
  const body = JSON.stringify({
    jsonrpc: '2.0',
    id,
    method,
    params: params ?? {},
  })

  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const authKey = getAuthKey()
  if (authKey) {
    headers['Authorization2'] = authKey
  }

  const res = await fetch('/v1/vt/', {
    method: 'POST',
    headers,
    body,
  })

  if (!res.ok) {
    if (res.status === 401) {
      setAuthKey(null)
      window.location.href = '/vt/login'
    }
    throw new RpcError(res.status, `HTTP ${res.status}`)
  }

  const json = await res.json()

  if (json.error) {
    if (json.error.code === 401) {
      setAuthKey(null)
      window.location.href = '/vt/login'
    }
    throw new RpcError(json.error.code, json.error.message, json.error.data)
  }

  return json.result as T
}
