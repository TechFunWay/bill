<template>
  <div class="min-h-dvh bg-background text-foreground">
    <a href="#main-content" class="fixed left-4 top-4 z-50 -translate-y-32 whitespace-nowrap rounded-full bg-accent px-4 py-2 font-bold text-brand-900 focus:translate-y-0">跳到主要内容</a>
    <div class="app-bg" aria-hidden="true"><div class="ledger-orb"></div><div class="ledger-dots"></div></div>

    <aside class="app-sidebar fixed inset-y-0 left-0 z-30 hidden w-60 flex-col border-r px-3.5 py-4 lg:flex">
      <RouterLink to="/admin" class="flex min-h-9 items-center gap-2.5 px-1.5">
        <span class="flex h-9 w-9 items-center justify-center rounded-full bg-accent text-brand-900 shadow-glow">
          <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M6 3h12v18l-3-2-3 2-3-2-3 2V3Zm3 5h6m-6 4h6"/></svg>
        </span>
        <div><div class="font-display text-base font-extrabold tracking-tight">账单</div><div class="sidebar-muted font-mono text-[9px] uppercase tracking-[0.16em]">Bill Ledger</div></div>
      </RouterLink>

      <div class="sidebar-panel mx-1.5 mt-4 rounded-2xl border p-3">
        <p class="sidebar-muted font-mono text-[9px] uppercase tracking-[0.16em]">Today</p>
        <p class="mt-1 text-[13px] font-semibold">{{ todayText }}</p>
        <div class="sidebar-track mt-2 h-1 overflow-hidden rounded-full"><div class="h-full w-2/3 rounded-full bg-accent"></div></div>
      </div>

      <nav class="mt-3 min-h-0 flex-1 space-y-0.5 overflow-y-auto pr-1" aria-label="主导航">
        <RouterLink v-for="item in mainNav" :key="item.to" :to="item.to" :class="isActive(item.to) ? 'nav-link-active' : 'nav-link-idle'" class="nav-link">
          <span class="nav-icon" v-html="item.icon"></span><span>{{ item.label }}</span>
        </RouterLink>
        <div class="sidebar-muted px-2.5 pb-1.5 pt-4 font-mono text-[9px] font-semibold uppercase tracking-[0.18em]">记账设置</div>
        <RouterLink to="/admin/personal/accounts" :class="isActive('/admin/personal/accounts') ? 'nav-link-active' : 'nav-link-idle'" class="nav-link"><span class="nav-icon" v-html="accountIcon"></span><span>账户管理</span></RouterLink>
        <RouterLink to="/admin/personal/settings" :class="isActive('/admin/personal/settings') ? 'nav-link-active' : 'nav-link-idle'" class="nav-link"><span class="nav-icon" v-html="categoryIcon"></span><span>分类与标签</span></RouterLink>
        <RouterLink to="/admin/settings" :class="isActive('/admin/settings') ? 'nav-link-active' : 'nav-link-idle'" class="nav-link"><span class="nav-icon" v-html="settingsIcon"></span><span>偏好设置</span></RouterLink>
        <template v-if="authStore.isAdmin">
          <div class="sidebar-muted px-2.5 pb-1.5 pt-4 font-mono text-[9px] font-semibold uppercase tracking-[0.18em]">系统管理</div>
          <RouterLink v-for="item in adminNav" :key="item.to" :to="item.to" :class="isActive(item.to) ? 'nav-link-active' : 'nav-link-idle'" class="nav-link"><span class="nav-icon" v-html="item.icon"></span><span>{{ item.label }}</span></RouterLink>
        </template>
      </nav>

      <div class="sidebar-profile rounded-[1.25rem] border p-2.5">
        <div class="flex items-center gap-2.5">
          <span class="avatar">{{ userInitial }}</span>
          <div class="min-w-0 flex-1"><div class="truncate text-[13px] font-bold">{{ authStore.user?.username }}</div><div class="sidebar-muted text-[11px]">{{ authStore.isAdmin ? '管理员' : '普通用户' }}</div></div>
          <button class="sidebar-logout icon-button" aria-label="退出登录" @click="logout"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 17l5-5-5-5m5 5H3m10-9h6v18h-6"/></svg></button>
        </div>
        <div v-if="versionInfo" class="sidebar-version mt-2 border-t pt-1.5 text-center font-mono text-[9px] uppercase tracking-wider">{{ versionInfo }}</div>
      </div>
    </aside>

    <div class="min-h-dvh min-w-0 lg:pl-60">
      <header class="app-header sticky top-0 z-20 flex h-14 items-center justify-between bg-background/88 px-3 backdrop-blur-xl lg:h-14 lg:px-6">
        <div class="flex min-w-0 items-center gap-3">
          <RouterLink to="/admin" class="flex h-9 w-9 items-center justify-center rounded-full bg-brand-100 text-brand-700 shadow-soft dark:bg-brand-800 dark:text-accent lg:hidden" aria-label="返回首页"><svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M6 3h12v18l-3-2-3 2-3-2-3 2V3Zm3 5h6m-6 4h6"/></svg></RouterLink>
          <div class="min-w-0"><p class="hidden font-mono text-[9px] uppercase tracking-[0.18em] text-muted-foreground lg:block">Bill Ledger</p><h1 class="truncate font-display text-base font-extrabold tracking-tight lg:text-lg">{{ currentTitle }}</h1></div>
        </div>
        <div class="flex items-center gap-1.5">
          <UiThemeToggle />
          <div ref="menuRef" class="relative lg:hidden">
            <button class="flex min-h-10 items-center gap-2 rounded-full px-1.5 transition-colors hover:bg-muted" :aria-expanded="menuOpen" aria-haspopup="menu" aria-label="打开用户菜单" @click.stop="menuOpen = !menuOpen"><span class="avatar h-8 w-8">{{ userInitial }}</span><svg class="h-4 w-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m6 9 6 6 6-6"/></svg></button>
            <Transition enter-active-class="transition duration-150" enter-from-class="scale-95 opacity-0" leave-active-class="transition duration-100" leave-to-class="scale-95 opacity-0">
              <div v-if="menuOpen" class="absolute right-0 mt-2 w-52 origin-top-right rounded-xl border border-border bg-surface p-2 shadow-xl">
                <RouterLink to="/admin/profile" class="menu-item" @click="menuOpen = false">个人资料</RouterLink>
                <RouterLink to="/admin/personal/accounts" class="menu-item" @click="menuOpen = false">账户管理</RouterLink>
                <RouterLink to="/admin/personal/settings" class="menu-item" @click="menuOpen = false">分类与标签</RouterLink>
                <RouterLink to="/admin/settings" class="menu-item" @click="menuOpen = false">偏好设置</RouterLink>
                <template v-if="authStore.isAdmin"><div class="my-1 border-t border-border"></div><RouterLink to="/admin/users" class="menu-item" @click="menuOpen = false">用户管理</RouterLink><RouterLink to="/admin/configs" class="menu-item" @click="menuOpen = false">系统配置</RouterLink><RouterLink to="/admin/audit" class="menu-item" @click="menuOpen = false">操作日志</RouterLink></template>
                <div class="my-1 border-t border-border"></div><button class="menu-item w-full text-rose-600" @click="logout">退出登录</button>
              </div>
            </Transition>
          </div>
        </div>
      </header>

      <main id="main-content" ref="mainRef" tabindex="-1" class="min-w-0 w-full overflow-x-clip p-3 pb-24 outline-none lg:px-6 lg:py-5 lg:pb-8">
        <!-- 赞赏横幅：管理员可见，未支持当前版本前常驻展示（关闭不记忆，
             刷新后再次出现，升级到新版本后也会再次出现）；文档流内展示 -->
        <div v-if="showDonateBanner" class="mb-4 flex items-center justify-between gap-3 rounded-xl bg-gradient-to-r from-brand-600 to-brand-800 px-4 py-2.5 text-sm text-white shadow-md">
          <span>☕ 账单应用免费、无广告、数据都在你自己的设备上。如果它帮到了你，欢迎请站长喝杯咖啡——1 元也是心意，完全自愿，不影响任何功能。</span>
          <span class="flex shrink-0 items-center gap-2">
            <button class="underline underline-offset-2 hover:opacity-80" @click="showSupportModal = true">去赞赏</button>
            <button class="hover:opacity-80" title="关闭" aria-label="关闭赞赏横幅" @click="dismissDonateBanner"><svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg></button>
          </span>
        </div>
        <RouterView v-slot="{ Component }"><Transition mode="out-in" enter-active-class="transition duration-200 ease-out" enter-from-class="translate-y-1 opacity-0" leave-active-class="transition duration-100" leave-to-class="opacity-0"><component :is="Component" /></Transition></RouterView>
      </main>
    </div>

    <nav class="mobile-bottom-nav fixed inset-x-0 z-30 border-x-0 border-b-0 border-border bg-surface/88 px-1 py-1.5 shadow-card backdrop-blur-2xl lg:hidden" aria-label="移动端主导航">
      <div class="mx-auto grid max-w-lg grid-cols-4">
        <RouterLink v-for="item in mobileNav" :key="item.to" :to="item.to" :class="isActive(item.to) ? 'mobile-nav-active' : 'text-muted-foreground'" class="flex min-h-9 flex-col items-center justify-center gap-0.5 rounded-xl text-[10px] font-bold transition-[background-color,color,transform] duration-200 active:scale-[0.96]">
          <span class="h-[18px] w-[18px]" v-html="item.icon"></span><span>{{ item.mobileLabel || item.label }}</span>
        </RouterLink>
      </div>
    </nav>

    <Modal v-model="showSecurityModal" title="设置安全问题" :closable="false"><div class="space-y-4"><p class="text-sm leading-relaxed text-muted-foreground">你还没有设置安全问题。设置后，忘记密码时可安全找回账户。</p><div class="flex gap-3"><button class="btn-ghost flex-1" @click="dismissSecurityPrompt">稍后再说</button><button class="btn-brand flex-1" @click="goToSecurityQuestions">去设置</button></div></div></Modal>

    <!-- 支持浮动按钮：手机端靠左，避免遮挡列表右侧的编辑/删除操作 -->
    <button class="fixed bottom-20 left-4 z-40 flex min-h-10 items-center gap-1.5 rounded-full bg-surface px-3.5 shadow-card ring-1 ring-border transition-transform hover:scale-105 lg:bottom-6 lg:left-auto lg:right-6" aria-label="支持作者" @click="showSupportModal = true">
      <svg class="h-4 w-4 text-rose-500" viewBox="0 0 24 24" fill="currentColor"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg>
      <span class="text-sm font-bold">支持</span>
    </button>

    <!-- 支持弹窗 -->
    <SupportModal v-model="showSupportModal" show-actions @dismiss="onSupportDismiss" @supported="onSupportConfirmed" />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { getVersion } from '../api/config'
