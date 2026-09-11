import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'
import './style.css'

// 飞牛授权走"网关跳板 + 一次性票据"：跳板页带回 fnos_ticket，这里先接住，
// 后续 /api/auth/fnos/login、/bind 请求由 request 拦截器附带 X-FnOS-Ticket。
function captureFnOSTicket() {
  const hash = window.location.hash.replace(/^#/, '')
  if (!hash) return
  const params = new URLSearchParams(hash)
  const ticket = params.get('fnos_ticket')
  if (!ticket) return
  sessionStorage.setItem('fnos_ticket', ticket)
  const fnosUsername = params.get('fnos_username')
  if (fnosUsername) sessionStorage.setItem('fnos_ticket_username', fnosUsername)
  params.delete('fnos_ticket')
  params.delete('fnos_username')
  const rest = params.toString()
  history.replaceState(null, '', window.location.pathname + window.location.search + (rest ? `#${rest}` : ''))
}

// 从飞牛桌面跳转过来时记住网关入口，登录页"飞牛授权登录"按钮据此打开跳板页。
function rememberFnOSGateway() {
  if (!document.referrer) return
  try {
    const desktop = new URL(document.referrer)
    if (desktop.protocol !== 'http:' && desktop.protocol !== 'https:') return
    if (desktop.hostname !== window.location.hostname) return
    if (desktop.origin === window.location.origin) return
    localStorage.setItem('fnos_gateway_url', `${desktop.origin}${import.meta.env.BASE_URL}fnos-entry.html`)
  } catch {}
}

captureFnOSTicket()
rememberFnOSGateway()

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
