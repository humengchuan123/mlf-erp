//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestStage3E(t *testing.T) {
	admin := newClient(t).login(
		env("TEST_ADMIN_USERNAME", "admin"),
		env("TEST_ADMIN_PASSWORD", "Admin@123456"),
	)

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, -1)
	startDate := monthStart.Format("2006-01-02")
	endDate := monthEnd.Format("2006-01-02")
	today := now.Format("2006-01-02")

	periods := getArray(admin, "/finance/periods?year="+fmt.Sprint(now.Year()), http.StatusOK)
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

	taxRates := listItems(admin, "/masterdata/tax-rates?page_size=100", http.StatusOK)
	vat13 := findItem(taxRates, "code", "VAT13")
	vat9 := findItem(taxRates, "code", "VAT9")
	vat0 := findItem(taxRates, "code", "VAT0")
	if vat13 == nil || vat9 == nil || vat0 == nil {
		t.Fatal("VAT13, VAT9 or VAT0 tax rate missing")
	}
	vat13ID := text(vat13, "id")
	vat9ID := text(vat9, "id")
	vat0ID := text(vat0, "id")

	customer := findItem(listItems(admin, "/masterdata/partners?keyword=CUS-DEMO-001&page_size=20", http.StatusOK), "code", "CUS-DEMO-001")
	supplier := findItem(listItems(admin, "/masterdata/partners?keyword=SUP-DEMO-001&page_size=20", http.StatusOK), "code", "SUP-DEMO-001")
	warehouse := findItem(listItems(admin, "/masterdata/warehouses?keyword=WH-SH&page_size=20", http.StatusOK), "code", "WH-SH")
	material := findItem(listItems(admin, "/masterdata/materials?keyword=MAT-DEMO-004&page_size=20", http.StatusOK), "code", "MAT-DEMO-004")
	if customer == nil || supplier == nil || warehouse == nil || material == nil {
		t.Fatal("demo master data missing")
	}
	customerID := text(customer, "id")
	supplierID := text(supplier, "id")
	warehouseID := text(warehouse, "id")
	materialID := text(material, "id")

	var salesInvoiceID string
	var salesTax float64
	t.Run("mixed tax rates on one order", func(t *testing.T) {
		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":        customerID,
			"warehouse_id":       warehouseID,
			"order_date":         endDate,
			"currency":           "CNY",
			"price_includes_tax": true,
			"remark":             "stage3e-mixed-tax",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "113.00", "tax_rate_id": vat13ID},
				{"material_id": materialID, "quantity": "1", "unit_price": "50.00", "tax_rate_id": vat0ID},
			},
		}, http.StatusCreated)
		if got := text(order, "net_amount"); got != "150.0000" {
			t.Fatalf("mixed net_amount = %s, want 150.0000", got)
		}
		if got := text(order, "tax_amount"); got != "13.0000" {
			t.Fatalf("mixed tax_amount = %s, want 13.0000", got)
		}
		if got := text(order, "total_amount"); got != "163.0000" {
			t.Fatalf("mixed total_amount = %s, want 163.0000", got)
		}
	})
	t.Run("sales tax inclusive and output VAT voucher", func(t *testing.T) {
		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":        customerID,
			"warehouse_id":       warehouseID,
			"order_date":         endDate,
			"currency":           "CNY",
			"price_includes_tax": true,
			"remark":             "stage3e-sales-tax",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "113.00", "tax_rate_id": vat13ID},
			},
		}, http.StatusCreated)
		if got := text(order, "net_amount"); got != "100.0000" {
			t.Fatalf("sales net_amount = %s, want 100.0000", got)
		}
		if got := text(order, "tax_amount"); got != "13.0000" {
			t.Fatalf("sales tax_amount = %s, want 13.0000", got)
		}
		if got := text(order, "total_amount"); got != "113.0000" {
			t.Fatalf("sales total_amount = %s, want 113.0000", got)
		}

		orderID := text(order, "id")
		admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		line := lines[0].(map[string]any)

		shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
			"shipment_date": today,
			"remark":        "stage3e-sales-tax-ship",
			"lines": []map[string]any{
				{"order_line_id": text(line, "id"), "quantity": "1"},
			},
		}, http.StatusCreated)
		if got := text(shipment, "net_amount"); got != "100.0000" {
			t.Fatalf("shipment net_amount = %s, want 100.0000", got)
		}
		if got := text(shipment, "tax_amount"); got != "13.0000" {
			t.Fatalf("shipment tax_amount = %s, want 13.0000", got)
		}
		if got := text(shipment, "total_amount"); got != "113.0000" {
			t.Fatalf("shipment total_amount = %s, want 113.0000", got)
		}
		salesInvoiceID = text(shipment, "invoice_id")
		salesTax = number(t, shipment, "tax_amount")

		invoice := admin.mustGet("/finance/invoices/"+salesInvoiceID, http.StatusOK)
		if got := text(invoice, "net_amount"); got != "100.0000" {
			t.Fatalf("invoice net_amount = %s, want 100.0000", got)
		}
		if got := text(invoice, "tax_amount"); got != "13.0000" {
			t.Fatalf("invoice tax_amount = %s, want 13.0000", got)
		}
		if got := text(invoice, "vat_invoice_status"); got != "pending" {
			t.Fatalf("invoice vat status = %s, want pending", got)
		}

		voucher := findVoucherBySource(admin, text(shipment, "code"))
		if voucher == nil {
			t.Fatal("sales tax voucher not found")
		}
		voucherDetail := admin.mustGet("/finance/vouchers/"+text(voucher, "id"), http.StatusOK)
		voucherLines, _ := voucherDetail["lines"].([]any)
		if got := number(t, findLineByAccount(t, voucherLines, "1122"), "debit"); !nearlyEqual(got, 113) {
			t.Fatalf("AR debit = %v, want 113", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "6001"), "credit"); !nearlyEqual(got, 100) {
			t.Fatalf("revenue credit = %v, want 100", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "222102"), "credit"); !nearlyEqual(got, 13) {
			t.Fatalf("output VAT credit = %v, want 13", got)
		}
	})

	var purchaseInvoiceID string
	var purchaseTax float64
	t.Run("purchase tax exclusive and input VAT voucher", func(t *testing.T) {
		order := admin.mustPost("/purchase/orders", map[string]any{
			"supplier_id":        supplierID,
			"warehouse_id":       warehouseID,
			"order_date":         endDate,
			"currency":           "CNY",
			"price_includes_tax": false,
			"remark":             "stage3e-purchase-tax",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "2", "unit_price": "100.00", "tax_rate_id": vat9ID},
			},
		}, http.StatusCreated)
		if got := text(order, "net_amount"); got != "200.0000" {
			t.Fatalf("purchase net_amount = %s, want 200.0000", got)
		}
		if got := text(order, "tax_amount"); got != "18.0000" {
			t.Fatalf("purchase tax_amount = %s, want 18.0000", got)
		}
		if got := text(order, "total_amount"); got != "218.0000" {
			t.Fatalf("purchase total_amount = %s, want 218.0000", got)
		}

		orderID := text(order, "id")
		admin.mustPost("/purchase/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/purchase/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/purchase/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		line := lines[0].(map[string]any)

		receipt := admin.mustPost("/purchase/orders/"+orderID+"/receive", map[string]any{
			"receipt_date": today,
			"remark":       "stage3e-purchase-tax-receive",
			"lines": []map[string]any{
				{"order_line_id": text(line, "id"), "quantity": "2"},
			},
		}, http.StatusCreated)
		if got := text(receipt, "net_amount"); got != "200.0000" {
			t.Fatalf("receipt net_amount = %s, want 200.0000", got)
		}
		if got := text(receipt, "tax_amount"); got != "18.0000" {
			t.Fatalf("receipt tax_amount = %s, want 18.0000", got)
		}
		purchaseInvoiceID = text(receipt, "payable_id")
		purchaseTax = number(t, receipt, "tax_amount")

		invoice := admin.mustGet("/finance/invoices/"+purchaseInvoiceID, http.StatusOK)
		if got := text(invoice, "total_amount"); got != "218.0000" {
			t.Fatalf("payable invoice total = %s, want 218.0000", got)
		}

		voucher := findVoucherBySource(admin, text(receipt, "code"))
		if voucher == nil {
			t.Fatal("purchase tax voucher not found")
		}
		voucherDetail := admin.mustGet("/finance/vouchers/"+text(voucher, "id"), http.StatusOK)
		voucherLines, _ := voucherDetail["lines"].([]any)
		if got := number(t, findLineByAccount(t, voucherLines, "1405"), "debit"); !nearlyEqual(got, 200) {
			t.Fatalf("inventory debit = %v, want 200", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "222101"), "debit"); !nearlyEqual(got, 18) {
			t.Fatalf("input VAT debit = %v, want 18", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "2202"), "credit"); !nearlyEqual(got, 218) {
			t.Fatalf("AP credit = %v, want 218", got)
		}
	})

	t.Run("invoice issue void duplicate and permissions", func(t *testing.T) {
		invoiceNo := fmt.Sprintf("VAT-%d", now.UnixNano())
		issued := admin.mustPost("/finance/invoices/"+salesInvoiceID+"/issue", map[string]any{
			"vat_invoice_no":   invoiceNo,
			"vat_invoice_type": "special",
			"vat_invoice_date": today,
			"buyer_tax_no":     "91310000BUYER",
			"seller_tax_no":    "91310000SELLER",
		}, http.StatusOK)
		if got := text(issued, "vat_invoice_status"); got != "issued" {
			t.Fatalf("issued status = %s, want issued", got)
		}
		admin.expectError(http.MethodPost, "/finance/invoices/"+salesInvoiceID+"/issue", map[string]any{
			"vat_invoice_no":   invoiceNo + "-2",
			"vat_invoice_type": "special",
			"vat_invoice_date": today,
		}, http.StatusConflict, "INVOICE_STATUS")
		admin.expectError(http.MethodPost, "/finance/invoices/"+purchaseInvoiceID+"/issue", map[string]any{
			"vat_invoice_no":   invoiceNo,
			"vat_invoice_type": "general",
			"vat_invoice_date": today,
		}, http.StatusConflict, "VAT_INVOICE_NO_DUPLICATE")

		voided := admin.mustPost("/finance/invoices/"+salesInvoiceID+"/void", nil, http.StatusOK)
		if got := text(voided, "vat_invoice_status"); got != "void" {
			t.Fatalf("voided status = %s, want void", got)
		}
		admin.expectError(http.MethodPost, "/finance/invoices/"+salesInvoiceID+"/void", nil, http.StatusConflict, "INVOICE_STATUS")
		admin.expectError(http.MethodPost, "/finance/invoices/"+salesInvoiceID+"/issue", map[string]any{
			"vat_invoice_no":   invoiceNo + "-3",
			"vat_invoice_type": "special",
			"vat_invoice_date": today,
		}, http.StatusConflict, "INVOICE_STATUS")

		demoPassword := env("TEST_DEMO_PASSWORD", "Demo@123456")
		dept := newClient(t).login("demo_dept", demoPassword)
		dept.expectError(http.MethodPost, "/finance/invoices/"+purchaseInvoiceID+"/issue", map[string]any{
			"vat_invoice_no":   invoiceNo + "-dept",
			"vat_invoice_type": "special",
			"vat_invoice_date": today,
		}, http.StatusForbidden, "FORBIDDEN")
		unauthorized := newClient(t)
		unauthorized.expectError(http.MethodGet, "/finance/reports/vat-summary?start_date="+startDate+"&end_date="+endDate, nil, http.StatusUnauthorized, "UNAUTHORIZED")
	})

	t.Run("VAT summary", func(t *testing.T) {
		report := admin.mustGet("/finance/reports/vat-summary?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
		rows, _ := report["rows"].([]any)
		if len(rows) == 0 {
			t.Fatal("VAT summary rows are empty")
		}
		totalOutput := number(t, report, "total_output_tax")
		totalInput := number(t, report, "total_input_tax")
		if totalOutput < salesTax {
			t.Fatalf("VAT output = %v, want >= %v", totalOutput, salesTax)
		}
		if totalInput < purchaseTax {
			t.Fatalf("VAT input = %v, want >= %v", totalInput, purchaseTax)
		}
		if got := number(t, report, "total_net_payable"); !nearlyEqual(got, totalOutput-totalInput) {
			t.Fatalf("VAT net payable = %v, want %v", got, totalOutput-totalInput)
		}
	})
}
