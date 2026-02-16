import type { IRpcError, RpcParams } from './rpc'

export interface IApiServerErrorParams {
  method: string
  params: RpcParams
  status: number
  headers?: Record<string, string>
}

export interface IApiRpcErrorParams {
  method: string
  params: RpcParams
  error: IRpcError
  headers?: Record<string, string>
}

export class ApiRpcError extends Error {
  __type = 'ApiRpcError' as const
  method: string
  params: RpcParams
  code: number | null
  data: unknown
  headers?: Record<string, string>

  constructor({ method, params, error }: IApiRpcErrorParams) {
    const message = error.message || `Method "${method}" returned code: ${error?.code ?? 'missing'}`

    super(message)

    this.name = 'ApiRpcError'
    this.method = method
    this.params = params
    this.code = error.code
    this.data = error.data
  }
}

export class ApiServerError extends Error {
  __type = 'ApiServerError' as const
  method: string
  params: RpcParams
  status: number

  constructor({ method, params, status }: IApiServerErrorParams) {
    const message = `Method "${method}" returned status ${status}`
    super(message)

    this.name = 'ApiServerError'
    this.method = method
    this.params = params
    this.status = status
  }
}

export class ApiConnectionError extends Error {
  __type = 'ApiConnectionError' as const
  event: object

  constructor(event: any) {
    super()
    this.name = 'ApiConnectionError'
    this.message = 'Api connection error'
    this.event = event
  }
}
