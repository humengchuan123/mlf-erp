//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStage3G(t *testing.T) {
	admin := newClient(t).login(
		env("TEST_ADMIN_USERNAME", "admin"),
		env("TEST_ADMIN_PASSWORD", "Admin@123456"),
	)

	year := ensureCashFlowYear(t, admin)
	ensureTestYear(t, admin, year)

	accounts := getArray(admin, "/finance/accounts", http.StatusOK)
	accountIDs := make(map[string]string, len(accounts))
	for _, account := range accounts {
		accountIDs[text(account, "code")] = text(account, "id")
	}
	for _, code := range []string{
		"1001", "1002", "1122", "1405", "1601", "2202", "2211", "2221", "222101",
		"4001", "4103", "4104", "6001",
	} {
		if accountIDs[code] == "" {
			t.Fatalf("account %s not found", code)
		}
	}

	// 1002 already has posted demo vouchers, so adding a child account is
	// rejected by the account service (409 CONFLICT). Root/subaccount matching is
	// covered by the GL unit tests; the API test exercises subaccount
	// classification with the existing 222101 tax subaccount below.
	bankID := accountIDs["1002"]

	jan := fmt.Sprintf("%04d-01-15", year)
	janStart := fmt.Sprintf("%04d-01-01", year)
	janEnd := fmt.Sprintf("%04d-01-31", year)
	feb := fmt.Sprintf("%04d-02-15", year)
	febStart := fmt.Sprintf("%04d-02-01", year)
	febEnd := time.Date(year, time.March, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": bankID, "debit": "100.00", "credit": "0"},
		{"account_id": accountIDs["1122"], "debit": "0", "credit": "100.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["2202"], "debit": "40.00", "credit": "0"},
		{"account_id": bankID, "debit": "0", "credit": "40.00"},
	})
	// 2221 is a parent account, so post to its leaf tax subaccount; cash flow
	// classification and indirect balances still resolve the 2221 root.
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["222101"], "debit": "10.00", "credit": "0"},
		{"account_id": bankID, "debit": "0", "credit": "10.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["2211"], "debit": "20.00", "credit": "0"},
		{"account_id": accountIDs["1001"], "debit": "0", "credit": "20.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["1601"], "debit": "50.00", "credit": "0"},
		{"account_id": bankID, "debit": "0", "credit": "50.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": bankID, "debit": "30.00", "credit": "0"},
		{"account_id": accountIDs["1601"], "debit": "0", "credit": "30.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": bankID, "debit": "200.00", "credit": "0"},
		{"account_id": accountIDs["4001"], "debit": "0", "credit": "200.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["4104"], "debit": "15.00", "credit": "0"},
		{"account_id": bankID, "debit": "0", "credit": "15.00"},
	})
	postCashFlowVoucher(t, admin, jan, []map[string]any{
		{"account_id": accountIDs["1001"], "debit": "25.00", "credit": "0"},
		{"account_id": bankID, "debit": "0", "credit": "25.00"},
	})

	janReport := admin.mustGet("/finance/reports/cash-flow?start_date="+janStart+"&end_date="+janEnd, http.StatusOK)
	janOperating := cashFlowSection(t, janReport, "operating")
	janInvesting := cashFlowSection(t, janReport, "investing")
	janFinancing := cashFlowSection(t, janReport, "financing")
	if got := cashFlowAmount(t, janOperating, "inflows", "sales_receipts"); !nearlyEqual(got, 100) {
		t.Fatalf("January sales receipts = %v, want 100", got)
	}
	if got := cashFlowAmount(t, janOperating, "outflows", "purchase_payments"); !nearlyEqual(got, 40) {
		t.Fatalf("January purchase payments = %v, want 40", got)
	}
	if got := cashFlowAmount(t, janOperating, "outflows", "tax_payments"); !nearlyEqual(got, 10) {
		t.Fatalf("January tax payments = %v, want 10", got)
	}
	if got := cashFlowAmount(t, janOperating, "outflows", "employee_payments"); !nearlyEqual(got, 20) {
		t.Fatalf("January employee payments = %v, want 20", got)
	}
	if got := cashFlowAmount(t, janInvesting, "inflows", "disposal_asset_receipts"); !nearlyEqual(got, 30) {
		t.Fatalf("January disposal asset receipts = %v, want 30", got)
	}
	if got := cashFlowAmount(t, janInvesting, "outflows", "asset_purchases"); !nearlyEqual(got, 50) {
		t.Fatalf("January asset purchases = %v, want 50", got)
	}
	if got := cashFlowAmount(t, janFinancing, "inflows", "capital_injections"); !nearlyEqual(got, 200) {
		t.Fatalf("January capital injections = %v, want 200", got)
	}
	if got := cashFlowAmount(t, janFinancing, "outflows", "dividend_interest_payments"); !nearlyEqual(got, 15) {
		t.Fatalf("January dividend payments = %v, want 15", got)
	}
	// The internal 1001/1002 transfer must not leak into any activity.
	for _, check := range []struct {
		section   map[string]any
		direction string
		code      string
	}{
		{janOperating, "inflows", "other_operating_inflows"},
		{janOperating, "outflows", "other_operating_outflows"},
		{janInvesting, "inflows", "other_investing_inflows"},
		{janInvesting, "outflows", "other_investing_outflows"},
		{janFinancing, "inflows", "other_financing_inflows"},
		{janFinancing, "outflows", "other_financing_outflows"},
	} {
		if got := number(t, cashFlowLine(t, check.section, check.direction, check.code), "amount"); !nearlyEqual(got, 0) {
			t.Fatalf("January %s/%s = %v, want 0", check.direction, check.code, got)
		}
	}
	if got := number(t, janOperating, "net"); !nearlyEqual(got, 30) {
		t.Fatalf("January operating net = %v, want 30", got)
	}
	if got := number(t, janInvesting, "net"); !nearlyEqual(got, -20) {
		t.Fatalf("January investing net = %v, want -20", got)
	}
	if got := number(t, janFinancing, "net"); !nearlyEqual(got, 185) {
		t.Fatalf("January financing net = %v, want 185", got)
	}
	if got := number(t, janReport, "net_increase"); !nearlyEqual(got, 195) {
		t.Fatalf("January net increase = %v, want 195", got)
	}
	if got := number(t, janReport, "opening_cash") + 195; !nearlyEqual(got, number(t, janReport, "closing_cash")) {
		t.Fatalf("January closing cash = %v, want opening + 195", number(t, janReport, "closing_cash"))
	}
	janIndirect := cashFlowSection(t, janReport, "indirect")
	if got := number(t, janIndirect, "direct_operating_net"); !nearlyEqual(got, 30) {
		t.Fatalf("January direct operating net = %v, want 30", got)
	}
	if got := number(t, janIndirect, "operating_net"); !nearlyEqual(got, 30) {
		t.Fatalf("January indirect operating net = %v, want 30", got)
	}
	if got := number(t, janIndirect, "difference"); !nearlyEqual(got, 0) {
		t.Fatalf("January indirect difference = %v, want 0", got)
	}
	if got := text(janIndirect, "non_cash_expenses"); got != "0.0000" {
		t.Fatalf("January non-cash expenses = %s, want 0.0000", got)
	}
	if items := cashFlowUnclassified(t, janReport); len(items) != 0 {
		t.Fatalf("January unclassified count = %d, want 0", len(items))
	}
	if got := text(cashFlowLine(t, janOperating, "inflows", "tax_refunds"), "amount"); got != "0.0000" {
		t.Fatalf("January tax refunds = %s, want 0.0000", got)
	}

	postCashFlowVoucher(t, admin, feb, []map[string]any{
		{"account_id": bankID, "debit": "5.00", "credit": "0"},
		{"account_id": accountIDs["4103"], "debit": "0", "credit": "5.00"},
	})

	febReport := admin.mustGet("/finance/reports/cash-flow?start_date="+febStart+"&end_date="+febEnd, http.StatusOK)
	febOperating := cashFlowSection(t, febReport, "operating")
	if got := cashFlowAmount(t, febOperating, "inflows", "other_operating_inflows"); !nearlyEqual(got, 5) {
		t.Fatalf("February other operating inflows = %v, want 5", got)
	}
	if got := number(t, febOperating, "net"); !nearlyEqual(got, 5) {
		t.Fatalf("February operating net = %v, want 5", got)
	}
	febUnclassified := cashFlowUnclassified(t, febReport)
	if len(febUnclassified) != 1 {
		t.Fatalf("February unclassified count = %d, want 1", len(febUnclassified))
	}
	febItem, ok := febUnclassified[0].(map[string]any)
	if !ok {
		t.Fatalf("February unclassified item = %v", febUnclassified[0])
	}
	if got := text(febItem, "amount"); got != "5.0000" {
		t.Fatalf("February unclassified amount = %s, want 5.0000", got)
	}
	if got := text(febItem, "reason"); !strings.Contains(got, "4103") {
		t.Fatalf("February unclassified reason = %s, want it to contain 4103", got)
	}
	if got := text(febItem, "direction"); got != "inflow" {
		t.Fatalf("February unclassified direction = %s, want inflow", got)
	}
	if got := text(febItem, "voucher_code"); got == "" {
		t.Fatal("February unclassified voucher_code is empty")
	}
	voucherDate := text(febItem, "voucher_date")
	if voucherDate == "" {
		t.Fatal("February unclassified voucher_date is empty")
	}
	if _, err := time.Parse("2006-01-02", voucherDate); err != nil {
		t.Fatalf("February unclassified voucher_date = %s, want YYYY-MM-DD: %v", voucherDate, err)
	}
	febIndirect := cashFlowSection(t, febReport, "indirect")
	if got := number(t, febIndirect, "direct_operating_net"); !nearlyEqual(got, 5) {
		t.Fatalf("February direct operating net = %v, want 5", got)
	}
	if got := number(t, febIndirect, "operating_net"); !nearlyEqual(got, 0) {
		t.Fatalf("February indirect operating net = %v, want 0", got)
	}
	if got := number(t, febIndirect, "difference"); !nearlyEqual(got, -5) {
		t.Fatalf("February indirect difference = %v, want -5", got)
	}

	mar := fmt.Sprintf("%04d-03-15", year)
	marStart := fmt.Sprintf("%04d-03-01", year)
	marEnd := fmt.Sprintf("%04d-03-31", year)
	postCashFlowVoucher(t, admin, mar, []map[string]any{
		{"account_id": bankID, "debit": "7.00", "credit": "0"},
		{"account_id": accountIDs["1122"], "debit": "0", "credit": "7.00"},
	})
	postCashFlowVoucher(t, admin, mar, []map[string]any{
		{"account_id": accountIDs["1122"], "debit": "30.00", "credit": "0"},
		{"account_id": accountIDs["6001"], "debit": "0", "credit": "30.00"},
	})
	postCashFlowVoucher(t, admin, mar, []map[string]any{
		{"account_id": accountIDs["1405"], "debit": "20.00", "credit": "0"},
		{"account_id": accountIDs["2202"], "debit": "0", "credit": "20.00"},
	})

	marReport := admin.mustGet("/finance/reports/cash-flow?start_date="+marStart+"&end_date="+marEnd, http.StatusOK)
	marOperating := cashFlowSection(t, marReport, "operating")
	if got := cashFlowAmount(t, marOperating, "inflows", "sales_receipts"); !nearlyEqual(got, 7) {
		t.Fatalf("March sales receipts = %v, want 7", got)
	}
	if got := number(t, marOperating, "net"); !nearlyEqual(got, 7) {
		t.Fatalf("March operating net = %v, want 7", got)
	}
	marIndirect := cashFlowSection(t, marReport, "indirect")
	if got := number(t, marIndirect, "net_profit"); !nearlyEqual(got, 30) {
		t.Fatalf("March net profit = %v, want 30", got)
	}
	if got := number(t, marIndirect, "receivable_increase"); !nearlyEqual(got, 23) {
		t.Fatalf("March receivable increase = %v, want 23", got)
	}
	if got := number(t, marIndirect, "inventory_increase"); !nearlyEqual(got, 20) {
		t.Fatalf("March inventory increase = %v, want 20", got)
	}
	if got := number(t, marIndirect, "payable_increase"); !nearlyEqual(got, 20) {
		t.Fatalf("March payable increase = %v, want 20", got)
	}
	if got := number(t, marIndirect, "direct_operating_net"); !nearlyEqual(got, 7) {
		t.Fatalf("March direct operating net = %v, want 7", got)
	}
	if got := number(t, marIndirect, "operating_net"); !nearlyEqual(got, 7) {
		t.Fatalf("March indirect operating net = %v, want 7", got)
	}
	if got := number(t, marIndirect, "difference"); !nearlyEqual(got, 0) {
		t.Fatalf("March indirect difference = %v, want 0", got)
	}
	if got := text(marIndirect, "non_cash_expenses"); got != "0.0000" {
		t.Fatalf("March non-cash expenses = %s, want 0.0000", got)
	}
	if got := text(marIndirect, "tax_payable_increase"); got != "0.0000" {
		t.Fatalf("March tax payable increase = %s, want 0.0000", got)
	}
	if got := text(marIndirect, "employee_payable_increase"); got != "0.0000" {
		t.Fatalf("March employee payable increase = %s, want 0.0000", got)
	}
	if got := number(t, marReport, "opening_cash") + 7; !nearlyEqual(got, number(t, marReport, "closing_cash")) {
		t.Fatalf("March closing cash = %v, want opening + 7", number(t, marReport, "closing_cash"))
	}
	if items := cashFlowUnclassified(t, marReport); len(items) != 0 {
		t.Fatalf("March unclassified count = %d, want 0", len(items))
	}

	aprStart := fmt.Sprintf("%04d-04-01", year)
	aprEnd := fmt.Sprintf("%04d-04-30", year)
	aprReport := admin.mustGet("/finance/reports/cash-flow?start_date="+aprStart+"&end_date="+aprEnd, http.StatusOK)
	if got := number(t, aprReport, "net_increase"); !nearlyEqual(got, 0) {
		t.Fatalf("April net increase = %v, want 0", got)
	}
	if got := number(t, aprReport, "opening_cash"); !nearlyEqual(got, number(t, aprReport, "closing_cash")) {
		t.Fatalf("April opening cash = %v, closing cash = %v, want equal", got, number(t, aprReport, "closing_cash"))
	}
	if items := cashFlowUnclassified(t, aprReport); len(items) != 0 {
		t.Fatalf("April unclassified count = %d, want 0", len(items))
	}
	aprOperating := cashFlowSection(t, aprReport, "operating")
	aprInvesting := cashFlowSection(t, aprReport, "investing")
	aprFinancing := cashFlowSection(t, aprReport, "financing")
	assertZeroCashFlowItems(t, aprOperating, "inflows", []string{
		"sales_receipts", "tax_refunds", "other_operating_inflows",
	})
	assertZeroCashFlowItems(t, aprOperating, "outflows", []string{
		"purchase_payments", "employee_payments", "tax_payments", "other_operating_outflows",
	})
	assertZeroCashFlowItems(t, aprInvesting, "inflows", []string{
		"disposal_asset_receipts", "other_investing_inflows",
	})
	assertZeroCashFlowItems(t, aprInvesting, "outflows", []string{
		"asset_purchases", "investment_payments", "other_investing_outflows",
	})
	assertZeroCashFlowItems(t, aprFinancing, "inflows", []string{
		"capital_injections", "borrowings", "other_financing_inflows",
	})
	assertZeroCashFlowItems(t, aprFinancing, "outflows", []string{
		"debt_repayments", "dividend_interest_payments", "other_financing_outflows",
	})
	for _, name := range []string{"operating", "investing", "financing"} {
		if got := text(cashFlowSection(t, aprReport, name), "net"); got != "0.0000" {
			t.Fatalf("April %s net = %s, want 0.0000", name, got)
		}
	}

	admin.expectError(http.MethodGet, "/finance/reports/cash-flow?start_date=2026-02-01&end_date=2026-01-01", nil, http.StatusBadRequest, "BAD_REQUEST")
	admin.expectError(http.MethodGet, "/finance/reports/cash-flow?start_date=bad-date", nil, http.StatusBadRequest, "BAD_REQUEST")
	admin.expectError(http.MethodGet, "/finance/reports/cash-flow?end_date=bad-date", nil, http.StatusBadRequest, "BAD_REQUEST")
	defaultReport := admin.mustGet("/finance/reports/cash-flow", http.StatusOK)
	if text(defaultReport, "start_date") == "" || text(defaultReport, "end_date") == "" {
		t.Fatalf("default cash flow range is empty: %v", defaultReport)
	}
	unauthorized := newClient(t)
	unauthorized.expectError(http.MethodGet, "/finance/reports/cash-flow", nil, http.StatusUnauthorized, "UNAUTHORIZED")

	dept := newClient(t).login("demo_dept", env("TEST_DEMO_PASSWORD", "Demo@123456"))
	deptReport := dept.mustGet("/finance/reports/cash-flow?start_date="+janStart+"&end_date="+janEnd, http.StatusOK)
	if got := number(t, deptReport, "net_increase"); !nearlyEqual(got, 0) {
		t.Fatalf("demo_dept net increase = %v, want 0, payload = %v", got, deptReport)
	}
}

