import api from '@/lib/axios'
import type { Config, ConfigUpdateRequest } from '@/types'

export async function getConfig(): Promise<Config> {
  const response = await api.get<Config>('/admin/config')
  return response.data
}

export async function updateConfig(data: ConfigUpdateRequest): Promise<void> {
  await api.post('/admin/config', data)
}

export async function triggerLauncherScan(launcher: string): Promise<void> {
  await api.post('/api/scan/launcher', { launcher })
}
