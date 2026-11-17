//go:build integration

package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func getArray(c *client, path string, wantStatus int) []map[string]any {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodGet, c.baseURL+apiPrefix+path, nil)
	if err != nil {
		c.t.Fatalf("build array request %s: %v", path, err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		c.t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		c.t.Fatalf("GET %s status = %d, want %d, body = %s", path, resp.StatusCode, wantStatus, string(raw))
	}
	var payload struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		c.t.Fatalf("decode array %s: %v, body = %s", path, err, string(raw))
	}
	return payload.Data
}

func findArrayItem(items []map[string]any, key, value string) map[string]any {
	for _, item := range items {
		if text(item, key) == value {
			return item
		}
	}
	return nil
}

func findVoucherBySource(c *client, sourceCode string) map[string]any {
	items := listItems(c, "/finance/vouchers?keyword="+url.QueryEscape(sourceCode)+"&page_size=50", http.StatusOK)
	for _, item := range items {
		if text(item, "source_code") == sourceCode {
			return item
		}
	}
	return nil
}

func findLineByAccount(t *testing.T, lines []any, accountCode string) map[string]any {
	t.Helper()
	for _, item := range lines {
		line, ok := item.(map[string]any)
		if ok && text(line, "account_code") == accountCode {
			return line
		}
	}
	t.Fatalf("voucher line for account %s not found", accountCode)
	return nil
}

