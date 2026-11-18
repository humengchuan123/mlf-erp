//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestStage3H(t *testing.T) {
	admin := newClient(t).login(
		env("TEST_ADMIN_USERNAME", "admin"),
		env("TEST_ADMIN_PASSWORD", "Admin@123456"),
	)

	now := time.Now()
	today := now.Format("2006-01-02")

	ensureTestYear(t, admin, now.Year())
	periods := getArray(admin, periodsPath(now.Year()), http.StatusOK)
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
	taxRates := listItems(admin, "/masterdata/tax-rates?page_size=100", http.StatusOK)
	vat13 := findItem(taxRates, "code", "VAT13")
	vat0 := findItem(taxRates, "code", "VAT0")
	if customer == nil || supplier == nil || warehouse == nil || material == nil || vat13 == nil || vat0 == nil {
		t.Fatal("demo master data missing")
	}
	customerID := text(customer, "id")
	supplierID := text(supplier, "id")
	warehouseID := text(warehouse, "id")
	materialID := text(material, "id")
	vat13ID := text(vat13, "id")
	vat0ID := text(vat0, "id")

	ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)

	var partialOrderID string
	var partialShipmentID string
	var partialShipmentLineID string
	var partialInvoiceID string
	var firstReturnID string
	var firstReturnCode string
	var secondReturnID string
	var secondReturnCode string

	t.Run("partial and full return with credit note inventory and voucher", func(t *testing.T) {
		orderID, shipmentID, shipmentLineID, invoiceID, lineCost := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat13ID,
			"2", "113.00", true, "stage3h-partial-return",
		)
		partialOrderID = orderID
		partialShipmentID = shipmentID
		partialShipmentLineID = shipmentLineID
		partialInvoiceID = invoiceID

		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "total_amount"); got != "226.0000" {
			t.Fatalf("invoice total = %s, want 226.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "226.0000" {
			t.Fatalf("invoice outstanding = %s, want 226.0000", got)
		}
		if got := text(invoice, "status"); got != "open" {
			t.Fatalf("invoice status = %s, want open", got)
		}

		baseline := inventoryBalance(t, admin, warehouseID, materialID)
		baselineOnHand := number(t, baseline, "on_hand")
		baselineAvgCost := number(t, baseline, "avg_cost")

		first := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3h-first-return")
		firstReturnID = text(first, "id")
		firstReturnCode = text(first, "code")
		if got := text(first, "net_amount"); got != "100.0000" {
			t.Fatalf("first return net = %s, want 100.0000", got)
		}
		if got := text(first, "tax_amount"); got != "13.0000" {
			t.Fatalf("first return tax = %s, want 13.0000", got)
		}
		if got := text(first, "total_amount"); got != "113.0000" {
			t.Fatalf("first return total = %s, want 113.0000", got)
		}
		if got := text(first, "base_net_amount"); got != "100.0000" {
			t.Fatalf("first return base net = %s, want 100.0000", got)
		}
		if got := text(first, "base_tax_amount"); got != "13.0000" {
			t.Fatalf("first return base tax = %s, want 13.0000", got)
		}
		if got := text(first, "base_total_amount"); got != "113.0000" {
			t.Fatalf("first return base total = %s, want 113.0000", got)
		}
		firstCost := number(t, first, "cost_amount")
		if !nearlyEqual(firstCost, lineCost/2) {
			t.Fatalf("first return cost = %v, want %v", firstCost, lineCost/2)
		}
		if got := text(first, "credit_note_amount"); got != "113.0000" {
			t.Fatalf("first credit note amount = %s, want 113.0000", got)
		}
		if got := text(first, "credit_note_applied_amount"); got != "113.0000" {
			t.Fatalf("first credit note applied = %s, want 113.0000", got)
		}
		if got := text(first, "credit_note_remaining_amount"); got != "0.0000" {
			t.Fatalf("first credit note remaining = %s, want 0.0000", got)
		}
		if got := text(first, "credit_note_status"); got != "applied" {
			t.Fatalf("first credit note status = %s, want applied", got)
		}

		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "credited_amount"); got != "113.0000" {
			t.Fatalf("invoice credited = %s, want 113.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "113.0000" {
			t.Fatalf("invoice outstanding = %s, want 113.0000", got)
		}
		if got := text(invoice, "status"); got != "partially_paid" {
			t.Fatalf("invoice status = %s, want partially_paid", got)
		}

		shipmentLine := shipmentLineByID(t, admin, partialOrderID, shipmentID, shipmentLineID)
		if got := number(t, shipmentLine, "returned_qty"); !nearlyEqual(got, 1) {
			t.Fatalf("shipment returned_qty = %v, want 1", got)
		}
		if got := number(t, shipmentLine, "returnable_qty"); !nearlyEqual(got, 1) {
			t.Fatalf("shipment returnable_qty = %v, want 1", got)
		}

		after := inventoryBalance(t, admin, warehouseID, materialID)
		if got := number(t, after, "on_hand"); !nearlyEqual(got, baselineOnHand+1) {
			t.Fatalf("on_hand after first return = %v, want %v", got, baselineOnHand+1)
		}
		expectedAvgCost := (baselineOnHand*baselineAvgCost + firstCost) / (baselineOnHand + 1)
		if got := number(t, after, "avg_cost"); !nearlyEqual(got, expectedAvgCost) {
			t.Fatalf("avg_cost after first return = %v, want %v", got, expectedAvgCost)
		}

		transactions := returnInventoryTransactions(t, admin, warehouseID, materialID, firstReturnID)
		if len(transactions) != 1 {
			t.Fatalf("return inventory transactions = %d, want 1", len(transactions))
		}
		transaction := transactions[0]
		if got := text(transaction, "direction"); got != "in" {
			t.Fatalf("return inventory direction = %s, want in", got)
		}
		if got := text(transaction, "biz_type"); got != "sales_return" {
			t.Fatalf("return inventory biz_type = %s, want sales_return", got)
		}
		if got := number(t, transaction, "quantity"); !nearlyEqual(got, 1) {
			t.Fatalf("return inventory quantity = %v, want 1", got)
		}
		if got := number(t, transaction, "amount"); !nearlyEqual(got, firstCost) {
			t.Fatalf("return inventory amount = %v, want %v", got, firstCost)
		}

		voucherLines := returnVoucherLines(t, admin, firstReturnID, firstReturnCode)
		assertVoucherBalanced(t, voucherLines)
		if got := number(t, findLineByAccount(t, voucherLines, "6001"), "debit"); !nearlyEqual(got, 100) {
			t.Fatalf("return revenue debit = %v, want 100", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "222102"), "debit"); !nearlyEqual(got, 13) {
			t.Fatalf("return output VAT debit = %v, want 13", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "1122"), "credit"); !nearlyEqual(got, 113) {
			t.Fatalf("return AR credit = %v, want 113", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "1405"), "debit"); !nearlyEqual(got, firstCost) {
			t.Fatalf("return inventory debit = %v, want %v", got, firstCost)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "6401"), "credit"); !nearlyEqual(got, firstCost) {
			t.Fatalf("return COGS credit = %v, want %v", got, firstCost)
		}

		second := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3h-second-return")
		secondReturnID = text(second, "id")
		secondReturnCode = text(second, "code")
		if got := text(second, "net_amount"); got != "100.0000" {
			t.Fatalf("second return net = %s, want 100.0000", got)
		}
		if got := text(second, "tax_amount"); got != "13.0000" {
			t.Fatalf("second return tax = %s, want 13.0000", got)
		}
		if got := text(second, "total_amount"); got != "113.0000" {
			t.Fatalf("second return total = %s, want 113.0000", got)
		}
		secondCost := number(t, second, "cost_amount")
		if !nearlyEqual(secondCost, lineCost-firstCost) {
			t.Fatalf("second return cost = %v, want %v", secondCost, lineCost-firstCost)
		}
		if got := text(second, "credit_note_amount"); got != "113.0000" {
			t.Fatalf("second credit note amount = %s, want 113.0000", got)
		}
		if got := text(second, "credit_note_applied_amount"); got != "113.0000" {
			t.Fatalf("second credit note applied = %s, want 113.0000", got)
		}
		if got := text(second, "credit_note_remaining_amount"); got != "0.0000" {
			t.Fatalf("second credit note remaining = %s, want 0.0000", got)
		}
		if got := text(second, "credit_note_status"); got != "applied" {
			t.Fatalf("second credit note status = %s, want applied", got)
		}

		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "credited_amount"); got != "226.0000" {
			t.Fatalf("invoice credited after full return = %s, want 226.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("invoice outstanding after full return = %s, want 0.0000", got)
		}
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("invoice status after full return = %s, want paid", got)
		}

		shipmentLine = shipmentLineByID(t, admin, partialOrderID, shipmentID, shipmentLineID)
		if got := number(t, shipmentLine, "returned_qty"); !nearlyEqual(got, 2) {
			t.Fatalf("shipment returned_qty after full return = %v, want 2", got)
		}
		if got := number(t, shipmentLine, "returnable_qty"); !nearlyEqual(got, 0) {
			t.Fatalf("shipment returnable_qty after full return = %v, want 0", got)
		}

		admin.expectError(http.MethodPost, "/sales/shipments/"+shipmentID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"shipment_line_id": shipmentLineID, "quantity": "1"},
			},
		}, http.StatusConflict, "OVER_RETURN")
	})

	t.Run("return list and detail contract", func(t *testing.T) {
		items := listItems(admin, "/sales/returns?shipment_id="+partialShipmentID+"&page_size=200", http.StatusOK)
		if len(items) != 2 {
			t.Fatalf("return list count = %d, want 2", len(items))
		}
		for _, item := range items {
			lines, _ := item["lines"].([]any)
			if len(lines) == 0 {
				t.Fatalf("return list item %s has no lines", text(item, "id"))
			}
			if text(item, "code") == "" || text(item, "shipment_id") != partialShipmentID {
				t.Fatalf("return list item missing code/shipment: %v", item)
			}
			if text(item, "customer_id") != customerID {
				t.Fatalf("return list customer = %s, want %s", text(item, "customer_id"), customerID)
			}
			if text(item, "total_amount") != "113.0000" {
				t.Fatalf("return list total = %s, want 113.0000", text(item, "total_amount"))
			}
		}
		row := findByID(items, firstReturnID)
		if row == nil {
			t.Fatalf("first return %s not found in list", firstReturnID)
		}
		if text(row, "shipment_code") == "" {
			t.Fatal("return list shipment_code is empty")
		}
		if text(row, "customer_name") == "" {
			t.Fatal("return list customer_name is empty")
		}

		detail := admin.mustGet("/sales/returns/"+firstReturnID, http.StatusOK)
		if text(detail, "credit_note_code") == "" {
			t.Fatal("return detail credit_note_code is empty")
		}
		if text(detail, "invoice_code") == "" {
			t.Fatal("return detail invoice_code is empty")
		}
		detailLines, _ := detail["lines"].([]any)
		if len(detailLines) != 1 {
			t.Fatalf("return detail lines = %d, want 1", len(detailLines))
		}
		if got := text(detailLines[0].(map[string]any), "shipment_line_id"); got != partialShipmentLineID {
			t.Fatalf("return detail shipment_line_id = %s, want %s", got, partialShipmentLineID)
		}
		if got := number(t, detail, "net_amount"); !nearlyEqual(got, 100) {
			t.Fatalf("return detail net = %v, want 100", got)
		}
		if got := number(t, detail, "tax_amount"); !nearlyEqual(got, 13) {
			t.Fatalf("return detail tax = %v, want 13", got)
		}
		if got := number(t, detail, "total_amount"); !nearlyEqual(got, 113) {
			t.Fatalf("return detail total = %v, want 113", got)
		}
	})

	t.Run("credit note list and detail", func(t *testing.T) {
		notes := listItems(admin, "/finance/credit-notes?partner_id="+customerID+"&page_size=200", http.StatusOK)
		found := make([]map[string]any, 0, 2)
		for _, note := range notes {
			if text(note, "source_type") != "sales_return" {
				continue
			}
			if text(note, "source_id") == firstReturnID || text(note, "source_id") == secondReturnID {
				found = append(found, note)
			}
		}
		if len(found) != 2 {
			t.Fatalf("credit notes for returns = %d, want 2", len(found))
		}
		codes := map[string]bool{firstReturnCode: true, secondReturnCode: true}
		for _, note := range found {
			if !codes[text(note, "source_code")] {
				t.Fatalf("credit note source_code = %s, want one of the return codes", text(note, "source_code"))
			}
			if text(note, "invoice_id") != partialInvoiceID {
				t.Fatalf("credit note invoice_id = %s, want %s", text(note, "invoice_id"), partialInvoiceID)
			}
			if text(note, "partner_id") != customerID {
				t.Fatalf("credit note partner_id = %s, want %s", text(note, "partner_id"), customerID)
			}
			if got := text(note, "amount"); got != "113.0000" {
				t.Fatalf("credit note amount = %s, want 113.0000", got)
			}
			if got := text(note, "applied_amount"); got != "113.0000" {
				t.Fatalf("credit note applied = %s, want 113.0000", got)
			}
			if got := text(note, "remaining_amount"); got != "0.0000" {
				t.Fatalf("credit note remaining = %s, want 0.0000", got)
			}
			if got := text(note, "status"); got != "applied" {
				t.Fatalf("credit note status = %s, want applied", got)
			}
		}

		note := findItem(found, "source_id", firstReturnID)
		if note == nil {
			t.Fatal("first return credit note not found")
		}
		detail := admin.mustGet("/finance/credit-notes/"+text(note, "id"), http.StatusOK)
		if got := text(detail, "source_code"); got != firstReturnCode {
			t.Fatalf("credit note detail source_code = %s, want %s", got, firstReturnCode)
		}
		if got := text(detail, "invoice_code"); got == "" {
			t.Fatal("credit note detail invoice_code is empty")
		}
		if got := text(detail, "amount"); got != "113.0000" {
			t.Fatalf("credit note detail amount = %s, want 113.0000", got)
		}
		if got := text(detail, "applied_amount"); got != "113.0000" {
			t.Fatalf("credit note detail applied = %s, want 113.0000", got)
		}
		if got := text(detail, "remaining_amount"); got != "0.0000" {
			t.Fatalf("credit note detail remaining = %s, want 0.0000", got)
		}
		if got := text(detail, "status"); got != "applied" {
			t.Fatalf("credit note detail status = %s, want applied", got)
		}
	})

	t.Run("rounding balanced partial returns", func(t *testing.T) {
		_, shipmentID, shipmentLineID, invoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"3", "33.333333", true, "stage3h-rounding-return",
		)

		first := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3h-rounding-1")
		if got := text(first, "net_amount"); got != "33.3333" {
			t.Fatalf("rounding first net = %s, want 33.3333", got)
		}
		if got := text(first, "tax_amount"); got != "0.0000" {
			t.Fatalf("rounding first tax = %s, want 0.0000", got)
		}
		if got := text(first, "total_amount"); got != "33.3333" {
			t.Fatalf("rounding first total = %s, want 33.3333", got)
		}
		firstLines := returnVoucherLines(t, admin, text(first, "id"), text(first, "code"))
		assertVoucherBalanced(t, firstLines)
		if got := number(t, findLineByAccount(t, firstLines, "6001"), "debit"); !nearlyEqual(got, 33.3333) {
			t.Fatalf("rounding revenue debit = %v, want 33.3333", got)
		}
		if got := number(t, findLineByAccount(t, firstLines, "1122"), "credit"); !nearlyEqual(got, 33.3333) {
			t.Fatalf("rounding AR credit = %v, want 33.3333", got)
		}

		second := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3h-rounding-2")
		if got := text(second, "net_amount"); got != "33.3334" {
			t.Fatalf("rounding second net = %s, want 33.3334", got)
		}
		if got := text(second, "total_amount"); got != "33.3334" {
			t.Fatalf("rounding second total = %s, want 33.3334", got)
		}
		third := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3h-rounding-3")
		if got := text(third, "net_amount"); got != "33.3333" {
			t.Fatalf("rounding third net = %s, want 33.3333", got)
		}
		if got := text(third, "total_amount"); got != "33.3333" {
			t.Fatalf("rounding third total = %s, want 33.3333", got)
		}

		total := number(t, first, "net_amount") + number(t, second, "net_amount") + number(t, third, "net_amount")
		if !nearlyEqual(total, 100) {
			t.Fatalf("rounding return net total = %v, want 100", total)
		}
		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := number(t, invoice, "credited_amount"); !nearlyEqual(got, 100) {
			t.Fatalf("rounding invoice credited = %v, want 100", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("rounding invoice outstanding = %s, want 0.0000", got)
		}
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("rounding invoice status = %s, want paid", got)
		}
	})

	t.Run("fully paid invoice return leaves open credit", func(t *testing.T) {
		_, shipmentID, shipmentLineID, invoiceID, lineCost := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat13ID,
			"1", "113.00", true, "stage3h-paid-return",
		)
		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": today,
			"currency":     "CNY",
			"amount":       "113.00",
			"method":       "bank",
			"reference":    "stage3h-paid-return",
		}, http.StatusCreated)
		admin.mustPost("/finance/payments/"+text(payment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": invoiceID, "amount": "113.00"},
			},
		}, http.StatusOK)
		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("paid invoice status = %s, want paid", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("paid invoice outstanding = %s, want 0.0000", got)
		}

		item := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3h-paid-invoice-return")
		if got := text(item, "credit_note_amount"); got != "113.0000" {
			t.Fatalf("open credit note amount = %s, want 113.0000", got)
		}
		if got := text(item, "credit_note_applied_amount"); got != "0.0000" {
			t.Fatalf("open credit note applied = %s, want 0.0000", got)
		}
		if got := text(item, "credit_note_remaining_amount"); got != "113.0000" {
			t.Fatalf("open credit note remaining = %s, want 113.0000", got)
		}
		if got := text(item, "credit_note_status"); got != "open" {
			t.Fatalf("open credit note status = %s, want open", got)
		}

		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "credited_amount"); got != "0.0000" {
			t.Fatalf("paid invoice credited = %s, want 0.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("paid invoice outstanding after return = %s, want 0.0000", got)
		}
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("paid invoice status after return = %s, want paid", got)
		}

		voucherLines := returnVoucherLines(t, admin, text(item, "id"), text(item, "code"))
		assertVoucherBalanced(t, voucherLines)
		if got := number(t, findLineByAccount(t, voucherLines, "6001"), "debit"); !nearlyEqual(got, 100) {
			t.Fatalf("paid return revenue debit = %v, want 100", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "222102"), "debit"); !nearlyEqual(got, 13) {
			t.Fatalf("paid return output VAT debit = %v, want 13", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "1122"), "credit"); !nearlyEqual(got, 113) {
			t.Fatalf("paid return AR credit = %v, want 113", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "1405"), "debit"); !nearlyEqual(got, lineCost) {
			t.Fatalf("paid return inventory debit = %v, want %v", got, lineCost)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "6401"), "credit"); !nearlyEqual(got, lineCost) {
			t.Fatalf("paid return COGS credit = %v, want %v", got, lineCost)
		}
	})

	t.Run("zero value return posts inventory only", func(t *testing.T) {
		_, shipmentID, shipmentLineID, invoiceID, lineCost := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "0.00", true, "stage3h-zero-value-return",
		)
		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "total_amount"); got != "0.0000" {
			t.Fatalf("zero value invoice total = %s, want 0.0000", got)
		}
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("zero value invoice status = %s, want paid", got)
		}
		baseline := inventoryBalance(t, admin, warehouseID, materialID)
		baselineOnHand := number(t, baseline, "on_hand")

		item := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3h-zero-value-return-post")
		if got := text(item, "net_amount"); got != "0.0000" {
			t.Fatalf("zero value return net = %s, want 0.0000", got)
		}
		if got := text(item, "tax_amount"); got != "0.0000" {
			t.Fatalf("zero value return tax = %s, want 0.0000", got)
		}
		if got := text(item, "total_amount"); got != "0.0000" {
			t.Fatalf("zero value return total = %s, want 0.0000", got)
		}
		if got := text(item, "credit_note_id"); got != "" {
			t.Fatalf("zero value return credit_note_id = %s, want empty", got)
		}
		if got := text(item, "credit_note_code"); got != "" {
			t.Fatalf("zero value return credit_note_code = %s, want empty", got)
		}

		after := inventoryBalance(t, admin, warehouseID, materialID)
		if got := number(t, after, "on_hand"); !nearlyEqual(got, baselineOnHand+1) {
			t.Fatalf("on_hand after zero value return = %v, want %v", got, baselineOnHand+1)
		}
		if lineCost > 0 {
			if got := text(item, "voucher_code"); got == "" {
				t.Fatal("zero value return voucher_code is empty with positive cost")
			}
			voucherLines := returnVoucherLines(t, admin, text(item, "id"), text(item, "code"))
			if len(voucherLines) != 2 {
				t.Fatalf("zero value return voucher lines = %d, want 2", len(voucherLines))
			}
			assertVoucherBalanced(t, voucherLines)
			if got := number(t, findLineByAccount(t, voucherLines, "1405"), "debit"); !nearlyEqual(got, lineCost) {
				t.Fatalf("zero value return inventory debit = %v, want %v", got, lineCost)
			}
			if got := number(t, findLineByAccount(t, voucherLines, "6401"), "credit"); !nearlyEqual(got, lineCost) {
				t.Fatalf("zero value return COGS credit = %v, want %v", got, lineCost)
			}
			for _, raw := range voucherLines {
				code := text(raw.(map[string]any), "account_code")
				if code == "6001" || code == "1122" || code == "222102" {
					t.Fatalf("zero value return voucher unexpectedly contains account %s", code)
				}
			}
		} else {
			if got := text(item, "voucher_code"); got != "" {
				t.Fatalf("zero cost return voucher_code = %s, want empty", got)
			}
			if voucher := findVoucherBySource(admin, text(item, "code")); voucher != nil {
				t.Fatalf("zero cost return should not post a voucher: %v", voucher)
			}
		}

		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "credited_amount"); got != "0.0000" {
			t.Fatalf("zero value invoice credited = %s, want 0.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("zero value invoice outstanding = %s, want 0.0000", got)
		}
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("zero value invoice status after return = %s, want paid", got)
		}

		shipmentLine := shipmentLineByID(t, admin, text(item, "order_id"), shipmentID, shipmentLineID)
		if got := number(t, shipmentLine, "returned_qty"); !nearlyEqual(got, 1) {
			t.Fatalf("zero value shipment returned_qty = %v, want 1", got)
		}
	})

	t.Run("payment allocation respects credited amount", func(t *testing.T) {
		_, shipmentID, shipmentLineID, invoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"2", "100.00", true, "stage3h-credit-payment",
		)
		item := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3h-credit-payment-return")
		if got := text(item, "total_amount"); got != "100.0000" {
			t.Fatalf("credit payment return total = %s, want 100.0000", got)
		}

		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "total_amount"); got != "200.0000" {
			t.Fatalf("credit payment invoice total = %s, want 200.0000", got)
		}
		if got := text(invoice, "credited_amount"); got != "100.0000" {
			t.Fatalf("credit payment invoice credited = %s, want 100.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "100.0000" {
			t.Fatalf("credit payment invoice outstanding = %s, want 100.0000", got)
		}
		if got := text(invoice, "status"); got != "partially_paid" {
			t.Fatalf("credit payment invoice status = %s, want partially_paid", got)
		}

		overPayment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": today,
			"currency":     "CNY",
			"amount":       "101.00",
			"method":       "bank",
			"reference":    "stage3h-credit-payment-over",
		}, http.StatusCreated)
		admin.expectError(http.MethodPost, "/finance/payments/"+text(overPayment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": invoiceID, "amount": "101.00"},
			},
		}, http.StatusBadRequest, "OVER_ALLOCATE")

		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": today,
			"currency":     "CNY",
			"amount":       "100.00",
			"method":       "bank",
			"reference":    "stage3h-credit-payment",
		}, http.StatusCreated)
		admin.mustPost("/finance/payments/"+text(payment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": invoiceID, "amount": "100.00"},
			},
		}, http.StatusOK)

		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "paid_amount"); got != "100.0000" {
			t.Fatalf("credit payment invoice paid = %s, want 100.0000", got)
		}
		if got := text(invoice, "credited_amount"); got != "100.0000" {
			t.Fatalf("credit payment invoice credited after payment = %s, want 100.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("credit payment invoice outstanding after payment = %s, want 0.0000", got)
		}
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("credit payment invoice status after payment = %s, want paid", got)
		}

		status, payload := admin.request(http.MethodPost, "/finance/payments/"+text(overPayment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": invoiceID, "amount": "1.00"},
			},
		}, admin.token)
		if status != http.StatusBadRequest && status != http.StatusConflict {
			t.Fatalf("allocate after credited payment status = %d, want 400 or 409, payload = %v", status, payload)
		}
	})

	t.Run("permissions and validation", func(t *testing.T) {
		unauthorized := newClient(t)
		unauthorized.expectError(http.MethodGet, "/sales/returns", nil, http.StatusUnauthorized, "UNAUTHORIZED")

		dept := newClient(t).login("demo_dept", env("TEST_DEMO_PASSWORD", "Demo@123456"))
		dept.mustGet("/sales/returns?page_size=20", http.StatusOK)
		dept.expectError(http.MethodPost, "/sales/shipments/"+partialShipmentID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"shipment_line_id": partialShipmentLineID, "quantity": "1"},
			},
		}, http.StatusForbidden, "FORBIDDEN")

		admin.expectError(http.MethodPost, "/sales/shipments/not-a-uuid/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"shipment_line_id": partialShipmentLineID, "quantity": "1"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/sales/shipments/"+partialShipmentID+"/returns", map[string]any{
			"return_date": today,
			"lines":       []map[string]any{},
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/sales/shipments/"+partialShipmentID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"shipment_line_id": partialShipmentLineID, "quantity": "0"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/sales/shipments/"+partialShipmentID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"shipment_line_id": partialShipmentLineID, "quantity": "-1"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/sales/shipments/"+partialShipmentID+"/returns", map[string]any{
			"return_date": "bad-date",
			"lines": []map[string]any{
				{"shipment_line_id": partialShipmentLineID, "quantity": "1"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")

		admin.expectError(http.MethodGet, "/sales/returns/00000000-0000-0000-0000-000000000000", nil, http.StatusNotFound, "NOT_FOUND")
		admin.expectError(http.MethodGet, "/sales/returns/not-a-uuid", nil, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodGet, "/finance/credit-notes/00000000-0000-0000-0000-000000000000", nil, http.StatusNotFound, "NOT_FOUND")
		admin.expectError(http.MethodGet, "/finance/credit-notes/not-a-uuid", nil, http.StatusBadRequest, "BAD_REQUEST")
	})

	t.Run("foreign currency return", func(t *testing.T) {
		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":        customerID,
			"warehouse_id":       warehouseID,
			"order_date":         today,
			"currency":           "USD",
			"exchange_rate":      "7.2",
			"price_includes_tax": true,
			"remark":             "stage3h-usd-return",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "100.00", "tax_rate_id": vat0ID},
			},
		}, http.StatusCreated)
		orderID := text(order, "id")
		admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		if len(lines) != 1 {
			t.Fatalf("USD order lines = %d, want 1", len(lines))
		}
		shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
			"shipment_date": today,
			"exchange_rate": "7.2",
			"remark":        "stage3h-usd-return-ship",
			"lines": []map[string]any{
				{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusCreated)
		shipmentID := text(shipment, "id")
		shipmentLines, _ := shipment["lines"].([]any)
		if len(shipmentLines) != 1 {
			t.Fatalf("USD shipment lines = %d, want 1", len(shipmentLines))
		}
		shipmentLineID := text(shipmentLines[0].(map[string]any), "id")
		invoiceID := text(shipment, "invoice_id")
		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "base_total_amount"); got != "720.0000" {
			t.Fatalf("USD invoice base total = %s, want 720.0000", got)
		}

		item := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3h-usd-return-post")
		if got := text(item, "currency"); got != "USD" {
			t.Fatalf("USD return currency = %s, want USD", got)
		}
		if got := text(item, "net_amount"); got != "100.0000" {
			t.Fatalf("USD return net = %s, want 100.0000", got)
		}
		if got := text(item, "base_net_amount"); got != "720.0000" {
			t.Fatalf("USD return base net = %s, want 720.0000", got)
		}
		if got := text(item, "total_amount"); got != "100.0000" {
			t.Fatalf("USD return total = %s, want 100.0000", got)
		}
		if got := text(item, "base_total_amount"); got != "720.0000" {
			t.Fatalf("USD return base total = %s, want 720.0000", got)
		}
		if got := text(item, "credit_note_amount"); got != "100.0000" {
			t.Fatalf("USD credit note amount = %s, want 100.0000", got)
		}
		note := admin.mustGet("/finance/credit-notes/"+text(item, "credit_note_id"), http.StatusOK)
		if got := text(note, "amount"); got != "100.0000" {
			t.Fatalf("USD credit note stored amount = %s, want 100.0000", got)
		}
		if got := text(note, "base_amount"); got != "720.0000" {
			t.Fatalf("USD credit note base amount = %s, want 720.0000", got)
		}

		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "base_credited_amount"); got != "720.0000" {
			t.Fatalf("USD invoice base credited = %s, want 720.0000", got)
		}
		if got := text(invoice, "credited_amount"); got != "100.0000" {
			t.Fatalf("USD invoice credited = %s, want 100.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("USD invoice outstanding = %s, want 0.0000", got)
		}
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("USD invoice status = %s, want paid", got)
		}

		voucherLines := returnVoucherLines(t, admin, text(item, "id"), text(item, "code"))
		assertVoucherBalanced(t, voucherLines)
		if got := number(t, findLineByAccount(t, voucherLines, "6001"), "debit"); !nearlyEqual(got, 720) {
			t.Fatalf("USD return revenue debit = %v, want 720", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "1122"), "credit"); !nearlyEqual(got, 720) {
			t.Fatalf("USD return AR credit = %v, want 720", got)
		}
	})

	t.Run("fx revaluation uses remaining receivable after partial return", func(t *testing.T) {
		year := cleanRevaluationYear(t, admin)
		ensureTestYear(t, admin, year)
		orderDate := fmt.Sprintf("%04d-01-15", year)
		returnDate := fmt.Sprintf("%04d-01-20", year)
		closeDate := fmt.Sprintf("%04d-01-31", year)

		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":        customerID,
			"warehouse_id":       warehouseID,
			"order_date":         orderDate,
			"currency":           "USD",
			"exchange_rate":      "7.2",
			"price_includes_tax": true,
			"remark":             "stage3h-fx-revaluation-return",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "2", "unit_price": "100.00", "tax_rate_id": vat0ID},
			},
		}, http.StatusCreated)
		orderID := text(order, "id")
		admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		if len(lines) != 1 {
			t.Fatalf("FX revaluation order lines = %d, want 1", len(lines))
		}
		shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
			"shipment_date": orderDate,
			"exchange_rate": "7.2",
			"remark":        "stage3h-fx-revaluation-return-ship",
			"lines": []map[string]any{
				{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "2"},
			},
		}, http.StatusCreated)
		shipmentID := text(shipment, "id")
		shipmentLines, _ := shipment["lines"].([]any)
		if len(shipmentLines) != 1 {
			t.Fatalf("FX revaluation shipment lines = %d, want 1", len(shipmentLines))
		}
		shipmentLineID := text(shipmentLines[0].(map[string]any), "id")
		invoiceID := text(shipment, "invoice_id")
		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "total_amount"); got != "200.0000" {
			t.Fatalf("FX revaluation invoice total = %s, want 200.0000", got)
		}
		if got := text(invoice, "base_total_amount"); got != "1440.0000" {
			t.Fatalf("FX revaluation invoice base total = %s, want 1440.0000", got)
		}

		item := postReturn(t, admin, shipmentID, shipmentLineID, "1", returnDate, "stage3h-fx-revaluation-return-post")
		if got := text(item, "net_amount"); got != "100.0000" {
			t.Fatalf("FX revaluation return net = %s, want 100.0000", got)
		}
		if got := text(item, "base_net_amount"); got != "720.0000" {
			t.Fatalf("FX revaluation return base net = %s, want 720.0000", got)
		}
		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "credited_amount"); got != "100.0000" {
			t.Fatalf("FX revaluation invoice credited = %s, want 100.0000", got)
		}
		if got := text(invoice, "base_credited_amount"); got != "720.0000" {
			t.Fatalf("FX revaluation invoice base credited = %s, want 720.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "100.0000" {
			t.Fatalf("FX revaluation invoice outstanding = %s, want 100.0000", got)
		}
		baseOutstanding := number(t, invoice, "base_total_amount") - number(t, invoice, "base_credited_amount")
		if !nearlyEqual(baseOutstanding, 720) {
			t.Fatalf("FX revaluation base outstanding = %v, want 720", baseOutstanding)
		}
		if got := text(invoice, "status"); got != "partially_paid" {
			t.Fatalf("FX revaluation invoice status = %s, want partially_paid", got)
		}

		admin.mustPost("/finance/exchange-rates", map[string]any{
			"currency_code": "USD",
			"rate_date":     closeDate,
			"rate":          "7.3",
		}, http.StatusCreated)

		periods := getArray(admin, periodsPath(year), http.StatusOK)
		var january map[string]any
		for _, period := range periods {
			if month, ok := period["month"].(float64); ok && int(month) == 1 {
				january = period
				break
			}
		}
		if january == nil {
			t.Fatalf("January %d period not found", year)
		}
		if text(january, "status") == "closed" {
			admin.mustPost("/finance/periods/"+text(january, "id")+"/reopen", nil, http.StatusOK)
		}
		admin.mustPost("/finance/periods/"+text(january, "id")+"/close", nil, http.StatusOK)

		janStart := fmt.Sprintf("%04d-01-01", year)
		janEnd := closeDate
		revaluations := listItems(admin,
			"/finance/vouchers?voucher_type=fx_revaluation&start_date="+janStart+"&end_date="+janEnd+"&page_size=50",
			http.StatusOK,
		)
		revaluation := findItem(revaluations, "source_code", fmt.Sprintf("%04d-01", year))
		if revaluation == nil {
			t.Fatalf("FX revaluation voucher for %04d-01 not found", year)
		}
		revaluationDetail := admin.mustGet("/finance/vouchers/"+text(revaluation, "id"), http.StatusOK)
		revaluationLines, _ := revaluationDetail["lines"].([]any)
		assertVoucherBalanced(t, revaluationLines)
		if got := number(t, findLineByAccount(t, revaluationLines, "1122"), "debit"); !nearlyEqual(got, 10) {
			t.Fatalf("FX revaluation AR debit = %v, want 10", got)
		}
		if got := number(t, findLineByAccount(t, revaluationLines, "660302"), "credit"); !nearlyEqual(got, 10) {
			t.Fatalf("FX revaluation gain credit = %v, want 10", got)
		}
		revalued := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(revalued, "base_revalued_amount"); got != "10.0000" {
			t.Fatalf("FX revaluated invoice base revalued = %s, want 10.0000", got)
		}
	})
}

