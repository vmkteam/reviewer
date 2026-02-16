import { ofetch } from 'ofetch'
import type { $Fetch } from 'ofetch'
import { withQuery } from 'ufo'
import { ApiServerError, ApiRpcError, ApiConnectionError } from './errors'
import { JsonRpcErrorCode } from './rpc'
import type { IRpcRequest, IRpcResponse, RpcParams } from './rpc'

function groupBy<T extends Record<string, any>>(arr: T[], key: keyof T): Record<string, T[]> {
  const result: Record<string, T[]> = {}
  for (const item of arr) {
    const k = String(item[key])
    if (!result[k]) result[k] = []
    result[k].push(item)
  }
  return result
}

export interface HttpRpcClientOptions {
  url: string
  user?: string
  password?: string
  isClient: boolean
}

interface PayloadLog {
  method: string
  params: RpcParams
  startTime: Date
  duration: number
  rpcResponse: IRpcResponse
  batch?: string
}

export interface ResponsePayloadLog extends PayloadLog {
  token?: string
  headers?: Record<string, string>
}

const HEADERS: Record<string, string> = {
  'Content-Type': 'application/json',
  'Authorization2': '',
  'Accept': 'application/json',
}

const isValidRpcResponse = (obj: any): obj is IRpcResponse => {
  if (typeof obj !== 'object' || obj === null) {
    return false
  }

  if (obj.jsonrpc !== '2.0') {
    return false
  }

  if (!(typeof obj.id === 'string' || typeof obj.id === 'number' || obj.id === null)) {
    return false
  }

  // Note: null is valid for jsonrpc. Check is strictly required
  if (obj.result === undefined && obj.error === undefined) {
    return false
  }

  if (obj.error !== undefined) {
    if (typeof obj.error !== 'object' || obj.error === null) {
      return false
    }
    // Note: Not checking code/message/data fields because they can be missing by server
  }

  return true
}

export default class HttpRpcClient {
  url: string
  batching: boolean
  ofetch: $Fetch
  refreshPromise: Promise<any> | null = null
  refreshedAt: Date | null = null
  refreshToken: ((data: { headers: Record<string, string>, body: IRpcRequest | IRpcRequest[] }) => Promise<any>) | undefined
  token: { value: string | undefined } | undefined
  user?: string
  password?: string
  allHeaders: Record<string, string>
  isClient: boolean
  nextIdRequest = 0

  constructor({ url, user, password, isClient }: HttpRpcClientOptions) {
    this.url = url
    this.batching = false
    this.ofetch = ofetch.create({
      timeout: 20000,
    })
    this.user = user
    this.password = password
    this.allHeaders = { ...HEADERS }
    this.isClient = isClient
  }

  get headers(): Record<string, string> {
    const result: Record<string, string> = { ...this.allHeaders }

    if (this.user) {
      result.Authorization = 'Basic ' + btoa(`${this.user}:${this.password}`)
    }

    if (this.token?.value) {
      result['Authorization2'] = this.token.value
    }

    return result
  }

  setHeader(key: string, value: string): void {
    this.allHeaders[key] = value
  }

  notify = async (method: string, params: RpcParams) => {
    return this.call(method, params, true)
  }

