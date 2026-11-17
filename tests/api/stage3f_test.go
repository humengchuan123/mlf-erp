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
	warehouse := findItem(listItems(admin, "/masterdata/warehouses?keyword=WH-SH&page_size=20", http.StatusOK), "code", "WH-SH")
	material := findItem(listItems(admin, "/masterdata/materials?keyword=MAT-DEMO-004&page_size=20", http.StatusOK), "code", "MAT-DEMO-004")
	zeroTax := findItem(listItems(admin, "/masterdata/tax-rates?keyword=VAT0&page_size=20", http.StatusOK), "code", "VAT0")
	if customer == nil || warehouse == nil || material == nil || zeroTax == nil {
		t.Fatal("demo master data missing")
	}
	customerID := text(customer, "id")
	warehouseID := text(warehouse, "id")
	materialID := text(material, "id")
	zeroTaxID := text(zeroTax, "id")

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
		admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
			"shipment_date": start,
			"exchange_rate": "7.2",
			"remark":        "stage3f-revaluation-ship",
			"lines": []map[string]any{
				{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusCreated)

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
		reversals := listItems(admin, "/finance/vouchers?voucher_type=fx_revaluation_reversal&start_date="+next+"&end_date="+next+"&page_size=50", http.StatusOK)
		if findItem(reversals, "source_code", fmt.Sprintf("%04d-01", year)) == nil {
			t.Fatal("FX revaluation reversal voucher not found")
		}

		admin.mustPost("/finance/periods/"+text(january, "id")+"/reopen", nil, http.StatusOK)
		reopenVouchers := listItems(admin, "/finance/vouchers?voucher_type=fx_revaluation_reopen&start_date="+start+"&end_date="+end+"&page_size=50", http.StatusOK)
		if findItem(reopenVouchers, "source_code", fmt.Sprintf("%04d-01", year)) == nil {
			t.Fatal("FX revaluation reopen voucher not found")
		}
		reversalReopen := listItems(admin, "/finance/vouchers?voucher_type=fx_revaluation_reversal_reopen&start_date="+next+"&end_date="+next+"&page_size=50", http.StatusOK)
		if findItem(reversalReopen, "source_code", fmt.Sprintf("%04d-01", year)) == nil {
			t.Fatal("FX revaluation reversal reopen voucher not found")
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
}

func cleanRevaluationYear(t *testing.T, admin *client) int {
	t.Helper()
	for year := 2080; year <= 2099; year++ {
		periods := getArray(admin, periodsPath(year), http.StatusOK)
		if len(periods) == 0 {
			return year
		}
	}
	t.Skip("no clean year left for FX revaluation test")
	return 0
}