func ensureStage3HStock(t *testing.T, admin *client, supplierID, warehouseID, materialID, date string) {
	t.Helper()
	data := admin.mustGet("/inventory/balances?warehouse_id="+warehouseID+"&material_id="+materialID, http.StatusOK)
	items, _ := data["items"].([]any)
	if len(items) > 0 {
		balance, ok := items[0].(map[string]any)
		if ok && number(t, balance, "on_hand") >= 20 {
			return
		}
	}

	order := admin.mustPost("/purchase/orders", map[string]any{
		"supplier_id":        supplierID,
		"warehouse_id":       warehouseID,
		"order_date":         date,
		"currency":           "CNY",
		"price_includes_tax": false,
		"remark":             "stage3h-stock",
		"lines": []map[string]any{
			{"material_id": materialID, "quantity": "100", "unit_price": "50.00"},
		},
	}, http.StatusCreated)
	orderID := text(order, "id")
	admin.mustPost("/purchase/orders/"+orderID+"/submit", nil, http.StatusOK)
	admin.mustPost("/purchase/orders/"+orderID+"/approve", nil, http.StatusOK)
	detail := admin.mustGet("/purchase/orders/"+orderID, http.StatusOK)
	lines, _ := detail["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("stock purchase order %s lines = %d, want 1", orderID, len(lines))
	}
	admin.mustPost("/purchase/orders/"+orderID+"/receive", map[string]any{
		"receipt_date": date,
		"remark":       "stage3h-stock-receipt",
		"lines": []map[string]any{
			{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": "100"},
		},
	}, http.StatusCreated)
}

func createShippedOrder(
	t *testing.T,
	admin *client,
	customerID, warehouseID, materialID, taxID, quantity, unitPrice string,
	priceIncludesTax bool,
	remark string,
) (string, string, string, string, float64) {
	t.Helper()
	date := time.Now().Format("2006-01-02")
	order := admin.mustPost("/sales/orders", map[string]any{
		"customer_id":        customerID,
		"warehouse_id":       warehouseID,
		"order_date":         date,
		"currency":           "CNY",
		"price_includes_tax": priceIncludesTax,
		"remark":             remark,
		"lines": []map[string]any{
			{"material_id": materialID, "quantity": quantity, "unit_price": unitPrice, "tax_rate_id": taxID},
		},
	}, http.StatusCreated)
	orderID := text(order, "id")
	admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
	admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
	detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
	lines, _ := detail["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("sales order %s lines = %d, want 1", orderID, len(lines))
	}
	shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
		"shipment_date": date,
		"remark":        remark + "-ship",
		"lines": []map[string]any{
			{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": quantity},
		},
	}, http.StatusCreated)
	shipmentID := text(shipment, "id")
	shipmentLines, _ := shipment["lines"].([]any)
	if len(shipmentLines) != 1 {
		t.Fatalf("sales shipment %s lines = %d, want 1", shipmentID, len(shipmentLines))
	}
	line := shipmentLines[0].(map[string]any)
	return orderID, shipmentID, text(line, "id"), text(shipment, "invoice_id"), number(t, line, "cost_amount")
}

