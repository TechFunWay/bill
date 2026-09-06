<template>
  <div class="page-container animate-fade-in">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <RouterLink to="/admin/shared" class="mb-2 inline-flex items-center gap-1 text-sm font-semibold text-muted-foreground hover:text-brand-600">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m15 18-6-6 6-6"/></svg>返回共享账本
        </RouterLink>
        <div class="flex flex-wrap items-center gap-3"><h1 class="font-display text-2xl font-bold sm:text-3xl">{{ detail?.ledger.name || '共享账本' }}</h1><span v-if="isArchived" class="badge bg-amber-500/10 text-amber-700 dark:text-amber-300">已归档</span><span v-if="isOwner" class="badge bg-brand-500/10 text-brand-700 dark:text-brand-300">所有者</span></div>
        <p class="mt-2 text-sm text-muted-foreground">{{ activeMembers.length }} 位成员 · {{ detail?.ledger.currency || 'CNY' }}</p>
      </div>
      <div class="flex w-full flex-wrap gap-2 sm:w-auto"><button v-if="isArchived && isOwner" class="btn-brand min-h-11 flex-1 sm:flex-none" :disabled="saving" @click="restoreLedger">{{ saving ? '恢复中' : '恢复账本' }}</button><button v-else-if="!isArchived && isOwner" class="btn-ghost min-h-11 flex-1 sm:flex-none" @click="openSettings">账本设置</button><button v-if="!isArchived" class="btn-brand min-h-11 flex-1 sm:flex-none" @click="openTransaction()"><svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v12m6-6H6"/></svg>记一笔</button></div>
    </div>

    <div v-if="loading" class="grid gap-4 md:grid-cols-3"><div v-for="n in 6" :key="n" class="surface h-40 animate-pulse rounded-2xl"></div></div>
    <template v-else-if="detail">
      <section v-if="isArchived" class="rounded-2xl border border-amber-300/60 bg-amber-50 p-4 text-amber-900 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-100">
        <div class="flex items-start gap-3"><svg class="mt-0.5 h-5 w-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 7h14M7 7l1 13h8l1-13M9 7V4h6v3"/></svg><div><h2 class="font-semibold">该账本已归档，只读查看</h2><p class="mt-1 text-sm opacity-80">归档期间不能记账、结算、编辑成员或修改设置。{{ isOwner ? '你可以恢复账本以继续使用。' : '' }}</p></div></div>
      </section>
      <div class="grid grid-cols-2 gap-3 md:grid-cols-3 md:gap-4">
        <article class="money-card money-card-primary"><span class="money-label">共同支出</span><strong class="money-value">{{ formatMoney(detail.totals.expense_cents, currency) }}</strong><span class="money-meta">{{ detail.totals.expense_count }} 笔支出</span></article>
        <article class="money-card"><span class="money-label">共同收入</span><strong class="money-value text-emerald-600">{{ formatMoney(detail.totals.income_cents, currency) }}</strong><span class="money-meta">退款、报销与共同收入</span></article>
        <article class="money-card col-span-2 md:col-span-1"><span class="money-label">我的余额</span><strong :class="myBalance > 0 ? 'text-emerald-600' : myBalance < 0 ? 'text-rose-600' : 'text-foreground'" class="money-value">{{ formatMoney(Math.abs(myBalance), currency) }}</strong><span class="money-meta">{{ myBalance > 0 ? '当前应收' : myBalance < 0 ? '当前应付' : '当前已结清' }}</span></article>
      </div>

      <div class="overflow-x-auto">
        <nav class="inline-flex min-w-full gap-1 rounded-xl bg-muted p-1 sm:min-w-0" aria-label="账本内容">
          <button v-for="item in tabs" :key="item.id" :class="tab === item.id ? 'bg-surface text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'" class="min-h-11 flex-1 whitespace-nowrap rounded-lg px-4 text-sm font-semibold transition-all sm:flex-none" @click="tab = item.id">{{ item.label }}<span v-if="item.count !== undefined" class="ml-1 text-xs opacity-70">{{ item.count }}</span></button>
        </nav>
      </div>

      <div v-if="tab === 'overview'" class="grid gap-4 xl:grid-cols-[1.1fr_1fr]">
        <section class="surface rounded-2xl p-4 sm:p-6">
          <div class="flex items-center justify-between"><div><h2 class="font-display text-lg font-bold">成员余额</h2><p class="mt-1 text-xs text-muted-foreground">正数应收，负数应付</p></div><span class="text-xs text-muted-foreground">已计入结算</span></div>
          <div class="mt-5 space-y-3">
            <div v-for="balance in detail.balances" :key="balance.user_id" class="flex items-center gap-3 rounded-xl border border-border p-3">
              <span class="avatar">{{ balance.username.charAt(0).toUpperCase() }}</span>
              <div class="min-w-0 flex-1"><div class="font-semibold">{{ balance.username }}<span v-if="balance.user_id === authStore.user?.id" class="ml-1 text-xs font-normal text-muted-foreground">你</span></div><p class="text-xs text-muted-foreground">经手 {{ formatMoney(Math.abs(balance.paid_cents), currency) }} · 分摊 {{ formatMoney(Math.abs(balance.share_cents), currency) }}</p></div>
              <div class="text-right"><span class="block text-xs text-muted-foreground">{{ balance.balance_cents > 0 ? '应收' : balance.balance_cents < 0 ? '应付' : '已结清' }}</span><strong :class="balance.balance_cents > 0 ? 'text-emerald-600' : balance.balance_cents < 0 ? 'text-rose-600' : 'text-foreground'" class="tabular-nums">{{ formatMoney(Math.abs(balance.balance_cents), currency) }}</strong></div>
            </div>
          </div>
        </section>
        <section class="surface rounded-2xl p-4 sm:p-6">
          <div><h2 class="font-display text-lg font-bold">补差建议</h2><p class="mt-1 text-xs text-muted-foreground">以较少转账次数结清当前余额</p></div>
          <div v-if="!transfers.length" class="empty-state h-56"><svg class="mb-3 h-9 w-9 text-emerald-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m5 13 4 4L19 7"/></svg>当前已经结清</div>
          <div v-else class="mt-5 space-y-3">
            <div v-for="(transfer, index) in transfers" :key="index" class="rounded-xl bg-muted/70 p-4">
              <div class="flex items-center gap-2 text-sm"><strong>{{ transfer.from_username }}</strong><svg class="h-4 w-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14m-4-4 4 4-4 4"/></svg><strong>{{ transfer.to_username }}</strong></div>
              <div class="mt-2 flex items-end justify-between"><span class="font-display text-xl font-bold text-rose-600">{{ formatMoney(transfer.amount_cents, currency) }}</span><button v-if="!isArchived && canSettle(transfer)" class="min-h-11 text-sm font-semibold text-brand-600" @click="openSettlement(transfer)">记录结算</button></div>
            </div>
          </div>
        </section>
      </div>

      <section v-else-if="tab === 'transactions'" class="surface overflow-hidden rounded-2xl">
        <div class="flex items-center justify-between gap-3 border-b border-border p-4 sm:p-5"><div><h2 class="font-display font-bold">共同账单</h2><p class="mt-1 text-xs text-muted-foreground">支出、收入和参与人分摊明细</p></div><button v-if="!isArchived" class="btn-brand min-h-11 shrink-0 px-3 py-2" @click="openTransaction()">记一笔</button></div>
        <div v-if="!detail.transactions.length" class="empty-state h-64">还没有共同账单</div>
        <div v-else class="divide-y divide-border">
          <article v-for="item in detail.transactions" :key="item.id" class="p-4 sm:p-5">
            <div class="flex items-start gap-3">
              <span :class="item.kind === 'income' ? 'bg-emerald-500/10 text-emerald-600' : 'bg-rose-500/10 text-rose-600'" class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl">
                <CategoryIcon :category="item.category" :kind="item.kind" class="h-5 w-5" />
              </span>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-x-2"><h3 class="font-semibold">{{ item.category }}</h3><span class="text-xs text-muted-foreground">{{ item.kind === 'expense' ? `${item.actor_username} 付款` : `${item.actor_username} 收款` }}</span></div>
                <p class="mt-1 text-sm text-muted-foreground"><span v-if="item.account_name">{{ item.account_name }} · </span>{{ item.note || '无备注' }}</p>
                <div class="mt-2 flex flex-wrap gap-1.5"><span v-for="share in item.shares" :key="share.user_id" class="badge bg-muted text-muted-foreground">{{ share.username }} {{ formatMoney(share.amount_cents, currency) }}</span></div>
                <BillAttachments v-if="item.attachments?.length" class="mt-2" :model-value="item.attachments" readonly compact />
                <p class="mt-2 text-xs text-muted-foreground">{{ formatDate(item.occurred_at) }} · {{ item.creator_username }} 记录</p>
              </div>
              <div class="shrink-0 text-right"><strong :class="item.kind === 'income' ? 'text-emerald-600' : ''" class="block text-lg tabular-nums">{{ item.kind === 'income' ? '+' : '-' }}{{ formatMoney(item.amount_cents, currency) }}</strong><div v-if="!isArchived && canEdit(item)" class="mt-2 flex justify-end gap-2"><button class="min-h-11 text-xs font-semibold text-brand-600" @click="openTransaction(item)">编辑</button><button class="min-h-11 text-xs font-semibold text-rose-600" @click="askDeleteTransaction(item)">删除</button></div></div>
            </div>
          </article>
        </div>
        <div v-if="detail.transactions_total > 0" class="flex flex-col gap-3 border-t border-border p-4 sm:flex-row sm:items-center sm:justify-between">
          <p class="text-sm text-muted-foreground">共 {{ detail.transactions_total }} 笔，第 {{ detail.transactions_page }} / {{ transactionPageCount }} 页</p>
          <div class="flex gap-2"><button class="btn-ghost min-h-11 px-3 py-2" :disabled="detail.transactions_page <= 1 || loading" aria-label="上一页账单" @click="changeTransactionPage(detail.transactions_page - 1)">上一页</button><button class="btn-ghost min-h-11 px-3 py-2" :disabled="detail.transactions_page >= transactionPageCount || loading" aria-label="下一页账单" @click="changeTransactionPage(detail.transactions_page + 1)">下一页</button></div>
        </div>
      </section>

      <section v-else-if="tab === 'settlements'" class="surface overflow-hidden rounded-2xl">
        <div class="flex items-center justify-between gap-3 border-b border-border p-4 sm:p-5"><div><h2 class="font-display font-bold">结算记录</h2><p class="mt-1 text-xs text-muted-foreground">成员之间已经实际支付的补差款</p></div><button v-if="!isArchived" class="btn-brand min-h-11 shrink-0 px-3 py-2" @click="openSettlement()">记录结算</button></div>
        <div v-if="!detail.settlements.length" class="empty-state h-64">还没有结算记录</div>
        <div v-else class="divide-y divide-border">
          <div v-for="item in detail.settlements" :key="item.id" class="flex items-center gap-3 p-4 sm:p-5">
            <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-brand-500/10 text-brand-600"><svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14m-4-4 4 4-4 4"/></svg></span>
            <div class="min-w-0 flex-1"><div class="font-semibold">{{ item.from_username }} 支付给 {{ item.to_username }}</div><p class="truncate text-xs text-muted-foreground">{{ item.note || formatDate(item.occurred_at) }}</p></div>
            <div class="text-right"><strong class="tabular-nums text-emerald-600">{{ formatMoney(item.amount_cents, currency) }}</strong><button v-if="!isArchived && (item.created_by === authStore.user?.id || isOwner)" class="mt-1 block min-h-11 text-xs font-semibold text-rose-600" @click="deleteSettlement(item)">删除</button></div>
          </div>
        </div>
      </section>

      <section v-else class="surface rounded-2xl p-4 sm:p-6">
        <div class="flex items-center justify-between gap-3"><div><h2 class="font-display text-lg font-bold">成员管理</h2><p class="mt-1 text-xs text-muted-foreground">所有者可以邀请或移除尚未产生账单的成员</p></div><button v-if="isOwner && !isArchived" class="btn-brand min-h-11 shrink-0 px-3 py-2" @click="showInvite = true">邀请成员</button></div>
        <div class="mt-5 grid gap-3 md:grid-cols-2">
          <div v-for="member in detail.members" :key="member.id" class="flex items-center gap-3 rounded-xl border border-border p-4">
            <span class="avatar">{{ member.username.charAt(0).toUpperCase() }}</span>
            <div class="min-w-0 flex-1"><strong>{{ member.username }}</strong><div class="mt-1 flex gap-2"><span class="text-xs text-muted-foreground">{{ member.role === 'owner' ? '所有者' : '成员' }}</span><span v-if="member.status !== 'active'" class="text-xs text-amber-600">{{ member.status === 'pending' ? '等待接受' : '已拒绝' }}</span></div></div>
            <button v-if="isOwner && !isArchived && member.role !== 'owner'" class="min-h-11 text-sm font-semibold text-rose-600" @click="removeMember(member)">移除</button>
          </div>
        </div>
      </section>
    </template>

    <Modal v-model="showTransaction" :title="editingTransaction ? '编辑共同账单' : '记一笔共同账单'">
      <form class="space-y-4" @submit.prevent="saveTransaction">
        <div class="grid grid-cols-2 gap-2 rounded-xl bg-muted p-1"><button type="button" :class="transactionForm.kind === 'expense' ? 'bg-surface text-rose-600 shadow-sm' : 'text-muted-foreground'" class="min-h-11 rounded-lg text-sm font-semibold" @click="setTransactionKind('expense')">共同支出</button><button type="button" :class="transactionForm.kind === 'income' ? 'bg-surface text-emerald-600 shadow-sm' : 'text-muted-foreground'" class="min-h-11 rounded-lg text-sm font-semibold" @click="setTransactionKind('income')">共同收入</button></div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2"><label class="form-field"><span>金额</span><input v-model="transactionForm.amount" required inputmode="decimal" class="input-field font-bold" placeholder="0.00" /></label><label class="form-field"><span>{{ transactionForm.kind === 'expense' ? '付款人' : '收款人' }}</span><select v-model.number="transactionForm.actor_user_id" required class="input-field"><option v-for="member in activeMembers" :key="member.user_id" :value="member.user_id">{{ member.username }}</option></select></label></div>
        <label v-if="detail?.ledger.show_account" class="form-field"><span>收支账户 <small>可选，仅限付款/收款人本人</small></span><select v-if="transactionForm.actor_user_id === authStore.user?.id" v-model.number="transactionForm.account_id" class="input-field"><option :value="0">不选择账户</option><option v-for="account in accounts" :key="account.id" :value="account.id">{{ account.name }} · {{ accountDetail(account) }}</option></select><div v-else class="input-field flex items-center text-muted-foreground">{{ editingTransaction?.account_name || '由该成员自行选择' }}</div></label>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2"><label v-if="detail?.ledger.show_category" class="form-field"><span>分类</span><select v-model="transactionForm.category" required class="input-field"><option value="" disabled>请选择</option><option v-for="item in sharedCategories" :key="item">{{ item }}</option></select></label><label class="form-field"><span>发生时间</span><input v-model="transactionForm.occurred_at" required type="datetime-local" class="input-field" /></label></div>
        <label v-if="detail?.ledger.show_note" class="form-field"><span>备注 <small>可选</small></span><input v-model="transactionForm.note" class="input-field" maxlength="500" placeholder="例如：周六晚餐" /></label>
        <BillAttachments v-model="transactionForm.attachments" @uploading="transactionAttachmentUploading = $event" />
        <fieldset class="rounded-xl border border-border p-4">
          <legend class="px-1 text-sm font-semibold">参与人与分摊方式</legend>
          <div class="grid grid-cols-2 gap-1 rounded-lg bg-muted p-1 sm:grid-cols-4">
            <button v-for="mode in splitModes" :key="mode.id" type="button" :class="transactionForm.splitMode === mode.id ? 'bg-surface shadow-sm' : 'text-muted-foreground'" class="min-h-11 rounded-md text-xs font-semibold" @click="transactionForm.splitMode = mode.id">{{ mode.label }}</button>
          </div>
          <div class="mt-3 space-y-2">
            <div v-for="member in activeMembers" :key="member.user_id" class="flex items-center gap-3">
              <label class="flex min-h-11 min-w-0 flex-1 cursor-pointer items-center gap-3 rounded-lg px-2 hover:bg-muted"><input v-model="participants" type="checkbox" :value="member.user_id" class="h-4 w-4 rounded text-brand-600" /><span class="truncate text-sm font-medium">{{ member.username }}</span></label>
              <div v-if="transactionForm.splitMode !== 'equal'" class="relative w-28"><input v-model="splitValues[member.user_id]" :disabled="!participants.includes(member.user_id)" inputmode="decimal" class="input-field py-2 pr-8 text-right text-sm" /><span class="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{{ splitUnit }}</span></div>
              <span v-else class="w-28 text-right text-sm tabular-nums text-muted-foreground">{{ formatMoney(equalPreview(member.user_id), currency) }}</span>
            </div>
          </div>
          <div class="mt-3 flex items-center justify-between border-t border-border pt-3 text-sm"><span class="text-muted-foreground">{{ splitSummaryLabel }}</span><strong :class="splitValid ? 'text-emerald-600' : 'text-rose-600'">{{ splitSummary }}</strong></div>
        </fieldset>
        <p v-if="formError" class="text-sm text-rose-600">{{ formError }}</p>
        <div class="flex gap-3 pt-1"><button type="button" class="btn-ghost flex-1 min-h-11" @click="showTransaction = false">取消</button><button class="btn-brand flex-1 min-h-11" :disabled="saving || transactionAttachmentUploading">{{ transactionAttachmentUploading ? '图片上传中' : saving ? '保存中' : '保存账单' }}</button></div>
      </form>
    </Modal>

    <Modal v-model="showSettlement" title="记录结算">
      <form class="space-y-4" @submit.prevent="saveSettlement">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2"><label class="form-field"><span>付款成员</span><select v-model.number="settlementForm.from_user_id" class="input-field"><option v-for="member in activeMembers" :key="member.user_id" :value="member.user_id">{{ member.username }}</option></select></label><label class="form-field"><span>收款成员</span><select v-model.number="settlementForm.to_user_id" class="input-field"><option v-for="member in activeMembers" :key="member.user_id" :value="member.user_id">{{ member.username }}</option></select></label></div>
        <label class="form-field"><span>金额</span><input v-model="settlementForm.amount" required inputmode="decimal" class="input-field text-lg font-bold" placeholder="0.00" /></label>
        <label class="form-field"><span>结算时间</span><input v-model="settlementForm.occurred_at" required type="datetime-local" class="input-field" /></label>
        <label class="form-field"><span>备注 <small>可选</small></span><input v-model="settlementForm.note" class="input-field" placeholder="例如：微信转账" /></label>
        <p v-if="formError" class="text-sm text-rose-600">{{ formError }}</p>
        <div class="flex gap-3"><button type="button" class="btn-ghost flex-1 min-h-11" @click="showSettlement = false">取消</button><button class="btn-brand flex-1 min-h-11" :disabled="saving">{{ saving ? '保存中' : '确认已结算' }}</button></div>
      </form>
    </Modal>

    <Modal v-model="showInvite" title="邀请成员"><form class="space-y-4" @submit.prevent="inviteMember"><label class="form-field"><span>用户名</span><input v-model="inviteUsername" required class="input-field" placeholder="输入已注册用户的用户名" /></label><p v-if="formError" class="text-sm text-rose-600">{{ formError }}</p><div class="flex gap-3"><button type="button" class="btn-ghost flex-1 min-h-11" @click="showInvite = false">取消</button><button class="btn-brand flex-1 min-h-11" :disabled="saving">发送邀请</button></div></form></Modal>
    <Modal v-model="showSettings" title="账本设置"><form class="space-y-4" @submit.prevent="saveSettings"><label class="form-field"><span>账本名称</span><input v-model="settingsForm.name" required class="input-field" /></label><label class="form-field"><span>货币</span><select v-model="settingsForm.currency" class="input-field"><option v-for="code in ['CNY','USD','EUR','JPY','HKD']" :key="code">{{ code }}</option></select></label><fieldset class="rounded-xl border border-border p-4"><legend class="px-1 text-sm font-semibold">账单附加字段</legend><p class="mb-2 text-xs text-muted-foreground">金额、收支方向、付款/收款人、时间和分摊始终启用。</p><label v-for="field in fieldOptions" :key="field.key" class="flex min-h-11 cursor-pointer items-center justify-between gap-3"><span><strong class="text-sm">{{ field.label }}</strong><small class="ml-2 text-muted-foreground">{{ field.hint }}</small></span><input v-model="settingsForm[field.key]" type="checkbox" class="h-4 w-4 accent-brand-700" /></label></fieldset><div class="flex gap-3"><button type="button" class="btn-ghost flex-1 min-h-11" @click="showSettings = false">取消</button><button class="btn-brand flex-1 min-h-11">保存</button></div><button type="button" class="min-h-11 w-full rounded-xl border border-rose-300 text-sm font-semibold text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-500/10" @click="askArchive">归档账本</button></form></Modal>
    <ConfirmDialog v-model="confirmDeleteTransaction" title="删除共同账单？" message="删除后所有成员的余额与补差建议都会立即更新。" confirm-text="删除" confirm-type="danger" @confirm="deleteTransaction" />
    <ConfirmDialog v-model="confirmArchive" title="归档这个账本？" message="归档后将从活跃账本中隐藏，历史数据仍会保留。" confirm-text="归档" confirm-type="danger" @confirm="archiveLedger" />
    <Toast :message="toast.message" :type="toast.type" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { billingApi, type BillAttachment, type BillKind, type LedgerDetail, type LedgerMember, type PersonalAccount, type SharedTransaction, type Transfer } from '../api/billing'
