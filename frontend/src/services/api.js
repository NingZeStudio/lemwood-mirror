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

export const getStatus = () => api.get('/status');
export const getLatest = () => api.get('/latest');
export const getStats = () => api.get('/stats');
export const getFiles = (path = '.') => api.get(`/files?path=${encodeURIComponent(path)}`);
export const scan = () => api.post('/scan');
export const getCaptchaConfig = () => api.get('/captcha/config');
export const verifyDownload = (lotNumber, captchaOutput, passToken, genTime, filePath, returnUrl, source) => 
    api.post('/download/verify', { 
        lot_number: lotNumber, 
        captcha_output: captchaOutput, 
        pass_token: passToken, 
        gen_time: genTime, 
        file_path: filePath,
        ...(returnUrl && { return_url: returnUrl }),
        ...(source && { source: source })
    });

export const prepareDownload = (filePath, returnUrl, source) => 
    api.post('/download/prepare', {
        file_path: filePath,
        ...(returnUrl && { return_url: returnUrl }),
        ...(source && { source: source })
    });

export const getDownloadLanding = (token) => 
    api.get(`/download/landing?token=${encodeURIComponent(token)}`);

