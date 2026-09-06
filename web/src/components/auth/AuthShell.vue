<template>
  <div class="relative min-h-dvh overflow-hidden bg-[#11121a] text-white">
    <div class="pointer-events-none absolute inset-0" aria-hidden="true">
      <div class="absolute -right-56 -top-56 h-[34rem] w-[34rem] rounded-full border-[6rem] border-[#ff8969]/10"></div>
      <div class="auth-grid absolute inset-0 opacity-25"></div>
    </div>
    <main class="relative z-10 flex min-h-dvh items-center justify-center px-5 py-10 sm:px-8">
      <div class="grid w-full max-w-5xl items-center gap-12 lg:grid-cols-[1fr_28rem]">
        <section class="hidden lg:block">
          <div class="flex items-center gap-3">
            <span class="flex h-12 w-12 items-center justify-center rounded-full bg-[#ff8969] text-[#1d2246] shadow-[0_16px_40px_-12px_rgba(255,137,105,.4)]">
              <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M6 3h12v18l-3-2-3 2-3-2-3 2V3Zm3 5h6m-6 4h6"/></svg>
            </span>
            <div><div class="font-display text-xl font-extrabold">账单</div><div class="font-mono text-[9px] uppercase tracking-[0.18em] text-white/35">Bill Ledger</div></div>
          </div>
          <p class="mt-16 font-mono text-[10px] uppercase tracking-[0.2em] text-[#ff9b80]">Money, made clear.</p>
          <h1 class="mt-4 max-w-xl font-display text-4xl font-extrabold leading-[1.12] tracking-[-0.04em] xl:text-5xl">自己的钱看得懂，<br /><span class="text-white/45">共同的钱算得清。</span></h1>
          <p class="mt-6 max-w-lg text-base leading-relaxed text-white/50">记录个人收支、多人共同分摊和补差结算，让每一次消费都有清晰去向。</p>
          <div class="mt-10 max-w-lg overflow-hidden rounded-[1.5rem] border border-white/10 bg-white/[0.04]">
            <div class="flex items-center justify-between border-b border-white/10 px-5 py-4">
              <span class="font-mono text-[10px] uppercase tracking-[0.16em] text-white/35">Monthly ledger</span>
              <span class="h-2.5 w-2.5 rounded-full bg-[#ff8969]"></span>
            </div>
            <div class="grid grid-cols-3 divide-x divide-white/10">
              <div v-for="(item, index) in benefits" :key="item.title" class="p-4">
                <div class="font-mono text-[10px] text-white/30">0{{ index + 1 }}</div>
                <div class="mt-3 text-sm font-bold">{{ item.title }}</div><div class="mt-1 text-[11px] leading-relaxed text-white/35">{{ item.desc }}</div>
              </div>
            </div>
          </div>
        </section>
        <section>
          <div class="mb-6 flex items-center justify-center gap-3 lg:hidden">
            <span class="flex h-11 w-11 items-center justify-center rounded-full bg-[#ff8969] text-[#1d2246]"><svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M6 3h12v18l-3-2-3 2-3-2-3 2V3Zm3 5h6m-6 4h6"/></svg></span>
            <div><div class="font-display text-xl font-extrabold">账单</div><div class="font-mono text-[9px] uppercase tracking-[0.18em] text-white/35">Bill Ledger</div></div>
          </div>
          <div class="rounded-[1.75rem] border border-white/10 bg-[#1b1d2a]/90 p-6 shadow-2xl backdrop-blur-xl sm:p-9">
            <p class="font-mono text-[10px] uppercase tracking-[0.16em] text-[#ff9b80]">Secure access</p>
            <h2 class="mt-2 font-display text-2xl font-extrabold tracking-tight">{{ title }}</h2>
            <p v-if="subtitle" class="mt-2 text-sm text-white/45">{{ subtitle }}</p>
            <div class="mt-7"><slot /></div>
          </div>
          <div v-if="$slots.footer" class="mt-6 text-center text-sm text-white/45"><slot name="footer" /></div>
        </section>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
const benefits = [
  { title: '个人账单', desc: '收支、分类与趋势分析' },
  { title: '共同分摊', desc: '四种分法灵活处理' },
  { title: '自动补差', desc: '算清谁该付给谁' },
]
</script>

<style scoped>
.auth-grid {
  background-image: linear-gradient(rgba(255, 137, 105, .08) 1px, transparent 1px);
  background-size: 100% 32px;
  mask-image: linear-gradient(to right, black, transparent 88%);
}
</style>
