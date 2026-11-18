//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestStage3I(t *testing.T) {
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

	supplier := findItem(listItems(admin, "/masterdata/partners?keyword=SUP-DEMO-001&page_size=20", http.StatusOK), "code", "SUP-DEMO-001")
	customer := findItem(listItems(admin, "/masterdata/partners?keyword=CUS-DEMO-001&page_size=20", http.StatusOK), "code", "CUS-DEMO-001")
	warehouse := findItem(listItems(admin, "/masterdata/warehouses?keyword=WH-SH&page_size=20", http.StatusOK), "code", "WH-SH")
	material := findItem(listItems(admin, "/masterdata/materials?keyword=MAT-DEMO-004&page_size=20", http.StatusOK), "code", "MAT-DEMO-004")
	emptyMaterial := findItem(listItems(admin, "/masterdata/materials?keyword=MAT-DEMO-020&page_size=20", http.StatusOK), "code", "MAT-DEMO-020")
	vat13 := findItem(listItems(admin, "/masterdata/tax-rates?keyword=VAT13&page_size=20", http.StatusOK), "code", "VAT13")
	vat0 := findItem(listItems(admin, "/masterdata/tax-rates?keyword=VAT0&page_size=20", http.StatusOK), "code", "VAT0")
	if supplier == nil || customer == nil || warehouse == nil || material == nil || emptyMaterial == nil || vat13 == nil || vat0 == nil {
		t.Fatal("demo master data missing")
	}
	supplierID := text(supplier, "id")
	customerID := text(customer, "id")
	warehouseID := text(warehouse, "id")
	materialID := text(material, "id")
	emptyMaterialID := text(emptyMaterial, "id")
	vat13ID := text(vat13, "id")
	vat0ID := text(vat0, "id")

	var partialReceiptID string
	var partialReceiptLineID string
	var partialOrderID string
	var partialInvoiceID string
	var firstReturnID string
	var firstReturnCode string
	var firstCreditNoteID string
	var secondReturnID string
	var secondReturnCode string

	t.Run("partial and full purchase return with credit note inventory and voucher", func(t *testing.T) {
		beforeReceipt := stage3IBalanceMaybe(t, admin, warehouseID, materialID)
		beforeReceiptOnHand := 0.0
		if beforeReceipt != nil {
			beforeReceiptOnHand = number(t, beforeReceipt, "on_hand")
		}

		receiptID, receiptLineID, orderID, invoiceID, lineCost := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat13ID,
			"2", "113.00", true, "CNY", "", today, today, "stage3i-partial-return",
		)
		partialReceiptID = receiptID
		partialReceiptLineID = receiptLineID
		partialOrderID = orderID
		partialInvoiceID = invoiceID

		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "total_amount"); got != "226.0000" {
			t.Fatalf("invoice total = %s, want 226.0000", got)
		}
		if got := text(invoice, "credited_amount"); got != "0.0000" {
			t.Fatalf("invoice credited = %s, want 0.0000", got)
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

		first := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3i-first-return")
		firstReturnID = text(first, "id")
		firstReturnCode = text(first, "code")
		if firstReturnID == "" || firstReturnCode == "" {
			t.Fatalf("first return id/code missing: %v", first)
		}
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
		if !nearlyEqual(firstCost, lineCost) {
			t.Fatalf("first return cost = %v, want %v", firstCost, lineCost)
		}
		firstCreditNoteID = text(first, "credit_note_id")
		if firstCreditNoteID == "" || text(first, "credit_note_code") == "" {
			t.Fatalf("first return credit note id/code missing: %v", first)
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
			t.Fatalf("invoice credited after first return = %s, want 113.0000", got)
		}
		if got := text(invoice, "base_credited_amount"); got != "113.0000" {
			t.Fatalf("invoice base credited after first return = %s, want 113.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "113.0000" {
			t.Fatalf("invoice outstanding after first return = %s, want 113.0000", got)
		}
		if got := text(invoice, "status"); got != "partially_paid" {
			t.Fatalf("invoice status after first return = %s, want partially_paid", got)
		}

		afterFirst := inventoryBalance(t, admin, warehouseID, materialID)
		if got := number(t, afterFirst, "on_hand"); !nearlyEqual(got, baselineOnHand-1) {
			t.Fatalf("on_hand after first return = %v, want %v", got, baselineOnHand-1)
		}
		expectedAvgCost := (baselineOnHand*baselineAvgCost - firstCost) / (baselineOnHand - 1)
		if got := number(t, afterFirst, "avg_cost"); !nearlyEqual(got, expectedAvgCost) {
			t.Fatalf("avg_cost after first return = %v, want %v", got, expectedAvgCost)
		}

		transactions := stage3IPurchaseReturnTransactions(t, admin, warehouseID, materialID, firstReturnID)
		if len(transactions) != 1 {
			t.Fatalf("return inventory transactions = %d, want 1", len(transactions))
		}
		transaction := transactions[0]
		if got := text(transaction, "direction"); got != "out" {
			t.Fatalf("return inventory direction = %s, want out", got)
		}
		if got := text(transaction, "biz_type"); got != "purchase_return" {
			t.Fatalf("return inventory biz_type = %s, want purchase_return", got)
		}
		if got := number(t, transaction, "quantity"); !nearlyEqual(got, 1) {
			t.Fatalf("return inventory quantity = %v, want 1", got)
		}
		if got := number(t, transaction, "amount"); !nearlyEqual(got, firstCost) {
			t.Fatalf("return inventory amount = %v, want %v", got, firstCost)
		}

		voucherLines := stage3IPurchaseReturnVoucherLines(t, admin, firstReturnID, firstReturnCode)
		assertVoucherBalanced(t, voucherLines)
		if got := number(t, findLineByAccount(t, voucherLines, "2202"), "debit"); !nearlyEqual(got, 113) {
			t.Fatalf("return AP debit = %v, want 113", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "1405"), "credit"); !nearlyEqual(got, 100) {
			t.Fatalf("return inventory credit = %v, want 100", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "222101"), "credit"); !nearlyEqual(got, 13) {
			t.Fatalf("return input VAT credit = %v, want 13", got)
		}

		receiptLine := stage3IReceiptLineByID(t, admin, orderID, receiptID, receiptLineID)
		if got := number(t, receiptLine, "returned_qty"); !nearlyEqual(got, 1) {
			t.Fatalf("receipt returned_qty after first return = %v, want 1", got)
		}
		if got := number(t, receiptLine, "returnable_qty"); !nearlyEqual(got, 1) {
			t.Fatalf("receipt returnable_qty after first return = %v, want 1", got)
		}

		second := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3i-second-return")
		secondReturnID = text(second, "id")
		secondReturnCode = text(second, "code")
		if secondReturnID == "" || secondReturnCode == "" {
			t.Fatalf("second return id/code missing: %v", second)
		}
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
		if !nearlyEqual(secondCost, lineCost) {
			t.Fatalf("second return cost = %v, want %v", secondCost, lineCost)
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
		if got := text(invoice, "base_credited_amount"); got != "226.0000" {
			t.Fatalf("invoice base credited after full return = %s, want 226.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("invoice outstanding after full return = %s, want 0.0000", got)
		}
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("invoice status after full return = %s, want paid", got)
		}

		receiptLine = stage3IReceiptLineByID(t, admin, orderID, receiptID, receiptLineID)
		if got := number(t, receiptLine, "returned_qty"); !nearlyEqual(got, 2) {
			t.Fatalf("receipt returned_qty after full return = %v, want 2", got)
		}
		if got := number(t, receiptLine, "returnable_qty"); !nearlyEqual(got, 0) {
			t.Fatalf("receipt returnable_qty after full return = %v, want 0", got)
		}

		afterSecond := inventoryBalance(t, admin, warehouseID, materialID)
		if got := number(t, afterSecond, "on_hand"); !nearlyEqual(got, beforeReceiptOnHand) {
			t.Fatalf("on_hand after full return = %v, want %v", got, beforeReceiptOnHand)
		}

		admin.expectError(http.MethodPost, "/purchase/receipts/"+receiptID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"receipt_line_id": receiptLineID, "quantity": "1"},
			},
		}, http.StatusConflict, "OVER_RETURN")
	})

	t.Run("return list and detail contract", func(t *testing.T) {
		items := listItems(admin, "/purchase/returns?receipt_id="+partialReceiptID+"&page_size=200", http.StatusOK)
		if len(items) != 2 {
			t.Fatalf("return list count = %d, want 2", len(items))
		}
		for _, item := range items {
			if text(item, "id") == "" || text(item, "code") == "" {
				t.Fatalf("return list item missing id/code: %v", item)
			}
			if text(item, "receipt_id") != partialReceiptID || text(item, "receipt_code") == "" {
				t.Fatalf("return list item missing receipt fields: %v", item)
			}
			if text(item, "order_id") != partialOrderID || text(item, "order_code") == "" {
				t.Fatalf("return list item missing order fields: %v", item)
			}
			if text(item, "supplier_id") != supplierID || text(item, "supplier_name") == "" {
				t.Fatalf("return list item missing supplier fields: %v", item)
			}
			if text(item, "warehouse_id") != warehouseID || text(item, "warehouse_name") == "" {
				t.Fatalf("return list item missing warehouse fields: %v", item)
			}
			if got := text(item, "return_date"); got != today {
				t.Fatalf("return list date = %s, want %s", got, today)
			}
			if got := text(item, "status"); got != "posted" {
				t.Fatalf("return list status = %s, want posted", got)
			}
			if got := text(item, "currency"); got != "CNY" {
				t.Fatalf("return list currency = %s, want CNY", got)
			}
			if got := text(item, "exchange_rate"); got != "1.00000000" {
				t.Fatalf("return list exchange_rate = %s, want 1.00000000", got)
			}
			included, _ := item["price_includes_tax"].(bool)
			if !included {
				t.Fatal("return list price_includes_tax = false, want true")
			}
			if got := text(item, "net_amount"); got != "100.0000" {
				t.Fatalf("return list net = %s, want 100.0000", got)
			}
			if got := text(item, "tax_amount"); got != "13.0000" {
				t.Fatalf("return list tax = %s, want 13.0000", got)
			}
			if got := text(item, "total_amount"); got != "113.0000" {
				t.Fatalf("return list total = %s, want 113.0000", got)
			}
			if got := text(item, "base_net_amount"); got != "100.0000" {
				t.Fatalf("return list base net = %s, want 100.0000", got)
			}
			if got := text(item, "base_tax_amount"); got != "13.0000" {
				t.Fatalf("return list base tax = %s, want 13.0000", got)
			}
			if got := text(item, "base_total_amount"); got != "113.0000" {
				t.Fatalf("return list base total = %s, want 113.0000", got)
			}
			if got := text(item, "cost_amount"); got != "100.0000" {
				t.Fatalf("return list cost = %s, want 100.0000", got)
			}
			if text(item, "credit_note_id") == "" || text(item, "credit_note_code") == "" {
				t.Fatalf("return list missing credit note fields: %v", item)
			}
			if got := text(item, "credit_note_amount"); got != "113.0000" {
				t.Fatalf("return list credit note amount = %s, want 113.0000", got)
			}
			if got := text(item, "credit_note_applied_amount"); got != "113.0000" {
				t.Fatalf("return list credit note applied = %s, want 113.0000", got)
			}
			if got := text(item, "credit_note_remaining_amount"); got != "0.0000" {
				t.Fatalf("return list credit note remaining = %s, want 0.0000", got)
			}
			if got := text(item, "credit_note_status"); got != "applied" {
				t.Fatalf("return list credit note status = %s, want applied", got)
			}
			if text(item, "invoice_id") != partialInvoiceID || text(item, "invoice_code") == "" {
				t.Fatalf("return list missing invoice fields: %v", item)
			}
			if text(item, "voucher_code") == "" {
				t.Fatal("return list voucher_code is empty")
			}
			if text(item, "reason") == "" || text(item, "remark") == "" {
				t.Fatalf("return list missing reason/remark: %v", item)
			}
			if text(item, "created_at") == "" {
				t.Fatal("return list created_at is empty")
			}
			lines, _ := item["lines"].([]any)
			if len(lines) != 1 {
				t.Fatalf("return list item %s lines = %d, want 1", text(item, "id"), len(lines))
			}
		}

		row := findByID(items, firstReturnID)
		if row == nil {
			t.Fatalf("first return %s not found in list", firstReturnID)
		}
		detail := admin.mustGet("/purchase/returns/"+firstReturnID, http.StatusOK)
		for _, key := range []string{
			"id", "code", "receipt_id", "receipt_code", "order_id", "order_code",
			"supplier_id", "supplier_name", "warehouse_id", "warehouse_name",
			"return_date", "status", "currency", "exchange_rate",
			"net_amount", "tax_amount", "total_amount",
			"base_net_amount", "base_tax_amount", "base_total_amount", "cost_amount",
			"credit_note_id", "credit_note_code", "credit_note_amount",
			"credit_note_applied_amount", "credit_note_remaining_amount", "credit_note_status",
			"invoice_id", "invoice_code", "voucher_code", "reason", "remark", "created_at",
		} {
			if text(detail, key) != text(row, key) {
				t.Fatalf("return detail %s = %q, list = %q", key, text(detail, key), text(row, key))
			}
		}
		detailLines, _ := detail["lines"].([]any)
		if len(detailLines) != 1 {
			t.Fatalf("return detail lines = %d, want 1", len(detailLines))
		}
		line, ok := detailLines[0].(map[string]any)
		if !ok {
			t.Fatalf("return detail line payload = %v", detailLines[0])
		}
		if got := text(line, "receipt_line_id"); got != partialReceiptLineID {
			t.Fatalf("return detail receipt_line_id = %s, want %s", got, partialReceiptLineID)
		}
		if got := text(line, "order_line_id"); got == "" {
			t.Fatal("return detail order_line_id is empty")
		}
		if got := text(line, "material_id"); got != materialID {
			t.Fatalf("return detail material_id = %s, want %s", got, materialID)
		}
		if text(line, "material_name") == "" {
			t.Fatal("return detail material_name is empty")
		}
		if got := number(t, line, "quantity"); !nearlyEqual(got, 1) {
			t.Fatalf("return detail quantity = %v, want 1", got)
		}
		if got := number(t, line, "unit_price"); !nearlyEqual(got, 113) {
			t.Fatalf("return detail unit_price = %v, want 113", got)
		}
		if got := text(line, "tax_rate_id"); got != vat13ID {
			t.Fatalf("return detail tax_rate_id = %s, want %s", got, vat13ID)
		}
		if got := number(t, line, "tax_rate"); !nearlyEqual(got, 0.13) {
			t.Fatalf("return detail tax_rate = %v, want 0.13", got)
		}
		if got := number(t, line, "amount"); !nearlyEqual(got, 100) {
			t.Fatalf("return detail amount = %v, want 100", got)
		}
		if got := number(t, line, "tax_amount"); !nearlyEqual(got, 13) {
			t.Fatalf("return detail tax_amount = %v, want 13", got)
		}
		if got := number(t, line, "gross_amount"); !nearlyEqual(got, 113) {
			t.Fatalf("return detail gross_amount = %v, want 113", got)
		}
		if got := number(t, line, "base_amount"); !nearlyEqual(got, 100) {
			t.Fatalf("return detail base_amount = %v, want 100", got)
		}
		if got := number(t, line, "base_tax_amount"); !nearlyEqual(got, 13) {
			t.Fatalf("return detail base_tax_amount = %v, want 13", got)
		}
		if got := number(t, line, "base_gross_amount"); !nearlyEqual(got, 113) {
			t.Fatalf("return detail base_gross_amount = %v, want 113", got)
		}
		if got := number(t, line, "unit_cost"); !nearlyEqual(got, 100) {
			t.Fatalf("return detail unit_cost = %v, want 100", got)
		}
		if got := number(t, line, "cost_amount"); !nearlyEqual(got, 100) {
			t.Fatalf("return detail cost_amount = %v, want 100", got)
		}

		admin.expectError(http.MethodGet, "/purchase/returns/00000000-0000-0000-0000-000000000000", nil, http.StatusNotFound, "NOT_FOUND")
		admin.expectError(http.MethodGet, "/purchase/returns/not-a-uuid", nil, http.StatusBadRequest, "BAD_REQUEST")

		supplierRows := listItems(admin, "/purchase/returns?supplier_id="+supplierID+"&page_size=200", http.StatusOK)
		if findByID(supplierRows, firstReturnID) == nil || findByID(supplierRows, secondReturnID) == nil {
			t.Fatal("supplier filter did not return the created purchase returns")
		}
		keywordRows := listItems(admin, "/purchase/returns?keyword="+url.QueryEscape(firstReturnCode)+"&page_size=200", http.StatusOK)
		if findByID(keywordRows, firstReturnID) == nil {
			t.Fatalf("keyword filter did not return purchase return %s", firstReturnCode)
		}

		receipts := listItems(admin, "/purchase/receipts?order_id="+partialOrderID+"&page_size=200", http.StatusOK)
		receiptRow := findByID(receipts, partialReceiptID)
		if receiptRow == nil {
			t.Fatalf("purchase receipt %s not found in receipt list", partialReceiptID)
		}
		receiptLines, _ := receiptRow["lines"].([]any)
		if len(receiptLines) != 1 {
			t.Fatalf("purchase receipt lines = %d, want 1", len(receiptLines))
		}
		receiptLine, ok := receiptLines[0].(map[string]any)
		if !ok {
			t.Fatalf("purchase receipt line payload = %v", receiptLines[0])
		}
		if got := text(receiptLine, "id"); got != partialReceiptLineID {
			t.Fatalf("purchase receipt line id = %s, want %s", got, partialReceiptLineID)
		}
		if got := number(t, receiptLine, "returned_qty"); !nearlyEqual(got, 2) {
			t.Fatalf("purchase receipt returned_qty = %v, want 2", got)
		}
		if got := number(t, receiptLine, "returnable_qty"); !nearlyEqual(got, 0) {
			t.Fatalf("purchase receipt returnable_qty = %v, want 0", got)
		}
	})

	t.Run("credit note type filter", func(t *testing.T) {
		purchaseNotes := listItems(admin, "/finance/credit-notes?credit_type=purchase_return&page_size=200", http.StatusOK)
		if len(purchaseNotes) == 0 {
			t.Fatal("no purchase_return credit notes returned")
		}
		for _, note := range purchaseNotes {
			if got := text(note, "credit_type"); got != "purchase_return" {
				t.Fatalf("credit note type = %s, want purchase_return", got)
			}
		}
		salesNotes := listItems(admin, "/finance/credit-notes?credit_type=sales_return&page_size=200", http.StatusOK)
		for _, note := range salesNotes {
			if got := text(note, "credit_type"); got != "sales_return" {
				t.Fatalf("credit note type = %s, want sales_return", got)
			}
		}
		admin.expectError(http.MethodGet, "/finance/credit-notes?credit_type=bogus", nil, http.StatusBadRequest, "BAD_REQUEST")

		note := admin.mustGet("/finance/credit-notes/"+firstCreditNoteID, http.StatusOK)
		if got := text(note, "credit_type"); got != "purchase_return" {
			t.Fatalf("purchase return credit note type = %s, want purchase_return", got)
		}
		if got := text(note, "source_type"); got != "purchase_return" {
			t.Fatalf("purchase return credit note source_type = %s, want purchase_return", got)
		}
		if got := text(note, "source_id"); got != firstReturnID {
			t.Fatalf("purchase return credit note source_id = %s, want %s", got, firstReturnID)
		}
	})

	t.Run("rounding balanced partial returns", func(t *testing.T) {
		receiptID, receiptLineID, _, invoiceID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"3", "33.333333", true, "CNY", "", today, today, "stage3i-rounding-return",
		)
		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "net_amount"); got != "100.0000" {
			t.Fatalf("rounding invoice net = %s, want 100.0000", got)
		}
		if got := text(invoice, "total_amount"); got != "100.0000" {
			t.Fatalf("rounding invoice total = %s, want 100.0000", got)
		}

		first := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3i-rounding-1")
		if got := text(first, "net_amount"); got != "33.3333" {
			t.Fatalf("rounding first net = %s, want 33.3333", got)
		}
		if got := text(first, "tax_amount"); got != "0.0000" {
			t.Fatalf("rounding first tax = %s, want 0.0000", got)
		}
		if got := text(first, "total_amount"); got != "33.3333" {
			t.Fatalf("rounding first total = %s, want 33.3333", got)
		}
		second := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3i-rounding-2")
		if got := text(second, "net_amount"); got != "33.3334" {
			t.Fatalf("rounding second net = %s, want 33.3334", got)
		}
		if got := text(second, "tax_amount"); got != "0.0000" {
			t.Fatalf("rounding second tax = %s, want 0.0000", got)
		}
		if got := text(second, "total_amount"); got != "33.3334" {
			t.Fatalf("rounding second total = %s, want 33.3334", got)
		}
		third := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3i-rounding-3")
		if got := text(third, "net_amount"); got != "33.3333" {
			t.Fatalf("rounding third net = %s, want 33.3333", got)
		}
		if got := text(third, "tax_amount"); got != "0.0000" {
			t.Fatalf("rounding third tax = %s, want 0.0000", got)
		}
		if got := text(third, "total_amount"); got != "33.3333" {
			t.Fatalf("rounding third total = %s, want 33.3333", got)
		}

		total := number(t, first, "net_amount") + number(t, second, "net_amount") + number(t, third, "net_amount")
		if !nearlyEqual(total, 100) {
			t.Fatalf("rounding return net total = %v, want 100", total)
		}
		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "credited_amount"); got != "100.0000" {
			t.Fatalf("rounding invoice credited = %s, want 100.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("rounding invoice outstanding = %s, want 0.0000", got)
		}
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("rounding invoice status = %s, want paid", got)
		}

		appliedTotal := 0.0
		remainingTotal := 0.0
		for _, item := range []map[string]any{first, second, third} {
			noteID := text(item, "credit_note_id")
			if noteID == "" {
				t.Fatal("rounding return credit_note_id is empty")
			}
			note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
			appliedTotal += number(t, note, "applied_amount")
			remainingTotal += number(t, note, "remaining_amount")
			if got := text(note, "status"); got != "applied" {
				t.Fatalf("rounding credit note status = %s, want applied", got)
			}
		}
		if !nearlyEqual(appliedTotal, 100) {
			t.Fatalf("rounding credit notes applied total = %v, want 100", appliedTotal)
		}
		if !nearlyEqual(remainingTotal, 0) {
			t.Fatalf("rounding credit notes remaining total = %v, want 0", remainingTotal)
		}
	})

	t.Run("fully paid payable invoice return leaves open credit", func(t *testing.T) {
		receiptID, receiptLineID, _, invoiceID, lineCost := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat13ID,
			"1", "113.00", true, "CNY", "", today, today, "stage3i-paid-return",
		)
		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "total_amount"); got != "113.0000" {
			t.Fatalf("paid invoice total = %s, want 113.0000", got)
		}
		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "payment",
			"partner_id":   supplierID,
			"payment_date": today,
			"currency":     "CNY",
			"amount":       "113.00",
			"method":       "bank",
			"reference":    "stage3i-paid-return",
		}, http.StatusCreated)
		admin.mustPost("/finance/payments/"+text(payment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": invoiceID, "amount": "113.00"},
			},
		}, http.StatusOK)
		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("paid invoice status = %s, want paid", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("paid invoice outstanding = %s, want 0.0000", got)
		}

		item := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3i-paid-invoice-return")
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
		if got := text(invoice, "paid_amount"); got != "113.0000" {
			t.Fatalf("paid invoice paid amount after return = %s, want 113.0000", got)
		}
		if got := text(invoice, "credited_amount"); got != "0.0000" {
			t.Fatalf("paid invoice credited after return = %s, want 0.0000", got)
		}
		if got := text(invoice, "base_credited_amount"); got != "0.0000" {
			t.Fatalf("paid invoice base credited after return = %s, want 0.0000", got)
		}
		if got := text(invoice, "outstanding_amount"); got != "0.0000" {
			t.Fatalf("paid invoice outstanding after return = %s, want 0.0000", got)
		}
		if got := text(invoice, "status"); got != "paid" {
			t.Fatalf("paid invoice status after return = %s, want paid", got)
		}

		voucherLines := stage3IPurchaseReturnVoucherLines(t, admin, text(item, "id"), text(item, "code"))
		assertVoucherBalanced(t, voucherLines)
		if got := number(t, findLineByAccount(t, voucherLines, "2202"), "debit"); !nearlyEqual(got, 113) {
			t.Fatalf("paid return AP debit = %v, want 113", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "1405"), "credit"); !nearlyEqual(got, 100) {
			t.Fatalf("paid return inventory credit = %v, want 100", got)
		}
		if got := number(t, findLineByAccount(t, voucherLines, "222101"), "credit"); !nearlyEqual(got, 13) {
			t.Fatalf("paid return input VAT credit = %v, want 13", got)
		}
		transactions := stage3IPurchaseReturnTransactions(t, admin, warehouseID, materialID, text(item, "id"))
		if len(transactions) != 1 {
			t.Fatalf("paid return inventory transactions = %d, want 1", len(transactions))
		}
		if got := number(t, transactions[0], "amount"); !nearlyEqual(got, lineCost) {
			t.Fatalf("paid return inventory amount = %v, want %v", got, lineCost)
		}
	})

	t.Run("payment allocation respects credited amount", func(t *testing.T) {
		receiptID, receiptLineID, _, invoiceID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"2", "100.00", true, "CNY", "", today, today, "stage3i-credit-payment",
		)
		item := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3i-credit-payment-return")
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
			"payment_type": "payment",
			"partner_id":   supplierID,
			"payment_date": today,
			"currency":     "CNY",
			"amount":       "101.00",
			"method":       "bank",
			"reference":    "stage3i-credit-payment-over",
		}, http.StatusCreated)
		admin.expectError(http.MethodPost, "/finance/payments/"+text(overPayment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": invoiceID, "amount": "101.00"},
			},
		}, http.StatusBadRequest, "OVER_ALLOCATE")

		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "payment",
			"partner_id":   supplierID,
			"payment_date": today,
			"currency":     "CNY",
			"amount":       "100.00",
			"method":       "bank",
			"reference":    "stage3i-credit-payment",
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

	t.Run("zero value purchase return", func(t *testing.T) {
		receiptID, receiptLineID, _, invoiceID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"1", "0.00", true, "CNY", "", today, today, "stage3i-zero-value-return",
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

		item := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3i-zero-value-return-post")
		for _, key := range []string{"net_amount", "tax_amount", "total_amount", "base_net_amount", "base_tax_amount", "base_total_amount", "cost_amount"} {
			if got := text(item, key); got != "0.0000" {
				t.Fatalf("zero value return %s = %s, want 0.0000", key, got)
			}
		}
		if got := text(item, "credit_note_id"); got != "" {
			t.Fatalf("zero value return credit_note_id = %s, want empty", got)
		}
		if got := text(item, "credit_note_code"); got != "" {
			t.Fatalf("zero value return credit_note_code = %s, want empty", got)
		}
		if got := text(item, "voucher_code"); got != "" {
			t.Fatalf("zero value return voucher_code = %s, want empty", got)
		}

		after := inventoryBalance(t, admin, warehouseID, materialID)
		if got := number(t, after, "on_hand"); !nearlyEqual(got, baselineOnHand-1) {
			t.Fatalf("on_hand after zero value return = %v, want %v", got, baselineOnHand-1)
		}
		if voucher := stage3IFindPurchaseReturnVoucher(admin, text(item, "id"), text(item, "code")); voucher != nil {
			t.Fatalf("zero value purchase return should not post a voucher: %v", voucher)
		}
	})

	t.Run("over-return insufficient stock and validation", func(t *testing.T) {
		balance := stage3IBalanceMaybe(t, admin, warehouseID, emptyMaterialID)
		if balance != nil && number(t, balance, "on_hand") > 0 {
			createShippedOrder(
				t, admin, customerID, warehouseID, emptyMaterialID, vat0ID,
				text(balance, "on_hand"), "10.00", true, "stage3i-consume-empty-stock",
			)
			balance = stage3IBalanceMaybe(t, admin, warehouseID, emptyMaterialID)
			if balance != nil {
				if got := number(t, balance, "on_hand"); !nearlyEqual(got, 0) {
					t.Fatalf("on_hand after consuming empty material stock = %v, want 0", got)
				}
			}
		}

		receiptID, receiptLineID, _, _, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, emptyMaterialID, vat0ID,
			"5", "10.00", true, "CNY", "", today, today, "stage3i-insufficient-stock",
		)
		balance = stage3IBalanceMaybe(t, admin, warehouseID, emptyMaterialID)
		if balance == nil {
			t.Fatal("inventory balance missing after purchasing empty material")
		}
		if got := number(t, balance, "on_hand"); !nearlyEqual(got, 5) {
			t.Fatalf("on_hand after purchasing empty material = %v, want 5", got)
		}
		createShippedOrder(
			t, admin, customerID, warehouseID, emptyMaterialID, vat0ID,
			"3", "10.00", true, "stage3i-ship-empty-stock",
		)
		balance = stage3IBalanceMaybe(t, admin, warehouseID, emptyMaterialID)
		if balance == nil {
			t.Fatal("inventory balance missing after shipping empty material")
		}
		if got := number(t, balance, "on_hand"); !nearlyEqual(got, 2) {
			t.Fatalf("on_hand after shipping empty material = %v, want 2", got)
		}
		admin.expectError(http.MethodPost, "/purchase/receipts/"+receiptID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"receipt_line_id": receiptLineID, "quantity": "5"},
			},
		}, http.StatusConflict, "INSUFFICIENT_STOCK")

		overReceiptID, overReceiptLineID, _, _, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"5", "10.00", true, "CNY", "", today, today, "stage3i-over-return",
		)
		admin.expectError(http.MethodPost, "/purchase/receipts/"+overReceiptID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"receipt_line_id": overReceiptLineID, "quantity": "6"},
			},
		}, http.StatusConflict, "OVER_RETURN")

		freshReceiptID, freshReceiptLineID, _, _, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"1", "10.00", true, "CNY", "", today, today, "stage3i-validation",
		)
		validLine := []map[string]any{{"receipt_line_id": freshReceiptLineID, "quantity": "1"}}
		admin.expectError(http.MethodPost, "/purchase/receipts/not-a-uuid/returns", map[string]any{
			"return_date": today,
			"lines":       validLine,
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/purchase/receipts/"+freshReceiptID+"/returns", map[string]any{
			"return_date": today,
			"lines":       []map[string]any{},
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/purchase/receipts/"+freshReceiptID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"receipt_line_id": freshReceiptLineID, "quantity": "0"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/purchase/receipts/"+freshReceiptID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"receipt_line_id": freshReceiptLineID, "quantity": "-1"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/purchase/receipts/"+freshReceiptID+"/returns", map[string]any{
			"return_date": "bad-date",
			"lines": []map[string]any{
				{"receipt_line_id": freshReceiptLineID, "quantity": "1"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
		yesterday := "2000-01-01"
		admin.expectError(http.MethodPost, "/purchase/receipts/"+freshReceiptID+"/returns", map[string]any{
			"return_date": yesterday,
			"lines": []map[string]any{
				{"receipt_line_id": freshReceiptLineID, "quantity": "1"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/purchase/receipts/"+freshReceiptID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"receipt_line_id": "00000000-0000-0000-0000-000000000000", "quantity": "1"},
			},
		}, http.StatusNotFound, "NOT_FOUND")
		admin.expectError(http.MethodPost, "/purchase/receipts/"+freshReceiptID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"receipt_line_id": partialReceiptLineID, "quantity": "1"},
			},
		}, http.StatusNotFound, "NOT_FOUND")

		admin.expectError(http.MethodGet, "/purchase/returns/00000000-0000-0000-0000-000000000000", nil, http.StatusNotFound, "NOT_FOUND")
		admin.expectError(http.MethodGet, "/purchase/returns/not-a-uuid", nil, http.StatusBadRequest, "BAD_REQUEST")
	})

	t.Run("permissions and validation", func(t *testing.T) {
		unauthorized := newClient(t)
		unauthorized.expectError(http.MethodPost, "/purchase/receipts/"+partialReceiptID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"receipt_line_id": partialReceiptLineID, "quantity": "1"},
			},
		}, http.StatusUnauthorized, "UNAUTHORIZED")
		unauthorized.expectError(http.MethodGet, "/purchase/returns", nil, http.StatusUnauthorized, "UNAUTHORIZED")

		dept := newClient(t).login("demo_dept", env("TEST_DEMO_PASSWORD", "Demo@123456"))
		dept.expectError(http.MethodPost, "/purchase/receipts/"+partialReceiptID+"/returns", map[string]any{
			"return_date": today,
			"lines": []map[string]any{
				{"receipt_line_id": partialReceiptLineID, "quantity": "1"},
			},
		}, http.StatusForbidden, "FORBIDDEN")

		status, payload := dept.request(http.MethodGet, "/purchase/returns?page_size=200", nil, dept.token)
		switch status {
		case http.StatusForbidden:
			if payload.Code != "FORBIDDEN" {
				t.Fatalf("demo_dept purchase returns code = %q, want FORBIDDEN", payload.Code)
			}
		case http.StatusOK:
			rawItems, _ := payload.Data["items"].([]any)
			for _, raw := range rawItems {
				row, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				if text(row, "id") == firstReturnID || text(row, "id") == secondReturnID {
					t.Fatalf("demo_dept can see purchase return %s outside its scope", text(row, "id"))
				}
			}
		default:
			t.Fatalf("demo_dept purchase returns status = %d, want 200 or 403, payload = %v", status, payload)
		}
	})

	t.Run("foreign currency purchase return and period revaluation", func(t *testing.T) {
		year := cleanRevaluationYear(t, admin)
		ensureTestYear(t, admin, year)
		orderDate := fmt.Sprintf("%04d-01-15", year)
		returnDate := fmt.Sprintf("%04d-01-20", year)
		closeDate := fmt.Sprintf("%04d-01-31", year)

		receiptID, receiptLineID, _, invoiceID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"2", "100.00", true, "USD", "7.2", orderDate, orderDate, "stage3i-fx-revaluation-return",
		)
		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoice, "total_amount"); got != "200.0000" {
			t.Fatalf("FX revaluation invoice total = %s, want 200.0000", got)
		}
		if got := text(invoice, "base_total_amount"); got != "1440.0000" {
			t.Fatalf("FX revaluation invoice base total = %s, want 1440.0000", got)
		}

		item := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", returnDate, "supplier return", "stage3i-fx-revaluation-return-post")
		if got := text(item, "currency"); got != "USD" {
			t.Fatalf("FX revaluation return currency = %s, want USD", got)
		}
		if got := text(item, "net_amount"); got != "100.0000" {
			t.Fatalf("FX revaluation return net = %s, want 100.0000", got)
		}
		if got := text(item, "base_net_amount"); got != "720.0000" {
			t.Fatalf("FX revaluation return base net = %s, want 720.0000", got)
		}
		if got := text(item, "total_amount"); got != "100.0000" {
			t.Fatalf("FX revaluation return total = %s, want 100.0000", got)
		}
		if got := text(item, "base_total_amount"); got != "720.0000" {
			t.Fatalf("FX revaluation return base total = %s, want 720.0000", got)
		}
		if got := text(item, "credit_note_applied_amount"); got != "100.0000" {
			t.Fatalf("FX revaluation credit note applied = %s, want 100.0000", got)
		}
		if got := text(item, "credit_note_remaining_amount"); got != "0.0000" {
			t.Fatalf("FX revaluation credit note remaining = %s, want 0.0000", got)
		}
		note := admin.mustGet("/finance/credit-notes/"+text(item, "credit_note_id"), http.StatusOK)
		if got := text(note, "applied_base_amount"); got != "720.0000" {
			t.Fatalf("FX revaluation credit note applied base = %s, want 720.0000", got)
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
		if got := text(invoice, "status"); got != "partially_paid" {
			t.Fatalf("FX revaluation invoice status = %s, want partially_paid", got)
		}
		baseOutstanding := number(t, invoice, "base_total_amount") - number(t, invoice, "base_credited_amount")
		if !nearlyEqual(baseOutstanding, 720) {
			t.Fatalf("FX revaluation base outstanding = %v, want 720", baseOutstanding)
		}

		admin.mustPost("/finance/exchange-rates", map[string]any{
			"currency_code": "USD",
			"rate_date":     closeDate,
			"rate":          "7.3",
		}, http.StatusCreated)

		yearPeriods := getArray(admin, periodsPath(year), http.StatusOK)
		var january map[string]any
		for _, period := range yearPeriods {
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
		revaluations := listItems(admin,
			"/finance/vouchers?voucher_type=fx_revaluation&start_date="+janStart+"&end_date="+closeDate+"&page_size=50",
			http.StatusOK,
		)
		revaluation := findItem(revaluations, "source_code", fmt.Sprintf("%04d-01", year))
		if revaluation == nil {
			t.Fatalf("FX revaluation voucher for %04d-01 not found", year)
		}
		revaluationDetail := admin.mustGet("/finance/vouchers/"+text(revaluation, "id"), http.StatusOK)
		revaluationLines, _ := revaluationDetail["lines"].([]any)
		assertVoucherBalanced(t, revaluationLines)
		if got := number(t, findLineByAccount(t, revaluationLines, "2202"), "credit"); !nearlyEqual(got, 10) {
			t.Fatalf("FX revaluation AP credit = %v, want 10", got)
		}
		if got := number(t, findLineByAccount(t, revaluationLines, "660301"), "debit"); !nearlyEqual(got, 10) {
			t.Fatalf("FX revaluation loss debit = %v, want 10", got)
		}
		revalued := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(revalued, "base_revalued_amount"); got != "10.0000" {
			t.Fatalf("FX revaluation invoice base revalued = %s, want 10.0000", got)
		}
	})
}