import { useAuthStore } from '../stores/auth'
import { apiMessage, centsToMoney, formatDate, formatMoney, isSplitValid, moneyToCents, sharedExpenseCategories, sharedIncomeCategories, splitByWeights, splitEqually, splitExactly, toLocalInput } from '../utils/billing'
import Modal from '../components/Modal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import Toast from '../components/Toast.vue'
import CategoryIcon from '../components/billing/CategoryIcon.vue'
import BillAttachments from '../components/billing/BillAttachments.vue'

type Tab = 'overview' | 'transactions' | 'settlements' | 'members'
type SplitMode = 'equal' | 'exact' | 'percent' | 'shares'
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const ledgerID = Number(route.params.id)
const detail = ref<LedgerDetail | null>(null)
const accounts = ref<PersonalAccount[]>([])
const loading = ref(true)
const saving = ref(false)
const transactionAttachmentUploading = ref(false)
const tab = ref<Tab>('overview')
const showTransaction = ref(false)
const showSettlement = ref(false)
const showInvite = ref(false)
const showSettings = ref(false)
const confirmDeleteTransaction = ref(false)
const confirmArchive = ref(false)
const editingTransaction = ref<SharedTransaction | null>(null)
const deletingTransaction = ref<SharedTransaction | null>(null)
const inviteUsername = ref('')
const formError = ref('')
const participants = ref<number[]>([])
const splitValues = reactive<Record<number, string>>({})
const toast = reactive<{ message: string; type: 'success' | 'error' }>({ message: '', type: 'success' })
const transactionForm = reactive({ kind: 'expense' as BillKind, actor_user_id: 0, account_id: 0, amount: '', category: '', note: '', occurred_at: toLocalInput(), splitMode: 'equal' as SplitMode, attachments: [] as BillAttachment[] })
const settlementForm = reactive({ from_user_id: 0, to_user_id: 0, amount: '', note: '', occurred_at: toLocalInput() })
const settingsForm = reactive({ name: '', currency: 'CNY', show_account: true, show_category: true, show_note: true })
const fieldOptions: Array<{ key: 'show_account' | 'show_category' | 'show_note'; label: string; hint: string }> = [{ key: 'show_account', label: '收支账户', hint: '可选' }, { key: 'show_category', label: '分类', hint: '启用时必选' }, { key: 'show_note', label: '备注', hint: '可选' }]
const tabs = computed(() => [{ id: 'overview' as Tab, label: '概览' }, { id: 'transactions' as Tab, label: '共同账单', count: detail.value?.transactions_total || 0 }, { id: 'settlements' as Tab, label: '结算记录', count: detail.value?.settlements.length || 0 }, { id: 'members' as Tab, label: '成员', count: activeMembers.value.length }])
const activeMembers = computed(() => (detail.value?.members || []).filter(item => item.status === 'active'))
const transfers = computed(() => detail.value?.suggested_transfers || [])
const isOwner = computed(() => detail.value?.ledger.owner_user_id === authStore.user?.id)
const isArchived = computed(() => detail.value?.ledger.archived === true)
const currency = computed(() => detail.value?.ledger.currency || 'CNY')
const transactionPageCount = computed(() => Math.max(1, Math.ceil((detail.value?.transactions_total || 0) / (detail.value?.transactions_page_size || 50))))
const myBalance = computed(() => detail.value?.balances.find(item => item.user_id === authStore.user?.id)?.balance_cents || 0)
const sharedCategories = computed(() => transactionForm.kind === 'expense' ? sharedExpenseCategories : sharedIncomeCategories)
const splitModes = [{ id: 'equal' as SplitMode, label: '等额' }, { id: 'exact' as SplitMode, label: '金额' }, { id: 'percent' as SplitMode, label: '比例' }, { id: 'shares' as SplitMode, label: '份额' }]
const amountCents = computed(() => moneyToCents(transactionForm.amount))
const splitUnit = computed(() => transactionForm.splitMode === 'exact' ? '元' : transactionForm.splitMode === 'percent' ? '%' : '份')
const splitSummaryLabel = computed(() => transactionForm.splitMode === 'equal' ? `${participants.value.length} 人等额分摊` : transactionForm.splitMode === 'exact' ? '已分配金额' : transactionForm.splitMode === 'percent' ? '比例合计' : '份额合计')
const splitSummary = computed(() => { if (transactionForm.splitMode === 'equal') return formatMoney(amountCents.value, currency.value); const sum = participants.value.reduce((total, id) => total + (Number(splitValues[id]) || 0), 0); return transactionForm.splitMode === 'exact' ? formatMoney(moneyToCents(sum), currency.value) : `${sum}${transactionForm.splitMode === 'percent' ? '%' : ' 份'}` })
const splitValid = computed(() => isSplitValid(transactionForm.splitMode, amountCents.value, participants.value, splitValues))

