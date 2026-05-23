import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import { getAccessToken, saveAccessToken } from '@/utils/token'
import { refreshAccessToken } from '@/api/auth'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

let refreshPromise: Promise<string> | null = null

request.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = getAccessToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

request.interceptors.response.use(
  (res) => res.data,
  async (error: AxiosError<{ message?: string }>) => {
    const config = error.config as InternalAxiosRequestConfig & { _retry?: boolean }
    if (error.response?.status === 401 && config && !config._retry) {
      config._retry = true
      const refresh = localStorage.getItem('competeai_refresh_token')
      if (refresh) {
        try {
          if (!refreshPromise) {
            refreshPromise = refreshAccessToken(refresh).finally(() => {
              refreshPromise = null
            })
          }
          const newAccess = await refreshPromise
          saveAccessToken(newAccess)
          config.headers.Authorization = `Bearer ${newAccess}`
          return request(config)
        } catch {
          localStorage.removeItem('competeai_access_token')
          localStorage.removeItem('competeai_refresh_token')
          window.location.href = '/login'
          return Promise.reject(error)
        }
      }
    }

    const msg =
      error.response?.data?.message ?? error.message ?? '请求失败'
    ElMessage.error(msg)
    return Promise.reject(error)
  },
)

export default request
