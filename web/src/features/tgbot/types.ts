export interface TgBotConfig {
  id: number
  bot_token: string
  enabled: boolean
  created_at: number
  updated_at: number
}

export interface TgBotTarget {
  chat_id: number
  thread_id: number
  enabled: boolean
}

export interface TgBotFeature {
  id: number
  name: string
  enabled: boolean
  targets: string // JSON array of TgBotTarget
  settings: string // JSON, feature-specific
  created_at: number
  updated_at: number
}

export interface TgBotConfigResponse {
  success: boolean
  message?: string
  data?: {
    config: TgBotConfig
    features: TgBotFeature[]
    running: boolean
    descriptions: Record<string, string>
  }
}

export interface MonitorSettings {
  chart_enabled: boolean
  exclude_users: string
  exclude_remarks: string
  active_users_only: boolean
  mask_username: boolean
  currency_symbol: string
  quota_per_unit: number
}

export interface GetkeySettings {
  allowed_groups: string[]
  quota_per_unit: number
}

export interface AdduserSettings {
  allowed_groups: string[]
  default_group: string
  remarks: string[]
  initial_quota: number
}

export interface BillingSettings {
  billing_rate: number
  usd_cny_rate: number
  currency_symbol: string
  exclude_users: string
  exclude_remarks: string
}

export interface InterceptSettings {
  alert_codes: number[]
  min_count: number
}
