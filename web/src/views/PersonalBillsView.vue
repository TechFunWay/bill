<template>
  <div class="page-container animate-fade-in">
    <PageHeader title="我的账单" description="每笔账单归入一个个人账本，用标签串联不同场景。">
      <template #actions>
        <RouterLink to="/admin/personal/settings" class="btn-ghost min-h-11">分类与标签</RouterLink>
        <button class="btn-ghost min-h-11" :disabled="exporting" @click="exportCsv">{{ exporting ? '导出中' : '导出 CSV' }}</button>
        <button class="btn-brand min-h-11" :disabled="!ledgers.length || !accounts.length" @click="openCreate">
          <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v12m6-6H6"/></svg>
          记一笔
        </button>
      </template>
    </PageHeader>

    <section v-if="!resourcesLoading && !ledgers.length" class="surface rounded-2xl p-6 sm:p-8">
      <div class="mx-auto flex max-w-xl flex-col items-center text-center">
        <span class="flex h-12 w-12 items-center justify-center rounded-2xl bg-brand-100 text-brand-700 dark:bg-brand-800 dark:text-accent">
          <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M4 6h16v14H4zM8 6V3h8v3m-8 5h8"/></svg>
        </span>
        <h2 class="mt-4 font-display text-xl font-bold">先创建一个个人账本</h2>
        <p class="mt-2 text-sm leading-6 text-muted-foreground">生活、交通或旅行可以分别记账。个人账本始终只属于你，不需要借用共享账本。</p>
        <RouterLink to="/admin/personal/ledgers" class="btn-brand mt-5 min-h-11">创建个人账本</RouterLink>
      </div>
    </section>

    <section v-else-if="!resourcesLoading && !accounts.length" class="surface rounded-2xl p-6 sm:p-8">
      <div class="mx-auto max-w-xl text-center"><h2 class="font-display text-xl font-bold">先添加一个收支账户</h2><p class="mt-2 text-sm leading-6 text-muted-foreground">个人账单需要明确资金从哪里支出或收入到哪里。银行卡只保存银行名称与卡号后四位。</p><RouterLink to="/admin/personal/accounts" class="btn-brand mt-5 min-h-11">前往账户管理</RouterLink></div>
    </section>

    <template v-else>
      <section class="surface rounded-2xl p-3 sm:p-4">
        <MobileFilterDisclosure v-model="filtersExpanded" panel-id="personal-bill-filters" title="筛选账单" :summary="billFilterSummary" :active-count="activeFilterCount">
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-[1.3fr_repeat(6,1fr)_auto]">
          <label class="filter-field sm:col-span-2 xl:col-span-1"><span>搜索</span><input v-model="filters.q" class="input-field" placeholder="备注或分类" @keyup.enter="applyFilters" /></label>
          <label class="filter-field"><span>账本</span><select v-model="filters.ledger_id" class="input-field"><option value="">全部账本</option><option v-for="item in ledgers" :key="item.id" :value="String(item.id)">{{ item.name }}</option></select></label>
          <label class="filter-field"><span>账户</span><select v-model="filters.account_id" class="input-field"><option value="">全部账户</option><option v-for="item in accounts" :key="item.id" :value="String(item.id)">{{ item.name }}</option></select></label>
          <label class="filter-field"><span>类型</span><select v-model="filters.kind" class="input-field"><option value="">全部</option><option value="expense">支出</option><option value="income">收入</option></select></label>
          <label class="filter-field"><span>分类</span><select v-model="filters.category" class="input-field"><option value="">全部</option><option v-for="item in allCategoryNames" :key="item">{{ item }}</option></select></label>
          <label class="filter-field"><span>标签</span><select v-model="filters.tag_id" class="input-field"><option value="">全部</option><option v-for="item in tags" :key="item.id" :value="String(item.id)">{{ item.name }}</option></select></label>
          <label class="filter-field"><span>日期范围</span><div class="grid grid-cols-1 gap-2 sm:grid-cols-2"><input v-model="filters.start" type="date" class="input-field px-3" aria-label="开始日期" /><input v-model="filters.end" type="date" class="input-field px-3" aria-label="结束日期" /></div></label>
          <div class="flex items-end gap-2 sm:col-span-2 xl:col-span-1"><button type="button" class="btn-ghost min-h-11 flex-1 px-3" :disabled="!activeFilterCount" @click="resetFilters">重置</button><button type="button" class="btn-brand min-h-11 flex-1 px-4" @click="applyFilters">筛选</button></div>
          </div>
        </MobileFilterDisclosure>
      </section>

      <section class="surface overflow-hidden rounded-2xl">
        <div v-if="loading" class="space-y-3 p-5"><div v-for="n in 6" :key="n" class="h-16 animate-pulse rounded-xl bg-muted"></div></div>
        <div v-else-if="!items.length" class="empty-state h-72"><span>没有找到符合条件的账单</span><button class="mt-4 min-h-11 text-sm font-semibold text-brand-600" @click="openCreate">记下第一笔</button></div>
        <template v-else>
          <div class="hidden overflow-x-auto lg:block">
            <table class="data-table">
              <thead><tr><th>日期</th><th>账本 / 账户</th><th>分类 / 备注</th><th>标签</th><th class="text-right">金额</th><th class="w-24 text-right">操作</th></tr></thead>
              <tbody><tr v-for="item in items" :key="item.id">
                <td class="whitespace-nowrap text-sm text-muted-foreground">{{ formatDate(item.occurred_at) }}</td>
                <td><div class="flex flex-col items-start gap-1"><span class="badge bg-brand-100 text-brand-700 dark:bg-brand-800 dark:text-accent">{{ ledgerName(item) }}</span><span class="text-xs text-muted-foreground">{{ item.account_name || '未设置账户' }}</span></div></td>
                <td><div class="flex items-start gap-3"><span :class="item.kind === 'income' ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300' : 'bg-rose-500/10 text-rose-700 dark:text-rose-300'" class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl"><CategoryIcon :category="item.category" :kind="item.kind" class="h-4 w-4" /></span><div class="min-w-0"><div class="font-semibold">{{ item.category }}</div><div v-if="item.note" class="mt-0.5 max-w-sm truncate text-xs text-muted-foreground">{{ item.note }}</div><BillAttachments v-if="item.attachments?.length" class="mt-2" :model-value="item.attachments" readonly compact /></div></div></td>
                <td><div class="flex max-w-xs flex-wrap gap-1"><span v-for="tag in item.tags || []" :key="tag.id" class="badge border border-border bg-muted">{{ tag.name }}</span><span v-if="!item.tags?.length" class="text-xs text-muted-foreground">—</span></div></td>
                <td :class="item.kind === 'income' ? 'text-emerald-600' : 'text-foreground'" class="text-right font-bold tabular-nums">{{ item.kind === 'income' ? '+' : '-' }}{{ formatMoney(item.amount_cents) }}</td>
                <td><div class="flex justify-end gap-1"><button class="icon-button" aria-label="编辑账单" @click="openEdit(item)"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 20h4L19 9l-4-4L4 16v4z"/></svg></button><button class="icon-button text-rose-600" aria-label="删除账单" @click="askDelete(item)"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7h16M9 7V4h6v3m-9 0 1 14h10l1-14"/></svg></button></div></td>
              </tr></tbody>
            </table>
          </div>
          <div class="divide-y divide-border lg:hidden">
            <article v-for="item in items" :key="item.id" class="px-3 py-2 transition-colors active:bg-muted/50">
              <div class="flex items-start gap-2">
                <span :class="item.kind === 'income' ? 'bg-emerald-500/10 text-emerald-600' : 'bg-rose-500/10 text-rose-600'" class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg">
                  <CategoryIcon :category="item.category" :kind="item.kind" class="h-4 w-4" />
                </span>
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-1">
                    <h3 class="text-[15px] font-semibold leading-5">{{ item.category }}</h3>
                    <span class="badge bg-brand-100 px-2 py-0.5 leading-4 text-brand-700 dark:bg-brand-800 dark:text-accent">{{ ledgerName(item) }}</span>
                  </div>
                  <p class="truncate text-xs leading-4 text-muted-foreground">{{ item.account_name || '未设置账户' }} · {{ item.note || formatDate(item.occurred_at) }}</p>
                  <div v-if="item.tags?.length" class="mt-1 flex flex-wrap gap-1">
                    <span v-for="tag in item.tags" :key="tag.id" class="badge border border-border bg-muted px-2 py-0.5 leading-4">{{ tag.name }}</span>
                  </div>
                  <BillAttachments v-if="item.attachments?.length" class="mt-1" :model-value="item.attachments" readonly compact />
                </div>
                <strong :class="item.kind === 'income' ? 'text-emerald-600' : ''" class="shrink-0 whitespace-nowrap text-[15px] leading-5 tabular-nums">{{ item.kind === 'income' ? '+' : '-' }}{{ formatMoney(item.amount_cents) }}</strong>
              </div>
              <div class="mt-0.5 flex items-center justify-between">
                <span class="text-xs leading-4 text-muted-foreground">{{ formatDate(item.occurred_at) }}</span>
                <div class="flex">
                  <button class="icon-button text-brand-600" aria-label="编辑账单" @click="openEdit(item)"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 20h4L19 9l-4-4L4 16v4z"/></svg></button>
                  <button class="icon-button text-rose-600" aria-label="删除账单" @click="askDelete(item)"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7h16M9 7V4h6v3m-9 0 1 14h10l1-14"/></svg></button>
                </div>
              </div>
            </article>
          </div>
          <div class="flex items-center justify-between border-t border-border px-3 py-2.5 sm:px-4 sm:py-3"><span class="text-xs text-muted-foreground">共 {{ total }} 笔</span><div class="flex gap-2"><button class="btn-ghost min-h-11 px-3" :disabled="page <= 1" @click="changePage(page - 1)">上一页</button><button class="btn-ghost min-h-11 px-3" :disabled="page * pageSize >= total" @click="changePage(page + 1)">下一页</button></div></div>
        </template>
      </section>
    </template>

    <Modal v-model="showForm" :title="editing ? '编辑账单' : '记一笔'">
      <form class="space-y-4" @submit.prevent="save">
        <div class="grid grid-cols-2 gap-2 rounded-xl bg-muted p-1" role="radiogroup" aria-label="账单类型"><button type="button" role="radio" :aria-checked="form.kind === 'expense'" :class="form.kind === 'expense' ? 'bg-surface text-rose-600 shadow-sm' : 'text-muted-foreground'" class="min-h-11 rounded-lg text-sm font-semibold" @click="setKind('expense')">支出</button><button type="button" role="radio" :aria-checked="form.kind === 'income'" :class="form.kind === 'income' ? 'bg-surface text-emerald-600 shadow-sm' : 'text-muted-foreground'" class="min-h-11 rounded-lg text-sm font-semibold" @click="setKind('income')">收入</button></div>
        <label class="form-field"><span>所属账本</span><select v-model.number="form.ledger_id" required class="input-field"><option :value="0" disabled>请选择账本</option><option v-for="item in ledgers" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
        <label class="form-field"><span>收支账户</span><select v-model.number="form.account_id" required class="input-field"><option :value="0" disabled>请选择账户</option><option v-for="item in accounts" :key="item.id" :value="item.id">{{ item.name }} · {{ accountDetail(item) }}</option></select></label>
        <label class="form-field"><span>金额</span><div class="relative"><span class="absolute left-4 top-1/2 -translate-y-1/2 font-semibold text-muted-foreground">¥</span><input v-model="form.amount" required inputmode="decimal" class="input-field pl-9 text-lg font-bold" placeholder="0.00" /></div></label>
        <CategoryPicker v-model="form.category" :kind="form.kind" :categories="categories" />
        <PersonalTagInput v-model="form.tag_ids" :tags="tags" @created="handleTagCreated" />
        <label class="form-field"><span>发生时间</span><input v-model="form.occurred_at" required type="datetime-local" class="input-field" /></label>
        <label class="form-field"><span>备注 <small>可选</small></span><textarea v-model="form.note" class="input-field min-h-24 resize-y" maxlength="500" placeholder="记录这笔钱的用途"></textarea></label>
        <BillAttachments v-model="form.attachments" @uploading="attachmentUploading = $event" />
        <p v-if="formError" role="alert" class="text-sm text-rose-600">{{ formError }}</p>
        <div class="flex gap-3 pt-2"><button type="button" class="btn-ghost min-h-11 flex-1" @click="showForm = false">取消</button><button class="btn-brand min-h-11 flex-1" :disabled="saving || attachmentUploading">{{ attachmentUploading ? '图片上传中' : saving ? '保存中' : '保存' }}</button></div>
      </form>
    </Modal>
    <ConfirmDialog v-model="showDelete" title="删除这笔账单？" message="删除后无法恢复，相关统计会同步更新。" confirm-text="删除" confirm-type="danger" @confirm="deleteItem" />
    <Toast :message="toast.message" :type="toast.type" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { billingApi, type BillAttachment, type BillKind, type PersonalAccount, type PersonalCategory, type PersonalLedger, type PersonalTag, type PersonalTransaction } from '../api/billing'