func TestStage3C(t *testing.T) {
	admin := newClient(t).login(
		env("TEST_ADMIN_USERNAME", "admin"),
		env("TEST_ADMIN_PASSWORD", "Admin@123456"),
	)

	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	endDate := now.Format("2006-01-02")

	accounts := getArray(admin, "/finance/accounts", http.StatusOK)
	if len(accounts) < 20 {
		t.Fatalf("account count = %d, want >= 20", len(accounts))
	}
	accountByCode := map[string]map[string]any{}
	for _, account := range accounts {
		accountByCode[text(account, "code")] = account
	}
	for _, code := range []string{"1001", "1002", "1122", "1405", "2202", "4001", "6001", "6401"} {
		if accountByCode[code] == nil {
			t.Fatalf("account %s not found", code)
		}
	}
	cashID := text(accountByCode["1001"], "id")
	capitalID := text(accountByCode["4001"], "id")
	arID := text(accountByCode["1122"], "id")

	periods := getArray(admin, "/finance/periods?year="+itoa(now.Year()), http.StatusOK)
	if len(periods) == 0 {
		t.Fatalf("no accounting periods for year %d", now.Year())
	}
	var currentPeriod map[string]any
	for _, period := range periods {
		if month, ok := period["month"].(float64); ok && int(month) == int(now.Month()) {
			currentPeriod = period
			break
		}
	}
	if currentPeriod == nil {
		t.Fatalf("current period month %d not found", now.Month())
	}
	if text(currentPeriod, "status") == "closed" {
		admin.mustPost("/finance/periods/"+text(currentPeriod, "id")+"/reopen", nil, http.StatusOK)
	}

	customer := findItem(listItems(admin, "/masterdata/partners?keyword=CUS-DEMO-001&page_size=20", http.StatusOK), "code", "CUS-DEMO-001")
	supplier := findItem(listItems(admin, "/masterdata/partners?keyword=SUP-DEMO-001&page_size=20", http.StatusOK), "code", "SUP-DEMO-001")
	warehouse := findItem(listItems(admin, "/masterdata/warehouses?keyword=WH-SH&page_size=20", http.StatusOK), "code", "WH-SH")
	material := findItem(listItems(admin, "/masterdata/materials?keyword=MAT-DEMO-004&page_size=20", http.StatusOK), "code", "MAT-DEMO-004")
	if customer == nil || supplier == nil || warehouse == nil || material == nil {
		t.Fatal("demo master data missing")
	}

	t.Run("purchase receipt auto voucher", func(t *testing.T) {
		order := admin.mustPost("/purchase/orders", map[string]any{
			"supplier_id":  text(supplier, "id"),
			"warehouse_id": text(warehouse, "id"),
			"order_date":   endDate,
			"currency":     "CNY",
			"remark":       "stage3c-purchase",
			"lines": []map[string]any{
				{"material_id": text(material, "id"), "quantity": "2", "unit_price": "100.00"},
			},
		}, http.StatusCreated)
		orderID := text(order, "id")
		admin.mustPost("/purchase/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/purchase/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/purchase/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		receipt := admin.mustPost("/purchase/orders/"+orderID+"/receive", map[string]any{
			"receipt_date": endDate,
			"remark":       "stage3c-purchase-receipt",
			"lines": []map[string]any{
				{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "2"},
			},
		}, http.StatusCreated)
		receiptCode := text(receipt, "code")

		voucher := findVoucherBySource(admin, receiptCode)
		if voucher == nil {
			t.Fatalf("purchase voucher not found for receipt %s", receiptCode)
		}
		if got := text(voucher, "voucher_type"); got != "purchase_receipt" {
			t.Fatalf("purchase voucher type = %q, want purchase_receipt", got)
		}
		detailVoucher := admin.mustGet("/finance/vouchers/"+text(voucher, "id"), http.StatusOK)
		voucherLines, _ := detailVoucher["lines"].([]any)
		inventoryLine := findLineByAccount(t, voucherLines, "1405")
		if got := number(t, inventoryLine, "debit"); got != 200 {
			t.Fatalf("inventory debit = %v, want 200", got)
		}
		apLine := findLineByAccount(t, voucherLines, "2202")
		if got := number(t, apLine, "credit"); got != 200 {
			t.Fatalf("AP credit = %v, want 200", got)
		}
	})

	var salesVoucherID string
	t.Run("sales shipment auto voucher", func(t *testing.T) {
		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  text(customer, "id"),
			"warehouse_id": text(warehouse, "id"),
			"order_date":   endDate,
			"currency":     "CNY",
			"remark":       "stage3c-sales",
			"lines": []map[string]any{
				{"material_id": text(material, "id"), "quantity": "2", "unit_price": "150.00"},
			},
		}, http.StatusCreated)
		orderID := text(order, "id")
		admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
			"shipment_date": endDate,
			"remark":        "stage3c-sales-shipment",
			"lines": []map[string]any{
				{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "2"},
			},
		}, http.StatusCreated)
		shipmentCode := text(shipment, "code")
		expectedCost := number(t, shipment, "total_cost")

		voucher := findVoucherBySource(admin, shipmentCode)
		if voucher == nil {
			t.Fatalf("sales voucher not found for shipment %s", shipmentCode)
		}
		salesVoucherID = text(voucher, "id")
		if got := text(voucher, "voucher_type"); got != "sales_shipment" {
			t.Fatalf("sales voucher type = %q, want sales_shipment", got)
		}
		detailVoucher := admin.mustGet("/finance/vouchers/"+salesVoucherID, http.StatusOK)
		voucherLines, _ := detailVoucher["lines"].([]any)
		arLine := findLineByAccount(t, voucherLines, "1122")
		if got := number(t, arLine, "debit"); got != 300 {
			t.Fatalf("AR debit = %v, want 300", got)
		}
		revenueLine := findLineByAccount(t, voucherLines, "6001")
		if got := number(t, revenueLine, "credit"); got != 300 {
			t.Fatalf("revenue credit = %v, want 300", got)
		}
		cogsLine := findLineByAccount(t, voucherLines, "6401")
		if got := number(t, cogsLine, "debit"); !nearlyEqual(got, expectedCost) {
			t.Fatalf("COGS debit = %v, want %v", got, expectedCost)
		}
		inventoryLine := findLineByAccount(t, voucherLines, "1405")
		if got := number(t, inventoryLine, "credit"); !nearlyEqual(got, expectedCost) {
			t.Fatalf("inventory credit = %v, want %v", got, expectedCost)
		}
	})

	t.Run("receipt and supplier payment auto vouchers", func(t *testing.T) {
		receiptPayment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   text(customer, "id"),
			"payment_date": endDate,
			"currency":     "CNY",
			"amount":       "100.00",
			"method":       "bank",
			"reference":    "stage3c-receipt",
		}, http.StatusCreated)
		receiptVoucher := findVoucherBySource(admin, text(receiptPayment, "code"))
		if receiptVoucher == nil {
			t.Fatalf("receipt voucher not found for payment %s", text(receiptPayment, "code"))
		}
		receiptDetail := admin.mustGet("/finance/vouchers/"+text(receiptVoucher, "id"), http.StatusOK)
		receiptLines, _ := receiptDetail["lines"].([]any)
		bankLine := findLineByAccount(t, receiptLines, "1002")
		if got := number(t, bankLine, "debit"); got != 100 {
			t.Fatalf("bank debit = %v, want 100", got)
		}
		arLine := findLineByAccount(t, receiptLines, "1122")
		if got := number(t, arLine, "credit"); got != 100 {
			t.Fatalf("AR credit = %v, want 100", got)
		}

		supplierPayment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "payment",
			"partner_id":   text(supplier, "id"),
			"payment_date": endDate,
			"currency":     "CNY",
			"amount":       "100.00",
			"method":       "bank",
			"reference":    "stage3c-payment",
		}, http.StatusCreated)
		paymentVoucher := findVoucherBySource(admin, text(supplierPayment, "code"))
		if paymentVoucher == nil {
			t.Fatalf("supplier payment voucher not found for payment %s", text(supplierPayment, "code"))
		}
		paymentDetail := admin.mustGet("/finance/vouchers/"+text(paymentVoucher, "id"), http.StatusOK)
		paymentLines, _ := paymentDetail["lines"].([]any)
		apLine := findLineByAccount(t, paymentLines, "2202")
		if got := number(t, apLine, "debit"); got != 100 {
			t.Fatalf("AP debit = %v, want 100", got)
		}
		bankLine = findLineByAccount(t, paymentLines, "1002")
		if got := number(t, bankLine, "credit"); got != 100 {
			t.Fatalf("bank credit = %v, want 100", got)
		}
	})

	t.Run("manual voucher post and reverse", func(t *testing.T) {
		voucher := admin.mustPost("/finance/vouchers", map[string]any{
			"voucher_date": endDate,
			"memo":         "stage3c manual voucher",
			"lines": []map[string]any{
				{"account_id": cashID, "summary": "test debit", "debit": "50.00", "credit": "0"},
				{"account_id": capitalID, "summary": "test credit", "debit": "0", "credit": "50.00"},
			},
		}, http.StatusCreated)
		voucherID := text(voucher, "id")
		if got := text(voucher, "status"); got != "draft" {
			t.Fatalf("manual voucher status = %q, want draft", got)
		}
		admin.mustPost("/finance/vouchers/"+voucherID+"/post", nil, http.StatusOK)
		posted := admin.mustGet("/finance/vouchers/"+voucherID, http.StatusOK)
		if got := text(posted, "status"); got != "posted" {
			t.Fatalf("posted voucher status = %q, want posted", got)
		}

		reversal := admin.mustPost("/finance/vouchers/"+voucherID+"/reverse", nil, http.StatusCreated)
		if got := text(reversal, "reversal_of_voucher_id"); got != voucherID {
			t.Fatalf("reversal_of_voucher_id = %q, want %s", got, voucherID)
		}
		original := admin.mustGet("/finance/vouchers/"+voucherID, http.StatusOK)
		if got := text(original, "status"); got != "reversed" {
			t.Fatalf("original status = %q, want reversed", got)
		}
	})

	t.Run("reports balance", func(t *testing.T) {
		trial := admin.mustGet("/finance/reports/trial-balance?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
		if got, want := number(t, trial, "total_debit"), number(t, trial, "total_credit"); !nearlyEqual(got, want) {
			t.Fatalf("trial balance debit = %v, credit = %v", got, want)
		}

		balance := admin.mustGet("/finance/reports/balance-sheet?as_of="+endDate, http.StatusOK)
		if got, want := number(t, balance, "total_assets"), number(t, balance, "total_liabilities_and_equity"); !nearlyEqual(got, want) {
			t.Fatalf("assets = %v, liabilities+equity = %v", got, want)
		}

		income := admin.mustGet("/finance/reports/income-statement?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
		expected := number(t, income, "total_revenue") - number(t, income, "total_expenses")
		if got := number(t, income, "net_profit"); !nearlyEqual(got, expected) {
			t.Fatalf("net profit = %v, want %v", got, expected)
		}

		ledger := admin.mustGet("/finance/reports/general-ledger?account_id="+arID+"&start_date="+startDate+"&end_date="+endDate, http.StatusOK)
		if text(ledger, "opening") == "" {
			t.Fatal("general ledger opening is empty")
		}
		if _, ok := ledger["entries"].([]any); !ok {
			t.Fatal("general ledger entries missing")
		}
	})

	t.Run("period close blocks posting", func(t *testing.T) {
		periodID := text(currentPeriod, "id")
		admin.mustPost("/finance/periods/"+periodID+"/close", nil, http.StatusOK)
		defer func() {
			admin.post("/finance/periods/"+periodID+"/reopen", nil)
		}()
		admin.expectError(http.MethodPost, "/finance/vouchers", map[string]any{
			"voucher_date": endDate,
			"memo":         "stage3c closed period",
			"lines": []map[string]any{
				{"account_id": cashID, "summary": "closed", "debit": "1.00", "credit": "0"},
				{"account_id": capitalID, "summary": "closed", "debit": "0", "credit": "1.00"},
			},
		}, http.StatusConflict, "PERIOD_CLOSED")
		admin.mustPost("/finance/periods/"+periodID+"/reopen", nil, http.StatusOK)
	})

	t.Run("data scope and permissions", func(t *testing.T) {
		demoPassword := env("TEST_DEMO_PASSWORD", "Demo@123456")
		dept := newClient(t).login("demo_dept", demoPassword)
		deptVouchers := dept.mustGet("/finance/vouchers?page_size=100", http.StatusOK)
		if got, _ := deptVouchers["total"].(float64); got != 0 {
			t.Fatalf("demo_dept voucher total = %v, want 0", got)
		}
		deptTrial := dept.mustGet("/finance/reports/trial-balance?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
		if got := number(t, deptTrial, "total_debit"); got != 0 {
			t.Fatalf("demo_dept trial debit = %v, want 0", got)
		}
		dept.expectError(http.MethodPost, "/finance/vouchers", map[string]any{
			"voucher_date": endDate,
			"lines": []map[string]any{
				{"account_id": cashID, "debit": "1.00", "credit": "0"},
				{"account_id": capitalID, "debit": "0", "credit": "1.00"},
			},
		}, http.StatusForbidden, "FORBIDDEN")

		anonymous := &client{t: t, baseURL: admin.baseURL}
		status, payload := anonymous.get("/finance/accounts")
		if status != http.StatusUnauthorized || payload.Code != "UNAUTHORIZED" {
			t.Fatalf("anonymous accounts status = %d code = %q, want 401 UNAUTHORIZED", status, payload.Code)
		}
	})
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
