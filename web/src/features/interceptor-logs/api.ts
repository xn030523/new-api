import { api } from '@/lib/api'

import type {
  GetInterceptorLogsParams,
  GetInterceptorLogsResponse,
  GetInterceptorLogStatsResponse,
} from './types'

export async function getInterceptorLogs(
  params: GetInterceptorLogsParams = {}
): Promise<GetInterceptorLogsResponse> {
  const searchParams = new URLSearchParams()
  if (params.page) searchParams.set('page', String(params.page))
  if (params.page_size) searchParams.set('page_size', String(params.page_size))
  if (params.username) searchParams.set('username', params.username)
  if (params.channel_id) {
    searchParams.set('channel_id', String(params.channel_id))
  }
  if (params.action) searchParams.set('action', params.action)
  if (params.model_name) searchParams.set('model_name', params.model_name)
  if (params.rule_type) searchParams.set('rule_type', params.rule_type)
  if (params.start) searchParams.set('start', String(params.start))
  if (params.end) searchParams.set('end', String(params.end))

  const res = await api.get(`/api/interceptor/logs/?${searchParams.toString()}`)
  return res.data
}

export async function getInterceptorLogStats(params: {
  start?: number
  end?: number
  limit?: number
}): Promise<GetInterceptorLogStatsResponse> {
  const searchParams = new URLSearchParams()
  if (params.start) searchParams.set('start', String(params.start))
  if (params.end) searchParams.set('end', String(params.end))
  if (params.limit) searchParams.set('limit', String(params.limit))

  const res = await api.get(
    `/api/interceptor/logs/stats?${searchParams.toString()}`
  )
  return res.data
}