import { apiMessage, centsToMoney, formatDate, formatMoney, moneyToCents, toLocalInput } from '../utils/billing'
import PageHeader from '../components/PageHeader.vue'
import MobileFilterDisclosure from '../components/MobileFilterDisclosure.vue'
import Modal from '../components/Modal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import Toast from '../components/Toast.vue'
import CategoryIcon from '../components/billing/CategoryIcon.vue'
import CategoryPicker from '../components/billing/CategoryPicker.vue'
import PersonalTagInput from '../components/billing/PersonalTagInput.vue'
import BillAttachments from '../components/billing/BillAttachments.vue'

const loading = ref(false), resourcesLoading = ref(false), saving = ref(false), exporting = ref(false), attachmentUploading = ref(false)
const route = useRoute()
const items = ref<PersonalTransaction[]>([]), ledgers = ref<PersonalLedger[]>([]), accounts = ref<PersonalAccount[]>([]), categories = ref<PersonalCategory[]>([]), tags = ref<PersonalTag[]>([])
const total = ref(0), page = ref(1), pageSize = 20
const filters = reactive({ q: '', ledger_id: '', account_id: '', tag_id: '', kind: '', category: '', start: '', end: '' })
const filtersExpanded = ref(false)
const activeFilterCount = computed(() => Object.values(filters).filter(value => String(value).trim() !== '').length)
const billFilterSummary = computed(() => activeFilterCount.value ? `已启用 ${activeFilterCount.value} 项条件` : '按账本、账户、日期等查找')
const showForm = ref(false), showDelete = ref(false)
const editing = ref<PersonalTransaction | null>(null), deleting = ref<PersonalTransaction | null>(null)
const formError = ref('')
const toast = reactive<{ message: string; type: 'success' | 'error' }>({ message: '', type: 'success' })
const form = reactive({ ledger_id: 0, account_id: 0, kind: 'expense' as BillKind, amount: '', category: '', tag_ids: [] as number[], note: '', occurred_at: toLocalInput(), attachments: [] as BillAttachment[] })
const categoryNames = (kind: BillKind) => categories.value.filter(item => item.kind === kind).map(item => item.name)
const currentCategoryNames = computed(() => {
  const names = categoryNames(form.kind)
  if (editing.value?.kind === form.kind && editing.value.category && !names.includes(editing.value.category)) return [editing.value.category, ...names]
  return names
})
const allCategoryNames = computed(() => [...new Set([...categoryNames('expense'), ...categoryNames('income')])])