func stage3ICreatePurchaseReceipt(
	t *testing.T,
	admin *client,
	supplierID, warehouseID, materialID, taxID, quantity, unitPrice string,
	priceIncludesTax bool,
	currency, exchangeRate, orderDate, receiptDate, remark string,
) (string, string, string, string, float64) {
	t.Helper()
	orderBody := map[string]any{
		"supplier_id":        supplierID,
		"warehouse_id":       warehouseID,
		"order_date":         orderDate,
		"currency":           currency,
		"price_includes_tax": priceIncludesTax,
		"remark":             remark,
		"lines": []map[string]any{
			{"material_id": materialID, "quantity": quantity, "unit_price": unitPrice, "tax_rate_id": taxID},
		},
	}
	if exchangeRate != "" {
		orderBody["exchange_rate"] = exchangeRate
	}
	order := admin.mustPost("/purchase/orders", orderBody, http.StatusCreated)
	orderID := text(order, "id")
	admin.mustPost("/purchase/orders/"+orderID+"/submit", nil, http.StatusOK)
	admin.mustPost("/purchase/orders/"+orderID+"/approve", nil, http.StatusOK)
	detail := admin.mustGet("/purchase/orders/"+orderID, http.StatusOK)
	lines, _ := detail["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("purchase order %s lines = %d, want 1", orderID, len(lines))
	}
	orderLineID := text(lines[0].(map[string]any), "id")

	receiveBody := map[string]any{
		"receipt_date": receiptDate,
		"remark":       remark + "-receive",
		"lines": []map[string]any{
			{"order_line_id": orderLineID, "quantity": quantity},
		},
	}
	if exchangeRate != "" {
		receiveBody["exchange_rate"] = exchangeRate
	}
	receipt := admin.mustPost("/purchase/orders/"+orderID+"/receive", receiveBody, http.StatusCreated)
	receiptID := text(receipt, "id")
	invoiceID := text(receipt, "payable_id")
	if receiptID == "" || invoiceID == "" || text(receipt, "payable_code") == "" {
		t.Fatalf("purchase receipt %s missing payable id/code: %v", receiptID, receipt)
	}
	receiptLines, _ := receipt["lines"].([]any)
	if len(receiptLines) != 1 {
		t.Fatalf("purchase receipt %s lines = %d, want 1", receiptID, len(receiptLines))
	}
	line, ok := receiptLines[0].(map[string]any)
	if !ok {
		t.Fatalf("purchase receipt line payload = %v", receiptLines[0])
	}
	lineID := text(line, "id")
	quantityValue := number(t, line, "quantity")
	if quantityValue == 0 {
		t.Fatalf("purchase receipt line quantity = 0")
	}
	return receiptID, lineID, orderID, invoiceID, number(t, line, "base_amount") / quantityValue
}

