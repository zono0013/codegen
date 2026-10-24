import axios, {
  AxiosInstance,
  AxiosRequestConfig,
  AxiosError,
  AxiosResponse,
} from 'axios'
import { tokenStore } from './tokenStore'
import { Problem } from './errorUtils'

export interface ApiResponse<T> {
  data: T
  status: number
}

class ApiClient {
  private axiosInstance: AxiosInstance

  constructor() {
    this.axiosInstance = axios.create({
      baseURL: process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:8080',
      timeout: 30000,
      headers: { 'Content-Type': 'application/json' },
      withCredentials: true,
    })

    this.setupInterceptors()
  }

  private setupInterceptors() {
    this.axiosInstance.interceptors.request.use(
      (config) => {
        const token = tokenStore.getAccessToken()
        if (token) config.headers.Authorization = `Bearer ${token}`
        return config
      },
      (error) => Promise.reject(error)
    )

    this.axiosInstance.interceptors.response.use(
      (response: AxiosResponse) => response,
      async (error: AxiosError) => {
        const originalRequest = error.config as AxiosRequestConfig & {
          _retry?: boolean
        }

        if (error.response?.status === 401 && !originalRequest._retry) {
          originalRequest._retry = true

          try {
            const response = await this.axiosInstance.post(
              '/auth/refresh',
              null,
              {
                _retry: true,
                headers: { Authorization: undefined },
              } as AxiosRequestConfig & { _retry?: boolean }
            )

            const authHeader = Object.keys(response.headers).find(
              (key) => key.toLowerCase() === 'authorization'
            )
            const newToken = authHeader
              ? response.headers[authHeader]?.replace(/^Bearer\s+/i, '')
              : null

            if (newToken) {
              tokenStore.setAccessToken(newToken)
              if (originalRequest.headers) {
                originalRequest.headers.Authorization = `Bearer ${newToken}`
              }
              return this.axiosInstance(originalRequest)
            }
          } catch {
            tokenStore.clear()
            if (typeof window !== 'undefined') {
              const publicPaths = ['/login', '/forgot-password', '/reset-password']
              if (!publicPaths.includes(window.location.pathname)) {
                window.location.href = '/login'
              }
            }
          }
        }

        // BE は RFC 7807 Problem Details を返す
        const problem: Problem = error.response?.data &&
          typeof error.response.data === 'object' &&
          'title' in (error.response.data as object)
          ? (error.response.data as Problem)
          : {
              type: 'about:blank',
              title: 'Internal Server Error',
              status: error.response?.status ?? 500,
              detail: error.message,
            }

        return Promise.reject(problem)
      }
    )
  }

  async get<T>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    const response = await this.axiosInstance.get<T>(url, config)
    return { data: response.data, status: response.status }
  }

  async post<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    const response = await this.axiosInstance.post<T>(url, data, config)
    return { data: response.data, status: response.status }
  }

  async put<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    const response = await this.axiosInstance.put<T>(url, data, config)
    return { data: response.data, status: response.status }
  }

  async patch<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    const response = await this.axiosInstance.patch<T>(url, data, config)
    return { data: response.data, status: response.status }
  }

  async delete<T>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    const response = await this.axiosInstance.delete<T>(url, config)
    return { data: response.data, status: response.status }
  }

  async postFormData<T>(url: string, formData: FormData, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.axiosInstance.post<T>(url, formData, {
      ...config,
      headers: { ...config?.headers, 'Content-Type': 'multipart/form-data' },
    })
    return response.data
  }

  getAxiosInstance(): AxiosInstance {
    return this.axiosInstance
  }
}

export const apiClient = new ApiClient()
export default apiClient
