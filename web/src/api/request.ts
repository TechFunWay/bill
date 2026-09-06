import axios from 'axios'
import { useAuthStore } from '../stores/auth'
import router from '../router'

const request = axios.create({
  baseURL: import.meta.env.BASE_URL,
  timeout: 30000,
})

request.interceptors.request.use((config) => {
  if (import.meta.env.BASE_URL !== '/' && config.url?.startsWith('/')) {
    config.url = config.url.slice(1)
  }
  const authStore = useAuthStore()
  if (authStore.token) {
    config.headers.Authorization = `Bearer ${authStore.token}`
  }
  // 飞牛授权登录的直连端口凭证：由网关跳板页签发，登录/绑定成功后由后端消费。
  const fnosTicket = sessionStorage.getItem('fnos_ticket')
  if (fnosTicket) {
    config.headers['X-FnOS-Ticket'] = fnosTicket
  }
  return config
})

request.interceptors.response.use(
  (response) => {
    return response
  },
  (error) => {
    if (error.response?.status === 401) {
      const authStore = useAuthStore()
      authStore.logout()
      router.push('/login')
    }
    return Promise.reject(error)
  }
)

export default request
