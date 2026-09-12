export interface InterceptorLog {
  id: number
  user_id: number
  username: string
  token_id: number
  token_name: string
  channel_id: number
  channel_name: string
  model_name: string
  rule_type: string
  rule_detail: string
  action: 'modified' | 'rejected'
  reject_reason: string
  request_body: string
  modified_body: string
  created_at: number
}

export interface GetInterceptorLogsParams {
  page?: number
  page_size?: number
  username?: string
  channel_id?: number
  action?: string
  model_name?: string
  rule_type?: string
  start?: number
  end?: number
}

export interface InterceptorLogStatRow {
  key: string
  total: number
  modified: number
  rejected: number
}

export interface InterceptorLogStats {
  total: number
  modified: number
  rejected: number
  by_user: InterceptorLogStatRow[]
  by_rule: InterceptorLogStatRow[]
  by_model: InterceptorLogStatRow[]
  by_channel: InterceptorLogStatRow[]
}

export interface GetInterceptorLogStatsResponse {
  success: boolean
  message?: string
  data?: InterceptorLogStats
}

export interface GetInterceptorLogsResponse {
  success: boolean
  message?: string
  data?: {
    items: InterceptorLog[]
    total: number
    page: number
  }
}