  call = async (method: string, params: RpcParams, notify = false) => {
    const rpcRequest = notify ? this.#createRpcNotify(method, params) : this.#createRpcRequest(method, params)

    if (this.batching) {
      return rpcRequest
    }

    const startTime = new Date()
    const startDuration = performance.now()

    const rpcResponse = await this.fetch(rpcRequest) as IRpcResponse

    const duration = Math.round(performance.now() - startDuration)

    this.#logToConsole(`${method} ${duration}ms`)
    this.#logToApiLogger({
      method,
      params,
      startTime,
      duration,
      rpcResponse,
    })

    if (rpcResponse.error) {
      const error = rpcResponse.error

      console.log('ApiRpcError', error)

      throw new ApiRpcError({
        method,
        params,
        error,
        headers: this.headers,
      })
    }

    return rpcResponse.result
  }

  async batch<T extends readonly unknown[] | []>(queue: () => T): Promise<{ -readonly [P in keyof T]: Awaited<T[P]> | null; }> {
    if (this.refreshPromise) {
      await this.refreshPromise
    }

    // eslint-disable-next-line no-async-promise-executor
    return new Promise(async (resolve, reject) => {
      try {
        this.batching = true
        const batch = await Promise.all(queue())
        this.batching = false

        const filteredBatch: IRpcRequest[] = batch.filter(Boolean) as IRpcRequest[]

        const startTime = new Date()
        const startDuration = performance.now()
        const responses = await this.fetch(filteredBatch) as IRpcResponse[]

        const resultDict = groupBy(responses, 'id')
        const errors = responses.filter(r => r.error)
        const duration = Math.round(performance.now() - startDuration)

        if (errors.length) {
          const firstError = errors[0]

          throw new ApiRpcError({
            method: firstError.id ? filteredBatch.filter(req => req.id === firstError.id)[0].method : filteredBatch.map(req => req.method).join(','),
            params: firstError.id ? filteredBatch.filter(req => req.id === firstError.id)[0].params : filteredBatch.map(req => req.params),
            error: firstError.error!,
            headers: this.headers,
          })
        }

        const result = batch.map((r) => {
          const id = (r as IRpcRequest)?.id

          if (!id || !resultDict[id]?.length) {
            return null
          }

          return resultDict[id][0].result
        })

        resolve(result as any) // { -readonly [P in keyof T]: Awaited<T[P]> | null; }

        const batchId = filteredBatch.map(({ id }) => id).join('-')

        filteredBatch.forEach((rpcRequest, index) => {
          const rpcResponse = rpcRequest.id ? resultDict[rpcRequest.id][0] : null

          if (!rpcResponse) {
            return
          }

          this.#logToApiLogger({
            method: rpcRequest.method,
            params: rpcRequest.params,
            startTime,
            duration: index === 0 ? duration : 0, // Only first value get duration in batch
            batch: batchId,
            rpcResponse,
          })
        })
      } catch (err) {
        reject(err)
      }
    })
  }

  async fetch(body: IRpcRequest | IRpcRequest[], refreshed = false): Promise<IRpcResponse | IRpcResponse[]> {
    const isArray = Array.isArray(body)
    this.#logToConsole('fetch', isArray ? 'batch' : body.method)

    if (this.refreshPromise) {
      try {
        await this.refreshPromise
      } catch (err) {
        this.#logToConsole('failed by refresh token operation', err)
      }
    }

    const queryParams = isArray
      ? { methods: body.map(request => request.method).join(',') }
      : { method: body.method }

    const url = withQuery(this.url, queryParams)

    try {
      const responseRaw = await this.ofetch.raw<IRpcResponse | IRpcResponse[] | string>(url, {
        method: 'POST',
        headers: this.headers,
        body: body,
      })

      const methodCurrent = isArray ? body.map(request => request.method).join(',') : body.method
      const paramsCurrent = isArray ? body.map(request => request.params) : body.params

      if (responseRaw.status >= 400) {
        this.#logToConsole('ApiServerError', {
          method: methodCurrent,
          params: paramsCurrent,
          status: responseRaw.status,
        })
        throw new ApiServerError({
          method: methodCurrent,
          params: paramsCurrent,
          status: responseRaw.status,
          headers: this.headers,
        })
      }

      // notify response
      if (responseRaw._data === '') {
        return []
      }

      if (responseRaw._data === undefined) {
        const isEmptyBatch = isArray && body.every(req => req.id == null)
        if (isEmptyBatch) {
          return []
        }
        const isEmpty = !isArray && body.id == null
        if (isEmpty) {
          return {} as IRpcResponse
        }

        throw new ApiRpcError({
          method: methodCurrent,
          params: paramsCurrent,
          error: {
            code: JsonRpcErrorCode.PARSE_ERROR,
            message: 'Invalid JSON-RPC response',
            data: JSON.stringify(responseRaw),
          },
        })
      }

      const response = responseRaw._data

      if (!isArray && !isValidRpcResponse(response)) {
        throw new ApiRpcError({
          method: methodCurrent,
          params: paramsCurrent,
          error: { code: JsonRpcErrorCode.PARSE_ERROR, message: 'Invalid JSON-RPC response', data: JSON.stringify(response) },
          headers: this.headers,
        })
      }

      if (isArray && !Array.isArray(response)) {
        throw new ApiRpcError({
          method: methodCurrent,
          params: paramsCurrent,
          error: {
            code: JsonRpcErrorCode.PARSE_ERROR,
            message: 'Invalid JSON-RPC response array structure',
            data: JSON.stringify(body),
          },
        })
      } else if (isArray) {
        const responses = response as IRpcResponse[]

        for (const singleResponse of responses) {
          if (!isValidRpcResponse(singleResponse)) {
            const id = (singleResponse as IRpcResponse)?.id
            throw new ApiRpcError({
              method: id ? body.filter(req => req.id === id)[0].method : methodCurrent,
              params: id ? body.filter(req => req.id === id)[0].params : paramsCurrent,
              error: {
                code: JsonRpcErrorCode.PARSE_ERROR,
                message: 'Invalid JSON-RPC response in batch',
                data: JSON.stringify(response),
              },
            })
          }
        }
      }

      const has401Error = isArray
        ? (response as IRpcResponse[]).some(r => r?.error?.code === 401)
        : (response as IRpcResponse).error?.code === 401

      if (has401Error && this.refreshToken && this.token?.value && !refreshed) {
        return this.#tryRefreshToken({
          headers: this.headers,
          body: body,
        })
      }

      return response as IRpcResponse | IRpcResponse[]
    } catch (err) {
      if (err instanceof ApiRpcError || err instanceof ApiServerError) {
        throw err
      }
      // Network errors / CORS / JSON parse errors / Timeout
      throw new ApiConnectionError(err)
    }
  }

  onResponse = (_payload: ResponsePayloadLog) => {}

  #tryRefreshToken = (data: { headers: Record<string, string>, body: IRpcRequest | IRpcRequest[] }): Promise<IRpcResponse | IRpcResponse[]> => {
    const method = Array.isArray(data.body) ? 'batch' : data.body.method
    if (this.refreshPromise) {
      this.#logToConsole('wait refresh', method)
      return this.refreshPromise.then(() => this.fetch(data.body, true))
    } else {
      this.#logToConsole('refresh', method)
      this.refreshPromise = new Promise((resolve, reject) => {
        this.refreshToken!(data)
          .then((token) => {
            if (this.token?.value) {
              this.token.value = token
            }

            if (!token && this.isClient) {
              window.location.reload()
            }
            this.refreshPromise = null
            this.refreshedAt = new Date()
          })
          .then(() => this.fetch(data.body, true))
          .then(resolve)
          .catch((err) => {
            this.#logToConsole('failed', method)
            this.refreshPromise = null
            this.refreshedAt = null
            reject(err)
          })
      })

      return this.refreshPromise
    }
  }

  #logToConsole = (message?: string, ...optionalParams: unknown[]) => {
    console.log(message, ...optionalParams)
  }

  #logToApiLogger = (payload: PayloadLog) => {
    if (payload.duration > 3000) {
      this.#logToConsole(`slow request ${payload.method}: ${payload.duration}`)
    }

    this.onResponse({
      ...payload,
      token: this.headers.Authorization2,
      headers: this.headers,
    })
  }

  #generateId = (): number => {
    this.nextIdRequest += 1

    return this.nextIdRequest
  }

  #createRpcRequest = (method: string, params: RpcParams = {}): IRpcRequest => {
    return {
      jsonrpc: '2.0',
      id: this.#generateId(),
      method,
      params,
    }
  }

  // https://www.jsonrpc.org/specification#notification
  #createRpcNotify = (method: string, params: RpcParams = {}): IRpcRequest => {
    return {
      jsonrpc: '2.0',
      method,
      params,
    }
  }
}
