import axios from 'axios'
import { message } from 'antd'

const api = axios.create({
  baseURL: '/api/v2',
  timeout: 30000,
})

api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('admin_token')
    if (token) {
      config.headers.Authorization = token
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

api.interceptors.response.use(
  (response) => {
    // v2 信封解包：将 response.data.data 提升到 response.data
    if (response.data && typeof response.data === 'object' && 'data' in response.data && 'meta' in response.data) {
      response.data = response.data.data
    }
    return response
  },
  (error) => {
    // v2 错误信封处理
    if (error.response?.data?.error) {
      error.message = error.response.data.error.message
    }
    if (error.response?.status === 401) {
      localStorage.removeItem('admin_token')
      window.location.href = '/admin/'
      message.error('会话已过期，请重新登录')
    }
    return Promise.reject(error)
  }
)

export default api
