//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestStage3F(t *testing.T) {
	admin := newClient(t).login(
		env("TEST_ADMIN_USERNAME", "admin"),
		env("TEST_ADMIN_PASSWORD", "Admin@123456"),
	)

	now := time.Now()
	today := now.Format("2006-01-02")
	currentStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")

	periods := getArray(admin, "/finance/periods?year="+strconv.Itoa(now.Year()), http.StatusOK)
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
	zeroTax := findItem(listItems(admin, "/masterdata/tax-rates?keyword=VAT0&page_size=20", http.StatusOK), "code", "VAT0")
	vat13 := findItem(listItems(admin, "/masterdata/tax-rates?keyword=VAT13&page_size=20", http.StatusOK), "code", "VAT13")
	if customer == nil || supplier == nil || warehouse == nil || material == nil || zeroTax == nil || vat13 == nil {
		t.Fatal("demo master data missing")
	}
	customerID := text(customer, "id")
	supplierID := text(supplier, "id")
	warehouseID := text(warehouse, "id")
	materialID := text(material, "id")
	zeroTaxID := text(zeroTax, "id")
	vat13ID := text(vat13, "id")

	t.Run("exchange rate API", func(t *testing.T) {
		rateDate := "2087-12-31"
		rate := admin.mustPost("/finance/exchange-rates", map[string]any{
			"currency_code": "USD",
			"rate_date":     rateDate,
			"rate":          "7.12345678",
		}, http.StatusCreated)
		if got := text(rate, "rate"); got != "7.12345678" {
			t.Fatalf("rate = %s, want 7.12345678", got)
		}
		list := listItems(admin, "/finance/exchange-rates?currency=USD&page_size=20", http.StatusOK)
		if findItem(list, "rate_date", rateDate) == nil {
			t.Fatal("exchange rate not found in list")
		}
		latest := admin.mustGet("/finance/exchange-rates/latest?currency=USD&date=2088-01-01", http.StatusOK)
		if got := text(latest, "rate"); got != "7.12345678" {
			t.Fatalf("latest rate = %s, want 7.12345678", got)
		}
		admin.expectError(http.MethodPost, "/finance/exchange-rates", map[string]any{
			"currency_code": "CNY", "rate_date": rateDate, "rate": "1",
		}, http.StatusBadRequest, "BAD_REQUEST")
		unauthorized := newClient(t)
		unauthorized.expectError(http.MethodGet, "/finance/exchange-rates", nil, http.StatusUnauthorized, "UNAUTHORIZED")
		dept := newClient(t).login("demo_dept", env("TEST_DEMO_PASSWORD", "Demo@123456"))
		dept.expectError(http.MethodGet, "/finance/exchange-rates", nil, http.StatusForbidden, "FORBIDDEN")
	})

	t.Run("automatic rate lookup and tax base amounts", func(t *testing.T) {
		admin.mustPost("/finance/exchange-rates", map[string]any{
			"currency_code": "USD", "rate_date": "2089-06-01", "rate": "7.5",
		}, http.StatusCreated)
		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":        customerID,
			"warehouse_id":       warehouseID,
			"order_date":         "2089-06-15",
			"currency":           "USD",
			"price_includes_tax": true,
			"remark":             "stage3f-auto-rate",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "113.00", "tax_rate_id": vat13ID},
			},
		}, http.StatusCreated)
		if got := text(order, "exchange_rate"); got != "7.50000000" {
			t.Fatalf("auto exchange rate = %s, want 7.50000000", got)
		}
		if got := text(order, "net_amount"); got != "100.0000" {
			t.Fatalf("order net = %s, want 100.0000", got)
		}
		if got := text(order, "tax_amount"); got != "13.0000" {
			t.Fatalf("order tax = %s, want 13.0000", got)
		}
		if got := text(order, "base_net_amount"); got != "750.0000" {
			t.Fatalf("order base net = %s, want 750.0000", got)
		}
		if got := text(order, "base_tax_amount"); got != "97.5000" {
			t.Fatalf("order base tax = %s, want 97.5000", got)
		}
		if got := text(order, "base_total_amount"); got != "847.5000" {
			t.Fatalf("order base total = %s, want 847.5000", got)
		}
		lines, _ := order["lines"].([]any)
		if len(lines) != 1 {
			t.Fatalf("order lines = %d, want 1", len(lines))
		}
		line := lines[0].(map[string]any)
		if text(line, "base_amount") != "750.0000" || text(line, "base_tax_amount") != "97.5000" || text(line, "base_gross_amount") != "847.5000" {
			t.Fatalf("line base amounts = %s/%s/%s", text(line, "base_amount"), text(line, "base_tax_amount"), text(line, "base_gross_amount"))
		}

		cny := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":        customerID,
			"warehouse_id":       warehouseID,
			"order_date":         "2079-12-01",
			"currency":           "CNY",
			"price_includes_tax": true,
			"remark":             "stage3f-cny-unchanged",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "100.00", "tax_rate_id": zeroTaxID},
			},
		}, http.StatusCreated)
		if got := text(cny, "exchange_rate"); got != "1.00000000" {
			t.Fatalf("CNY exchange rate = %s, want 1.00000000", got)
		}
		if got := text(cny, "base_total_amount"); got != "100.0000" {
			t.Fatalf("CNY base total = %s, want 100.0000", got)
		}

		admin.expectError(http.MethodPost, "/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   "2079-12-01",
			"currency":     "USD",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "100.00", "tax_rate_id": zeroTaxID},
			},
		}, http.StatusBadRequest, "EXCHANGE_RATE_NOT_FOUND")
		admin.expectError(http.MethodPost, "/sales/orders", map[string]any{
			"customer_id":   customerID,
			"warehouse_id":  warehouseID,
			"order_date":    "2089-06-15",
			"currency":      "USD",
			"exchange_rate": "-1",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "100.00", "tax_rate_id": zeroTaxID},
			},
		}, http.StatusBadRequest, "INVALID_EXCHANGE_RATE")
	})

	t.Run("realized FX loss on receivable allocation", func(t *testing.T) {
		invoiceID := createStage3FSalesInvoice(t, admin, customerID, warehouseID, materialID, zeroTaxID, today, "7.2", "stage3f-fx-loss")
		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type":  "receipt",
			"partner_id":    customerID,
			"payment_date":  today,
			"currency":      "USD",
			"exchange_rate": "7.0",
			"amount":        "100.00",
			"method":        "bank",
			"reference":     "stage3f-fx-loss-receipt",
		}, http.StatusCreated)
		admin.mustPost("/finance/payments/"+text(payment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{{"invoice_id": invoiceID, "amount": "100.00"}},
		}, http.StatusOK)
		paidInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(paidInvoice, "status"); got != "paid" {
			t.Fatalf("invoice status = %s, want paid", got)
		}
		settlements := listItems(admin, "/finance/vouchers?voucher_type=fx_settlement&start_date="+currentStart+"&end_date="+today+"&page_size=100", http.StatusOK)
		settlement := findItem(settlements, "source_code", text(payment, "code"))
		if settlement == nil {
			t.Fatal("FX loss settlement voucher not found")
		}
		detail := admin.mustGet("/finance/vouchers/"+text(settlement, "id"), http.StatusOK)
		lines, _ := detail["lines"].([]any)
		if got := number(t, findLineByAccount(t, lines, "660301"), "debit"); !nearlyEqual(got, 20) {
			t.Fatalf("FX loss debit = %v, want 20", got)
		}
		if got := number(t, findLineByAccount(t, lines, "1122"), "credit"); !nearlyEqual(got, 20) {
			t.Fatalf("FX loss AR credit = %v, want 20", got)
		}
	})

	t.Run("realized FX loss on payable allocation", func(t *testing.T) {
		payableID := createStage3FPayable(t, admin, supplierID, warehouseID, materialID, zeroTaxID, today, "7.2", "stage3f-ap-fx-loss")
		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type":  "payment",
			"partner_id":    supplierID,
			"payment_date":  today,
			"currency":      "USD",
			"exchange_rate": "7.4",
			"amount":        "100.00",
			"method":        "bank",
			"reference":     "stage3f-ap-fx-loss-payment",
		}, http.StatusCreated)
		admin.mustPost("/finance/payments/"+text(payment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{{"invoice_id": payableID, "amount": "100.00"}},
		}, http.StatusOK)
		paidInvoice := admin.mustGet("/finance/invoices/"+payableID, http.StatusOK)
		if got := text(paidInvoice, "status"); got != "paid" {
			t.Fatalf("payable status = %s, want paid", got)
		}
		settlements := listItems(admin, "/finance/vouchers?voucher_type=fx_settlement&start_date="+currentStart+"&end_date="+today+"&page_size=100", http.StatusOK)
		settlement := findItem(settlements, "source_code", text(payment, "code"))
		if settlement == nil {
			t.Fatal("AP FX loss settlement voucher not found")
		}
		detail := admin.mustGet("/finance/vouchers/"+text(settlement, "id"), http.StatusOK)
		lines, _ := detail["lines"].([]any)
		if got := number(t, findLineByAccount(t, lines, "660301"), "debit"); !nearlyEqual(got, 20) {
			t.Fatalf("AP FX loss debit = %v, want 20", got)
		}
		if got := number(t, findLineByAccount(t, lines, "2202"), "credit"); !nearlyEqual(got, 20) {
			t.Fatalf("AP FX loss credit = %v, want 20", got)
		}
	})

	t.Run("realized FX gain on receivable allocation", func(t *testing.T) {
		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":        customerID,
			"warehouse_id":       warehouseID,
			"order_date":         today,
			"currency":           "USD",
			"exchange_rate":      "7.2",
			"price_includes_tax": true,
			"remark":             "stage3f-fx-payment",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "100.00", "tax_rate_id": zeroTaxID},
			},
		}, http.StatusCreated)
		if got := text(order, "base_total_amount"); got != "720.0000" {
			t.Fatalf("order base total = %s, want 720.0000", got)
		}
		orderID := text(order, "id")
		admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
			"shipment_date": today,
			"exchange_rate": "7.2",
			"remark":        "stage3f-fx-payment-ship",
			"lines": []map[string]any{
				{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusCreated)
		shipmentInvoiceID := text(shipment, "invoice_id")
		if got := text(shipment, "base_total_amount"); got != "720.0000" {
			t.Fatalf("shipment base total = %s, want 720.0000", got)
		}
		invoice := admin.mustGet("/finance/invoices/"+shipmentInvoiceID, http.StatusOK)
		if got := text(invoice, "base_total_amount"); got != "720.0000" {
			t.Fatalf("invoice base total = %s, want 720.0000", got)
		}

		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type":  "receipt",
			"partner_id":    customerID,
			"payment_date":  today,
			"currency":      "USD",
			"exchange_rate": "7.4",
			"amount":        "100.00",
			"method":        "bank",
			"reference":     "stage3f-fx-payment-receipt",
		}, http.StatusCreated)
		if got := text(payment, "base_amount"); got != "740.0000" {
			t.Fatalf("payment base amount = %s, want 740.0000", got)
		}
		paymentID := text(payment, "id")
		admin.mustPost("/finance/payments/"+paymentID+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": shipmentInvoiceID, "amount": "100.00"},
			},
		}, http.StatusOK)

		paidInvoice := admin.mustGet("/finance/invoices/"+shipmentInvoiceID, http.StatusOK)
		if got := text(paidInvoice, "status"); got != "paid" {
			t.Fatalf("invoice status = %s, want paid", got)
		}
		if got := text(paidInvoice, "base_paid_amount"); got != "720.0000" {
			t.Fatalf("invoice base paid = %s, want 720.0000", got)
		}

		settlements := listItems(admin, "/finance/vouchers?voucher_type=fx_settlement&start_date="+currentStart+"&end_date="+today+"&page_size=100", http.StatusOK)
		settlement := findItem(settlements, "source_code", text(payment, "code"))
		if settlement == nil {
			t.Fatal("FX settlement voucher not found")
		}
		settlementDetail := admin.mustGet("/finance/vouchers/"+text(settlement, "id"), http.StatusOK)
		settlementLines, _ := settlementDetail["lines"].([]any)
		if got := number(t, findLineByAccount(t, settlementLines, "1122"), "debit"); !nearlyEqual(got, 20) {
			t.Fatalf("FX settlement AR debit = %v, want 20", got)
		}
		if got := number(t, findLineByAccount(t, settlementLines, "660302"), "credit"); !nearlyEqual(got, 20) {
			t.Fatalf("FX settlement gain credit = %v, want 20", got)
		}
	})

	t.Run("period end revaluation and reopen", func(t *testing.T) {
		year := cleanRevaluationYear(t, admin)
		ensureTestYear(t, admin, year)
		start := fmt.Sprintf("%04d-01-15", year)
		end := fmt.Sprintf("%04d-01-31", year)
		next := fmt.Sprintf("%04d-02-01", year)
		for date, rate := range map[string]string{
			start: "7.2",
			end:   "7.3",
			next:  "7.4",
		} {
			admin.mustPost("/finance/exchange-rates", map[string]any{
				"currency_code": "USD", "rate_date": date, "rate": rate,
			}, http.StatusCreated)
		}

		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":        customerID,
			"warehouse_id":       warehouseID,
			"order_date":         start,
			"currency":           "USD",
			"exchange_rate":      "7.2",
			"price_includes_tax": true,
			"remark":             "stage3f-revaluation",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "100.00", "tax_rate_id": zeroTaxID},
			},
		}, http.StatusCreated)
		orderID := text(order, "id")
		admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
			"shipment_date": start,
			"exchange_rate": "7.2",
			"remark":        "stage3f-revaluation-ship",
			"lines": []map[string]any{
				{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusCreated)
		invoiceID := text(shipment, "invoice_id")

		yearPeriods := getArray(admin, periodsPath(year), http.StatusOK)
		var january map[string]any
		for _, period := range yearPeriods {
			if month, ok := period["month"].(float64); ok && int(month) == 1 {
				january = period
			}
		}
		if january == nil {
			t.Fatal("January period not found")
		}
		if text(january, "status") == "closed" {
			admin.mustPost("/finance/periods/"+text(january, "id")+"/reopen", nil, http.StatusOK)
		}
		admin.mustPost("/finance/periods/"+text(january, "id")+"/close", nil, http.StatusOK)

		revaluations := listItems(admin, "/finance/vouchers?voucher_type=fx_revaluation&start_date="+start+"&end_date="+end+"&page_size=50", http.StatusOK)
		revaluation := findItem(revaluations, "source_code", fmt.Sprintf("%04d-01", year))
		if revaluation == nil {
			t.Fatal("FX revaluation voucher not found")
		}
		revaluationDetail := admin.mustGet("/finance/vouchers/"+text(revaluation, "id"), http.StatusOK)
		revaluationLines, _ := revaluationDetail["lines"].([]any)
		if got := number(t, findLineByAccount(t, revaluationLines, "1122"), "debit"); !nearlyEqual(got, 10) {
			t.Fatalf("revaluation AR debit = %v, want 10", got)
		}
		if got := number(t, findLineByAccount(t, revaluationLines, "660302"), "credit"); !nearlyEqual(got, 10) {
			t.Fatalf("revaluation gain credit = %v, want 10", got)
		}
		revaluedInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(revaluedInvoice, "base_revalued_amount"); got != "10.0000" {
			t.Fatalf("invoice base revalued = %s, want 10.0000", got)
		}

		reversals := listItems(admin, "/finance/vouchers?voucher_type=fx_revaluation_reversal&start_date="+next+"&end_date="+next+"&page_size=50", http.StatusOK)
		reversal := findItem(reversals, "source_code", fmt.Sprintf("%04d-01", year))
		if reversal == nil {
			t.Fatal("FX revaluation reversal voucher not found")
		}
		reversalDetail := admin.mustGet("/finance/vouchers/"+text(reversal, "id"), http.StatusOK)
		reversalLines, _ := reversalDetail["lines"].([]any)
		if got := number(t, findLineByAccount(t, reversalLines, "1122"), "credit"); !nearlyEqual(got, 10) {
			t.Fatalf("revaluation reversal AR credit = %v, want 10", got)
		}
		if got := number(t, findLineByAccount(t, reversalLines, "660302"), "debit"); !nearlyEqual(got, 10) {
			t.Fatalf("revaluation reversal gain debit = %v, want 10", got)
		}

		admin.mustPost("/finance/periods/"+text(january, "id")+"/reopen", nil, http.StatusOK)
		reopenVouchers := listItems(admin, "/finance/vouchers?voucher_type=fx_revaluation_reopen&start_date="+start+"&end_date="+end+"&page_size=50", http.StatusOK)
		reopen := findItem(reopenVouchers, "source_code", fmt.Sprintf("%04d-01", year))
		if reopen == nil {
			t.Fatal("FX revaluation reopen voucher not found")
		}
		reopenDetail := admin.mustGet("/finance/vouchers/"+text(reopen, "id"), http.StatusOK)
		reopenLines, _ := reopenDetail["lines"].([]any)
		if got := number(t, findLineByAccount(t, reopenLines, "1122"), "credit"); !nearlyEqual(got, 10) {
			t.Fatalf("reopen AR credit = %v, want 10", got)
		}
		if got := number(t, findLineByAccount(t, reopenLines, "660302"), "debit"); !nearlyEqual(got, 10) {
			t.Fatalf("reopen gain debit = %v, want 10", got)
		}
		reversalReopen := listItems(admin, "/finance/vouchers?voucher_type=fx_revaluation_reversal_reopen&start_date="+next+"&end_date="+next+"&page_size=50", http.StatusOK)
		reversalReopenVoucher := findItem(reversalReopen, "source_code", fmt.Sprintf("%04d-01", year))
		if reversalReopenVoucher == nil {
			t.Fatal("FX revaluation reversal reopen voucher not found")
		}
		reversalReopenDetail := admin.mustGet("/finance/vouchers/"+text(reversalReopenVoucher, "id"), http.StatusOK)
		reversalReopenLines, _ := reversalReopenDetail["lines"].([]any)
		if got := number(t, findLineByAccount(t, reversalReopenLines, "1122"), "debit"); !nearlyEqual(got, 10) {
			t.Fatalf("reversal reopen AR debit = %v, want 10", got)
		}
		if got := number(t, findLineByAccount(t, reversalReopenLines, "660302"), "credit"); !nearlyEqual(got, 10) {
			t.Fatalf("reversal reopen gain credit = %v, want 10", got)
		}
		reopenedInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(reopenedInvoice, "base_revalued_amount"); got != "0.0000" {
			t.Fatalf("invoice base revalued after reopen = %s, want 0.0000", got)
		}
		reopened := getArray(admin, periodsPath(year), http.StatusOK)
		for _, period := range reopened {
			if month, ok := period["month"].(float64); ok && int(month) == 1 {
				if text(period, "status") != "open" {
					t.Fatalf("period status = %s, want open", text(period, "status"))
				}
			}
		}
	})

	t.Run("consecutive period revaluation", func(t *testing.T) {
		year := cleanRevaluationYear(t, admin)
		ensureTestYear(t, admin, year)
		janStart := fmt.Sprintf("%04d-01-15", year)
		janEnd := fmt.Sprintf("%04d-01-31", year)
		febEnd := time.Date(year, time.March, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		marEnd := fmt.Sprintf("%04d-03-31", year)
		for date, rate := range map[string]string{
			janStart: "7.2",
			janEnd:   "7.3",
			febEnd:   "7.4",
			marEnd:   "7.2",
		} {
			admin.mustPost("/finance/exchange-rates", map[string]any{
				"currency_code": "USD", "rate_date": date, "rate": rate,
			}, http.StatusCreated)
		}

		fxCustomer := admin.mustPost("/masterdata/partners", map[string]any{
			"code": fmt.Sprintf("CUS-FX-%d", time.Now().UnixNano()),
			"name": "FX Revaluation Customer",
			"type": "customer",
		}, http.StatusCreated)
		fxCustomerID := text(fxCustomer, "id")

		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":        fxCustomerID,
			"warehouse_id":       warehouseID,
			"order_date":         janStart,
			"currency":           "USD",
			"exchange_rate":      "7.2",
			"price_includes_tax": true,
			"remark":             "stage3f-consecutive-revaluation",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "100.00", "tax_rate_id": zeroTaxID},
			},
		}, http.StatusCreated)
		orderID := text(order, "id")
		admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
			"shipment_date": janStart,
			"exchange_rate": "7.2",
			"remark":        "stage3f-consecutive-revaluation-ship",
			"lines": []map[string]any{
				{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusCreated)
		invoiceID := text(shipment, "invoice_id")

		periodByMonth := map[int]map[string]any{}
		for _, period := range getArray(admin, periodsPath(year), http.StatusOK) {
			if month, ok := period["month"].(float64); ok {
				periodByMonth[int(month)] = period
			}
		}
		closeMonth := func(month int) {
			period := periodByMonth[month]
			if period == nil {
				t.Fatalf("period %04d-%02d not found", year, month)
			}
			if text(period, "status") == "closed" {
				admin.mustPost("/finance/periods/"+text(period, "id")+"/reopen", nil, http.StatusOK)
			}
			admin.mustPost("/finance/periods/"+text(period, "id")+"/close", nil, http.StatusOK)
		}

		closeMonth(1)
		closeMonth(2)
		febRevaluations := listItems(admin, "/finance/vouchers?voucher_type=fx_revaluation&start_date="+febEnd+"&end_date="+febEnd+"&page_size=50", http.StatusOK)
		febRevaluation := findItem(febRevaluations, "source_code", fmt.Sprintf("%04d-02", year))
		if febRevaluation == nil {
			t.Fatal("February FX revaluation voucher not found")
		}
		febDetail := admin.mustGet("/finance/vouchers/"+text(febRevaluation, "id"), http.StatusOK)
		febLines, _ := febDetail["lines"].([]any)
		if got := number(t, findLineByAccountPartner(t, febLines, "1122", fxCustomerID), "debit"); !nearlyEqual(got, 20) {
			t.Fatalf("February revaluation AR debit = %v, want 20", got)
		}
		if got := number(t, findLineByAccountPartner(t, febLines, "660302", fxCustomerID), "credit"); !nearlyEqual(got, 20) {
			t.Fatalf("February revaluation gain credit = %v, want 20", got)
		}
		afterFeb := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(afterFeb, "base_revalued_amount"); got != "20.0000" {
			t.Fatalf("February base revalued = %s, want 20.0000", got)
		}

		closeMonth(3)
		afterMar := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(afterMar, "base_revalued_amount"); got != "0.0000" {
			t.Fatalf("March base revalued = %s, want 0.0000", got)
		}
		marStart := fmt.Sprintf("%04d-03-01", year)
		marReversals := listItems(admin, "/finance/vouchers?voucher_type=fx_revaluation_reversal&start_date="+marStart+"&end_date="+marEnd+"&page_size=50", http.StatusOK)
		marReversal := findItem(marReversals, "source_code", fmt.Sprintf("%04d-02", year))
		if marReversal == nil {
			t.Fatal("March reversal of February revaluation not found")
		}
		marReversalDetail := admin.mustGet("/finance/vouchers/"+text(marReversal, "id"), http.StatusOK)
		marReversalLines, _ := marReversalDetail["lines"].([]any)
		if got := number(t, findLineByAccountPartner(t, marReversalLines, "1122", fxCustomerID), "credit"); !nearlyEqual(got, 20) {
			t.Fatalf("March reversal AR credit = %v, want 20", got)
		}
		marRevaluations := listItems(admin, "/finance/vouchers?voucher_type=fx_revaluation&start_date="+marStart+"&end_date="+marEnd+"&page_size=50", http.StatusOK)
		for _, periodVoucher := range marRevaluations {
			detail := admin.mustGet("/finance/vouchers/"+text(periodVoucher, "id"), http.StatusOK)
			lines, _ := detail["lines"].([]any)
			if lookupLineByAccountPartner(lines, "1122", fxCustomerID) != nil || lookupLineByAccountPartner(lines, "660302", fxCustomerID) != nil {
				t.Fatal("March zero-adjustment revaluation line should not be posted")
			}
		}
	})
}