import UiThemeToggle from '../components/ui/ThemeToggle.vue'
import Modal from '../components/Modal.vue'
import SupportModal from '../components/SupportModal.vue'

const billsIcon = '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 3h12v18l-3-2-3 2-3-2-3 2zM9 8h6m-6 4h6"/></svg>'
const ledgersIcon = '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 4h12a2 2 0 0 1 2 2v14H7a2 2 0 0 1-2-2V4Zm0 13h14M9 8h6"/></svg>'
const sharedIcon = '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 20v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2m7-10a4 4 0 1 0 0-8 4 4 0 0 0 0 8zm8 1v6m3-3h-6"/></svg>'
const analysisIcon = '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 20V10m6 10V4m6 16v-7m4 7H2"/></svg>'
const settingsIcon = '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M8 12h12M4 18h16M8 4v4m8 2v4m-8 2v4"/></svg>'
const categoryIcon = '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.9" d="M4 5.5A1.5 1.5 0 0 1 5.5 4H10v6H4V5.5ZM14 4h4.5A1.5 1.5 0 0 1 20 5.5V10h-6V4ZM4 14h6v6H5.5A1.5 1.5 0 0 1 4 18.5V14Zm10 0h6v4.5a1.5 1.5 0 0 1-1.5 1.5H14v-6Z"/></svg>'
const accountIcon = '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7h18v11H3zM3 10h18m-14 5h4"/></svg>'
const mainNav = [{ to: '/admin/personal/ledgers', label: '我的账本', icon: ledgersIcon }, { to: '/admin/shared', label: '共享账本', icon: sharedIcon }, { to: '/admin/personal', label: '我的账单', icon: billsIcon }, { to: '/admin/analysis', label: '统计分析', icon: analysisIcon }]
const mobileNav = mainNav.map(item => ({ ...item, mobileLabel: item.label === '我的账单' ? '账单' : item.label === '我的账本' ? '账本' : item.label === '共享账本' ? '共享' : '统计' }))
const adminNav = [
  { to: '/admin/users', label: '用户管理', icon: sharedIcon },
  { to: '/admin/configs', label: '系统配置', icon: settingsIcon },
  { to: '/admin/audit', label: '操作日志', icon: billsIcon },
]
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const menuOpen = ref(false)
const menuRef = ref<HTMLElement | null>(null)
const mainRef = ref<HTMLElement | null>(null)
const versionInfo = ref('')
const showSecurityModal = ref(false)

