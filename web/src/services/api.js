import axios from 'axios';

const api = axios.create({
    baseURL: import.meta.env.VITE_API_BASE_URL || 'https://miawa.cn/api'
});

export default api;

export const getStatus = () => api.get('/status');
export const getLatest = () => api.get('/latest');
export const getStats = () => api.get('/stats');
export const getFiles = (path = '.') => api.get(`/files?path=${encodeURIComponent(path)}`);
export const scan = () => api.post('/scan');
export const getCaptchaConfig = () => api.get('/captcha/config');
export const verifyDownload = (lotNumber, captchaOutput, passToken, genTime, filePath) => 
    api.post('/download/verify', { 
        lot_number: lotNumber, 
        captcha_output: captchaOutput, 
        pass_token: passToken, 
        gen_time: genTime, 
        file_path: filePath 
    });
