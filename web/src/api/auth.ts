import request from './request'

export function login(username: string, password: string, remember: boolean) {
  return request.post('/api/auth/login', {
    username,
    password,
    remember,
  })
}

export function register(username: string, password: string) {
  return request.post('/api/auth/register', {
    username,
    password,
  })
}

export function fnosLogin(remember: boolean) {
  return request.post('/api/auth/fnos/login', { remember })
}

// 只读当前网关身份：网关域上就地取当前 NAS 用户，无需跳板页。
// 方法必须与后端一致（GET）：方法不匹配会落进 404 兜底返回 SPA 网页，
// 前端就会误报「未获取到飞牛 NAS 登录信息」。
export function fnosIdentity() {
  return request.get('/api/auth/fnos/identity')
}

export function bindFnOSAccount(mode: 'register' | 'bind', username: string, password: string, remember: boolean) {
  return request.post('/api/auth/fnos/bind', { mode, username, password, remember })
}

export function checkAuth() {
  return request.get('/api/auth/check')
}

// 退出登录：服务端清会话 Cookie（Cookie 不分端口，只清前端会在网关上被认回去）。
export function logout() {
  return request.post('/api/auth/logout')
}

export function getCurrentUser() {
  return request.get('/api/auth/me')
}

export function checkSetupRequired() {
  return request.get('/api/auth/setup-required')
}

export function changePassword(oldPassword: string, newPassword: string) {
  return request.put('/api/auth/password', {
    old_password: oldPassword,
    new_password: newPassword,
  })
}

export function regenerateAPIKey() {
  return request.post('/api/auth/apikey')
}