func cleanRevaluationYear(t *testing.T, admin *client) int {
	t.Helper()
	for year := 2080; year <= 2500; year++ {
		periods := getArray(admin, periodsPath(year), http.StatusOK)
		if len(periods) == 0 {
			return year
		}
	}
	t.Fatal("no clean year left for FX revaluation test")
	return 0
}

func createStage3FSalesInvoice(
	t *testing.T,
	admin *client,
	customerID, warehouseID, materialID, taxRateID, date, rate, remark string,
) string {
	t.Helper()
	order := admin.mustPost("/sales/orders", map[string]any{
		"customer_id":        customerID,
		"warehouse_id":       warehouseID,
		"order_date":         date,
		"currency":           "USD",
		"exchange_rate":      rate,
		"price_includes_tax": true,
		"remark":             remark,
		"lines": []map[string]any{
			{"material_id": materialID, "quantity": "1", "unit_price": "100.00", "tax_rate_id": taxRateID},
		},
	}, http.StatusCreated)
	orderID := text(order, "id")
	admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
	admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
	detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
	lines, _ := detail["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("sales order lines = %d, want 1", len(lines))
	}
	shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
		"shipment_date": date,
		"exchange_rate": rate,
		"remark":        remark + "-ship",
		"lines": []map[string]any{
			{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "1"},
		},
	}, http.StatusCreated)
	return text(shipment, "invoice_id")
}

