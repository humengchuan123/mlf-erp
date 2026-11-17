import { api } from './client'
import type { Paged } from './system'

export interface Invoice {
  id: string
  code: string
  invoice_type: string
  source_type: string
  source_id: string
  source_code: string
  partner_id: string
  partner_name: string
  invoice_date: string
  due_date: string
  currency: string
  exchange_rate: string
  status: string
  net_amount: string
  tax_amount: string
  total_amount: string
  base_net_amount: string
  base_tax_amount: string
  base_total_amount: string
  base_paid_amount: string
  base_revalued_amount: string
  paid_amount: string
  outstanding_amount: string
  vat_invoice_no: string
  vat_invoice_type: string
  vat_invoice_date: string
  vat_invoice_status: string
  buyer_tax_no: string
  seller_tax_no: string
  remark: string
  created_at: string
  updated_at: string
}

export interface IssueInvoiceInput {
  vat_invoice_no: string
  vat_invoice_type: string
  vat_invoice_date: string
  buyer_tax_no?: string
  seller_tax_no?: string
}

export interface InvoiceAllocation {
  id: string
  payment_id: string
  payment_code: string
  payment_type: string
  payment_date: string
  amount: string
  created_at: string
}

export interface InvoiceDetail extends Invoice {
  allocations: InvoiceAllocation[]
}

export interface Payment {
  id: string
  code: string
  payment_type: string
  partner_id: string
  partner_name: string
  payment_date: string
  currency: string
  exchange_rate: string
  amount: string
  base_amount: string
  allocated_amount: string
  base_allocated_amount: string
  unallocated_amount: string
  status: string
  method: string
  reference: string
  remark: string
  voucher_code: string
  created_at: string
}

export interface PaymentInput {
  payment_type: string
  partner_id: string
  payment_date: string
  currency: string
  exchange_rate?: string
  amount: string
  method?: string
  reference?: string
  remark?: string
}

export interface ExchangeRate {
  id: string
  currency_code: string
  rate_date: string
  rate: string
  created_at: string
  updated_at: string
}

export interface ExchangeRateInput {
  currency_code: string
  rate_date: string
  rate: string
}

export interface LatestExchangeRate {
  currency_code: string
  date: string
  rate: string
}

export interface AllocationInput {
  invoice_id: string
  amount: string
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

export const financeApi = {
  invoices(params: {
    invoice_type?: string
    status?: string
    vat_invoice_status?: string
    partner_id?: string
    keyword?: string
    page?: number
    page_size?: number
  } = {}): Promise<Paged<Invoice>> {
    return api.get<Paged<Invoice>>(`/finance/invoices${queryString(params)}`)
  },
  invoice(id: string): Promise<InvoiceDetail> {
    return api.get<InvoiceDetail>(`/finance/invoices/${id}`)
  },
  issueInvoice(id: string, input: IssueInvoiceInput): Promise<Invoice> {
    return api.post<Invoice>(`/finance/invoices/${id}/issue`, input)
  },
  voidInvoice(id: string): Promise<Invoice> {
    return api.post<Invoice>(`/finance/invoices/${id}/void`)
  },
  payments(params: {
    payment_type?: string
    status?: string
    partner_id?: string
    keyword?: string
    page?: number
    page_size?: number
  } = {}): Promise<Paged<Payment>> {
    return api.get<Paged<Payment>>(`/finance/payments${queryString(params)}`)
  },
  payment(id: string): Promise<Payment> {
    return api.get<Payment>(`/finance/payments/${id}`)
  },
  createPayment(input: PaymentInput): Promise<Payment> {
    return api.post<Payment>('/finance/payments', input)
  },
  allocate(paymentId: string, allocations: AllocationInput[]): Promise<Payment> {
    return api.post<Payment>(`/finance/payments/${paymentId}/allocate`, { allocations })
  },
  exchangeRates: {
    list(params: { currency?: string; start_date?: string; end_date?: string; page?: number; page_size?: number } = {}): Promise<Paged<ExchangeRate>> {
      return api.get<Paged<ExchangeRate>>(`/finance/exchange-rates${queryString(params)}`)
    },
    latest(currency: string, date?: string): Promise<LatestExchangeRate> {
      return api.get<LatestExchangeRate>(`/finance/exchange-rates/latest${queryString({ currency, date })}`)
    },
    upsert(input: ExchangeRateInput): Promise<ExchangeRate> {
      return api.post<ExchangeRate>('/finance/exchange-rates', input)
    },
    update(id: string, input: ExchangeRateInput): Promise<ExchangeRate> {
      return api.put<ExchangeRate>(`/finance/exchange-rates/${id}`, input)
    }
  }
}
