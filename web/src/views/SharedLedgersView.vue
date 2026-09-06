<template>
  <div class="page-container animate-fade-in">
    <PageHeader title="共享账本" description="共同记录、按参与人分摊，系统自动算出谁该付给谁。">
      <template #actions><button class="btn-brand min-h-11" @click="showCreate = true"><svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v12m6-6H6"/></svg>新建账本</button></template>
    </PageHeader>

    <div class="inline-flex rounded-xl bg-muted p-1" role="tablist" aria-label="账本状态">
      <button :class="ledgerStatus === 'active' ? 'bg-surface text-foreground shadow-sm' : 'text-muted-foreground'" class="min-h-11 rounded-lg px-4 text-sm font-semibold" role="tab" :aria-selected="ledgerStatus === 'active'" @click="ledgerStatus = 'active'">活跃账本 <span class="ml-1 text-xs opacity-70">{{ activeLedgers.length }}</span></button>
      <button :class="ledgerStatus === 'archived' ? 'bg-surface text-foreground shadow-sm' : 'text-muted-foreground'" class="min-h-11 rounded-lg px-4 text-sm font-semibold" role="tab" :aria-selected="ledgerStatus === 'archived'" @click="ledgerStatus = 'archived'">已归档 <span class="ml-1 text-xs opacity-70">{{ archivedLedgers.length }}</span></button>
    </div>

    <section v-if="invitations.length" class="rounded-2xl border border-amber-300/60 bg-amber-50 p-4 dark:border-amber-500/30 dark:bg-amber-500/10">
      <div class="flex items-center gap-3">
        <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-amber-500/15 text-amber-700 dark:text-amber-300"><svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5h16v14H4zM4 7l8 6 8-6"/></svg></span>
        <div><h2 class="font-semibold">待处理邀请</h2><p class="text-sm text-muted-foreground">接受后即可查看并参与记账。</p></div>
      </div>
      <div class="mt-4 grid gap-3 md:grid-cols-2">
        <div v-for="item in invitations" :key="item.id" class="flex flex-col gap-3 rounded-xl bg-surface p-3 shadow-sm sm:flex-row sm:items-center sm:justify-between sm:p-4">
          <div><strong>{{ item.name }}</strong><p class="mt-1 text-xs text-muted-foreground">{{ item.owner_username }} 邀请你加入</p></div>
          <div class="flex w-full gap-2 sm:w-auto"><button class="btn-ghost min-h-11 flex-1 px-3 py-2 sm:flex-none" @click="respond(item, false)">拒绝</button><button class="btn-brand min-h-11 flex-1 px-3 py-2 sm:flex-none" @click="respond(item, true)">接受</button></div>
        </div>
      </div>
    </section>

    <div v-if="loading" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3"><div v-for="n in 6" :key="n" class="surface h-48 animate-pulse rounded-2xl"></div></div>
    <div v-else-if="!visibleLedgers.length" class="surface empty-state min-h-80 rounded-2xl">
      <svg class="mb-4 h-12 w-12 text-muted-foreground/60" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M4 7h16v12H4zM8 7V5h8v2M8 12h8m-8 4h5"/></svg>
      <h2 class="font-semibold text-foreground">{{ ledgerStatus === 'archived' ? '还没有已归档账本' : '还没有共享账本' }}</h2>
      <p class="mt-1 max-w-sm text-center">{{ ledgerStatus === 'archived' ? '归档账本会保留历史数据，可随时查看。' : '适合旅行、合租、家庭和团队活动，所有人都能共同记账。' }}</p>
      <button v-if="ledgerStatus === 'active'" class="btn-brand mt-5 min-h-11" @click="showCreate = true">创建第一个账本</button>
    </div>
    <div v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <RouterLink v-for="item in visibleLedgers" :key="item.id" :to="`/admin/shared/${item.id}`" class="surface group relative overflow-hidden rounded-2xl p-5 transition-all hover:-translate-y-1 hover:border-brand-300 hover:shadow-card">
        <div class="absolute right-0 top-0 h-24 w-24 translate-x-8 -translate-y-8 rounded-full bg-brand-500/8 transition-transform group-hover:scale-125"></div>
        <div class="relative flex items-start justify-between">
          <div class="flex items-center gap-3">
            <span class="flex h-11 w-11 items-center justify-center rounded-xl bg-brand-500/10 text-brand-600 dark:text-brand-300"><svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7h16v12H4zM8 7V5h8v2"/></svg></span>
            <div><h2 class="font-display font-bold">{{ item.name }}</h2><p class="mt-0.5 text-xs text-muted-foreground">{{ item.member_count }} 位成员</p></div>
          </div>
          <span v-if="item.archived" class="badge bg-amber-500/10 text-amber-700 dark:text-amber-300">已归档</span>
          <span v-else-if="item.member_role === 'owner'" class="badge bg-brand-500/10 text-brand-700 dark:text-brand-300">我创建的</span>
        </div>
        <div class="relative mt-6 grid grid-cols-2 gap-3">
          <div><span class="text-xs text-muted-foreground">累计共同支出</span><strong class="mt-1 block text-lg tabular-nums">{{ formatMoney(item.expense_cents, item.currency) }}</strong></div>
          <div class="text-right"><span class="text-xs text-muted-foreground">{{ item.balance_cents > 0 ? '当前应收' : item.balance_cents < 0 ? '当前应付' : '当前已结清' }}</span><strong :class="item.balance_cents > 0 ? 'text-emerald-600' : item.balance_cents < 0 ? 'text-rose-600' : 'text-foreground'" class="mt-1 block text-lg tabular-nums">{{ formatMoney(Math.abs(item.balance_cents), item.currency) }}</strong></div>
        </div>
        <div class="relative mt-5 flex items-center justify-between border-t border-border pt-4 text-xs text-muted-foreground"><span>{{ item.owner_username }} 创建</span><span class="flex items-center gap-1 font-semibold text-brand-600">进入账本<svg class="h-4 w-4 transition-transform group-hover:translate-x-1" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m9 18 6-6-6-6"/></svg></span></div>
      </RouterLink>
    </div>

    <Modal v-model="showCreate" title="新建共享账本">
      <form class="space-y-4" @submit.prevent="createLedger">
        <label class="form-field"><span>账本名称</span><input v-model="form.name" required maxlength="100" class="input-field" placeholder="例如：周末旅行、合租生活" /></label>
        <label class="form-field"><span>货币</span><select v-model="form.currency" class="input-field"><option value="CNY">人民币 CNY</option><option value="USD">美元 USD</option><option value="EUR">欧元 EUR</option><option value="JPY">日元 JPY</option><option value="HKD">港币 HKD</option></select></label>
        <label class="form-field"><span>邀请成员 <small>可选，填写用户名并用逗号分隔</small></span><textarea v-model="form.usernames" class="input-field min-h-24 resize-y" placeholder="xiaoming, xiaohong"></textarea></label>
        <p class="rounded-xl bg-muted p-3 text-xs leading-relaxed text-muted-foreground">成员会先收到邀请，接受后才能参与记账。你可以稍后继续添加成员。</p>
        <p v-if="formError" class="text-sm text-rose-600">{{ formError }}</p>
        <div class="flex gap-3 pt-2"><button type="button" class="btn-ghost flex-1 min-h-11" @click="showCreate = false">取消</button><button class="btn-brand flex-1 min-h-11" :disabled="saving">{{ saving ? '创建中' : '创建账本' }}</button></div>
      </form>
    </Modal>
    <Toast :message="toast.message" :type="toast.type" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { billingApi, type LedgerListItem } from '../api/billing'