func createStage3FPayable(
	t *testing.T,
	admin *client,
	supplierID, warehouseID, materialID, taxRateID, date, rate, remark string,
) string {
	t.Helper()
	order := admin.mustPost("/purchase/orders", map[string]any{
		"supplier_id":        supplierID,
		"warehouse_id":       warehouseID,
		"order_date":         date,
		"currency":           "USD",
		"exchange_rate":      rate,
		"price_includes_tax": false,
		"remark":             remark,
		"lines": []map[string]any{
			{"material_id": materialID, "quantity": "1", "unit_price": "100.00", "tax_rate_id": taxRateID},
		},
	}, http.StatusCreated)
	orderID := text(order, "id")
	admin.mustPost("/purchase/orders/"+orderID+"/submit", nil, http.StatusOK)
	admin.mustPost("/purchase/orders/"+orderID+"/approve", nil, http.StatusOK)
	detail := admin.mustGet("/purchase/orders/"+orderID, http.StatusOK)
	lines, _ := detail["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("purchase order lines = %d, want 1", len(lines))
	}
	receipt := admin.mustPost("/purchase/orders/"+orderID+"/receive", map[string]any{
		"receipt_date":  date,
		"exchange_rate": rate,
		"remark":        remark + "-receive",
		"lines": []map[string]any{
			{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "1"},
		},
	}, http.StatusCreated)
	return text(receipt, "payable_id")
}

func lookupLineByAccountPartner(lines []any, accountCode, partnerID string) map[string]any {
	for _, raw := range lines {
		line, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if text(line, "account_code") == accountCode && text(line, "partner_id") == partnerID {
			return line
		}
	}
	return nil
}

func findLineByAccountPartner(t *testing.T, lines []any, accountCode, partnerID string) map[string]any {
	t.Helper()
	line := lookupLineByAccountPartner(lines, accountCode, partnerID)
	if line == nil {
		t.Fatalf("voucher line account=%s partner=%s not found", accountCode, partnerID)
	}
	return line
}
