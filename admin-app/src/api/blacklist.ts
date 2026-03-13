import api from '@/lib/axios'
import type { BlacklistItem, AddBlacklistRequest } from '@/types'

export async function getBlacklist(): Promise<BlacklistItem[]> {
  const response = await api.get<BlacklistItem[]>('/admin/blacklist')
  return response.data
}

export async function addBlacklist(data: AddBlacklistRequest): Promise<void> {
  await api.post('/admin/blacklist', data)
}

export async function removeBlacklist(ip: string): Promise<void> {
  await api.delete('/admin/blacklist', {
    params: { ip },
  })
}