function listData<T>(response: any): T[] { const data = response.data?.data; return Array.isArray(data) ? data : data?.items || [] }
function notify(message: string, type: 'success' | 'error' = 'success') { toast.message = ''; setTimeout(() => { toast.message = message; toast.type = type }) }
function params() { return { ...filters, page: page.value, pageSize } }
function ledgerName(item: PersonalTransaction) { return item.ledger_name || ledgers.value.find(ledger => ledger.id === item.ledger_id)?.name || '未知账本' }
function accountDetail(item: PersonalAccount) { return item.type === 'bank_card' ? `${item.institution} · ${item.card_kind === 'credit' ? '信用卡' : item.card_kind === 'debit' ? '储蓄卡' : '银行卡'} · ${item.identifier ? `尾号 ${item.identifier}` : '未填写卡号'}` : item.institution }
function syncLedgerFilterFromRoute() { const raw = Array.isArray(route.query.ledger_id) ? route.query.ledger_id[0] : route.query.ledger_id; const id = Number(raw); filters.ledger_id = Number.isInteger(id) && ledgers.value.some(ledger => ledger.id === id) ? String(id) : '' }
async function loadResources() { resourcesLoading.value = true; try { const [ledgerRes, accountRes, categoryRes, tagRes] = await Promise.all([billingApi.personalLedgers(), billingApi.personalAccounts(), billingApi.personalCategories(), billingApi.personalTags()]); ledgers.value = listData(ledgerRes); accounts.value = listData(accountRes); categories.value = listData(categoryRes); tags.value = listData(tagRes) } catch (error) { notify(apiMessage(error, '记账资料加载失败'), 'error') } finally { resourcesLoading.value = false } }
async function load() { if (!ledgers.value.length) { items.value = []; total.value = 0; return } loading.value = true; try { const res = await billingApi.personalList(params()); items.value = res.data.data?.items || []; total.value = res.data.data?.total || 0 } catch (error) { notify(apiMessage(error, '账单加载失败'), 'error') } finally { loading.value = false } }
function applyFilters() { page.value = 1; filtersExpanded.value = false; load() }
function resetFilters() { Object.assign(filters, { q: '', ledger_id: '', account_id: '', tag_id: '', kind: '', category: '', start: '', end: '' }); applyFilters() }
function changePage(value: number) { page.value = value; load() }
function resetForm() { editing.value = null; Object.assign(form, { ledger_id: Number(filters.ledger_id) || ledgers.value[0]?.id || 0, account_id: Number(filters.account_id) || accounts.value[0]?.id || 0, kind: 'expense', amount: '', category: '', tag_ids: [], note: '', occurred_at: toLocalInput(), attachments: [] }); formError.value = ''; attachmentUploading.value = false }
function openCreate() { if (!ledgers.value.length) return; resetForm(); showForm.value = true }
function openEdit(item: PersonalTransaction) { editing.value = item; Object.assign(form, { ledger_id: item.ledger_id, account_id: item.account_id, kind: item.kind, amount: centsToMoney(item.amount_cents), category: item.category, tag_ids: (item.tags || []).map(tag => tag.id), note: item.note, occurred_at: toLocalInput(item.occurred_at), attachments: [...(item.attachments || [])] }); formError.value = ''; attachmentUploading.value = false; showForm.value = true }
function setKind(kind: BillKind) { form.kind = kind; if (!currentCategoryNames.value.includes(form.category)) form.category = '' }
function handleTagCreated(tag: PersonalTag) { if (!tags.value.some(item => item.id === tag.id)) tags.value.push(tag) }
async function save() { const amount = moneyToCents(form.amount); if (!form.ledger_id) { formError.value = '请选择账本'; return } if (!form.account_id) { formError.value = '请选择收支账户'; return } if (amount <= 0) { formError.value = '请输入大于 0 的金额'; return } if (!form.category) { formError.value = '请选择分类'; return } saving.value = true; formError.value = ''; try { const payload = { ledger_id: form.ledger_id, account_id: form.account_id, kind: form.kind, amount_cents: amount, category: form.category, tag_ids: form.tag_ids, note: form.note.trim(), occurred_at: new Date(form.occurred_at).toISOString(), attachments: form.attachments.map(({ path, name }) => ({ path, name })) }; if (editing.value) await billingApi.personalUpdate(editing.value.id, payload); else await billingApi.personalCreate(payload); showForm.value = false; notify(editing.value ? '账单已更新' : '账单已记录'); load() } catch (error) { formError.value = apiMessage(error) } finally { saving.value = false } }
function askDelete(item: PersonalTransaction) { deleting.value = item; showDelete.value = true }
async function deleteItem() { if (!deleting.value) return; try { await billingApi.personalDelete(deleting.value.id); notify('账单已删除'); if (items.value.length === 1 && page.value > 1) page.value--; load() } catch (error) { notify(apiMessage(error), 'error') } }
async function exportCsv() { exporting.value = true; try { const res = await billingApi.personalExport(filters); const url = URL.createObjectURL(new Blob([res.data], { type: 'text/csv;charset=utf-8' })); const anchor = document.createElement('a'); anchor.href = url; anchor.download = `个人账单-${new Date().toISOString().slice(0, 10)}.csv`; anchor.click(); URL.revokeObjectURL(url) } catch (error) { notify(apiMessage(error, '导出失败'), 'error') } finally { exporting.value = false } }
watch(() => route.query.ledger_id, async () => { if (resourcesLoading.value) return; syncLedgerFilterFromRoute(); page.value = 1; await load() })
onMounted(async () => { await loadResources(); syncLedgerFilterFromRoute(); await load() })
</script>
