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
  "path: 'sales/shipments'",
  'ShipmentListView.vue',
  'sales.shipment.read',
  "path: 'finance/invoices'",
  'InvoiceListView.vue',
  'finance.invoice.read',
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
  'finance.report.read'
])

assertFile('apps/web/src/layouts/DefaultLayout.vue', [
  'sales.order.read',
  'sales.shipment.read',
  'finance.invoice.read',
  'finance.payment.read',
  '销售订单',
  '发货单',
  '应收应付',
  '收付款',
  '会计科目',
  '会计期间',
  '会计凭证',
  '财务报表'
])

assertFile('apps/web/src/views/sales/OrderListView.vue', [
  'PageHeader',
  'el-table',
  'el-dialog',
  'el-drawer',
  'salesApi.ship',
  'sales.order.write',
  '发货'
])

assertFile('apps/web/src/views/sales/ShipmentListView.vue', [
  'PageHeader',
  'el-table',
  'el-drawer',
  'salesApi.shipments',
  'total_cost'
])

assertFile('apps/web/src/views/finance/InvoiceListView.vue', [
  'PageHeader',
  'el-table',
  'el-drawer',
  'financeApi.invoices',
  'financeApi.invoice',
  'outstanding_amount',
  'allocations'
])

assertFile('apps/web/src/views/finance/PaymentListView.vue', [
  'PageHeader',
  'el-table',
  'el-dialog',
  'financeApi.payments',
  'financeApi.createPayment',
  'financeApi.allocate',
  'unallocated_amount'
])

assertFile('packages/sdk/src/index.ts', ["./sales", "./finance", "./gl"])

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
  'glApi.periods.reopen'
])

assertFile('apps/web/src/views/finance/VoucherListView.vue', [
  'PageHeader',
  'el-table',
  'el-dialog',
  'el-drawer',
  'glApi.vouchers.list',
  'glApi.vouchers.create',
  'glApi.vouchers.post',
  'glApi.vouchers.reverse'
])

assertFile('apps/web/src/views/finance/ReportView.vue', [
  'PageHeader',
  'el-tabs',
  'glApi.reports.trialBalance',
  'glApi.reports.generalLedger',
  'glApi.reports.balanceSheet',
  'glApi.reports.incomeStatement'
])

assertFile('packages/sdk/src/sales.ts', [
  'salesApi',
  'list(',
  'get(',
  'create(',
  'update(',
  'submit(',
  'approve(',
  'cancel(',
  'ship(',
  'shipments('
])

assertFile('packages/sdk/src/finance.ts', [
  'financeApi',
  'invoices(',
  'invoice(',
  'payments(',
  'payment(',
  'createPayment(',
  'allocate('
])

assertFile('packages/sdk/src/gl.ts', [
  'glApi',
  'accounts',
  'periods',
  'vouchers',
  'trialBalance(',
  'generalLedger(',
  'balanceSheet(',
  'incomeStatement('
])

assertFile('package.json', ['test:api:stage3b', 'test:frontend'])

if (failures.length > 0) {
  console.error('Frontend completeness check failed:')
  for (const failure of failures) {
    console.error(`- ${failure}`)
  }
  process.exit(1)
}

console.log('Frontend completeness check passed.')
