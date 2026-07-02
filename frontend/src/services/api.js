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
export const getCaptchaConfig = () => api.get(globalConfig.api.endpoints.captchaConfig)
export const verifyDownload = (lotNumber, captchaOutput, passToken, genTime, filePath, returnUrl, source) =>
    api.post(globalConfig.api.endpoints.downloadVerify, {
        lot_number: lotNumber,
        captcha_output: captchaOutput,
        pass_token: passToken,
        gen_time: genTime,
        file_path: filePath,
        ...(returnUrl && { return_url: returnUrl }),
        ...(source && { source: source })
    })

export const prepareDownload = (filePath, returnUrl, source) =>
    api.post(globalConfig.api.endpoints.downloadPrepare, {
        file_path: filePath,
        ...(returnUrl && { return_url: returnUrl }),
        ...(source && { source: source })
    })

export const getDownloadLanding = (token) =>
    api.get(`${globalConfig.api.endpoints.downloadLanding}?token=${encodeURIComponent(token)}`)

