import { api } from './client'
import type { Paged } from './system'

export interface GlAccount {
  id: string
  code: string
  name: string
  type: string
  direction: string
  parent_id: string
  level: number
  is_leaf: boolean
  status: string
  created_at: string
  updated_at: string
}

export interface GlAccountInput {
  code: string
  name: string
  type: string
  direction: string
  parent_id?: string
  status?: string
}

export interface GlPeriod {
  id: string
  year: number
  month: number
  start_date: string
  end_date: string
  status: string
  closed_at?: string
  closed_by: string
}

export interface YearCloseStatus {
  year: number
  closed: boolean
  voucher_id?: string
  voucher_code?: string
  status?: string
  closed_at?: string
}

export interface GlVoucherLine {
  id: string
  line_no: number
  account_id: string
  account_code: string
  account_name: string
  summary: string
  debit: string
  credit: string
  partner_id: string
  department_id: string
  project_code: string
}

export interface GlVoucher {
  id: string
  code: string
  voucher_date: string
  period_id: string
  voucher_type: string
  source_type: string
  source_id: string
  source_code: string
  status: string
  memo: string
  created_by: string
  posted_by: string
  posted_at?: string
  reversal_of_voucher_id: string
  total_amount: string
  created_at: string
  updated_at: string
  lines: GlVoucherLine[]
}

export interface GlVoucherLineInput {
  account_id: string
  summary: string
  debit: string
  credit: string
  partner_id?: string
  department_id?: string
  project_code?: string
}

export interface GlVoucherInput {
  voucher_date: string
  memo?: string
  lines: GlVoucherLineInput[]
}

export interface TrialBalanceRow {
  account_id: string
  code: string
  name: string
  type: string
  direction: string
  opening: string
  period_debit: string
  period_credit: string
  closing: string
}

export interface TrialBalance {
  start_date: string
  end_date: string
  rows: TrialBalanceRow[]
  total_debit: string
  total_credit: string
}

export interface GeneralLedgerEntry {
  voucher_id: string
  voucher_code: string
  voucher_date: string
  summary: string
  debit: string
  credit: string
  balance: string
}

export interface GeneralLedger {
  account: GlAccount
  start_date: string
  end_date: string
  opening: string
  entries: GeneralLedgerEntry[]
  closing: string
}

export interface ReportLine {
  code: string
  name: string
  amount: string
}

export interface BalanceSheet {
  as_of: string
  assets: ReportLine[]
  liabilities: ReportLine[]
  equity: ReportLine[]
  total_assets: string
  total_liabilities: string
  total_equity: string
  current_profit: string
  total_liabilities_and_equity: string
}

export interface IncomeStatement {
  start_date: string
  end_date: string
  revenue: ReportLine[]
  expenses: ReportLine[]
  total_revenue: string
  total_expenses: string
  net_profit: string
}

export interface Dimension {
  id: string
  code: string
  name: string
}

export interface Dimensions {
  partners: Dimension[]
  departments: Dimension[]
}

export interface AuxiliaryBalanceRow {
  account_id: string
  account_code: string
  account_name: string
  dimension_id: string
  dimension_code: string
  dimension_name: string
  opening: string
  period_debit: string
  period_credit: string
  closing: string
}

export interface AuxiliaryBalance {
  dimension: string
  start_date: string
  end_date: string
  rows: AuxiliaryBalanceRow[]
  total_debit: string
  total_credit: string
}

function queryString(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      search.set(key, String(value))
    }
  }
  const query = search.toString()
  return query ? `?${query}` : ''
}

export const glApi = {
  accounts: {
    list(): Promise<GlAccount[]> {
      return api.get<GlAccount[]>('/finance/accounts')
    },
    create(input: GlAccountInput): Promise<GlAccount> {
      return api.post<GlAccount>('/finance/accounts', input)
    },
    update(id: string, input: GlAccountInput): Promise<GlAccount> {
      return api.put<GlAccount>(`/finance/accounts/${id}`, input)
    }
  },
  periods: {
    list(year?: number): Promise<GlPeriod[]> {
      return api.get<GlPeriod[]>(`/finance/periods${queryString({ year })}`)
    },
    ensureYear(year: number): Promise<unknown> {
      return api.post('/finance/periods/ensure', { year })
    },
    close(id: string): Promise<unknown> {
      return api.post(`/finance/periods/${id}/close`)
    },
    reopen(id: string): Promise<unknown> {
      return api.post(`/finance/periods/${id}/reopen`)
    },
    closeYear(year: number): Promise<unknown> {
      return api.post(`/finance/years/${year}/close`)
    },
    yearCloseStatus(year: number): Promise<YearCloseStatus> {
      return api.get<YearCloseStatus>(`/finance/years/${year}/close`)
    }
  },
  dimensions(): Promise<Dimensions> {
    return api.get<Dimensions>('/finance/dimensions')
  },
  vouchers: {
    list(params: {
      status?: string
      voucher_type?: string
      keyword?: string
      start_date?: string
      end_date?: string
      page?: number
      page_size?: number
    } = {}): Promise<Paged<GlVoucher>> {
      return api.get<Paged<GlVoucher>>(`/finance/vouchers${queryString(params)}`)
    },
    get(id: string): Promise<GlVoucher> {
      return api.get<GlVoucher>(`/finance/vouchers/${id}`)
    },
    create(input: GlVoucherInput): Promise<GlVoucher> {
      return api.post<GlVoucher>('/finance/vouchers', input)
    },
    update(id: string, input: GlVoucherInput): Promise<GlVoucher> {
      return api.put<GlVoucher>(`/finance/vouchers/${id}`, input)
    },
    post(id: string): Promise<unknown> {
      return api.post(`/finance/vouchers/${id}/post`)
    },
    reverse(id: string): Promise<GlVoucher> {
      return api.post<GlVoucher>(`/finance/vouchers/${id}/reverse`)
    }
  },
  reports: {
    trialBalance(params: { start_date?: string; end_date?: string } = {}): Promise<TrialBalance> {
      return api.get<TrialBalance>(`/finance/reports/trial-balance${queryString(params)}`)
    },
    generalLedger(params: {
      account_id: string
      start_date?: string
      end_date?: string
    }): Promise<GeneralLedger> {
      return api.get<GeneralLedger>(`/finance/reports/general-ledger${queryString(params)}`)
    },
    balanceSheet(params: { as_of?: string } = {}): Promise<BalanceSheet> {
      return api.get<BalanceSheet>(`/finance/reports/balance-sheet${queryString(params)}`)
    },
    incomeStatement(params: { start_date?: string; end_date?: string } = {}): Promise<IncomeStatement> {
      return api.get<IncomeStatement>(`/finance/reports/income-statement${queryString(params)}`)
    },
    auxiliaryBalance(params: {
      dimension: string
      account_id?: string
      start_date?: string
      end_date?: string
    }): Promise<AuxiliaryBalance> {
      return api.get<AuxiliaryBalance>(`/finance/reports/auxiliary-balance${queryString(params)}`)
    }
  }
}
