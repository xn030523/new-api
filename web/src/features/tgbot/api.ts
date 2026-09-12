import { api } from '@/lib/api'

import type { TgBotConfigResponse, TgBotConfig, TgBotFeature } from './types'

export async function getTgBotConfig(): Promise<TgBotConfigResponse> {
  const res = await api.get('/api/tgbot/')
  return res.data
}

export async function updateTgBotConfig(config: Partial<TgBotConfig>): Promise<{ success: boolean; message?: string }> {
  const res = await api.put('/api/tgbot/', config)
  return res.data
}

export async function updateTgBotFeature(feature: Partial<TgBotFeature> & { name: string }): Promise<{ success: boolean; message?: string }> {
  const res = await api.put('/api/tgbot/feature', feature)
  return res.data
}
