<template>
  <AuthShell title="欢迎回来" subtitle="登录后继续整理你的个人与共享账本">
    <form @submit.prevent="handleLogin" class="space-y-5">
      <div v-if="fnosBindingRequired" class="rounded-2xl border border-brand-400/30 bg-brand-400/10 px-5 py-4 text-sm leading-6 text-white/85">
        当前飞牛 NAS 用户 <span class="font-semibold text-brand-200">{{ fnosUsername || '已登录用户' }}</span> 尚未绑定。
        请输入已有应用账号的密码；验证成功后会保留并继续使用原来的账本和账单数据。
      </div>

      <AuthField v-model="username" label="用户名" autocomplete="username" required placeholder="请输入用户名" autofocus>
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
        </template>
      </AuthField>

      <AuthField v-model="password" label="密码" type="password" autocomplete="current-password" required placeholder="请输入密码">
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect x="5" y="11" width="14" height="9" rx="2" stroke-width="2"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 11V8a4 4 0 118 0v3"/></svg>
        </template>
        <template #labelRight>
          <router-link to="/forgot-password" class="text-xs font-medium text-brand-300 hover:text-brand-200 transition-colors">忘记密码？</router-link>
        </template>
      </AuthField>

      <div class="rounded-xl border border-white/10 bg-white/[0.04] px-3.5 py-3">
        <div class="flex items-center justify-between gap-3">
          <label class="inline-flex min-h-11 cursor-pointer items-center gap-2 text-sm font-medium text-white">
            <input
              v-model="rememberLogin"
              type="checkbox"
              class="h-4 w-4 rounded border-white/30 accent-brand-400 focus-visible:ring-2 focus-visible:ring-brand-300/70"
            />
            记住登录
          </label>
          <span class="text-xs text-white/70">{{ authStore.loginExpiryDays }} 天内免登录</span>
        </div>
        <p class="text-xs text-white/65">仅保存登录凭证，不会保存密码</p>
      </div>

      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0 -translate-y-1" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="errorMsg" class="flex items-center gap-2 text-sm text-red-300 bg-red-500/10 border border-red-500/25 rounded-xl px-3.5 py-2.5">
          <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M5.07 19h13.86a2 2 0 001.74-3L13.74 4a2 2 0 00-3.48 0L3.34 16a2 2 0 001.73 3z"/></svg>
          {{ errorMsg }}
        </div>
      </transition>

      <button type="submit" :disabled="loading" class="btn-premium">
        <svg v-if="loading" class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
        {{ loading ? (fnosBindingRequired ? '登录并绑定中...' : '登录中...') : (fnosBindingRequired ? '登录并绑定' : '登录') }}
      </button>

      <button v-if="fnosEnabled" type="button" :disabled="loading" @click="handleFnOSLogin" class="w-full min-h-12 inline-flex items-center justify-center rounded-xl border border-white/15 bg-transparent px-4 py-3 text-sm font-semibold text-white transition-colors hover:bg-white/[0.08] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-300/70 disabled:cursor-not-allowed disabled:opacity-60">
        {{ loading ? '正在获取飞牛账号…' : '使用飞牛 NAS 登录' }}
      </button>
    </form>

    <Teleport to="body">
      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="showFnOSConfirm" class="fixed inset-0 z-[100] flex items-center justify-center bg-black/65 px-4 backdrop-blur-sm" @click.self="cancelFnOSConfirm">
          <section role="dialog" aria-modal="true" aria-labelledby="fnos-confirm-title" class="w-full max-w-md rounded-3xl border border-white/15 bg-[#171827] p-6 text-white shadow-2xl sm:p-8">
            <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-500 text-2xl font-bold shadow-lg shadow-brand-500/25">{{ fnosConfirmUsername.slice(0, 1) || '飞' }}</div>
            <h2 id="fnos-confirm-title" class="mt-5 text-center text-2xl font-bold">确认使用飞牛 NAS 登录</h2>
            <p class="mt-2 text-center text-sm leading-6 text-white/65">“账单”将使用下方飞牛 NAS 账号完成授权登录</p>
            <div class="mt-6 flex items-center justify-center gap-3 rounded-2xl border border-white/10 bg-white/[0.06] px-4 py-4">
              <span class="flex h-10 w-10 items-center justify-center rounded-full bg-brand-400/25 font-semibold text-brand-100">{{ fnosConfirmUsername.slice(0, 1) || '飞' }}</span>
              <span class="font-semibold">{{ fnosConfirmUsername || '当前飞牛 NAS 用户' }}</span>
            </div>
            <button type="button" :disabled="loading" @click="confirmFnOSLogin" class="btn-premium mt-6">{{ loading ? '正在登录…' : '确认登录' }}</button>
            <button type="button" :disabled="loading" @click="switchFnOSAccount" class="mt-3 w-full rounded-xl px-4 py-3 text-sm font-semibold text-brand-200 transition-colors hover:bg-white/[0.06] hover:text-brand-100 disabled:opacity-60">使用其他飞牛账号</button>
            <button type="button" :disabled="loading" @click="cancelFnOSConfirm" class="mt-1 w-full rounded-xl px-4 py-2 text-sm text-white/55 transition-colors hover:text-white/80 disabled:opacity-60">取消</button>
          </section>
        </div>
      </transition>
    </Teleport>

    <template #footer v-if="authStore.allowRegister">
      还没有账号？
      <router-link to="/register" class="font-semibold text-brand-300 hover:text-brand-200 transition-colors">立即注册</router-link>
    </template>
  </AuthShell>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { bindFnOSAccount, fnosLogin, login } from '../api/auth'
