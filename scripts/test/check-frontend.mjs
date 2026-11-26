import { readFileSync, existsSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const failures = []

function read(path) {
  return readFileSync(resolve(root, path), 'utf8')
}

function assertFile(path, checks) {
  if (!existsSync(resolve(root, path))) {
    failures.push(`missing file: ${path}`)
    return
  }
  const content = read(path)
  for (const check of checks) {
    if (!content.includes(check)) {
      failures.push(`${path} missing: ${check}`)
    }
  }
}

assertFile('apps/web/src/router/index.ts', [
  "path: 'sales/orders'",
  'OrderListView.vue',
  'sales.order.read',
  "path: 'purchase/receipts'",
  'PurchaseReceiptListView.vue',
  'purchase.receipt.read',
  "path: 'purchase/returns'",
  'PurchaseReturnListView.vue',
  'purchase.return.read',
  "path: 'sales/shipments'",
  'ShipmentListView.vue',
  'sales.shipment.read',
  "path: 'sales/returns'",
  'SalesReturnListView.vue',
  'sales.return.read',
  "path: 'finance/invoices'",
  'InvoiceListView.vue',
  'finance.invoice.read',
  "path: 'finance/credit-notes'",
  'CreditNoteListView.vue',
  "path: 'finance/payments'",
  'PaymentListView.vue',
  'finance.payment.read',
  "path: 'finance/accounts'",
  'AccountListView.vue',
  'finance.account.read',
  "path: 'finance/periods'",
  'PeriodListView.vue',
  'finance.period.read',
  "path: 'finance/vouchers'",
  'VoucherListView.vue',
  'finance.voucher.read',
  "path: 'finance/reports'",
  'ReportView.vue',
  'finance.report.read',
  "path: 'finance/exchange-rates'",
  'ExchangeRateListView.vue',
  'finance.exchange_rate.read',
  "path: 'system/approval-flows'",
  'system-approval-flows',
  'ApprovalFlowListView.vue',
  "path: 'system/pending-approvals'",
  'system-pending-approvals',
  'PendingApprovalView.vue',
  'system.approval.read'
])

assertFile('apps/web/src/layouts/DefaultLayout.vue', [
  'sales.order.read',
  'sales.shipment.read',
  'sales.return.read',
  'purchase.receipt.read',
  'purchase.return.read',
  'finance.invoice.read',
  'finance.payment.read',
  '销售订单',
  '发货单',
  '销售退货',
  '采购收货',
  '采购退货',
  '应收应付',
  '贷项单',
  'index="/purchase/receipts"',
  'index="/purchase/returns"',
  'index="/sales/returns"',
  'index="/finance/credit-notes"',
  '收付款',
  '会计科目',
  '会计期间',
  '会计凭证',
  '财务报表',
  '汇率管理',
  '审批流程',
  '待我审批',
  'system.approval.read',
  'index="/system/approval-flows"',
  'index="/system/pending-approvals"'
])

assertFile('apps/web/src/views/system/ApprovalFlowListView.vue', [
  'PageHeader',
  'ApprovalFlowListView',
  'approvalApi.flows',
  'approvalApi.createFlow',
  'approvalApi.updateFlow',
  'approvalApi.removeFlow',
  'approvalApi.setFlowActive',
  'FLOW_IN_USE',
  '审批步骤'
])

assertFile('apps/web/src/views/system/PendingApprovalView.vue', [
  'PageHeader',
  'PendingApprovalView',
  'approvalApi.pending',
  'approvalApi.request(',
  'salesApi.approve',
  'purchaseApi.approve',
  'salesApi.reject',
  'purchaseApi.reject',
  '审批动作'
])

assertFile('apps/web/src/views/finance/ExchangeRateListView.vue', [
  'PageHeader',
  'el-table',
  'el-dialog',
  'financeApi.exchangeRates.list',
  'financeApi.exchangeRates.upsert',
  'financeApi.exchangeRates.update'
])

assertFile('apps/web/src/views/sales/OrderListView.vue', [
  'PageHeader',
  'el-table',
  'el-dialog',
  'el-drawer',
  'salesApi.ship',
  'sales.order.write',
  'price_includes_tax',
  'tax_rate_id',
  'tax_amount',
  'gross_amount',
  'exchange_rate',
  'base_total_amount',
  'shipIdempotencyKey',
  'newIdempotencyKey',
  '发货',
  'approvalApi.requests',
  '审批历史',
  'canReadApproval',
  'system.approval.read',
  'v-if="canReadApproval"',
  'if (canReadApproval.value)'
])

assertFile('apps/web/src/views/purchase/OrderListView.vue', [
  'PageHeader',
  'el-table',
  'el-dialog',
  'purchaseApi.receive',
  'price_includes_tax',
  'tax_rate_id',
  'tax_amount',
  'gross_amount',
  'exchange_rate',
  'base_total_amount',
  'receiveIdempotencyKey',
  'newIdempotencyKey',
  'approvalApi.requests',
  '审批历史',
  'canReadApproval',
  'system.approval.read',
  'v-if="canReadApproval"',
  'if (canReadApproval.value)'
])

assertFile('apps/web/src/views/purchase/PurchaseReceiptListView.vue', [
  'PageHeader',
  'el-table',
  'purchaseApi.receipts(',
  'purchaseApi.returns.create',
  'returned_qty',
  'returnable_qty',
  'return_qty',
  'returnIdempotencyKey',
  'newIdempotencyKey'
])

assertFile('apps/web/src/views/purchase/PurchaseReturnListView.vue', [
  'PageHeader',
  'el-table',
  'purchaseApi.returns.list',
  'purchaseApi.returns.get',
  'credit_note_remaining_amount',
  'voucher_code'
])

assertFile('apps/web/src/views/sales/ShipmentListView.vue', [
  'PageHeader',
  'el-table',
  'el-drawer',
  'salesApi.shipments',
  'salesApi.returns.create',
  'returnable_qty',
  'net_amount',
  'tax_amount',
  'total_cost',
  'returnIdempotencyKey',
  'newIdempotencyKey'
])

assertFile('apps/web/src/views/sales/SalesReturnListView.vue', [
  'PageHeader',
  'el-table',
  'el-drawer',
  'salesApi.returns.list',
  'salesApi.returns.get',
  'credit_note'
])

assertFile('apps/web/src/views/finance/CreditNoteListView.vue', [
  'PageHeader',
  'el-table',
  'el-drawer',
  'financeApi.creditNotes.list',
  'financeApi.creditNotes.get',
  'credit_type',
  'creditTypeLabel',
  'purchase_return',
  'creditNotes.allocate',
  'creditNotes.refund',
  'creditNotes.allocationCandidates',
  'refunded_amount',
  'prop="refunded_amount"',
  'detail.allocations',
  'detail.refunds',
  'canAllocateNote',
  'refundCreditBase',
  'refundCashBase',
  'refundFxAdjustment',
  'el-dialog',
  'finance.credit_note.apply',
  'finance.credit_note.refund',
  'applyIdempotencyKey',
  'refundIdempotencyKey',
  'newIdempotencyKey'
])

assertFile('apps/web/src/views/finance/InvoiceListView.vue', [
  'PageHeader',
  'el-table',
  'el-drawer',
  'el-dialog',
  'financeApi.invoices',
  'financeApi.invoice',
  'financeApi.issueInvoice',
  'financeApi.voidInvoice',
  'vat_invoice_status',
  'net_amount',
  'tax_amount',
  'exchange_rate',
  'base_total_amount',
  'credited_amount',
  'base_credited_amount',
  'outstanding_amount',
  'detail.allocations',
  'detail.credit_note_allocations',
  'credit_note_code',
  '贷项抵用',
  'payment_allocation_reversals',
  '付款反核销',
  'fx_reversal_voucher_codes',
  '收付款反核销记录'
])

assertFile('apps/web/src/views/finance/PaymentListView.vue', [
  'PageHeader',
  'el-table',
  'el-dialog',
  'financeApi.payments',
  'financeApi.createPayment',
  'financeApi.allocate',
  'unallocated_amount',
  'payments.unallocate',
  'finance.payment.reverse',
  'PaymentAllocationReversal',
  'reversals',
  'fx_reversal_voucher_codes',
  '反核销',
  'reversal_date',
  '@click="openReversal(row)"',
  'reverseSelected',
  'invoice_ids',
  'createIdempotencyKey',
  'allocateIdempotencyKey',
  'reverseIdempotencyKey',
  'shouldRetainIdempotencyKey',
  'newIdempotencyKey'
])

assertFile('packages/sdk/src/index.ts', ["./sales", "./finance", "./gl", "./approval"])

assertFile('packages/sdk/src/approval.ts', [
  'ApprovalFlow',
  'ApprovalRequest',
  'ApprovalFlowStep',
  'ApprovalAction',
  'approvalApi',
  '/system/approval-flows',
  'createFlow',
  'updateFlow',
  'removeFlow',
  'setFlowActive',
  '/system/approval-flows/${id}/active',
  '/approvals/pending',
  '/approvals/requests',
  'pending(',
  'requests(',
  'request(',
  'completed_at?: string | null',
  'tenant-level'
])

assertFile('packages/sdk/src/client.ts', [
  'RequestOptions',
  'idempotencyKey',
  'Idempotency-Key'
])

assertFile('apps/web/src/utils/idempotency.ts', [
  'shouldRetainIdempotencyKey',
  'newIdempotencyKey',
  'randomUUID',
  'getRandomValues',
  'IDEMPOTENCY_CONFLICT',
  'IDEMPOTENCY_IN_PROGRESS',
  'IDEMPOTENCY_RESPONSE_UNAVAILABLE'
])

assertFile('apps/web/src/views/finance/AccountListView.vue', [
  'PageHeader',
  'el-table',
  'el-dialog',
  'glApi.accounts.list',
  'glApi.accounts.create',
  'glApi.accounts.update'
])

assertFile('apps/web/src/views/finance/PeriodListView.vue', [
  'PageHeader',
  'el-table',
  'glApi.periods.list',
  'glApi.periods.close',
  'glApi.periods.reopen',
  'glApi.periods.ensureYear',
  'glApi.periods.closeYear',
  'glApi.periods.yearCloseStatus'
])

assertFile('apps/web/src/views/finance/VoucherListView.vue', [
  'PageHeader',
  'el-table',
  'el-dialog',
  'el-drawer',
  'glApi.vouchers.list',
  'glApi.vouchers.create',
  'glApi.vouchers.post',
  'glApi.vouchers.reverse',
  'glApi.dimensions',
  'project_code'
])

assertFile('apps/web/src/views/finance/ReportView.vue', [
  'PageHeader',
  'el-tabs',
  'glApi.reports.trialBalance',
  'glApi.reports.generalLedger',
  'glApi.reports.balanceSheet',
  'glApi.reports.incomeStatement',
  'glApi.reports.auxiliaryBalance',
  'glApi.reports.vatSummary',
  'glApi.reports.cashFlow',
  'cashRange',
  'loadCashFlow',
  'name="cash"',
  '现金流量表',
  'unclassified',
  'indirect'
])

assertFile('packages/sdk/src/sales.ts', [
  'salesApi',
  'list(',
  'get(',
  'create(',
  'update(',
  'submit(',
  'approve(',
  'reject(',
  '/reject',
  'cancel(',
  'ship(',
  'shipments(',
  'returns',
  '/sales/returns',
  '/sales/shipments/${shipmentID}/returns',
  'SalesReturn',
  'CreateSalesReturnInput',
  'returned_qty',
  'returnable_qty',
  'tax_rate_id',
  'gross_amount',
  'exchange_rate',
  'base_total_amount',
  'idempotencyKey'
])

assertFile('packages/sdk/src/finance.ts', [
  'financeApi',
  'invoices(',
  'invoice(',
  'issueInvoice(',
  'voidInvoice(',
  'creditNotes',
  '/finance/credit-notes/${id}/allocations',
  '/finance/credit-notes/${id}/allocation-candidates',
  '/finance/credit-notes/${id}/refunds',
  'allocationCandidates(id: string)',
  'refund(id: string, input: RefundCreditNoteInput)',
  'CreditNote',
  'CreditNoteSummary',
  'CreditNoteAllocation',
  'CreditNoteRefund',
  'AllocateCreditNoteInput',
  'RefundCreditNoteInput',
  'InvoiceCreditNoteAllocation',
  'credit_type',
  '/finance/credit-notes',
  'credit_note_allocations',
  'remaining_amount',
  'exchangeRates',
  'exchange_rate',
  'base_total_amount',
  'vat_invoice_status',
  'payments(',
  'payments.get',
  'payment(',
  'createPayment(',
  'unallocate(id: string, input: UnallocatePaymentInput)',
  '/finance/payments/${id}/unallocate',
  'PaymentAllocationReversal',
  'UnallocatePaymentInput',
  'payment_allocation_reversals',
  'fx_reversal_voucher_codes',
  'idempotencyKey'
])

assertFile('packages/sdk/src/purchase.ts', [
  'purchaseApi',
  'reject(',
  '/reject',
  'returns',
  '/purchase/returns',
  '/purchase/receipts/${receiptID}/returns',
  'PurchaseReturn',
  'CreatePurchaseReturnInput',
  'returned_qty',
  'returnable_qty',
  'tax_rate_id',
  'gross_amount',
  'exchange_rate',
  'base_total_amount',
  'idempotencyKey'
])

assertFile('api/openapi.yaml', [
  '/system/approval-flows:',
  '/system/approval-flows/{id}/active:',
  '/approvals/pending:',
  '/approvals/requests:',
  'ApprovalFlow:',
  'ApprovalFlowActiveInput:',
  'ApprovalRequest:',
  'ApprovalFlowInput:',
  'completed_at:',
  "type: [string, 'null']",
  'approval_pending:',
  'current_step:',
  '空字符串表示租户级',
  '/purchase/receipts/{id}/returns:',
  '/purchase/returns:',
  '/purchase/returns/{id}:',
  '/sales/shipments/{id}/returns:',
  '/sales/returns:',
  '/sales/returns/{id}:',
  '/finance/credit-notes:',
  '/finance/credit-notes/{id}:',
  '/finance/credit-notes/{id}/allocations:',
  '/finance/credit-notes/{id}/allocation-candidates:',
  '/finance/credit-notes/{id}/refunds:',
  '/finance/payments/{id}/unallocate:',
  'CreatePurchaseReturnInput',
  'PurchaseReturn',
  'CreateSalesReturnInput',
  'SalesReturn',
  'CreditNoteSummary',
  'CreditNoteAllocation',
  'CreditNoteRefund',
  'AllocateCreditNoteInput',
  'RefundCreditNoteInput',
  'credit_note_allocations',
  'PaymentDetailResponse',
  'PaymentDetail:',
  'PaymentAllocation:',
  'PaymentAllocationReversal:',
  'UnallocatePaymentInput',
  'payment_allocation_reversals',
  'fx_reversal_voucher_codes',
  'IdempotencyKey',
  'IDEMPOTENCY_CONFLICT',
  'IDEMPOTENCY_IN_PROGRESS'
])

assertFile('packages/sdk/src/gl.ts', [
  'glApi',
  'accounts',
  'periods',
  'vouchers',
  'dimensions(',
  'trialBalance(',
  'generalLedger(',
  'balanceSheet(',
  'incomeStatement(',
  'auxiliaryBalance(',
  'vatSummary(',
  'CashFlowStatement',
  'cashFlow',
  '/finance/reports/cash-flow'
])

assertFile('package.json', ['test:api:stage3b', 'test:api:stage3e', 'test:api:stage3f', 'test:api:stage3g', 'test:api:stage3h', 'test:frontend'])

if (failures.length > 0) {
  console.error('Frontend completeness check failed:')
  for (const failure of failures) {
    console.error(`- ${failure}`)
  }
  process.exit(1)
}

console.log('Frontend completeness check passed.')
