import axios from 'axios';

const api = axios.create({
    baseURL: 'https://mirror.lemwood.icu/api'
});

export default api;

export const getStatus = () => api.get('/status');
export const getLatest = () => api.get('/latest');
export const getStats = () => api.get('/stats');
export const getFiles = (path = '.') => api.get(`/files?path=${encodeURIComponent(path)}`);
export const scan = () => api.post('/scan');