import { useAuthStore } from '../stores/auth'
import AuthShell from '../components/auth/AuthShell.vue'
import AuthField from '../components/auth/AuthField.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const username = ref('')
const password = ref('')
const loading = ref(false)
const errorMsg = ref('')
const fnosBindingRequired = ref(false)
const fnosUsername = ref('')
const fnosConfirmUsername = ref('')
const showFnOSConfirm = ref(false)
const fnosEnabled = import.meta.env.VITE_FNOS_APP === 'true'
const REMEMBER_PREFERENCE_KEY = 'remember_login_preference'
const rememberLogin = ref(localStorage.getItem(REMEMBER_PREFERENCE_KEY) !== 'false')

function clearFnOSTicket() {
  sessionStorage.removeItem('fnos_ticket')
  sessionStorage.removeItem('fnos_ticket_username')
}

async function handleLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    localStorage.setItem(REMEMBER_PREFERENCE_KEY, String(rememberLogin.value))
    const res = fnosBindingRequired.value
      ? await bindFnOSAccount('bind', username.value, password.value, rememberLogin.value)
      : await login(username.value, password.value, rememberLogin.value)
    if (res.data?.code === 0) {
      if (fnosBindingRequired.value) clearFnOSTicket()
      authStore.setToken(res.data.data.token, rememberLogin.value)
      authStore.setUser(res.data.data.user)
      router.push('/admin')
    } else {
      errorMsg.value = res.data?.message || '登录失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '网络错误'
  } finally {
    loading.value = false
  }
}

// 登录页运行在应用自身端口上，无法直接拿到网关身份头；改为打开网关跳板页，
// 跳板页签发一次性票据并带回当前 NAS 用户名，先弹确认框再登录。
function handleFnOSLogin() {
  const gatewayURL = localStorage.getItem('fnos_gateway_url')
  if (!gatewayURL) {
    errorMsg.value = '请先从飞牛桌面打开本应用，再使用飞牛授权登录'
    return
  }
  loading.value = true
  window.location.href = gatewayURL
}

function openFnOSConfirm() {
  fnosConfirmUsername.value = sessionStorage.getItem('fnos_ticket_username') || ''
  showFnOSConfirm.value = true
}

function cancelFnOSConfirm() {
  showFnOSConfirm.value = false
  clearFnOSTicket()
}

// 切换飞牛账号：跳到飞牛桌面的登录页，登录成功后经 redirect_uri 回到本页。
function switchFnOSAccount() {
  const gatewayURL = localStorage.getItem('fnos_gateway_url')
  if (!gatewayURL) {
    showFnOSConfirm.value = false
    clearFnOSTicket()
    errorMsg.value = '无法定位飞牛桌面入口，请从飞牛桌面重新打开本应用'
    return
  }
  const appLoginPath = `${window.location.origin}${import.meta.env.BASE_URL}login`
  window.location.href = `${new URL(gatewayURL).origin}/login?redirect_uri=${encodeURIComponent(appLoginPath)}`
}

async function loginWithTicket(remember: boolean) {
  const res = await fnosLogin(remember)
  if (res.data?.code !== 0) {
    clearFnOSTicket()
    showFnOSConfirm.value = false
    errorMsg.value = res.data?.message || '飞牛 NAS 登录失败'
    return
  }
  if (res.data.data?.binding_required) {
    showFnOSConfirm.value = false
    if (res.data.data.has_accounts || res.data.data.suggested_mode === 'bind') {
      fnosBindingRequired.value = true
      fnosUsername.value = res.data.data.fnos_username || ''
      username.value = res.data.data.suggested_username || ''
      return
    }
    // 没有任何应用账号：转注册页创建并绑定，票据保留给绑定请求使用。
    router.push({
      name: 'Register',
      query: {
        fnos: 'bind',
        fnos_username: res.data.data.fnos_username || '',
        fnos_mode: 'register',
        remember: remember ? '1' : '0',
      },
    })
    return
  }
  clearFnOSTicket()
  localStorage.setItem(REMEMBER_PREFERENCE_KEY, String(remember))
  authStore.setToken(res.data.data.token, remember)
  authStore.setUser(res.data.data.user)
  showFnOSConfirm.value = false
  router.push('/admin')
}

async function confirmFnOSLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    await loginWithTicket(rememberLogin.value)
  } catch (err: any) {
    clearFnOSTicket()
    showFnOSConfirm.value = false
    errorMsg.value = err.response?.data?.message || '飞牛 NAS 登录失败，请重新点击飞牛授权登录'
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  if (fnosEnabled && route.query.fnos_account_switched === '1') {
    await router.replace({ name: 'Login' })
  }
  // 从网关跳板页带票据回来：先弹确认框，用户确认后才真正登录。
  if (fnosEnabled && sessionStorage.getItem('fnos_ticket') && !authStore.isAuthenticated) {
    openFnOSConfirm()
  }
})
</script>
