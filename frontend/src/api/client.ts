let rpcId = 0

export class RpcError extends Error {
  code: number
  data?: unknown

  constructor(code: number, message: string, data?: unknown) {
    super(message)
    this.code = code
    this.data = data
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

  const res = await fetch('/v1/rpc/', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
  })

  if (!res.ok) {
    throw new RpcError(res.status, `HTTP ${res.status}`)
  }

  const json = await res.json()

  if (json.error) {
    throw new RpcError(json.error.code, json.error.message, json.error.data)
  }

  return json.result as T
}
