import axios from 'axios'
import { useAuthStore } from '../stores/auth'
import router from '../router'
import { onFnOSGatewayOrigin } from '../utils/gateway'

const request = axios.create({
  baseURL: import.meta.env.BASE_URL,
  timeout: 30000,
})

request.interceptors.request.use((config) => {
  if (import.meta.env.BASE_URL !== '/' && config.url?.startsWith('/')) {
    config.url = config.url.slice(1)
  }
  const authStore = useAuthStore()
  // 网关域上飞牛接入层会把应用自己的 Authorization 当成它的会话令牌拦掉
  // （回 200 纯文本 "invalid token"），所以这里一律不发该头，登录态由服务端
  // 在登录成功时种下的 HttpOnly 会话 Cookie 维持。
  if (authStore.token && !onFnOSGatewayOrigin()) {
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
      // 不阻塞错误链：登出通知服务端失败也不影响跳登录页。
      void authStore.logout()
      router.push('/login')
    }
    return Promise.reject(error)
  }
)

export default request
