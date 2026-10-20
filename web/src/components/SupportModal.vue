<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="modelValue" class="fixed inset-0 z-50 flex items-center justify-center p-4" @click.self="close">
        <div class="absolute inset-0 bg-black/55" @click="close"></div>
        <!-- 精简为一屏展示（mobile-ui 规范 2026-10-01） -->
        <div role="dialog" aria-modal="true" aria-labelledby="support-modal-title" class="modal-panel relative w-full max-w-[320px] rounded-2xl bg-surface px-5 pb-4 pt-5 text-center text-foreground shadow-xl">
          <button class="icon-button absolute right-2.5 top-2.5" aria-label="关闭" @click="close">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>

          <h3 id="support-modal-title" class="font-display text-base font-extrabold text-brand-700 dark:text-accent">☕ 请站长喝杯咖啡？</h3>

          <p class="mt-2 text-xs leading-relaxed text-muted-foreground">
            应用免费无广告、数据保存在本机。若对你有帮助，欢迎赞赏——<strong class="text-foreground">金额随意，1 元也是心意</strong>。
          </p>

          <div class="my-2.5 flex justify-center">
            <div>
              <img :src="qrImage" alt="微信收款码" class="h-auto w-[150px] rounded-lg border border-border bg-white object-contain" @error="hideOnError" />
              <span class="mt-1 block text-[11px] text-muted-foreground">微信扫码</span>
            </div>
          </div>

          <div class="mt-3 flex justify-center gap-3 border-t border-border pt-3">
            <a href="http://techfunway.wycto.cn" target="_blank" rel="noopener noreferrer" class="text-[11px] text-brand-700 underline-offset-2 hover:underline dark:text-accent">🏠 博主首页</a>
            <a href="http://techfunway.wycto.cn" target="_blank" rel="noopener noreferrer" class="text-[11px] text-brand-700 underline-offset-2 hover:underline dark:text-accent">📖 应用文档</a>
          </div>

          <template v-if="showActions">
            <p class="mt-2.5 text-[11px] leading-4 text-muted-foreground/80">已支付可点「已支持」发送一次匿名计数（与支付记录无关）</p>
            <p v-if="errorText" class="mt-1.5 text-xs text-rose-500">{{ errorText }}</p>
            <div class="mt-2.5 flex justify-center gap-2.5">
              <button class="btn-ghost min-h-9 flex-1 py-2 text-sm" @click="close">暂不支持</button>
              <button class="btn-brand min-h-9 flex-1 py-2 text-sm" :disabled="sending" @click="confirmSupported">{{ sending ? '发送中…' : '❤ 已支持' }}</button>
            </div>
          </template>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import request from '../api/request'
import qrImage from '../assets/wechat-qr.png'

defineProps<{
  modelValue: boolean
  // 是否显示底部操作按钮（暂不支持 / 已支持）
  showActions?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  /** 用户点【暂不支持】或关闭：本次运行内不再提示 */
  dismiss: []
  /** 【已支持】上报成功：永不再提示 */
  supported: []
}>()

const sending = ref(false)
const errorText = ref('')

const close = () => {
  emit('dismiss')
  emit('update:modelValue', false)
}

const hideOnError = (e: Event) => {
  const img = e.currentTarget as HTMLImageElement
  img.style.display = 'none'
}

// 【已支持】发送匿名支持计数，成功才通知父组件永久记住
const confirmSupported = async () => {
  sending.value = true
  errorText.value = ''
  let ok = false
  try {
    const res = await request.post('/api/donate/support')
    ok = res.data?.data?.ok === true
  } catch {
    ok = false
  }
  sending.value = false
  if (ok) {
    emit('supported')
    emit('update:modelValue', false)
  } else {
    errorText.value = '发送失败，请稍后重试（不影响你的支持 ❤）'
  }
}
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
