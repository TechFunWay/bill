import request from './request'

export type BillKind = 'expense' | 'income'
export type PersonalAccountType = 'bank_card' | 'wechat' | 'alipay'
export type BankCardKind = 'debit' | 'credit'

export interface BillAttachment {
  id?: number
  path: string
  name: string
  sort_order?: number
  created_at?: string
}

export interface PersonalTransaction {
  id: number
  user_id: number
  ledger_id: number
  ledger_name?: string
  account_id: number
  account_name?: string
  account_type?: PersonalAccountType
  kind: BillKind
  amount_cents: number
  category: string
  note: string
  occurred_at: string
  created_at: string
  updated_at: string
  tags: PersonalTag[]
  attachments?: BillAttachment[]
}

export interface PersonalAccount {
  id: number
  user_id?: number
  type: PersonalAccountType
  name: string
  institution: string
  identifier: string
  card_kind?: BankCardKind
  archived: boolean
  transaction_count?: number
  created_at?: string
  updated_at?: string
}

export interface PersonalAccountPayload {
  type: PersonalAccountType
  name: string
  institution: string
  identifier: string
  card_kind?: BankCardKind
  card_number?: string
}

export interface PersonalAccountOptions {
  banks: BankOption[]
  card_kinds: Array<{ value: BankCardKind; label: string }>
}

export interface BankOption {
  code: string
  name: string
  short: string
  color: string
}

export interface PersonalLedger {
  id: number
  user_id?: number
  name: string
  currency: string
  cover?: string
  sort_order?: number
  is_default?: boolean
  archived?: boolean
  transaction_count?: number
  created_at?: string
  updated_at?: string
}

export interface PersonalCategory {
  id: number
  user_id?: number
  name: string
  kind: BillKind
  parent_id: number
  parent_name?: string
  is_default?: boolean
  transaction_count?: number
}

export interface PersonalTag {
  id: number
  user_id?: number
  name: string
  color?: string
  transaction_count?: number
}

export interface StatsSummary {
  income_cents: number
  expense_cents: number
  balance_cents: number
  count: number
}

export interface PersonalStats {
  summary: StatsSummary
  categories: Array<{ category: string; amount_cents: number; count: number }>
  trend: Array<{ month: string; income_cents: number; expense_cents: number }>
}

export interface LedgerListItem {
  id: number
  name: string
  currency: string
  owner_user_id: number
  owner_username: string
  archived: boolean
  member_status: 'pending' | 'active' | 'rejected'
  member_role: 'owner' | 'member'
  member_count: number
  balance_cents: number
  expense_cents: number
  updated_at: string
}

export interface LedgerMember {
  id: number
  ledger_id: number
  user_id: number
  username: string
  role: 'owner' | 'member'
  status: 'pending' | 'active' | 'rejected'
  created_at: string
}

export interface SharedShare {
  user_id: number
  username?: string
  amount_cents: number
}

export interface SharedTransaction {
  id: number
  ledger_id: number
  created_by: number
  creator_username: string
  kind: BillKind
  actor_user_id: number
  actor_username: string
  account_id: number
  account_name?: string
  account_type?: PersonalAccountType
  amount_cents: number
  category: string
  note: string
  occurred_at: string
  shares: SharedShare[]
  attachments?: BillAttachment[]
}

export interface Balance {
  user_id: number
  username: string
  balance_cents: number
  paid_cents: number
  share_cents: number
}

export interface Transfer {
  from_user_id: number
  from_username: string
  to_user_id: number
  to_username: string
  amount_cents: number
}

export interface Settlement {
  id: number
  ledger_id: number
  from_user_id: number
  from_username: string
  to_user_id: number
  to_username: string
  amount_cents: number
  note: string
  occurred_at: string
  created_by: number
}

export interface LedgerDetail {
  ledger: {
    id: number
    name: string
    currency: string
    owner_user_id: number
    archived: boolean
    show_account: boolean
    show_category: boolean
    show_note: boolean
    created_at: string
    updated_at: string
  }
  members: LedgerMember[]
  transactions: SharedTransaction[]
  settlements: Settlement[]
  balances: Balance[]
  suggested_transfers: Transfer[] | null
  totals: { expense_cents: number; income_cents: number; expense_count: number; income_count: number }
  transactions_total: number
  transactions_page: number
  transactions_page_size: number
}

export interface PersonalPayload {
  ledger_id: number
  account_id: number
  kind: BillKind
  amount_cents: number
  category: string
  note: string
  occurred_at: string
  tag_ids: number[]
  attachments?: BillAttachment[]
}

export interface SharedPayload extends Pick<PersonalPayload, 'kind' | 'amount_cents' | 'category' | 'note' | 'occurred_at'> {
  actor_user_id: number
  account_id?: number
  shares: SharedShare[]
  attachments?: BillAttachment[]
}