func stage3IPostPurchaseReturn(t *testing.T, admin *client, receiptID, receiptLineID, quantity, date, reason, remark string) map[string]any {
	t.Helper()
	return admin.mustPost("/purchase/receipts/"+receiptID+"/returns", map[string]any{
		"return_date": date,
		"reason":      reason,
		"remark":      remark,
		"lines": []map[string]any{
			{"receipt_line_id": receiptLineID, "quantity": quantity},
		},
	}, http.StatusCreated)
}

func stage3IReceiptLineByID(t *testing.T, admin *client, orderID, receiptID, lineID string) map[string]any {
	t.Helper()
	receipts := listItems(admin, "/purchase/receipts?order_id="+orderID+"&page_size=200", http.StatusOK)
	for _, receipt := range receipts {
		if text(receipt, "id") != receiptID {
			continue
		}
		lines, _ := receipt["lines"].([]any)
		for _, raw := range lines {
			line, ok := raw.(map[string]any)
			if ok && text(line, "id") == lineID {
				return line
			}
		}
	}
	t.Fatalf("purchase receipt line %s for receipt %s not found", lineID, receiptID)
	return nil
}

func stage3IPurchaseReturnTransactions(t *testing.T, admin *client, warehouseID, materialID, returnID string) []map[string]any {
	t.Helper()
	items := listItems(admin,
		"/inventory/transactions?warehouse_id="+warehouseID+"&material_id="+materialID+"&page_size=200",
		http.StatusOK,
	)
	matches := make([]map[string]any, 0, 1)
	for _, item := range items {
		if text(item, "biz_id") == returnID && text(item, "biz_type") == "purchase_return" {
			matches = append(matches, item)
		}
	}
	return matches
}