function notify(message: string, type: 'success' | 'error' = 'success') { toast.message = ''; setTimeout(() => { toast.message = message; toast.type = type }) }
async function load(page = detail.value?.transactions_page || 1) { loading.value = true; try { const res = await billingApi.ledger(ledgerID, { transaction_page: page, transaction_page_size: 50 }); detail.value = res.data.data } catch (e) { notify(apiMessage(e, '账本加载失败'), 'error') } finally { loading.value = false } }
function changeTransactionPage(page: number) { if (page < 1 || page > transactionPageCount.value) return; load(page) }
function canEdit(item: SharedTransaction) { return isOwner.value || item.created_by === authStore.user?.id }
function canSettle(transfer: Transfer) { return isOwner.value || transfer.from_user_id === authStore.user?.id || transfer.to_user_id === authStore.user?.id }
function equalPreview(userID: number) { return splitEqually(amountCents.value, participants.value).find(share => share.user_id === userID)?.amount_cents || 0 }
function setTransactionKind(kind: BillKind) { transactionForm.kind = kind; if (!sharedCategories.value.includes(transactionForm.category)) transactionForm.category = '' }
function accountDetail(item: PersonalAccount) { return item.type === 'bank_card' ? `${item.institution} · ${item.card_kind === 'credit' ? '信用卡' : item.card_kind === 'debit' ? '储蓄卡' : '银行卡'} · ${item.identifier ? `尾号 ${item.identifier}` : '未填写卡号'}` : item.institution }
function resetSplit() { participants.value = activeMembers.value.map(item => item.user_id); activeMembers.value.forEach(item => { splitValues[item.user_id] = '' }) }
function openTransaction(item?: SharedTransaction) { editingTransaction.value = item || null; formError.value = ''; transactionAttachmentUploading.value = false; resetSplit(); if (item) { Object.assign(transactionForm, { kind: item.kind, actor_user_id: item.actor_user_id, account_id: item.account_id || 0, amount: centsToMoney(item.amount_cents), category: item.category === '未分类' ? '' : item.category, note: item.note, occurred_at: toLocalInput(item.occurred_at), splitMode: 'exact', attachments: [...(item.attachments || [])] }); participants.value = item.shares.map(share => share.user_id); item.shares.forEach(share => { splitValues[share.user_id] = centsToMoney(share.amount_cents) }) } else { Object.assign(transactionForm, { kind: 'expense', actor_user_id: authStore.user?.id || activeMembers.value[0]?.user_id || 0, account_id: 0, amount: '', category: '', note: '', occurred_at: toLocalInput(), splitMode: 'equal', attachments: [] }) } showTransaction.value = true }
function buildShares() { if (transactionForm.splitMode === 'equal') return splitEqually(amountCents.value, participants.value); if (transactionForm.splitMode === 'exact') return splitExactly(participants.value, splitValues); return splitByWeights(amountCents.value, participants.value, splitValues) }
async function saveTransaction() { if (!splitValid.value) { formError.value = '请检查金额、参与人和分摊合计'; return } saving.value = true; formError.value = ''; try { const payload = { kind: transactionForm.kind, actor_user_id: transactionForm.actor_user_id, account_id: transactionForm.account_id || 0, amount_cents: amountCents.value, category: transactionForm.category, note: transactionForm.note, occurred_at: new Date(transactionForm.occurred_at).toISOString(), shares: buildShares(), attachments: transactionForm.attachments.map(({ path, name }) => ({ path, name })) }; if (editingTransaction.value) await billingApi.sharedUpdate(ledgerID, editingTransaction.value.id, payload); else await billingApi.sharedCreate(ledgerID, payload); showTransaction.value = false; notify(editingTransaction.value ? '共同账单已更新' : '共同账单已记录'); await load(); tab.value = 'transactions' } catch (e) { formError.value = apiMessage(e) } finally { saving.value = false } }
function askDeleteTransaction(item: SharedTransaction) { deletingTransaction.value = item; confirmDeleteTransaction.value = true }
async function deleteTransaction() { if (!deletingTransaction.value) return; try { await billingApi.sharedDelete(ledgerID, deletingTransaction.value.id); const remaining = Math.max(0, (detail.value?.transactions_total || 1) - 1); const lastPage = Math.max(1, Math.ceil(remaining / 50)); notify('共同账单已删除'); await load(Math.min(detail.value?.transactions_page || 1, lastPage)) } catch (e) { notify(apiMessage(e), 'error') } }
function openSettlement(transfer?: Transfer) { formError.value = ''; Object.assign(settlementForm, { from_user_id: transfer?.from_user_id || authStore.user?.id || activeMembers.value[0]?.user_id || 0, to_user_id: transfer?.to_user_id || activeMembers.value.find(item => item.user_id !== authStore.user?.id)?.user_id || 0, amount: transfer ? centsToMoney(transfer.amount_cents) : '', note: '', occurred_at: toLocalInput() }); showSettlement.value = true }
async function saveSettlement() { const amount = moneyToCents(settlementForm.amount); if (amount <= 0 || settlementForm.from_user_id === settlementForm.to_user_id) { formError.value = '请选择不同的付款人与收款人，并填写有效金额'; return } saving.value = true; try { await billingApi.settlementCreate(ledgerID, { from_user_id: settlementForm.from_user_id, to_user_id: settlementForm.to_user_id, amount_cents: amount, note: settlementForm.note, occurred_at: new Date(settlementForm.occurred_at).toISOString() }); showSettlement.value = false; notify('结算已记录'); await load(); tab.value = 'settlements' } catch (e) { formError.value = apiMessage(e) } finally { saving.value = false } }
async function deleteSettlement(item: { id: number }) { try { await billingApi.settlementDelete(ledgerID, item.id); notify('结算记录已删除'); load() } catch (e) { notify(apiMessage(e), 'error') } }
async function inviteMember() { saving.value = true; formError.value = ''; try { await billingApi.invite(ledgerID, inviteUsername.value.trim()); showInvite.value = false; inviteUsername.value = ''; notify('邀请已发送'); load() } catch (e) { formError.value = apiMessage(e) } finally { saving.value = false } }
async function removeMember(member: LedgerMember) { if (!window.confirm(`确认移除 ${member.username}？`)) return; try { await billingApi.removeMember(ledgerID, member.user_id); notify('成员已移除'); load() } catch (e) { notify(apiMessage(e), 'error') } }
function openSettings() { if (!detail.value) return; Object.assign(settingsForm, { name: detail.value.ledger.name, currency: detail.value.ledger.currency, show_account: detail.value.ledger.show_account, show_category: detail.value.ledger.show_category, show_note: detail.value.ledger.show_note }); showSettings.value = true }
async function saveSettings() { try { await billingApi.ledgerUpdate(ledgerID, settingsForm); showSettings.value = false; notify('账本设置已保存'); load() } catch (e) { notify(apiMessage(e), 'error') } }
function askArchive() { showSettings.value = false; confirmArchive.value = true }
async function archiveLedger() { try { await billingApi.ledgerArchive(ledgerID); notify('账本已归档'); setTimeout(() => router.push('/admin/shared'), 500) } catch (e) { notify(apiMessage(e), 'error') } }
async function restoreLedger() { saving.value = true; try { await billingApi.ledgerRestore(ledgerID); notify('账本已恢复'); await load() } catch (e) { notify(apiMessage(e), 'error') } finally { saving.value = false } }
watch(() => transactionForm.actor_user_id, value => { if (editingTransaction.value && value === editingTransaction.value.actor_user_id) return; if (value !== authStore.user?.id) transactionForm.account_id = 0 })
onMounted(async () => { await Promise.all([load(), billingApi.personalAccounts().then(res => { accounts.value = res.data?.data || [] }).catch(() => { accounts.value = [] })]) })
</script>
