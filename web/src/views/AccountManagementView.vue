<template>
  <div class="page-container animate-fade-in">
    <PageHeader title="账户管理" description="管理记账使用的银行卡、微信和支付宝账户。银行卡号可以稍后补充，填写后也不会保存完整号码。">
      <template #actions><button class="btn-brand min-h-11 w-full sm:w-auto" @click="openCreate">添加账户</button></template>
    </PageHeader>
    <section class="grid grid-cols-3 gap-2 sm:gap-3"><article v-for="summary in summaries" :key="summary.type" class="surface min-w-0 rounded-2xl p-3 sm:p-4"><p class="truncate text-xs text-muted-foreground sm:text-sm">{{ summary.label }}</p><strong class="mt-2 block font-display text-xl sm:text-2xl">{{ summary.count }}</strong></article></section>
    <div class="flex flex-wrap items-center justify-between gap-2"><h2 class="font-display text-lg font-bold">我的收支账户</h2><label class="inline-flex min-h-11 cursor-pointer items-center gap-2 text-sm text-muted-foreground"><input v-model="showArchived" type="checkbox" class="h-4 w-4 accent-brand-700" />显示已归档</label></div>
    <section v-if="loading" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3"><div v-for="n in 6" :key="n" class="surface h-40 animate-pulse rounded-2xl"></div></section>
    <section v-else-if="!visibleAccounts.length" class="surface empty-state min-h-64 rounded-2xl"><p>还没有账户</p><button class="btn-brand mt-4 min-h-11" @click="openCreate">添加第一个账户</button></section>
    <section v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <article v-for="account in visibleAccounts" :key="account.id" :class="account.archived ? 'opacity-65' : ''" class="surface rounded-2xl p-4 sm:p-5">
        <div class="flex items-start gap-3 sm:gap-4"><span v-if="account.type !== 'bank_card'" :class="typeStyle(account.type)" class="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl" v-html="typeIcon(account.type)"></span><BankLogo v-else :bank="bankByName(account.institution)" class="m-1"/><div class="min-w-0 flex-1"><div class="flex flex-wrap items-center gap-2"><h3 class="truncate font-display text-lg font-bold">{{ account.name }}</h3><span v-if="account.archived" class="badge bg-muted text-muted-foreground">已归档</span></div><p class="mt-1 break-words text-sm text-muted-foreground">{{ accountDetail(account) }}</p></div></div>
        <div class="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3 sm:mt-5 sm:pt-4"><span class="text-xs text-muted-foreground">已关联 {{ account.transaction_count || 0 }} 笔账单</span><div class="flex gap-2"><button v-if="!account.archived" class="min-h-11 px-2 text-sm font-semibold text-brand-600" @click="openEdit(account)">编辑</button><button v-if="!account.archived" class="min-h-11 px-2 text-sm font-semibold text-rose-600" @click="askArchive(account)">归档</button><button v-else class="min-h-11 px-2 text-sm font-semibold text-brand-600" @click="restore(account)">恢复</button></div></div>
      </article>
    </section>
    <Modal v-model="showForm" :title="editing ? '编辑账户' : '添加账户'">
      <form class="space-y-4 overscroll-contain" @submit.prevent="save">
        <fieldset><legend class="text-sm font-semibold">账户类型</legend><div class="mt-2 grid grid-cols-3 gap-2"><button v-for="option in types" :key="option.value" type="button" :aria-pressed="form.type === option.value" :class="form.type === option.value ? 'border-brand-500 bg-brand-100 text-brand-800 dark:bg-brand-800 dark:text-accent' : 'border-border bg-surface text-muted-foreground'" class="min-h-12 touch-manipulation rounded-xl border px-1 text-sm font-semibold transition-colors" @click="selectType(option.value)">{{ option.label }}</button></div></fieldset>
        <label class="form-field"><span>账户名称</span><input v-model.trim="form.name" required maxlength="80" class="input-field" :placeholder="form.type === 'bank_card' ? '例如：工资卡' : form.type === 'wechat' ? '例如：日常微信' : '例如：生活支付宝'" /></label>
        <template v-if="form.type === 'bank_card'">
          <fieldset><legend class="text-sm font-semibold">银行卡类型</legend><div class="mt-2 grid grid-cols-2 gap-2 rounded-xl bg-muted p-1"><button v-for="option in cardKinds" :key="option.value" type="button" :aria-pressed="form.card_kind === option.value" :class="form.card_kind === option.value ? 'bg-surface text-foreground shadow-sm' : 'text-muted-foreground'" class="min-h-11 touch-manipulation rounded-lg text-sm font-semibold" @click="form.card_kind = option.value">{{ option.label }}</button></div></fieldset>
          <label class="form-field"><span>开户银行</span><BankPicker v-model="form.institution" :banks="banks" :loading="optionsLoading" /></label>
          <label class="form-field"><span>{{ editing?.type === 'bank_card' && editing.identifier ? '更换银行卡号' : '银行卡号' }} <small>可选</small></span><input :value="form.card_number" inputmode="numeric" autocomplete="off" enterkeyhint="done" maxlength="23" class="input-field font-mono tracking-wider" :placeholder="editing?.type === 'bank_card' && editing.identifier ? `当前尾号 ${editing.identifier}，留空不变` : '可暂不填写，后期再补充'" aria-describedby="card-number-help" @input="onCardInput" @blur="validateCardInput" /><small id="card-number-help" :class="cardError ? 'text-rose-600' : 'text-muted-foreground'">{{ cardError || (editing?.type === 'bank_card' && editing.identifier ? '输入新卡号后会重新校验和验重。' : '留空可直接保存；填写后仅保留安全指纹与后四位。') }}</small></label>
        </template>
        <label v-else class="form-field"><span>账号备注 <small>可选</small></span><input v-model.trim="form.identifier" maxlength="64" class="input-field" placeholder="用于区分多个同类账户，不建议填写完整账号" /></label>
        <p class="rounded-xl bg-muted px-3 py-2 text-xs leading-5 text-muted-foreground">请勿填写密码、验证码、支付口令等敏感信息。重复银行卡会在保存时提示。</p>
        <p v-if="formError" role="alert" class="text-sm text-rose-600">{{ formError }}</p>
        <div class="sticky -bottom-5 -mx-5 flex gap-3 border-t border-border bg-surface px-5 pb-[max(0.25rem,env(safe-area-inset-bottom))] pt-3 sm:static sm:mx-0 sm:border-0 sm:p-0"><button type="button" class="btn-ghost min-h-12 flex-1" @click="showForm = false">取消</button><button class="btn-brand min-h-12 flex-1" :disabled="saving || optionsLoading">{{ saving ? '保存中' : '保存账户' }}</button></div>
      </form>
    </Modal>
    <ConfirmDialog v-model="showArchive" title="归档这个账户？" message="历史账单仍会显示该账户，但新账单将不能再选择它。" confirm-text="归档" confirm-type="danger" @confirm="archive" />
    <Toast :message="toast.message" :type="toast.type" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { billingApi, type BankCardKind, type BankOption, type PersonalAccount, type PersonalAccountType } from '../api/billing'
