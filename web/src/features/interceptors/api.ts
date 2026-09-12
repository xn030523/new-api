import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  Interceptor,
  InterceptorPreset,
  RuleTypeDescription,
} from './types'

export async function getInterceptors(): Promise<Interceptor[]> {
  const res = await api.get<{ success: boolean; data: Interceptor[] }>(
    '/api/interceptor/'
  )
  return requireServerSuccess(res.data).data
}

export async function getInterceptor(id: number): Promise<Interceptor> {
  const res = await api.get<{ success: boolean; data: Interceptor }>(
    `/api/interceptor/${id}`
  )
  return requireServerSuccess(res.data).data
}

export async function createInterceptor(
  data: Partial<Interceptor>
): Promise<void> {
  const res = await api.post<{ success: boolean; message?: string }>(
    '/api/interceptor/',
    data
  )
  requireServerSuccess(res.data)
}

export async function updateInterceptor(
  data: Partial<Interceptor>
): Promise<void> {
  const res = await api.put<{ success: boolean; message?: string }>(
    '/api/interceptor/',
    data
  )
  requireServerSuccess(res.data)
}

export async function deleteInterceptor(id: number): Promise<void> {
  const res = await api.delete<{ success: boolean; message?: string }>(
    `/api/interceptor/${id}`
  )
  requireServerSuccess(res.data)
}

export async function getRuleTypes(): Promise<RuleTypeDescription> {
  const res = await api.get<{ success: boolean; data: RuleTypeDescription }>(
    '/api/interceptor/rule_types'
  )
  return requireServerSuccess(res.data).data
}

export async function getInterceptorPresets(): Promise<InterceptorPreset[]> {
  const res = await api.get<{ success: boolean; data: InterceptorPreset[] }>(
    '/api/interceptor/presets'
  )
  return requireServerSuccess(res.data).data
}
