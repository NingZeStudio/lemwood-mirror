import axios from 'axios'
import { globalConfig } from '@/lib/globalConfig'

const api = axios.create({
    baseURL: globalConfig.api.baseUrl
})

// v2 信封解包：将 response.data.data 提升到 response.data
api.interceptors.response.use((response) => {
    if (response.data && typeof response.data === 'object' && 'data' in response.data && 'meta' in response.data) {
        response.data = response.data.data
    }
    return response
})

export default api;

export const getStatus = () => api.get(globalConfig.api.endpoints.status)
export const getLatest = () => api.get(globalConfig.api.endpoints.latest)
export const getStats = () => api.get(globalConfig.api.endpoints.stats)
export const scan = () => api.post(globalConfig.api.endpoints.scan)
export const getPowConfig = () => api.get(globalConfig.api.endpoints.powConfig)

// PoW 下载验证（替代极验）：创建挑战 → 浏览器求解 → 提交授权
export const createDownloadChallenge = (filePath) =>
    api.get(`${globalConfig.api.endpoints.downloadChallenge}?file_path=${encodeURIComponent(filePath)}`)

export const authorizeDownload = (challenge, solution) =>
    api.post(globalConfig.api.endpoints.downloadAuthorize, {
        challenge,
        solution
    })

export const prepareDownload = (filePath, returnUrl, source) =>
    api.post(globalConfig.api.endpoints.downloadPrepare, {
        file_path: filePath,
        ...(returnUrl && { return_url: returnUrl }),
        ...(source && { source: source })
    })

export const getDownloadLanding = (token) =>
    api.get(`${globalConfig.api.endpoints.downloadLanding}?token=${encodeURIComponent(token)}`)