import { apiMessage } from '../utils/billing'
import PageHeader from '../components/PageHeader.vue'
import Modal from '../components/Modal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import Toast from '../components/Toast.vue'
import BankLogo from '../components/billing/BankLogo.vue'
import BankPicker from '../components/billing/BankPicker.vue'
const types = [{ value: 'bank_card' as const, label: '银行卡' }, { value: 'wechat' as const, label: '微信' }, { value: 'alipay' as const, label: '支付宝' }]
const fallbackCardKinds = [{ value: 'debit' as const, label: '储蓄卡' }, { value: 'credit' as const, label: '信用卡' }]
const icons = { bank_card: '<svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M3 7h18v11H3zM3 10h18m-14 5h4"/></svg>', wechat: '<svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M4 11a7 6 0 0 1 12.5-3.7A6 5 0 0 1 20 16l1 3-3-1a7 5 0 0 1-9.5-2A7 6 0 0 1 4 11Z"/></svg>', alipay: '<svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M5 5h14M8 9h8m-9 4c3 4 7 5 12 6m-1-8c-2 5-6 8-13 9M12 3v8"/></svg>' }
const accounts = ref<PersonalAccount[]>([]), banks = ref<BankOption[]>([]), cardKinds = ref<Array<{ value: BankCardKind; label: string }>>(fallbackCardKinds)
const loading = ref(true), optionsLoading = ref(true), saving = ref(false), showArchived = ref(false), showForm = ref(false), showArchive = ref(false)
const editing = ref<PersonalAccount | null>(null), archiving = ref<PersonalAccount | null>(null), formError = ref(''), cardError = ref('')
const toast = reactive<{ message: string; type: 'success' | 'error' }>({ message: '', type: 'success' })
const form = reactive({ type: 'bank_card' as PersonalAccountType, name: '', institution: '', identifier: '', card_kind: 'debit' as BankCardKind, card_number: '' })
const visibleAccounts = computed(() => accounts.value.filter(item => showArchived.value || !item.archived))
const summaries = computed(() => types.map(type => ({ ...type, type: type.value, count: accounts.value.filter(item => item.type === type.value && !item.archived).length })))
function notify(message: string, type: 'success' | 'error' = 'success') { toast.message = ''; setTimeout(() => { toast.message = message; toast.type = type }) }
function typeIcon(type: PersonalAccountType) { return icons[type] }
function typeStyle(type: PersonalAccountType) { return type === 'bank_card' ? 'bg-brand-100 text-brand-700 dark:bg-brand-800 dark:text-accent' : type === 'wechat' ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300' : 'bg-sky-500/10 text-sky-700 dark:text-sky-300' }
function cardKindLabel(kind?: BankCardKind) { return cardKinds.value.find(item => item.value === kind)?.label || '银行卡' }
function accountDetail(item: PersonalAccount) { const base = item.type === 'bank_card' ? `${item.institution} · ${cardKindLabel(item.card_kind)} · ${item.identifier ? `尾号 ${item.identifier}` : '未填写卡号'}` : item.institution; return item.type !== 'bank_card' && item.identifier ? `${base} · ${item.identifier}` : base }
function bankByName(name: string) { return banks.value.find(bank => bank.name === name) }
function selectType(type: PersonalAccountType) { form.type = type; form.institution = type === 'wechat' ? '微信' : type === 'alipay' ? '支付宝' : ''; form.identifier = ''; form.card_number = ''; form.card_kind = 'debit'; cardError.value = '' }
function openCreate() { editing.value = null; Object.assign(form, { type: 'bank_card', name: '', institution: '', identifier: '', card_kind: 'debit', card_number: '' }); formError.value = ''; cardError.value = ''; showForm.value = true }
function openEdit(item: PersonalAccount) { editing.value = item; Object.assign(form, { type: item.type, name: item.name, institution: item.institution, identifier: item.identifier, card_kind: item.card_kind || 'debit', card_number: '' }); formError.value = ''; cardError.value = ''; showForm.value = true }
function cardDigits() { return form.card_number.replace(/\D/g, '') }
function onCardInput(event: Event) { const input = event.target as HTMLInputElement; const digits = input.value.replace(/\D/g, '').slice(0, 19); form.card_number = digits.replace(/(\d{4})(?=\d)/g, '$1 '); input.value = form.card_number; cardError.value = '' }
function validLuhn(value: string) { let sum = 0, double = false; for (let i = value.length - 1; i >= 0; i--) { let digit = Number(value[i]); if (double) { digit *= 2; if (digit > 9) digit -= 9 } sum += digit; double = !double } return sum % 10 === 0 }
function validateCardInput() { const digits = cardDigits(); if (!digits) { cardError.value = ''; return true } if (digits.length < 12 || digits.length > 19) { cardError.value = '请输入 12 至 19 位银行卡号'; return false } if (!validLuhn(digits)) { cardError.value = '银行卡号校验失败，请检查后重试'; return false } cardError.value = ''; return true }
async function load() { loading.value = true; try { const res = await billingApi.personalAccounts(true); accounts.value = res.data?.data || [] } catch (error) { notify(apiMessage(error, '账户加载失败'), 'error') } finally { loading.value = false } }
async function loadOptions() { optionsLoading.value = true; try { const res = await billingApi.personalAccountOptions(); banks.value = res.data?.data?.banks || []; cardKinds.value = res.data?.data?.card_kinds || fallbackCardKinds } catch (error) { notify(apiMessage(error, '银行列表加载失败'), 'error') } finally { optionsLoading.value = false } }
async function save() { if (form.type === 'bank_card' && !form.institution) { formError.value = '请选择开户银行'; return } if (form.type === 'bank_card' && !validateCardInput()) return; saving.value = true; formError.value = ''; try { const payload = { type: form.type, name: form.name, institution: form.institution, identifier: form.type === 'bank_card' ? '' : form.identifier, card_kind: form.type === 'bank_card' ? form.card_kind : undefined, card_number: form.type === 'bank_card' ? cardDigits() : undefined }; if (editing.value) await billingApi.personalAccountUpdate(editing.value.id, payload); else await billingApi.personalAccountCreate(payload); showForm.value = false; notify(editing.value ? '账户已更新' : '账户已添加'); await load() } catch (error) { formError.value = apiMessage(error) } finally { saving.value = false } }
function askArchive(item: PersonalAccount) { archiving.value = item; showArchive.value = true }
async function archive() { if (!archiving.value) return; try { await billingApi.personalAccountArchive(archiving.value.id); notify('账户已归档'); await load() } catch (error) { notify(apiMessage(error), 'error') } }
async function restore(item: PersonalAccount) { try { await billingApi.personalAccountRestore(item.id); notify('账户已恢复'); await load() } catch (error) { notify(apiMessage(error), 'error') } }
onMounted(() => Promise.all([load(), loadOptions()]))
</script>
