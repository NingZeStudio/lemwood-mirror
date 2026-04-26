import api from '@/lib/axios'
import type { FileInfo } from '@/types'

export async function getFiles(path: string): Promise<FileInfo[]> {
  const response = await api.get<FileInfo[]>('/admin/files', {
    params: { path },
  })
  return response.data
}

export async function deleteFile(path: string): Promise<void> {
  await api.delete('/admin/files', {
    params: { path },
  })
}

export async function downloadFile(path: string): Promise<Blob> {
  const response = await api.get('/admin/files/download', {
    params: { path },
    responseType: 'blob',
  })
  return response.data
}

export async function uploadFile(path: string, file: File): Promise<void> {
  const formData = new FormData()
  formData.append('file', file)
  await api.post('/admin/files', formData, {
    params: { path },
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
}

export function getDownloadUrl(path: string): string {
  const token = localStorage.getItem('admin_token')
  return `https://mirror.lemwood.icu/api/admin/files/download?path=${encodeURIComponent(path)}&token=${encodeURIComponent(token || '')}`
}
