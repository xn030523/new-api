export interface InterceptorRule {
  type: string
  config?: Record<string, unknown>
}

export interface Interceptor {
  id: number
  channel_id: number
  name: string
  description: string
  rules: string
  enabled: boolean
  priority: number
  created_at: number
  updated_at: number
}

export type InterceptorFormData = Omit<
  Interceptor,
  'id' | 'created_at' | 'updated_at'
>

export interface RuleTypeDescription {
  [key: string]: string
}

export interface InterceptorPreset {
  name: string
  description: string
  rules: InterceptorRule[]
}