import { apiMessage, formatMoney } from '../utils/billing'
import PageHeader from '../components/PageHeader.vue'
import Modal from '../components/Modal.vue'
import Toast from '../components/Toast.vue'

const loading = ref(false)
const saving = ref(false)
const showCreate = ref(false)
const formError = ref('')
const ledgers = ref<LedgerListItem[]>([])
const ledgerStatus = ref<'active' | 'archived'>('active')
const form = reactive({ name: '', currency: 'CNY', usernames: '' })
const toast = reactive<{ message: string; type: 'success' | 'error' }>({ message: '', type: 'success' })
const invitations = computed(() => ledgers.value.filter(item => item.member_status === 'pending'))
const activeLedgers = computed(() => ledgers.value.filter(item => item.member_status === 'active' && !item.archived))
const archivedLedgers = computed(() => ledgers.value.filter(item => item.member_status === 'active' && item.archived))
const visibleLedgers = computed(() => ledgerStatus.value === 'active' ? activeLedgers.value : archivedLedgers.value)
function notify(message: string, type: 'success' | 'error' = 'success') { toast.message = ''; setTimeout(() => { toast.message = message; toast.type = type }) }
async function load() { loading.value = true; try { const res = await billingApi.ledgers(); ledgers.value = res.data.data || [] } catch (e) { notify(apiMessage(e, '共享账本加载失败'), 'error') } finally { loading.value = false } }
async function respond(item: LedgerListItem, accept: boolean) { try { await billingApi.respondInvitation(item.id, accept); notify(accept ? '已加入共享账本' : '已拒绝邀请'); load() } catch (e) { notify(apiMessage(e), 'error') } }
async function createLedger() { if (!form.name.trim()) return; saving.value = true; formError.value = ''; try { await billingApi.ledgerCreate({ name: form.name.trim(), currency: form.currency, usernames: form.usernames.split(/[,，\s]+/).map(item => item.trim()).filter(Boolean) }); showCreate.value = false; Object.assign(form, { name: '', currency: 'CNY', usernames: '' }); notify('共享账本已创建'); load() } catch (e) { formError.value = apiMessage(e) } finally { saving.value = false } }
onMounted(load)
</script>