// ===== 赞赏支持状态 =====
const showSupportModal = ref(false)
const appVersion = ref('')
const serverStartedAt = ref('')
// 支持记录按版本号记忆（donate_supported_version）：支持过当前版本后
// 本版本内不再提示；升级到新版本后旧记录失效，横幅与弹窗重新出现
const supportedVersion = ref(localStorage.getItem('donate_supported_version') || '')
const donateSupported = computed(() => !!appVersion.value && supportedVersion.value === appVersion.value)
// 横幅本次会话是否已被手动关闭：关闭不持久化，刷新浏览器后再次出现
const donateBannerClosed = ref(false)

// 顶部横幅：管理员可见；未支持当前版本前常驻展示，支持过当前版本后隐藏
const showDonateBanner = computed(() => {
  return authStore.isAdmin
    && !donateSupported.value
    && !donateBannerClosed.value
})

const dismissDonateBanner = () => {
  donateBannerClosed.value = true
}

// 启动弹窗：登录后首次进入时弹一次；
// 点【暂不支持】后同版本同次运行内不再弹，应用重启后再次提示；
// 【已支持】发送成功后本版本内不再提示，升级到新版本后会再次提示。
// 抑制记录必须「同版本 + 同次运行」才有效：升级换版本后旧记录失效。
const scheduleSupportModal = () => {
  if (!authStore.isAuthenticated) return
  if (!authStore.isAdmin) return
  if (donateSupported.value) return
  const dismissedStart = localStorage.getItem('donate_dismissed_start')
  const dismissedVersion = localStorage.getItem('donate_dismissed_version')
  const sameRun = !!serverStartedAt.value && dismissedStart === serverStartedAt.value
  const sameVersion = !!appVersion.value && dismissedVersion === appVersion.value
  if (sameRun && sameVersion) return
  if (sameRun && !appVersion.value) return
  showSupportModal.value = true
}

