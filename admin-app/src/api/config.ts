import api from '@/lib/axios'
import type { Config, ConfigUpdateRequest, SelfUpdateStatus } from '@/types'

export async function getConfig(): Promise<Config> {
  const response = await api.get<Config>('/admin/config')
  return response.data
}

export async function updateConfig(data: ConfigUpdateRequest): Promise<void> {
  await api.post('/admin/config', data)
}

export async function triggerLauncherScan(launcher: string): Promise<void> {
  await api.post('/admin/scans/launcher', { launcher })
}

export async function getSelfUpdateStatus(): Promise<SelfUpdateStatus> {
  const response = await api.get<SelfUpdateStatus>('/admin/self-update/status')
  return response.data
}

export async function checkSelfUpdate(): Promise<SelfUpdateStatus> {
  const response = await api.post<SelfUpdateStatus>('/admin/self-update/check')
  return response.data
}

export async function applySelfUpdate(): Promise<SelfUpdateStatus> {
  const response = await api.post<SelfUpdateStatus>('/admin/self-update/apply')
  return response.data
}

export async function restartSelfUpdate(): Promise<{ status: string; message: string }> {
  const response = await api.post<{ status: string; message: string }>('/admin/self-update/restart')
  return response.data
}
