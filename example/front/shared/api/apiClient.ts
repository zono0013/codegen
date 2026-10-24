import axios, { AxiosError, AxiosInstance, AxiosRequestConfig } from 'axios'
import { ApiError, Problem } from './errorUtils'

export interface ApiResponse<T> {
  data: T
  status: number
}

// 認証（トークンの付与や更新）は持たない。必要になったら axios のインターセプターで足す（docs/coding-conventions.md）。
class ApiClient {
  private axiosInstance: AxiosInstance

  constructor() {
    this.axiosInstance = axios.create({
      baseURL: process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:8080',
      timeout: 30000,
      headers: { 'Content-Type': 'application/json' },
    })

    this.axiosInstance.interceptors.response.use(
      (response) => response,
      (error: AxiosError) => Promise.reject(toApiError(error))
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

  getAxiosInstance(): AxiosInstance {
    return this.axiosInstance
  }
}

// BE は RFC 9457 Problem Details を返す。返さない失敗（ネットワークエラーなど）も同じ形にそろえる
function toApiError(error: AxiosError): ApiError {
  const data = error.response?.data
  const problem: Problem =
    data && typeof data === 'object' && 'title' in data
      ? (data as Problem)
      : {
          type: 'about:blank',
          title: 'Internal Server Error',
          status: error.response?.status ?? 500,
          detail: error.message,
        }
  return new ApiError(problem, { cause: error })
}

export const apiClient = new ApiClient()
export default apiClient