// 关闭弹窗（暂不支持 / ✕ / 点遮罩）：同版本同次运行内不再弹出
const onSupportDismiss = () => {
  showSupportModal.value = false
  if (serverStartedAt.value) localStorage.setItem('donate_dismissed_start', serverStartedAt.value)
  if (appVersion.value) localStorage.setItem('donate_dismissed_version', appVersion.value)
}

// 已支持成功：记住当前版本，升级到新版本后会重新提示
const onSupportConfirmed = () => {
  supportedVersion.value = appVersion.value
  if (appVersion.value) localStorage.setItem('donate_supported_version', appVersion.value)
  showSupportModal.value = false
}

const titleMap: Record<string, string> = { '/admin': '总览', '/admin/personal': '我的账单', '/admin/personal/ledgers': '我的账本', '/admin/personal/accounts': '账户管理', '/admin/personal/settings': '分类与标签', '/admin/shared': '共享账本', '/admin/analysis': '统计分析', '/admin/profile': '个人资料', '/admin/settings': '偏好设置', '/admin/users': '用户管理', '/admin/configs': '系统配置', '/admin/audit': '操作日志' }
const currentTitle = computed(() => route.path.startsWith('/admin/shared/') ? '共享账本详情' : titleMap[route.path] || '账单')
const userInitial = computed(() => (authStore.user?.username || 'U').charAt(0).toUpperCase())
const todayText = computed(() => new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }).format(new Date()))
function isActive(to: string) { if (to === '/admin') return route.path === '/admin'; if (to === '/admin/personal') return route.path === to; return route.path.startsWith(to) }
// 退出登录是异步的（要先通知服务端清会话 Cookie），await 后再跳登录页。
async function logout() { menuOpen.value = false; await authStore.logout(); router.push('/login') }
function goToSecurityQuestions() { showSecurityModal.value = false; authStore.dismissSecurityPrompt(); router.push('/admin/profile') }
function dismissSecurityPrompt() { showSecurityModal.value = false; authStore.dismissSecurityPrompt() }
function onClickOutside(event: MouseEvent) { if (menuRef.value && !menuRef.value.contains(event.target as Node)) menuOpen.value = false }
watch(() => authStore.isAuthenticated && !authStore.hasSecurityQuestions && !authStore.securityPromptDismissed, value => { if (value) showSecurityModal.value = true }, { immediate: true })
watch(() => route.fullPath, async () => { menuOpen.value = false; await nextTick(); mainRef.value?.focus({ preventScroll: true }) })
onMounted(async () => {
  document.addEventListener('click', onClickOutside)
  try {
    const res = await getVersion()
    const data = res.data?.data
    if (data) {
      versionInfo.value = `${data.appName} ${data.version}`
      appVersion.value = data.version || ''
      serverStartedAt.value = data.startedAt || ''
    }
  } catch {}
  // 等页面稳定后再弹赞赏窗口，避免与安全问题弹窗叠加
  setTimeout(() => scheduleSupportModal(), 2500)
})
onBeforeUnmount(() => document.removeEventListener('click', onClickOutside))
</script>

