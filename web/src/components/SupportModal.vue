<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="modelValue" class="fixed inset-0 z-50 flex items-end justify-center lg:items-center lg:p-4" @click.self="close">
        <div class="absolute inset-0 bg-black/55" @click="close"></div>
        <div role="dialog" aria-modal="true" aria-labelledby="support-modal-title" class="modal-panel relative max-h-[92dvh] w-full max-w-sm overflow-auto rounded-t-[1.5rem] bg-surface px-6 pb-6 pt-5 text-center text-foreground shadow-xl lg:max-h-[90vh] lg:rounded-2xl">
          <button class="icon-button absolute right-3 top-3" aria-label="关闭" @click="close">
            <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>

          <h3 id="support-modal-title" class="font-display text-xl font-extrabold text-brand-700 dark:text-accent">☕ 请站长喝杯咖啡？</h3>

          <div class="mt-3 space-y-1 text-sm leading-6 text-muted-foreground">
            <p>嘿～看到这里的都是真爱！🎉</p>
            <p>愿意支持的随意一下，感觉麻烦的直接跳过～</p>
            <p class="text-xs italic opacity-80">反正我也就随便说说，你也就随便看看 😜</p>
          </div>

          <div class="my-4 flex justify-center">
            <div>
              <img :src="qrImage" alt="微信收款码" class="h-auto w-[190px] rounded-lg border border-border bg-white object-contain" @error="hideOnError" />
              <span class="mt-1.5 block text-[13px] text-muted-foreground">微信扫码</span>
            </div>
          </div>

          <div class="mt-4 flex justify-center gap-4 border-t border-border pt-4">
            <a href="http://techfunway.wycto.cn" target="_blank" rel="noopener noreferrer" class="text-[13px] text-brand-700 underline-offset-2 hover:underline dark:text-accent">🏠 博主首页</a>
            <a href="http://techfunway.wycto.cn" target="_blank" rel="noopener noreferrer" class="text-[13px] text-brand-700 underline-offset-2 hover:underline dark:text-accent">📖 应用文档</a>
          </div>

          <template v-if="showActions">
            <p class="mt-3 text-xs leading-5 text-muted-foreground">支付后可点击下方按钮发送一次匿名支持计数（仅设备统计信息，与你的微信账号和支付记录无任何关联）</p>
            <p v-if="errorText" class="mt-2 text-xs text-rose-500">{{ errorText }}</p>
            <div class="mt-3 flex justify-center gap-3">
              <button class="btn-ghost min-h-11 flex-1" @click="close">暂不支持</button>
              <button class="btn-brand min-h-11 flex-1" :disabled="sending" @click="confirmSupported">{{ sending ? '发送中…' : '❤ 已支持' }}</button>
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
