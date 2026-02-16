export type RpcParams = Record<string, unknown> | unknown[] // or simple - object | unknown[]

export const JsonRpcErrorCode = {
  PARSE_ERROR: -32700,
  INVALID_REQUEST: -32600,
  METHOD_NOT_FOUND: -32601,
  INVALID_PARAMS: -32602,
  INTERNAL_ERROR: -32603,
} as const

export interface IRpcError {
  code: number
  data: string
  message: string
}

export interface IRpcRequest {
  jsonrpc: string
  id?: number | string | null
  method: string
  params: RpcParams
}

export interface IRpcResponse<T = unknown> {
  jsonrpc: string
  id: string | number | null
  result?: T
  error?: IRpcError
  extensions?: object
}