func postReturn(t *testing.T, admin *client, shipmentID, shipmentLineID, quantity, date, remark string) map[string]any {
	t.Helper()
	return admin.mustPost("/sales/shipments/"+shipmentID+"/returns", map[string]any{
		"return_date": date,
		"reason":      "customer return",
		"remark":      remark,
		"lines": []map[string]any{
			{"shipment_line_id": shipmentLineID, "quantity": quantity},
		},
	}, http.StatusCreated)
}

func inventoryBalance(t *testing.T, admin *client, warehouseID, materialID string) map[string]any {
	t.Helper()
	data := admin.mustGet("/inventory/balances?warehouse_id="+warehouseID+"&material_id="+materialID, http.StatusOK)
	items, _ := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("inventory balance rows = %d, want 1", len(items))
	}
	balance, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("inventory balance payload = %v", items[0])
	}
	return balance
}

func shipmentLineByID(t *testing.T, admin *client, orderID, shipmentID, lineID string) map[string]any {
	t.Helper()
	shipments := listItems(admin, "/sales/shipments?order_id="+orderID+"&page_size=200", http.StatusOK)
	for _, shipment := range shipments {
		if text(shipment, "id") != shipmentID {
			continue
		}
		lines, _ := shipment["lines"].([]any)
		for _, raw := range lines {
			line, ok := raw.(map[string]any)
			if ok && text(line, "id") == lineID {
				return line
			}
		}
	}
	t.Fatalf("shipment line %s for shipment %s not found", lineID, shipmentID)
	return nil
}

