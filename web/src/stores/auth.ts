import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { checkAuth, checkSetupRequired as apiCheckSetupRequired } from '../api/auth'
import { getPublicConfigs, getUserConfigMeta } from '../api/config'
import { getSecurityQuestions } from '../api/security'
import { useThemeStore } from './theme'

const TOKEN_KEY = 'token'

function readStoredToken(): string {
  return localStorage.getItem(TOKEN_KEY) || sessionStorage.getItem(TOKEN_KEY) || ''
}

function clearStoredToken() {
  localStorage.removeItem(TOKEN_KEY)
  sessionStorage.removeItem(TOKEN_KEY)
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(readStoredToken())
  const user = ref<any>(null)
  const requireLogin = ref(true)
  const allowRegister = ref(true)
  const loginExpiryDays = ref(30)
  const setupRequired = ref(false)
  const siteTitle = ref('账单')
  watch(siteTitle, (t) => { document.title = t }, { immediate: true })
  const hasSecurityQuestions = ref(true)
  const DISMISS_KEY = 'security_prompt_dismissed_at'
  const DISMISS_TTL = 60 * 60 * 1000 // 1 hour
  const securityPromptDismissed = ref((() => {
    const ts = localStorage.getItem(DISMISS_KEY)
    if (!ts) return false
    return Date.now() - Number(ts) < DISMISS_TTL
  })())
  let initialized = false
  let initPromise: Promise<void> | null = null

  const isAuthenticated = computed(() => !!token.value && !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function init() {
    if (initialized) return
    if (initPromise) return initPromise

    initPromise = (async () => {
      try {
        const configsRes = await getPublicConfigs()
        if (configsRes.data?.code === 0) {
          requireLogin.value = configsRes.data.data?.require_login !== 'false'
          allowRegister.value = configsRes.data.data?.allow_register !== 'false'
          const expiryDays = Number.parseInt(configsRes.data.data?.login_expiry_days || '', 10)
          if (Number.isInteger(expiryDays) && expiryDays >= 1 && expiryDays <= 365) {
            loginExpiryDays.value = expiryDays
          }
          if (configsRes.data.data?.site_title) {
            siteTitle.value = configsRes.data.data.site_title
          }
        }
      } catch {}

      await checkSetupRequired()

      if (setupRequired.value && token.value) {
        token.value = ''
        clearStoredToken()
        user.value = null
      }

      if (token.value) {
        try {
          const res = await checkAuth()
          if (res.data?.code === 0 && res.data.data?.authenticated) {
            user.value = res.data.data.user
            try {
              const secRes = await getSecurityQuestions()
              if (secRes.data?.code === 0) {
                hasSecurityQuestions.value = !!secRes.data.data?.has_questions
              }
            } catch {}
            try {
              const metaRes = await getUserConfigMeta()
              if (metaRes.data?.code === 0 && Array.isArray(metaRes.data.data)) {
                const themeItem = metaRes.data.data.find((it: any) => it.key === 'theme_mode')
                if (themeItem && ['system', 'light', 'dark'].includes(themeItem.value)) {
                  const themeStore = useThemeStore()
                  if (themeStore.mode !== themeItem.value) {
                    themeStore.setMode(themeItem.value)
                  }
                }
              }
            } catch {}
          } else {
            token.value = ''
            user.value = null
            clearStoredToken()
          }
        } catch {
          token.value = ''
          user.value = null
          clearStoredToken()
        }
      }
      initialized = true
    })()

    return initPromise
  }

  function resetInit() {
    initialized = false
    initPromise = null
  }

  function setToken(newToken: string, remember = true) {
    token.value = newToken
    clearStoredToken()
    const storage = remember ? localStorage : sessionStorage
    storage.setItem(TOKEN_KEY, newToken)
  }

  function setUser(newUser: any) {
    user.value = newUser
  }

  function logout() {
    token.value = ''
    user.value = null
    clearStoredToken()
    localStorage.removeItem(DISMISS_KEY)
    securityPromptDismissed.value = false
  }

  async function checkSetupRequired() {
    try {
      const setupRes = await apiCheckSetupRequired()
      if (setupRes.data?.code === 0) {
        setupRequired.value = !!setupRes.data.data?.setup_required
      }
    } catch {}
    return setupRequired.value
  }

  function dismissSecurityPrompt() {
    securityPromptDismissed.value = true
    localStorage.setItem(DISMISS_KEY, String(Date.now()))
  }

  function refreshSecurityQuestions() {
    if (!token.value) return
    getSecurityQuestions()
      .then(res => {
        if (res.data?.code === 0) {
          hasSecurityQuestions.value = !!res.data.data?.has_questions
          if (hasSecurityQuestions.value) {
            localStorage.removeItem(DISMISS_KEY)
          }
        }
      })
      .catch(() => {})
  }

  return { token, user, requireLogin, allowRegister, loginExpiryDays, setupRequired, siteTitle, hasSecurityQuestions, securityPromptDismissed, isAuthenticated, isAdmin, init, resetInit, setToken, setUser, logout, checkSetupRequired, dismissSecurityPrompt, refreshSecurityQuestions }
})