<style scoped>
.nav-link { @apply flex min-h-9 items-center gap-2.5 rounded-xl px-2.5 text-[13px] font-bold transition-all duration-200; }
.app-sidebar { background-color: rgb(var(--color-sidebar)); border-color: rgb(var(--color-sidebar-border)); color: rgb(var(--color-sidebar-foreground)); }
.sidebar-muted { color: rgb(var(--color-sidebar-muted)); }
.sidebar-panel, .sidebar-profile { border-color: rgb(var(--color-sidebar-border)); background-color: rgb(var(--color-sidebar-panel) / .68); }
.sidebar-track { background-color: rgb(var(--color-sidebar-muted) / .2); }
.sidebar-logout { color: rgb(var(--color-sidebar-muted)); }
.sidebar-logout:hover { background-color: rgb(var(--color-sidebar-foreground) / .08); color: rgb(var(--color-sidebar-foreground)); }
.sidebar-version { border-color: rgb(var(--color-sidebar-border)); color: rgb(var(--color-sidebar-muted) / .72); }
.nav-link-active { @apply bg-accent text-brand-900 shadow-soft; }
.nav-link-idle { color: rgb(var(--color-sidebar-muted)); }
.nav-link-idle:hover { background-color: rgb(var(--color-sidebar-foreground) / .06); color: rgb(var(--color-sidebar-foreground)); }
.nav-icon { @apply h-[18px] w-[18px] shrink-0; }
.menu-item { @apply flex min-h-10 items-center rounded-lg px-3 text-[13px] font-medium hover:bg-muted; }
.app-bg { position: fixed; inset: 0; z-index: -10; overflow: hidden; pointer-events: none; }
.ledger-orb { position: absolute; width: 28rem; height: 28rem; top: -17rem; right: -12rem; border-radius: 9999px; border: 5rem solid rgb(var(--color-accent) / .13); }
.ledger-dots { position: absolute; inset: 0; opacity: .18; background-image: radial-gradient(rgb(var(--color-brand-800) / .45) .7px, transparent .7px); background-size: 18px 18px; mask-image: linear-gradient(to bottom, #000, transparent 46%); }
.mobile-nav-active { @apply bg-brand-100 text-brand-700 shadow-soft dark:bg-brand-800 dark:text-accent; }
.mobile-bottom-nav {
  bottom: 0;
  padding-bottom: max(.375rem, env(safe-area-inset-bottom));
}
.app-header { box-shadow: 0 1px 0 rgb(var(--color-border) / .72); }

@media (prefers-reduced-transparency: reduce) {
  .app-header, .mobile-bottom-nav { background-color: rgb(var(--color-background)); backdrop-filter: none; }
}
</style>