func returnInventoryTransactions(t *testing.T, admin *client, warehouseID, materialID, returnID string) []map[string]any {
	t.Helper()
	items := listItems(admin,
		"/inventory/transactions?warehouse_id="+warehouseID+"&material_id="+materialID+"&page_size=200",
		http.StatusOK,
	)
	matches := make([]map[string]any, 0, 1)
	for _, item := range items {
		if text(item, "biz_id") == returnID && text(item, "biz_type") == "sales_return" {
			matches = append(matches, item)
		}
	}
	return matches
}

func returnVoucherLines(t *testing.T, admin *client, returnID, returnCode string) []any {
	t.Helper()
	voucher := findVoucherBySource(admin, returnCode)
	if voucher == nil {
		t.Fatalf("return voucher for %s not found", returnCode)
	}
	if got := text(voucher, "source_id"); got != returnID {
		t.Fatalf("return voucher source_id = %s, want %s", got, returnID)
	}
	if got := text(voucher, "voucher_type"); got != "sales_return" {
		t.Fatalf("return voucher type = %s, want sales_return", got)
	}
	detail := admin.mustGet("/finance/vouchers/"+text(voucher, "id"), http.StatusOK)
	lines, _ := detail["lines"].([]any)
	return lines
}

func assertVoucherBalanced(t *testing.T, lines []any) {
	t.Helper()
	debitTotal := 0.0
	creditTotal := 0.0
	for _, raw := range lines {
		line, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("voucher line payload = %v", raw)
		}
		debitTotal += number(t, line, "debit")
		creditTotal += number(t, line, "credit")
	}
	if !nearlyEqual(debitTotal, creditTotal) {
		t.Fatalf("voucher debit total = %v, credit total = %v, want equal", debitTotal, creditTotal)
	}
}