func stage3IFindPurchaseReturnVoucher(admin *client, returnID, returnCode string) map[string]any {
	voucher := findVoucherBySource(admin, returnCode)
	if voucher == nil {
		return nil
	}
	if text(voucher, "source_id") != returnID {
		return nil
	}
	return voucher
}

func stage3IPurchaseReturnVoucherLines(t *testing.T, admin *client, returnID, returnCode string) []any {
	t.Helper()
	voucher := stage3IFindPurchaseReturnVoucher(admin, returnID, returnCode)
	if voucher == nil {
		t.Fatalf("purchase return voucher for %s not found", returnCode)
	}
	if got := text(voucher, "voucher_type"); got != "purchase_return" {
		t.Fatalf("purchase return voucher type = %s, want purchase_return", got)
	}
	detail := admin.mustGet("/finance/vouchers/"+text(voucher, "id"), http.StatusOK)
	lines, _ := detail["lines"].([]any)
	return lines
}

func stage3IBalanceMaybe(t *testing.T, admin *client, warehouseID, materialID string) map[string]any {
	t.Helper()
	data := admin.mustGet("/inventory/balances?warehouse_id="+warehouseID+"&material_id="+materialID, http.StatusOK)
	items, _ := data["items"].([]any)
	if len(items) == 0 {
		return nil
	}
	balance, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("inventory balance payload = %v", items[0])
	}
	return balance
}