export const billingApi = {
  dashboard: () => request.get('/api/billing/dashboard'),
  users: (q = '') => request.get('/api/billing/users', { params: { q } }),

  personalList: (params: Record<string, unknown>) => request.get('/api/billing/personal/transactions', { params }),
  personalCreate: (data: PersonalPayload) => request.post('/api/billing/personal/transactions', data),
  personalUpdate: (id: number, data: PersonalPayload) => request.put(`/api/billing/personal/transactions/${id}`, data),
  personalDelete: (id: number) => request.delete(`/api/billing/personal/transactions/${id}`),
  personalStats: (params: Record<string, unknown> = {}) => request.get('/api/billing/personal/stats', { params }),
  personalExport: (params: Record<string, unknown> = {}) => request.get('/api/billing/personal/export', { params, responseType: 'blob' }),
  personalAccounts: (includeArchived = false) => request.get('/api/billing/personal/accounts', { params: includeArchived ? { include_archived: true } : {} }),
  personalAccountOptions: () => request.get('/api/billing/personal/account-options'),
  personalAccountCreate: (data: PersonalAccountPayload) => request.post('/api/billing/personal/accounts', data),
  personalAccountUpdate: (id: number, data: PersonalAccountPayload) => request.put(`/api/billing/personal/accounts/${id}`, data),
  personalAccountArchive: (id: number) => request.delete(`/api/billing/personal/accounts/${id}`),
  personalAccountRestore: (id: number) => request.post(`/api/billing/personal/accounts/${id}/restore`),
  personalLedgers: (includeArchived = false) => request.get('/api/billing/personal/ledgers', { params: includeArchived ? { include_archived: true } : {} }),
  personalLedgerCreate: (data: { name: string; currency: string; cover?: string }) => request.post('/api/billing/personal/ledgers', data),
  personalLedgerUpdate: (id: number, data: { name: string; currency: string; cover?: string }) => request.put(`/api/billing/personal/ledgers/${id}`, data),
  personalLedgerOrder: (ledgerIds: number[]) => request.put('/api/billing/personal/ledgers/order', { ledger_ids: ledgerIds }),
  personalLedgerDelete: (id: number) => request.delete(`/api/billing/personal/ledgers/${id}`),
  personalLedgerRestore: (id: number) => request.post(`/api/billing/personal/ledgers/${id}/restore`),
  uploadImage: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request.post('/api/upload', form)
  },
  personalCategories: () => request.get('/api/billing/personal/categories'),
  personalCategoryCreate: (data: { name: string; kind: BillKind; parent_id: number }) => request.post('/api/billing/personal/categories', data),
  personalCategoryUpdate: (id: number, data: { name: string; kind: BillKind; parent_id: number }) => request.put(`/api/billing/personal/categories/${id}`, data),
  personalCategoryDelete: (id: number) => request.delete(`/api/billing/personal/categories/${id}`),
  personalTags: () => request.get('/api/billing/personal/tags'),
  personalTagCreate: (data: { name: string }) => request.post('/api/billing/personal/tags', data),
  personalTagUpdate: (id: number, data: { name: string }) => request.put(`/api/billing/personal/tags/${id}`, data),
  personalTagDelete: (id: number) => request.delete(`/api/billing/personal/tags/${id}`),

  ledgers: () => request.get('/api/billing/ledgers'),
  ledgerCreate: (data: { name: string; currency: string; usernames: string[] }) => request.post('/api/billing/ledgers', data),
  ledger: (id: number, params: { transaction_page?: number; transaction_page_size?: number } = {}) => request.get(`/api/billing/ledgers/${id}`, { params }),
  ledgerUpdate: (id: number, data: { name: string; currency: string; show_account?: boolean; show_category?: boolean; show_note?: boolean }) => request.put(`/api/billing/ledgers/${id}`, data),
  ledgerArchive: (id: number) => request.delete(`/api/billing/ledgers/${id}`),
  ledgerRestore: (id: number) => request.post(`/api/billing/ledgers/${id}/restore`),
  invite: (id: number, username: string) => request.post(`/api/billing/ledgers/${id}/members`, { username }),
  removeMember: (id: number, userId: number) => request.delete(`/api/billing/ledgers/${id}/members/${userId}`),
  respondInvitation: (id: number, accept: boolean) => request.post(`/api/billing/ledgers/${id}/invitations/respond`, { accept }),

  sharedCreate: (id: number, data: SharedPayload) => request.post(`/api/billing/ledgers/${id}/transactions`, data),
  sharedUpdate: (id: number, transactionId: number, data: SharedPayload) => request.put(`/api/billing/ledgers/${id}/transactions/${transactionId}`, data),
  sharedDelete: (id: number, transactionId: number) => request.delete(`/api/billing/ledgers/${id}/transactions/${transactionId}`),
  settlementCreate: (id: number, data: { from_user_id: number; to_user_id: number; amount_cents: number; note: string; occurred_at: string }) =>
    request.post(`/api/billing/ledgers/${id}/settlements`, data),
  settlementDelete: (id: number, settlementId: number) => request.delete(`/api/billing/ledgers/${id}/settlements/${settlementId}`),
}