func ensureCashFlowYear(t *testing.T, admin *client) int {
	t.Helper()
	for year := 2080; year <= 2500; year++ {
		if len(getArray(admin, periodsPath(year), http.StatusOK)) == 0 {
			return year
		}
	}
	t.Fatal("no clean year left for cash flow test")
	return 0
}

func postCashFlowVoucher(t *testing.T, admin *client, date string, lines []map[string]any) string {
	t.Helper()
	voucher := admin.mustPost("/finance/vouchers", map[string]any{
		"voucher_date": date, "memo": "stage3g cash flow", "lines": lines,
	}, http.StatusCreated)
	voucherID := text(voucher, "id")
	admin.mustPost("/finance/vouchers/"+voucherID+"/post", nil, http.StatusOK)
	return voucherID
}

func cashFlowSection(t *testing.T, report map[string]any, name string) map[string]any {
	t.Helper()
	section, ok := report[name].(map[string]any)
	if !ok {
		t.Fatalf("cash flow section %s missing: %v", name, report)
	}
	return section
}

func cashFlowUnclassified(t *testing.T, report map[string]any) []any {
	t.Helper()
	raw, ok := report["unclassified"].([]any)
	if !ok || raw == nil {
		t.Fatalf("unclassified = %v, want array", report["unclassified"])
	}
	return raw
}

func cashFlowLine(t *testing.T, section map[string]any, direction, code string) map[string]any {
	t.Helper()
	items, _ := section[direction].([]any)
	for _, raw := range items {
		line, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("cash flow line %s/%s has invalid payload %v", direction, code, raw)
		}
		if text(line, "code") == code {
			return line
		}
	}
	t.Fatalf("cash flow line %s/%s not found", direction, code)
	return nil
}

func cashFlowAmount(t *testing.T, section map[string]any, direction, code string) float64 {
	t.Helper()
	return number(t, cashFlowLine(t, section, direction, code), "amount")
}

func assertZeroCashFlowItems(t *testing.T, section map[string]any, direction string, codes []string) {
	t.Helper()
	for _, code := range codes {
		line := cashFlowLine(t, section, direction, code)
		if got := text(line, "amount"); got != "0.0000" {
			t.Fatalf("%s/%s = %s, want 0.0000", direction, code, got)
		}
	}
}
