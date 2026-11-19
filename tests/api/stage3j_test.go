//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

type stage3JAllocation struct {
	invoiceID string
	amount    string
}

type stage3JRequest struct {
	path string
	body any
}

type stage3JHTTPResult struct {
	status int
	code   string
	err    error
}

var stage3JHTTPClient = &http.Client{Timeout: 30 * time.Second}

func TestStage3J(t *testing.T) {
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
	customer2 := findItem(listItems(admin, "/masterdata/partners?keyword=CUS-DEMO-002&page_size=20", http.StatusOK), "code", "CUS-DEMO-002")
	supplier := findItem(listItems(admin, "/masterdata/partners?keyword=SUP-DEMO-001&page_size=20", http.StatusOK), "code", "SUP-DEMO-001")
	warehouse := findItem(listItems(admin, "/masterdata/warehouses?keyword=WH-SH&page_size=20", http.StatusOK), "code", "WH-SH")
	eastWarehouse := findItem(listItems(admin, "/masterdata/warehouses?keyword=WH-EAST&page_size=20", http.StatusOK), "code", "WH-EAST")
	material := findItem(listItems(admin, "/masterdata/materials?keyword=MAT-DEMO-004&page_size=20", http.StatusOK), "code", "MAT-DEMO-004")
	eastMaterial := findItem(listItems(admin, "/masterdata/materials?keyword=MAT-DEMO-011&page_size=20", http.StatusOK), "code", "MAT-DEMO-011")
	vat0 := findItem(listItems(admin, "/masterdata/tax-rates?keyword=VAT0&page_size=20", http.StatusOK), "code", "VAT0")
	if customer == nil || customer2 == nil || supplier == nil || warehouse == nil || eastWarehouse == nil ||
		material == nil || eastMaterial == nil || vat0 == nil {
		t.Fatal("demo master data missing")
	}
	customerID := text(customer, "id")
	customer2ID := text(customer2, "id")
	supplierID := text(supplier, "id")
	warehouseID := text(warehouse, "id")
	eastWarehouseID := text(eastWarehouse, "id")
	materialID := text(material, "id")
	eastMaterialID := text(eastMaterial, "id")
	vat0ID := text(vat0, "id")

	ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)

	// Track credit notes created by this test so the demo_dept data-scope check
	// cannot silently miss a note created by an earlier subtest.
	var stage3JNoteIDs []string
	trackNote := func(id string) string {
		if id != "" {
			stage3JNoteIDs = append(stage3JNoteIDs, id)
		}
		return id
	}

	t.Run("sales credit applies across receivable invoices", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)

		_, shipmentAID, shipmentALineID, invoiceAID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3j-apply-invoice-a",
		)
		stage3JPayInvoice(t, admin, "receipt", customerID, invoiceAID, "CNY", "100.00", "", today, "stage3j-apply-a-payment")
		returned := postReturn(t, admin, shipmentAID, shipmentALineID, "1", today, "stage3j-apply-return")
		noteID := text(returned, "credit_note_id")
		noteCode := text(returned, "credit_note_code")
		trackNote(noteID)
		if noteID == "" || noteCode == "" {
			t.Fatalf("credit note id/code missing: %v", returned)
		}

		note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, note, "amount", "100.0000")
		stage3JExpect(t, note, "base_amount", "100.0000")
		stage3JExpect(t, note, "applied_amount", "0.0000")
		stage3JExpect(t, note, "refunded_amount", "0.0000")
		stage3JExpect(t, note, "remaining_amount", "100.0000")
		stage3JExpect(t, note, "status", "open")
		if allocations := stage3JObjects(t, note, "allocations"); len(allocations) != 0 {
			t.Fatalf("credit note allocations after paid-invoice return = %d, want 0", len(allocations))
		}
		invoiceA := admin.mustGet("/finance/invoices/"+invoiceAID, http.StatusOK)
		stage3JExpect(t, invoiceA, "status", "paid")
		stage3JExpect(t, invoiceA, "credited_amount", "0.0000")

		_, _, _, invoiceBID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "50.00", true, "stage3j-apply-invoice-b",
		)
		_, _, _, invoiceCID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "50.00", true, "stage3j-apply-invoice-c",
		)
		invoiceB := admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)

		applied := stage3JAllocate(t, admin, noteID, today, stage3JAllocation{invoiceBID, "30.00"})
		stage3JExpect(t, applied, "applied_amount", "30.0000")
		stage3JExpect(t, applied, "remaining_amount", "70.0000")
		stage3JExpect(t, applied, "status", "open")
		allocations := stage3JObjects(t, applied, "allocations")
		if len(allocations) != 1 {
			t.Fatalf("allocations after first application = %d, want 1", len(allocations))
		}
		stage3JExpect(t, allocations[0], "invoice_id", invoiceBID)
		stage3JExpect(t, allocations[0], "invoice_code", text(invoiceB, "code"))
		stage3JExpect(t, allocations[0], "amount", "30.0000")
		stage3JExpect(t, allocations[0], "base_amount", "30.0000")
		stage3JExpect(t, allocations[0], "allocation_date", today)
		invoiceB = admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
		stage3JExpect(t, invoiceB, "credited_amount", "30.0000")
		stage3JExpect(t, invoiceB, "outstanding_amount", "20.0000")
		stage3JExpect(t, invoiceB, "status", "partially_paid")

		applied = stage3JAllocate(t, admin, noteID, today, stage3JAllocation{invoiceCID, "50.00"})
		stage3JExpect(t, applied, "applied_amount", "80.0000")
		stage3JExpect(t, applied, "remaining_amount", "20.0000")
		if got := len(stage3JObjects(t, applied, "allocations")); got != 2 {
			t.Fatalf("allocations after second application = %d, want 2", got)
		}
		invoiceC := admin.mustGet("/finance/invoices/"+invoiceCID, http.StatusOK)
		stage3JExpect(t, invoiceC, "credited_amount", "50.0000")
		stage3JExpect(t, invoiceC, "outstanding_amount", "0.0000")
		stage3JExpect(t, invoiceC, "status", "paid")

		applied = stage3JAllocate(t, admin, noteID, today, stage3JAllocation{invoiceBID, "20.00"})
		stage3JExpect(t, applied, "applied_amount", "100.0000")
		stage3JExpect(t, applied, "applied_base_amount", "100.0000")
		stage3JExpect(t, applied, "remaining_amount", "0.0000")
		stage3JExpect(t, applied, "remaining_base_amount", "0.0000")
		stage3JExpect(t, applied, "status", "applied")
		if got := len(stage3JObjects(t, applied, "allocations")); got != 3 {
			t.Fatalf("allocations after third application = %d, want 3", got)
		}
		invoiceB = admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
		stage3JExpect(t, invoiceB, "credited_amount", "50.0000")
		stage3JExpect(t, invoiceB, "outstanding_amount", "0.0000")
		stage3JExpect(t, invoiceB, "status", "paid")

		detail := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		allocations = stage3JObjects(t, detail, "allocations")
		if len(allocations) != 3 {
			t.Fatalf("credit note allocation history = %d, want 3", len(allocations))
		}
		bTotal := 0.0
		cTotal := 0.0
		for _, allocation := range allocations {
			if got := text(allocation, "allocation_date"); got != today {
				t.Fatalf("allocation date = %s, want %s", got, today)
			}
			if got, base := number(t, allocation, "amount"), number(t, allocation, "base_amount"); !nearlyEqual(got, base) {
				t.Fatalf("allocation amount/base mismatch: %v", allocation)
			}
			if text(allocation, "invoice_code") == "" {
				t.Fatalf("allocation invoice_code is empty: %v", allocation)
			}
			switch text(allocation, "invoice_id") {
			case invoiceBID:
				bTotal += number(t, allocation, "amount")
			case invoiceCID:
				cTotal += number(t, allocation, "amount")
			default:
				t.Fatalf("unexpected allocation invoice_id = %s", text(allocation, "invoice_id"))
			}
		}
		if !nearlyEqual(bTotal, 50) || !nearlyEqual(cTotal, 50) {
			t.Fatalf("allocation totals by invoice = %v/%v, want 50/50", bTotal, cTotal)
		}
		if refunds := stage3JObjects(t, detail, "refunds"); len(refunds) != 0 {
			t.Fatalf("credit note refunds = %d, want 0", len(refunds))
		}

		rows := listItems(admin, "/finance/credit-notes?partner_id="+customerID+"&page_size=200", http.StatusOK)
		row := findByID(rows, noteID)
		if row == nil {
			t.Fatalf("credit note %s not found in list", noteID)
		}
		for _, key := range []string{
			"amount", "base_amount", "applied_amount", "applied_base_amount",
			"refunded_amount", "refunded_base_amount", "remaining_amount",
			"remaining_base_amount", "status", "credit_type", "partner_id",
		} {
			if text(row, key) != text(detail, key) {
				t.Fatalf("credit note list %s = %q, detail = %q", key, text(row, key), text(detail, key))
			}
		}
		if got := text(row, "code"); got != noteCode {
			t.Fatalf("credit note list code = %s, want %s", got, noteCode)
		}
		// Allocation is a subledger operation: no voucher should use the credit
		// note code as its source.
		if voucher := findVoucherBySource(admin, noteCode); voucher != nil {
			t.Fatalf("credit note allocation unexpectedly posted voucher %s", text(voucher, "code"))
		}
	})

	t.Run("purchase credit applies to payable invoice", func(t *testing.T) {
		receiptAID, receiptALineID, _, invoiceAID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "CNY", "", today, today, "stage3j-purchase-invoice-a",
		)
		stage3JPayInvoice(t, admin, "payment", supplierID, invoiceAID, "CNY", "100.00", "", today, "stage3j-purchase-a-payment")
		returned := stage3IPostPurchaseReturn(t, admin, receiptAID, receiptALineID, "1", today, "supplier return", "stage3j-purchase-return")
		noteID := text(returned, "credit_note_id")
		trackNote(noteID)
		if noteID == "" {
			t.Fatalf("purchase credit note id missing: %v", returned)
		}
		note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, note, "credit_type", "purchase_return")
		stage3JExpect(t, note, "amount", "100.0000")
		stage3JExpect(t, note, "applied_amount", "0.0000")
		stage3JExpect(t, note, "remaining_amount", "100.0000")
		stage3JExpect(t, note, "status", "open")

		_, _, _, invoiceBID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"1", "50.00", true, "CNY", "", today, today, "stage3j-purchase-invoice-b",
		)
		applied := stage3JAllocate(t, admin, noteID, today, stage3JAllocation{invoiceBID, "50.00"})
		stage3JExpect(t, applied, "applied_amount", "50.0000")
		stage3JExpect(t, applied, "remaining_amount", "50.0000")
		stage3JExpect(t, applied, "status", "open")
		invoiceB := admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
		stage3JExpect(t, invoiceB, "credited_amount", "50.0000")
		stage3JExpect(t, invoiceB, "outstanding_amount", "0.0000")
		stage3JExpect(t, invoiceB, "status", "paid")
	})

	t.Run("initial application allocation history", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, shipmentID, shipmentLineID, salesInvoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"2", "100.00", true, "stage3j-initial-sales",
		)
		salesReturn := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3j-initial-sales-return")
		stage3JExpect(t, salesReturn, "credit_note_applied_amount", "100.0000")
		stage3JExpect(t, salesReturn, "credit_note_status", "applied")
		salesNoteID := trackNote(text(salesReturn, "credit_note_id"))
		salesNote := admin.mustGet("/finance/credit-notes/"+salesNoteID, http.StatusOK)
		stage3JExpect(t, salesNote, "applied_amount", "100.0000")
		stage3JExpect(t, salesNote, "applied_base_amount", "100.0000")
		salesAllocations := stage3JObjects(t, salesNote, "allocations")
		if len(salesAllocations) != 1 {
			t.Fatalf("initial sales allocation rows = %d, want 1", len(salesAllocations))
		}
		stage3JExpect(t, salesAllocations[0], "invoice_id", salesInvoiceID)
		stage3JExpect(t, salesAllocations[0], "credit_note_id", salesNoteID)
		stage3JExpect(t, salesAllocations[0], "amount", "100.0000")
		stage3JExpect(t, salesAllocations[0], "base_amount", "100.0000")
		stage3JExpect(t, salesAllocations[0], "allocation_date", today)

		receiptID, receiptLineID, _, purchaseInvoiceID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"2", "100.00", true, "CNY", "", today, today, "stage3j-initial-purchase",
		)
		purchaseReturn := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3j-initial-purchase-return")
		stage3JExpect(t, purchaseReturn, "credit_note_applied_amount", "100.0000")
		purchaseNoteID := trackNote(text(purchaseReturn, "credit_note_id"))
		purchaseNote := admin.mustGet("/finance/credit-notes/"+purchaseNoteID, http.StatusOK)
		purchaseAllocations := stage3JObjects(t, purchaseNote, "allocations")
		if len(purchaseAllocations) != 1 {
			t.Fatalf("initial purchase allocation rows = %d, want 1", len(purchaseAllocations))
		}
		stage3JExpect(t, purchaseAllocations[0], "invoice_id", purchaseInvoiceID)
		stage3JExpect(t, purchaseAllocations[0], "amount", "100.0000")
		stage3JExpect(t, purchaseAllocations[0], "base_amount", "100.0000")
		stage3JExpect(t, purchaseAllocations[0], "allocation_date", today)

		_, paidShipmentID, paidShipmentLineID, paidInvoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3j-initial-paid",
		)
		stage3JPayInvoice(t, admin, "receipt", customerID, paidInvoiceID, "CNY", "100.00", "", today, "stage3j-initial-paid-payment")
		paidReturn := postReturn(t, admin, paidShipmentID, paidShipmentLineID, "1", today, "stage3j-initial-paid-return")
		stage3JExpect(t, paidReturn, "credit_note_applied_amount", "0.0000")
		paidNoteID := trackNote(text(paidReturn, "credit_note_id"))
		paidNote := admin.mustGet("/finance/credit-notes/"+paidNoteID, http.StatusOK)
		if allocations := stage3JObjects(t, paidNote, "allocations"); len(allocations) != 0 {
			t.Fatalf("paid-invoice return allocations = %d, want 0", len(allocations))
		}
	})

	t.Run("allocation validation", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		ensureStage3HStock(t, admin, supplierID, eastWarehouseID, eastMaterialID, today)
		_, shipmentID, shipmentLineID, paidInvoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3j-validation-paid",
		)
		stage3JPayInvoice(t, admin, "receipt", customerID, paidInvoiceID, "CNY", "100.00", "", today, "stage3j-validation-paid-payment")
		returned := postReturn(t, admin, shipmentID, shipmentLineID, "1", today, "stage3j-validation-note")
		noteID := text(returned, "credit_note_id")
		trackNote(noteID)
		if noteID == "" {
			t.Fatalf("validation credit note id missing: %v", returned)
		}
		note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, note, "remaining_amount", "100.0000")

		_, _, _, invoiceBID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "50.00", true, "stage3j-validation-b",
		)
		_, _, _, invoiceCID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "200.00", true, "stage3j-validation-c",
		)
		_, _, _, otherPartnerInvoiceID, _ := createShippedOrder(
			t, admin, customer2ID, warehouseID, materialID, vat0ID,
			"1", "50.00", true, "stage3j-validation-other-partner",
		)
		_, _, _, usdInvoiceID := stage3JCreateShippedInvoice(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "40.00", "USD", "7.2", today, "stage3j-validation-usd",
		)
		_, _, _, payableInvoiceID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"1", "50.00", true, "CNY", "", today, today, "stage3j-validation-payable",
		)
		_, _, _, eastInvoiceID, _ := createShippedOrder(
			t, admin, customerID, eastWarehouseID, eastMaterialID, vat0ID,
			"1", "50.00", true, "stage3j-validation-east",
		)

		path := "/finance/credit-notes/" + noteID + "/allocations"
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, otherPartnerInvoiceID, "10.00"), http.StatusBadRequest, "PARTNER_MISMATCH")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, usdInvoiceID, "10.00"), http.StatusBadRequest, "CURRENCY_MISMATCH")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, payableInvoiceID, "10.00"), http.StatusBadRequest, "INVOICE_TYPE_MISMATCH")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, eastInvoiceID, "10.00"), http.StatusBadRequest, "ORG_MISMATCH")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, paidInvoiceID, "10.00"), http.StatusConflict, "INVOICE_PAID")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, invoiceBID, "51.00"), http.StatusConflict, "OVER_APPLY")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, invoiceCID, "101.00"), http.StatusConflict, "OVER_APPLY")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, invoiceBID, "0"), http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, invoiceBID, "-1"), http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, map[string]any{
			"allocation_date": today,
			"allocations": []map[string]any{
				{"invoice_id": invoiceBID, "amount": "1.00"},
				{"invoice_id": invoiceBID, "amount": "1.00"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody("2000-01-01", invoiceBID, "10.00"), http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody("bad-date", invoiceBID, "10.00"), http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/finance/credit-notes/00000000-0000-0000-0000-000000000002/allocations", stage3JAllocationBody(today, invoiceBID, "10.00"), http.StatusNotFound, "NOT_FOUND")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, "00000000-0000-0000-0000-000000000003", "10.00"), http.StatusNotFound, "NOT_FOUND")

		note = admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, note, "applied_amount", "0.0000")
		stage3JExpect(t, note, "remaining_amount", "100.0000")
		stage3JExpect(t, note, "status", "open")
		if allocations := stage3JObjects(t, note, "allocations"); len(allocations) != 0 {
			t.Fatalf("allocations after rejected requests = %d, want 0", len(allocations))
		}

		applied := stage3JAllocate(t, admin, noteID, today, stage3JAllocation{invoiceCID, "100.00"})
		stage3JExpect(t, applied, "applied_amount", "100.0000")
		stage3JExpect(t, applied, "remaining_amount", "0.0000")
		stage3JExpect(t, applied, "status", "applied")
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, invoiceBID, "1.00"), http.StatusConflict, "CREDIT_NOTE_CLOSED")

		// Guard: payment over-allocation must remain 400 OVER_ALLOCATE while credit-note over-invoice returns 409 OVER_APPLY.
		overPayment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": today,
			"currency":     "CNY",
			"amount":       "60.00",
			"method":       "bank",
			"reference":    "stage3j-validation-over-payment",
		}, http.StatusCreated)
		admin.expectError(http.MethodPost, "/finance/payments/"+text(overPayment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": invoiceBID, "amount": "60.00"},
			},
		}, http.StatusBadRequest, "OVER_ALLOCATE")
	})

	t.Run("allocation candidates", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		ensureStage3HStock(t, admin, supplierID, eastWarehouseID, eastMaterialID, today)
		_, _, _, candidateInvoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "40.00", true, "stage3j-candidates-open",
		)
		_, _, _, paidCandidateID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "40.00", true, "stage3j-candidates-paid-target",
		)
		stage3JPayInvoice(t, admin, "receipt", customerID, paidCandidateID, "CNY", "40.00", "", today, "stage3j-candidates-paid-target-payment")
		_, _, _, otherPartnerID, _ := createShippedOrder(
			t, admin, customer2ID, warehouseID, materialID, vat0ID,
			"1", "40.00", true, "stage3j-candidates-other-partner",
		)
		_, _, _, usdInvoiceID := stage3JCreateShippedInvoice(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "40.00", "USD", "7.2", today, "stage3j-candidates-usd",
		)
		_, _, _, eastInvoiceID, _ := createShippedOrder(
			t, admin, customerID, eastWarehouseID, eastMaterialID, vat0ID,
			"1", "40.00", true, "stage3j-candidates-east",
		)

		_, salesNoteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"50.00", "CNY", "", today, "stage3j-candidates-sales-note",
		)
		trackNote(salesNoteID)

		candidates := getArray(admin, "/finance/credit-notes/"+salesNoteID+"/allocation-candidates", http.StatusOK)
		if findItem(candidates, "id", candidateInvoiceID) == nil {
			t.Fatalf("open same-partner invoice %s missing from candidates", candidateInvoiceID)
		}
		for _, excludedID := range []string{paidCandidateID, otherPartnerID, usdInvoiceID, eastInvoiceID} {
			if findItem(candidates, "id", excludedID) != nil {
				t.Fatalf("invoice %s should not be an allocation candidate", excludedID)
			}
		}
		for _, candidate := range candidates {
			if got := text(candidate, "invoice_type"); got != "receivable" {
				t.Fatalf("candidate invoice_type = %s, want receivable", got)
			}
			if got := text(candidate, "partner_id"); got != customerID {
				t.Fatalf("candidate partner_id = %s, want %s", got, customerID)
			}
			if got := text(candidate, "currency"); got != "CNY" {
				t.Fatalf("candidate currency = %s, want CNY", got)
			}
			if got := text(candidate, "status"); got == "paid" {
				t.Fatalf("candidate invoice %s is paid", text(candidate, "id"))
			}
			if got := number(t, candidate, "outstanding_amount"); got <= 0 {
				t.Fatalf("candidate invoice %s outstanding = %v, want > 0", text(candidate, "id"), got)
			}
		}

		purchaseReceiptID, purchaseLineID, _, purchaseInvoiceID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"1", "50.00", true, "CNY", "", today, today, "stage3j-candidates-purchase-note",
		)
		stage3JPayInvoice(t, admin, "payment", supplierID, purchaseInvoiceID, "CNY", "50.00", "", today, "stage3j-candidates-purchase-note-payment")
		purchaseReturn := stage3IPostPurchaseReturn(t, admin, purchaseReceiptID, purchaseLineID, "1", today, "supplier return", "stage3j-candidates-purchase-note-return")
		purchaseNoteID := text(purchaseReturn, "credit_note_id")
		trackNote(purchaseNoteID)
		_, _, _, payableCandidateID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"1", "40.00", true, "CNY", "", today, today, "stage3j-candidates-purchase-target",
		)
		purchaseCandidates := getArray(admin, "/finance/credit-notes/"+purchaseNoteID+"/allocation-candidates", http.StatusOK)
		if findItem(purchaseCandidates, "id", payableCandidateID) == nil {
			t.Fatalf("open payable invoice %s missing from purchase credit note candidates", payableCandidateID)
		}
		for _, candidate := range purchaseCandidates {
			if got := text(candidate, "invoice_type"); got != "payable" {
				t.Fatalf("purchase candidate invoice_type = %s, want payable", got)
			}
			if got := text(candidate, "partner_id"); got != supplierID {
				t.Fatalf("purchase candidate partner_id = %s, want %s", got, supplierID)
			}
			if got := text(candidate, "currency"); got != "CNY" {
				t.Fatalf("purchase candidate currency = %s, want CNY", got)
			}
			if got := text(candidate, "status"); got == "paid" {
				t.Fatalf("purchase candidate invoice %s is paid", text(candidate, "id"))
			}
			if got := number(t, candidate, "outstanding_amount"); got <= 0 {
				t.Fatalf("purchase candidate invoice %s outstanding = %v, want > 0", text(candidate, "id"), got)
			}
		}
	})

	t.Run("multi invoice allocation in one request", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "CNY", "", today, "stage3j-multi-allocation",
		)
		trackNote(noteID)
		_, _, _, invoiceAID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "40.00", true, "stage3j-multi-allocation-a",
		)
		_, _, _, invoiceBID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "60.00", true, "stage3j-multi-allocation-b",
		)

		applied := stage3JAllocate(t, admin, noteID, today,
			stage3JAllocation{invoiceAID, "30.00"},
			stage3JAllocation{invoiceBID, "20.00"},
		)
		stage3JExpect(t, applied, "applied_amount", "50.0000")
		stage3JExpect(t, applied, "applied_base_amount", "50.0000")
		stage3JExpect(t, applied, "remaining_amount", "50.0000")
		stage3JExpect(t, applied, "remaining_base_amount", "50.0000")
		stage3JExpect(t, applied, "status", "open")
		allocations := stage3JObjects(t, applied, "allocations")
		if len(allocations) != 2 {
			t.Fatalf("multi invoice allocation rows = %d, want 2", len(allocations))
		}
		byInvoice := make(map[string]map[string]any, len(allocations))
		for _, allocation := range allocations {
			byInvoice[text(allocation, "invoice_id")] = allocation
		}
		if len(byInvoice) != 2 {
			t.Fatalf("multi invoice allocation distinct invoices = %d, want 2", len(byInvoice))
		}
		for invoiceID, wantAmount := range map[string]string{invoiceAID: "30.0000", invoiceBID: "20.0000"} {
			allocation := byInvoice[invoiceID]
			if allocation == nil {
				t.Fatalf("multi invoice allocation row for %s missing", invoiceID)
			}
			stage3JExpect(t, allocation, "amount", wantAmount)
			stage3JExpect(t, allocation, "base_amount", wantAmount)
			stage3JExpect(t, allocation, "allocation_date", today)
		}

		invoiceA := admin.mustGet("/finance/invoices/"+invoiceAID, http.StatusOK)
		stage3JExpect(t, invoiceA, "credited_amount", "30.0000")
		stage3JExpect(t, invoiceA, "base_credited_amount", "30.0000")
		stage3JExpect(t, invoiceA, "outstanding_amount", "10.0000")
		stage3JExpect(t, invoiceA, "status", "partially_paid")
		invoiceB := admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
		stage3JExpect(t, invoiceB, "credited_amount", "20.0000")
		stage3JExpect(t, invoiceB, "base_credited_amount", "20.0000")
		stage3JExpect(t, invoiceB, "outstanding_amount", "40.0000")
		stage3JExpect(t, invoiceB, "status", "partially_paid")
	})

	t.Run("allocation rolls back on second line failure", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "CNY", "", today, "stage3j-allocation-rollback",
		)
		trackNote(noteID)
		_, _, _, firstInvoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "50.00", true, "stage3j-allocation-rollback-first",
		)
		_, _, _, secondInvoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "10.00", true, "stage3j-allocation-rollback-second",
		)
		beforeNote := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		beforeFirst := admin.mustGet("/finance/invoices/"+firstInvoiceID, http.StatusOK)
		beforeSecond := admin.mustGet("/finance/invoices/"+secondInvoiceID, http.StatusOK)

		// First line fits, second line exceeds the invoice outstanding amount.
		// The whole transaction must roll back, including the first line.
		admin.expectError(http.MethodPost, "/finance/credit-notes/"+noteID+"/allocations", map[string]any{
			"allocation_date": today,
			"allocations": []map[string]any{
				{"invoice_id": firstInvoiceID, "amount": "30.00"},
				{"invoice_id": secondInvoiceID, "amount": "20.00"},
			},
		}, http.StatusConflict, "OVER_APPLY")

		afterNote := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		for _, key := range []string{"applied_amount", "applied_base_amount", "remaining_amount", "remaining_base_amount", "status"} {
			if text(afterNote, key) != text(beforeNote, key) {
				t.Fatalf("credit note %s after failed multi-line allocation = %q, before = %q", key, text(afterNote, key), text(beforeNote, key))
			}
		}
		if allocations := stage3JObjects(t, afterNote, "allocations"); len(allocations) != 0 {
			t.Fatalf("allocation rows after failed multi-line allocation = %d, want 0", len(allocations))
		}
		for _, invoice := range []struct {
			id     string
			before map[string]any
		}{
			{firstInvoiceID, beforeFirst},
			{secondInvoiceID, beforeSecond},
		} {
			after := admin.mustGet("/finance/invoices/"+invoice.id, http.StatusOK)
			for _, key := range []string{"credited_amount", "base_credited_amount", "outstanding_amount", "status"} {
				if text(after, key) != text(invoice.before, key) {
					t.Fatalf("invoice %s %s after failed multi-line allocation = %q, before = %q", invoice.id, key, text(after, key), text(invoice.before, key))
				}
			}
		}

		// The rollback must leave the note usable for a valid allocation.
		applied := stage3JAllocate(t, admin, noteID, today, stage3JAllocation{firstInvoiceID, "30.00"})
		stage3JExpect(t, applied, "applied_amount", "30.0000")
		stage3JExpect(t, applied, "remaining_amount", "70.0000")
	})

	t.Run("full cross-rate allocation rejected", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "USD", "7.2", today, "stage3j-cross-rate-note",
		)
		trackNote(noteID)
		_, _, _, invoiceID := stage3JCreateShippedInvoice(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", "USD", "7.3", today, "stage3j-cross-rate-invoice",
		)
		path := "/finance/credit-notes/" + noteID + "/allocations"
		admin.expectError(http.MethodPost, path, stage3JAllocationBody(today, invoiceID, "100.00"), http.StatusConflict, "CROSS_RATE_ALLOCATION")

		note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, note, "applied_amount", "0.0000")
		stage3JExpect(t, note, "remaining_amount", "100.0000")
		stage3JExpect(t, note, "status", "open")
		if allocations := stage3JObjects(t, note, "allocations"); len(allocations) != 0 {
			t.Fatalf("cross-rate rejected allocation rows = %d, want 0", len(allocations))
		}
		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3JExpect(t, invoice, "credited_amount", "0.0000")
		stage3JExpect(t, invoice, "base_credited_amount", "0.0000")
		stage3JExpect(t, invoice, "outstanding_amount", "100.0000")
		stage3JExpect(t, invoice, "status", "open")

		// Partial cross-rate allocation stays allowed because the invoice keeps
		// original-currency outstanding for period revaluation.
		applied := stage3JAllocate(t, admin, noteID, today, stage3JAllocation{invoiceID, "50.00"})
		stage3JExpect(t, applied, "applied_amount", "50.0000")
		stage3JExpect(t, applied, "applied_base_amount", "360.0000")
		stage3JExpect(t, applied, "remaining_amount", "50.0000")
		stage3JExpect(t, applied, "remaining_base_amount", "360.0000")
		stage3JExpect(t, applied, "status", "open")
		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3JExpect(t, invoice, "credited_amount", "50.0000")
		stage3JExpect(t, invoice, "base_credited_amount", "360.0000")
		stage3JExpect(t, invoice, "outstanding_amount", "50.0000")
		stage3JExpect(t, invoice, "status", "partially_paid")
	})

	t.Run("refund default rate and invalid zero rate", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "USD", "7.2", today, "stage3j-default-rate-note",
		)
		trackNote(noteID)
		admin.mustPost("/finance/exchange-rates", map[string]any{
			"currency_code": "USD",
			"rate_date":     today,
			"rate":          "7.25",
		}, http.StatusCreated)

		refunded := stage3JRefund(t, admin, noteID, today, "40.00", "", "bank", "stage3j-default-rate-refund")
		stage3JExpect(t, refunded, "refunded_amount", "40.0000")
		stage3JExpect(t, refunded, "refunded_base_amount", "288.0000")
		stage3JExpect(t, refunded, "remaining_amount", "60.0000")
		refunds := stage3JObjects(t, refunded, "refunds")
		if len(refunds) != 1 {
			t.Fatalf("default rate refund rows = %d, want 1", len(refunds))
		}
		refund := refunds[0]
		stage3JExpect(t, refund, "exchange_rate", "7.25000000")
		stage3JExpect(t, refund, "base_amount", "288.0000")
		stage3JExpect(t, refund, "cash_base_amount", "290.0000")
		stage3JExpect(t, refund, "fx_adjustment", "2.0000")

		admin.expectError(http.MethodPost, "/finance/credit-notes/"+noteID+"/refunds", map[string]any{
			"refund_date": today, "amount": "10.00", "method": "bank", "exchange_rate": "0",
		}, http.StatusBadRequest, "INVALID_EXCHANGE_RATE")
		after := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, after, "refunded_amount", "40.0000")
		stage3JExpect(t, after, "remaining_amount", "60.0000")
		if got := len(stage3JObjects(t, after, "refunds")); got != 1 {
			t.Fatalf("refund rows after invalid zero rate = %d, want 1", got)
		}
	})

	t.Run("cash refund uses cash account", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "CNY", "", today, "stage3j-cash-refund",
		)
		trackNote(noteID)
		refunded := stage3JRefund(t, admin, noteID, today, "25.00", "1", "cash", "stage3j-cash-refund-posting")
		refunds := stage3JObjects(t, refunded, "refunds")
		if len(refunds) != 1 {
			t.Fatalf("cash refund rows = %d, want 1", len(refunds))
		}
		refund := refunds[0]
		stage3JExpect(t, refund, "method", "cash")
		stage3JExpect(t, refund, "exchange_rate", "1.00000000")
		stage3JExpect(t, refund, "base_amount", "25.0000")
		stage3JExpect(t, refund, "cash_base_amount", "25.0000")
		stage3JExpect(t, refund, "fx_adjustment", "0.0000")
		lines := stage3JRefundVoucherLines(t, admin, refund)
		assertVoucherBalanced(t, lines)
		if got := number(t, findLineByAccount(t, lines, "1122"), "debit"); !nearlyEqual(got, 25) {
			t.Fatalf("cash refund AR debit = %v, want 25", got)
		}
		if got := number(t, findLineByAccount(t, lines, "1001"), "credit"); !nearlyEqual(got, 25) {
			t.Fatalf("cash refund cash credit = %v, want 25", got)
		}
		if stage3JLookupLineByAccount(lines, "1002") != nil {
			t.Fatal("cash refund voucher unexpectedly uses the bank account")
		}
	})

	t.Run("negative fx refund posts gain", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "USD", "7.2", today, "stage3j-negative-fx-note",
		)
		trackNote(noteID)
		refunded := stage3JRefund(t, admin, noteID, today, "50.00", "7.1", "bank", "stage3j-negative-fx-refund")
		refunds := stage3JObjects(t, refunded, "refunds")
		if len(refunds) != 1 {
			t.Fatalf("negative FX refund rows = %d, want 1", len(refunds))
		}
		refund := refunds[0]
		stage3JExpect(t, refund, "exchange_rate", "7.10000000")
		stage3JExpect(t, refund, "base_amount", "360.0000")
		stage3JExpect(t, refund, "cash_base_amount", "355.0000")
		stage3JExpect(t, refund, "fx_adjustment", "-5.0000")
		lines := stage3JRefundVoucherLines(t, admin, refund)
		assertVoucherBalanced(t, lines)
		if got := number(t, findLineByAccount(t, lines, "1122"), "debit"); !nearlyEqual(got, 360) {
			t.Fatalf("negative FX refund AR debit = %v, want 360", got)
		}
		if got := number(t, findLineByAccount(t, lines, "1002"), "credit"); !nearlyEqual(got, 355) {
			t.Fatalf("negative FX refund bank credit = %v, want 355", got)
		}
		if got := number(t, findLineByAccount(t, lines, "660302"), "credit"); !nearlyEqual(got, 5) {
			t.Fatalf("negative FX refund gain credit = %v, want 5", got)
		}
		if stage3JLookupLineByAccount(lines, "660301") != nil {
			t.Fatal("negative FX refund voucher unexpectedly contains an FX loss line")
		}
	})

	t.Run("customer refund with fx", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "USD", "7.2", today, "stage3j-customer-refund",
		)
		trackNote(noteID)
		note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, note, "currency", "USD")
		stage3JExpect(t, note, "amount", "100.0000")
		stage3JExpect(t, note, "base_amount", "720.0000")
		stage3JExpect(t, note, "applied_amount", "0.0000")
		stage3JExpect(t, note, "remaining_amount", "100.0000")
		stage3JExpect(t, note, "status", "open")

		invoiceID := text(note, "invoice_id")
		invoiceBefore := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		creditedBefore := text(invoiceBefore, "credited_amount")
		baseCreditedBefore := text(invoiceBefore, "base_credited_amount")

		refunded := stage3JRefund(t, admin, noteID, today, "50.00", "7.3", "bank", "stage3j-customer-refund-1")
		stage3JExpect(t, refunded, "refunded_amount", "50.0000")
		stage3JExpect(t, refunded, "refunded_base_amount", "360.0000")
		stage3JExpect(t, refunded, "remaining_amount", "50.0000")
		stage3JExpect(t, refunded, "remaining_base_amount", "360.0000")
		stage3JExpect(t, refunded, "status", "open")
		refunds := stage3JObjects(t, refunded, "refunds")
		if len(refunds) != 1 {
			t.Fatalf("refund rows after first refund = %d, want 1", len(refunds))
		}
		firstRefund := refunds[0]
		stage3JExpect(t, firstRefund, "currency", "USD")
		stage3JExpect(t, firstRefund, "exchange_rate", "7.30000000")
		stage3JExpect(t, firstRefund, "amount", "50.0000")
		stage3JExpect(t, firstRefund, "base_amount", "360.0000")
		stage3JExpect(t, firstRefund, "cash_base_amount", "365.0000")
		stage3JExpect(t, firstRefund, "fx_adjustment", "5.0000")
		stage3JExpect(t, firstRefund, "method", "bank")
		if text(firstRefund, "voucher_code") == "" {
			t.Fatal("first refund voucher_code is empty")
		}
		firstVoucherLines := stage3JRefundVoucherLines(t, admin, firstRefund)
		assertVoucherBalanced(t, firstVoucherLines)
		if got := number(t, findLineByAccount(t, firstVoucherLines, "1122"), "debit"); !nearlyEqual(got, 360) {
			t.Fatalf("customer refund AR debit = %v, want 360", got)
		}
		if got := number(t, findLineByAccount(t, firstVoucherLines, "1002"), "credit"); !nearlyEqual(got, 365) {
			t.Fatalf("customer refund bank credit = %v, want 365", got)
		}
		if got := number(t, findLineByAccount(t, firstVoucherLines, "660301"), "debit"); !nearlyEqual(got, 5) {
			t.Fatalf("customer refund fx loss debit = %v, want 5", got)
		}

		firstRefundID := text(firstRefund, "id")
		refunded = stage3JRefund(t, admin, noteID, today, "50.00", "7.2", "bank", "stage3j-customer-refund-2")
		stage3JExpect(t, refunded, "refunded_amount", "100.0000")
		stage3JExpect(t, refunded, "refunded_base_amount", "720.0000")
		stage3JExpect(t, refunded, "remaining_amount", "0.0000")
		stage3JExpect(t, refunded, "remaining_base_amount", "0.0000")
		stage3JExpect(t, refunded, "status", "applied")
		refunds = stage3JObjects(t, refunded, "refunds")
		if len(refunds) != 2 {
			t.Fatalf("refund rows after second refund = %d, want 2", len(refunds))
		}
		var secondRefund map[string]any
		for _, refund := range refunds {
			if text(refund, "id") != firstRefundID {
				secondRefund = refund
				break
			}
		}
		if secondRefund == nil {
			t.Fatal("second refund row not found")
		}
		stage3JExpect(t, secondRefund, "exchange_rate", "7.20000000")
		stage3JExpect(t, secondRefund, "base_amount", "360.0000")
		stage3JExpect(t, secondRefund, "cash_base_amount", "360.0000")
		stage3JExpect(t, secondRefund, "fx_adjustment", "0.0000")
		secondVoucherLines := stage3JRefundVoucherLines(t, admin, secondRefund)
		assertVoucherBalanced(t, secondVoucherLines)
		if len(secondVoucherLines) != 2 {
			t.Fatalf("second refund voucher lines = %d, want 2 (no FX line)", len(secondVoucherLines))
		}
		if got := number(t, findLineByAccount(t, secondVoucherLines, "1122"), "debit"); !nearlyEqual(got, 360) {
			t.Fatalf("second customer refund AR debit = %v, want 360", got)
		}
		if got := number(t, findLineByAccount(t, secondVoucherLines, "1002"), "credit"); !nearlyEqual(got, 360) {
			t.Fatalf("second customer refund bank credit = %v, want 360", got)
		}
		for _, raw := range secondVoucherLines {
			line := raw.(map[string]any)
			if code := text(line, "account_code"); code == "660301" || code == "660302" {
				t.Fatalf("second refund voucher unexpectedly contains FX account %s", code)
			}
		}

		invoiceAfter := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoiceAfter, "credited_amount"); got != creditedBefore {
			t.Fatalf("invoice credited changed by refund: %s -> %s", creditedBefore, got)
		}
		if got := text(invoiceAfter, "base_credited_amount"); got != baseCreditedBefore {
			t.Fatalf("invoice base credited changed by refund: %s -> %s", baseCreditedBefore, got)
		}
	})

	t.Run("supplier refund with fx", func(t *testing.T) {
		receiptID, receiptLineID, _, invoiceID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "USD", "7.2", today, today, "stage3j-supplier-refund",
		)
		stage3JPayInvoice(t, admin, "payment", supplierID, invoiceID, "USD", "100.00", "7.2", today, "stage3j-supplier-refund-payment")
		returned := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3j-supplier-refund-return")
		noteID := text(returned, "credit_note_id")
		trackNote(noteID)
		note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, note, "credit_type", "purchase_return")
		stage3JExpect(t, note, "currency", "USD")
		stage3JExpect(t, note, "amount", "100.0000")
		stage3JExpect(t, note, "base_amount", "720.0000")
		stage3JExpect(t, note, "remaining_amount", "100.0000")
		stage3JExpect(t, note, "status", "open")

		invoiceBefore := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		creditedBefore := text(invoiceBefore, "credited_amount")
		baseCreditedBefore := text(invoiceBefore, "base_credited_amount")

		refunded := stage3JRefund(t, admin, noteID, today, "40.00", "7.3", "bank", "stage3j-supplier-refund-partial")
		stage3JExpect(t, refunded, "refunded_amount", "40.0000")
		stage3JExpect(t, refunded, "refunded_base_amount", "288.0000")
		stage3JExpect(t, refunded, "remaining_amount", "60.0000")
		stage3JExpect(t, refunded, "remaining_base_amount", "432.0000")
		stage3JExpect(t, refunded, "status", "open")
		refunds := stage3JObjects(t, refunded, "refunds")
		if len(refunds) != 1 {
			t.Fatalf("supplier refund rows after partial refund = %d, want 1", len(refunds))
		}
		firstRefund := refunds[0]
		stage3JExpect(t, firstRefund, "amount", "40.0000")
		stage3JExpect(t, firstRefund, "base_amount", "288.0000")
		stage3JExpect(t, firstRefund, "cash_base_amount", "292.0000")
		stage3JExpect(t, firstRefund, "fx_adjustment", "4.0000")
		firstLines := stage3JRefundVoucherLines(t, admin, firstRefund)
		assertVoucherBalanced(t, firstLines)
		if got := number(t, findLineByAccount(t, firstLines, "1002"), "debit"); !nearlyEqual(got, 292) {
			t.Fatalf("supplier partial refund bank debit = %v, want 292", got)
		}
		if got := number(t, findLineByAccount(t, firstLines, "2202"), "credit"); !nearlyEqual(got, 288) {
			t.Fatalf("supplier partial refund AP credit = %v, want 288", got)
		}
		if got := number(t, findLineByAccount(t, firstLines, "660302"), "credit"); !nearlyEqual(got, 4) {
			t.Fatalf("supplier partial refund fx gain credit = %v, want 4", got)
		}

		firstRefundID := text(firstRefund, "id")
		refunded = stage3JRefund(t, admin, noteID, today, "60.00", "7.2", "bank", "stage3j-supplier-refund-final")
		stage3JExpect(t, refunded, "refunded_amount", "100.0000")
		stage3JExpect(t, refunded, "refunded_base_amount", "720.0000")
		stage3JExpect(t, refunded, "remaining_amount", "0.0000")
		stage3JExpect(t, refunded, "remaining_base_amount", "0.0000")
		stage3JExpect(t, refunded, "status", "applied")
		refunds = stage3JObjects(t, refunded, "refunds")
		if len(refunds) != 2 {
			t.Fatalf("supplier refund rows after final refund = %d, want 2", len(refunds))
		}
		var finalRefund map[string]any
		for _, refund := range refunds {
			if text(refund, "id") != firstRefundID {
				finalRefund = refund
				break
			}
		}
		if finalRefund == nil {
			t.Fatal("supplier final refund row not found")
		}
		stage3JExpect(t, finalRefund, "amount", "60.0000")
		stage3JExpect(t, finalRefund, "base_amount", "432.0000")
		stage3JExpect(t, finalRefund, "cash_base_amount", "432.0000")
		stage3JExpect(t, finalRefund, "fx_adjustment", "0.0000")
		finalLines := stage3JRefundVoucherLines(t, admin, finalRefund)
		assertVoucherBalanced(t, finalLines)
		if len(finalLines) != 2 {
			t.Fatalf("supplier final refund voucher lines = %d, want 2 (no FX line)", len(finalLines))
		}
		if got := number(t, findLineByAccount(t, finalLines, "1002"), "debit"); !nearlyEqual(got, 432) {
			t.Fatalf("supplier final refund bank debit = %v, want 432", got)
		}
		if got := number(t, findLineByAccount(t, finalLines, "2202"), "credit"); !nearlyEqual(got, 432) {
			t.Fatalf("supplier final refund AP credit = %v, want 432", got)
		}
		for _, raw := range finalLines {
			line := raw.(map[string]any)
			if code := text(line, "account_code"); code == "660301" || code == "660302" {
				t.Fatalf("supplier final refund voucher unexpectedly contains FX account %s", code)
			}
		}

		invoiceAfter := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := text(invoiceAfter, "credited_amount"); got != creditedBefore {
			t.Fatalf("supplier invoice credited changed by refunds: %s -> %s", creditedBefore, got)
		}
		if got := text(invoiceAfter, "base_credited_amount"); got != baseCreditedBefore {
			t.Fatalf("supplier invoice base credited changed by refunds: %s -> %s", baseCreditedBefore, got)
		}
	})

	t.Run("zero base refunds through API", func(t *testing.T) {
		const tinyRate = "0.00005"

		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, customerNoteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"2.00", "USD", tinyRate, today, "stage3j-zero-base-customer",
		)
		trackNote(customerNoteID)
		customerNote := admin.mustGet("/finance/credit-notes/"+customerNoteID, http.StatusOK)
		stage3JExpect(t, customerNote, "amount", "2.0000")
		stage3JExpect(t, customerNote, "base_amount", "0.0001")
		stage3JExpect(t, customerNote, "remaining_amount", "2.0000")
		stage3JExpect(t, customerNote, "remaining_base_amount", "0.0001")
		stage3JExpect(t, customerNote, "status", "open")

		first := stage3JRefund(t, admin, customerNoteID, today, "1.00", tinyRate, "bank", "stage3j-zero-base-customer-1")
		stage3JExpect(t, first, "refunded_amount", "1.0000")
		stage3JExpect(t, first, "refunded_base_amount", "0.0001")
		stage3JExpect(t, first, "remaining_amount", "1.0000")
		stage3JExpect(t, first, "remaining_base_amount", "0.0000")
		stage3JExpect(t, first, "status", "open")
		refunds := stage3JObjects(t, first, "refunds")
		if len(refunds) != 1 {
			t.Fatalf("zero-base customer first refund rows = %d, want 1", len(refunds))
		}
		firstRefund := refunds[0]
		stage3JExpect(t, firstRefund, "base_amount", "0.0001")
		stage3JExpect(t, firstRefund, "cash_base_amount", "0.0001")
		stage3JExpect(t, firstRefund, "fx_adjustment", "0.0000")
		firstLines := stage3JRefundVoucherLines(t, admin, firstRefund)
		assertVoucherBalanced(t, firstLines)
		if len(firstLines) != 2 {
			t.Fatalf("zero-base customer first voucher lines = %d, want 2", len(firstLines))
		}
		if got := text(findLineByAccount(t, firstLines, "1122"), "debit"); got != "0.0001" {
			t.Fatalf("zero-base customer first AR debit = %s, want 0.0001", got)
		}
		if got := text(findLineByAccount(t, firstLines, "1002"), "credit"); got != "0.0001" {
			t.Fatalf("zero-base customer first bank credit = %s, want 0.0001", got)
		}

		firstRefundID := text(firstRefund, "id")
		second := stage3JRefund(t, admin, customerNoteID, today, "1.00", tinyRate, "bank", "stage3j-zero-base-customer-2")
		stage3JExpect(t, second, "refunded_amount", "2.0000")
		stage3JExpect(t, second, "refunded_base_amount", "0.0001")
		stage3JExpect(t, second, "remaining_amount", "0.0000")
		stage3JExpect(t, second, "remaining_base_amount", "0.0000")
		stage3JExpect(t, second, "status", "applied")
		refunds = stage3JObjects(t, second, "refunds")
		if len(refunds) != 2 {
			t.Fatalf("zero-base customer refund rows = %d, want 2", len(refunds))
		}
		var secondRefund map[string]any
		for _, refund := range refunds {
			if text(refund, "id") != firstRefundID {
				secondRefund = refund
				break
			}
		}
		if secondRefund == nil {
			t.Fatal("zero-base customer second refund row not found")
		}
		stage3JExpect(t, secondRefund, "base_amount", "0.0000")
		stage3JExpect(t, secondRefund, "cash_base_amount", "0.0001")
		stage3JExpect(t, secondRefund, "fx_adjustment", "0.0001")
		secondLines := stage3JRefundVoucherLines(t, admin, secondRefund)
		assertVoucherBalanced(t, secondLines)
		if len(secondLines) != 2 {
			t.Fatalf("zero-base customer second voucher lines = %d, want 2", len(secondLines))
		}
		if stage3JLookupLineByAccount(secondLines, "1122") != nil {
			t.Fatal("zero-base customer refund voucher unexpectedly contains an AR line")
		}
		if got := text(findLineByAccount(t, secondLines, "1002"), "credit"); got != "0.0001" {
			t.Fatalf("zero-base customer second bank credit = %s, want 0.0001", got)
		}
		if got := text(findLineByAccount(t, secondLines, "660301"), "debit"); got != "0.0001" {
			t.Fatalf("zero-base customer second fx loss debit = %s, want 0.0001", got)
		}

		receiptID, receiptLineID, _, invoiceID, _ := stage3ICreatePurchaseReceipt(
			t, admin, supplierID, warehouseID, materialID, vat0ID,
			"1", "2.00", true, "USD", tinyRate, today, today, "stage3j-zero-base-supplier",
		)
		stage3JPayInvoice(t, admin, "payment", supplierID, invoiceID, "USD", "2.00", tinyRate, today, "stage3j-zero-base-supplier-payment")
		returned := stage3IPostPurchaseReturn(t, admin, receiptID, receiptLineID, "1", today, "supplier return", "stage3j-zero-base-supplier-return")
		supplierNoteID := text(returned, "credit_note_id")
		trackNote(supplierNoteID)
		supplierNote := admin.mustGet("/finance/credit-notes/"+supplierNoteID, http.StatusOK)
		stage3JExpect(t, supplierNote, "amount", "2.0000")
		stage3JExpect(t, supplierNote, "base_amount", "0.0001")
		stage3JExpect(t, supplierNote, "remaining_amount", "2.0000")
		stage3JExpect(t, supplierNote, "remaining_base_amount", "0.0001")
		stage3JExpect(t, supplierNote, "status", "open")

		first = stage3JRefund(t, admin, supplierNoteID, today, "1.00", tinyRate, "bank", "stage3j-zero-base-supplier-1")
		stage3JExpect(t, first, "refunded_amount", "1.0000")
		stage3JExpect(t, first, "refunded_base_amount", "0.0001")
		stage3JExpect(t, first, "remaining_amount", "1.0000")
		stage3JExpect(t, first, "remaining_base_amount", "0.0000")
		refunds = stage3JObjects(t, first, "refunds")
		if len(refunds) != 1 {
			t.Fatalf("zero-base supplier first refund rows = %d, want 1", len(refunds))
		}
		firstRefund = refunds[0]
		stage3JExpect(t, firstRefund, "base_amount", "0.0001")
		stage3JExpect(t, firstRefund, "cash_base_amount", "0.0001")
		stage3JExpect(t, firstRefund, "fx_adjustment", "0.0000")
		firstLines = stage3JRefundVoucherLines(t, admin, firstRefund)
		assertVoucherBalanced(t, firstLines)
		if len(firstLines) != 2 {
			t.Fatalf("zero-base supplier first voucher lines = %d, want 2", len(firstLines))
		}
		if got := text(findLineByAccount(t, firstLines, "1002"), "debit"); got != "0.0001" {
			t.Fatalf("zero-base supplier first bank debit = %s, want 0.0001", got)
		}
		if got := text(findLineByAccount(t, firstLines, "2202"), "credit"); got != "0.0001" {
			t.Fatalf("zero-base supplier first AP credit = %s, want 0.0001", got)
		}

		firstRefundID = text(firstRefund, "id")
		second = stage3JRefund(t, admin, supplierNoteID, today, "1.00", tinyRate, "bank", "stage3j-zero-base-supplier-2")
		stage3JExpect(t, second, "refunded_amount", "2.0000")
		stage3JExpect(t, second, "refunded_base_amount", "0.0001")
		stage3JExpect(t, second, "remaining_amount", "0.0000")
		stage3JExpect(t, second, "remaining_base_amount", "0.0000")
		stage3JExpect(t, second, "status", "applied")
		refunds = stage3JObjects(t, second, "refunds")
		if len(refunds) != 2 {
			t.Fatalf("zero-base supplier refund rows = %d, want 2", len(refunds))
		}
		secondRefund = nil
		for _, refund := range refunds {
			if text(refund, "id") != firstRefundID {
				secondRefund = refund
				break
			}
		}
		if secondRefund == nil {
			t.Fatal("zero-base supplier second refund row not found")
		}
		stage3JExpect(t, secondRefund, "base_amount", "0.0000")
		stage3JExpect(t, secondRefund, "cash_base_amount", "0.0001")
		stage3JExpect(t, secondRefund, "fx_adjustment", "0.0001")
		secondLines = stage3JRefundVoucherLines(t, admin, secondRefund)
		assertVoucherBalanced(t, secondLines)
		if len(secondLines) != 2 {
			t.Fatalf("zero-base supplier second voucher lines = %d, want 2", len(secondLines))
		}
		if stage3JLookupLineByAccount(secondLines, "2202") != nil {
			t.Fatal("zero-base supplier refund voucher unexpectedly contains an AP line")
		}
		if got := text(findLineByAccount(t, secondLines, "1002"), "debit"); got != "0.0001" {
			t.Fatalf("zero-base supplier second bank debit = %s, want 0.0001", got)
		}
		if got := text(findLineByAccount(t, secondLines, "660302"), "credit"); got != "0.0001" {
			t.Fatalf("zero-base supplier second fx gain credit = %s, want 0.0001", got)
		}
	})

	t.Run("refund required fields and validation", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "CNY", "", today, "stage3j-refund-validation",
		)
		trackNote(noteID)
		path := "/finance/credit-notes/" + noteID + "/refunds"
		admin.expectError(http.MethodPost, path, map[string]any{
			"amount": "10.00", "method": "bank",
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, map[string]any{
			"refund_date": "", "amount": "10.00", "method": "bank",
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, map[string]any{
			"refund_date": today, "amount": "10.00",
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, map[string]any{
			"refund_date": today, "amount": "10.00", "method": "",
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, map[string]any{
			"refund_date": today, "amount": "0", "method": "bank",
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, map[string]any{
			"refund_date": today, "amount": "-1", "method": "bank",
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, map[string]any{
			"refund_date": today, "amount": "10.00", "method": "wire",
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, map[string]any{
			"refund_date": today, "amount": "100.01", "method": "bank",
		}, http.StatusConflict, "OVER_REFUND")
		admin.expectError(http.MethodPost, path, map[string]any{
			"refund_date": "2000-01-01", "amount": "10.00", "method": "bank",
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/finance/credit-notes/00000000-0000-0000-0000-000000000004/refunds", map[string]any{
			"refund_date": today, "amount": "10.00", "method": "bank",
		}, http.StatusNotFound, "NOT_FOUND")

		note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, note, "refunded_amount", "0.0000")
		stage3JExpect(t, note, "remaining_amount", "100.0000")
		stage3JExpect(t, note, "status", "open")
		if refunds := stage3JObjects(t, note, "refunds"); len(refunds) != 0 {
			t.Fatalf("refund rows after rejected requests = %d, want 0", len(refunds))
		}

		refunded := stage3JRefund(t, admin, noteID, today, "100.00", "1", "bank", "stage3j-refund-validation-full")
		stage3JExpect(t, refunded, "remaining_amount", "0.0000")
		stage3JExpect(t, refunded, "status", "applied")
		admin.expectError(http.MethodPost, path, map[string]any{
			"refund_date": today, "amount": "1.00", "method": "bank",
		}, http.StatusConflict, "CREDIT_NOTE_CLOSED")
		note = admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		if refunds := stage3JObjects(t, note, "refunds"); len(refunds) != 1 {
			t.Fatalf("refund rows after closed-note request = %d, want 1", len(refunds))
		}
	})

	t.Run("period closed refund rolls back", func(t *testing.T) {
		year := cleanRevaluationYear(t, admin)
		ensureTestYear(t, admin, year)
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		orderDate := fmt.Sprintf("%04d-03-10", year)
		refundDate := fmt.Sprintf("%04d-03-15", year)
		marchStart := fmt.Sprintf("%04d-03-01", year)
		marchEnd := fmt.Sprintf("%04d-03-31", year)

		_, shipmentID, shipmentLineID, invoiceID := stage3JCreateShippedInvoice(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", "CNY", "", orderDate, "stage3j-period-closed",
		)
		stage3JPayInvoice(t, admin, "receipt", customerID, invoiceID, "CNY", "100.00", "", orderDate, "stage3j-period-closed-payment")
		returned := postReturn(t, admin, shipmentID, shipmentLineID, "1", refundDate, "stage3j-period-closed-return")
		noteID := text(returned, "credit_note_id")
		trackNote(noteID)
		if noteID == "" {
			t.Fatalf("period closed credit note id missing: %v", returned)
		}
		before := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, before, "refunded_amount", "0.0000")
		stage3JExpect(t, before, "remaining_amount", "100.0000")
		stage3JExpect(t, before, "status", "open")
		if refunds := stage3JObjects(t, before, "refunds"); len(refunds) != 0 {
			t.Fatalf("refund rows before period close = %d, want 0", len(refunds))
		}

		yearPeriods := getArray(admin, periodsPath(year), http.StatusOK)
		var march map[string]any
		for _, period := range yearPeriods {
			if month, ok := period["month"].(float64); ok && int(month) == 3 {
				march = period
				break
			}
		}
		if march == nil {
			t.Fatalf("March %d period not found", year)
		}
		marchID := text(march, "id")
		if text(march, "status") == "closed" {
			admin.mustPost("/finance/periods/"+marchID+"/reopen", nil, http.StatusOK)
		}
		admin.mustPost("/finance/periods/"+marchID+"/close", nil, http.StatusOK)
		defer func() {
			admin.mustPost("/finance/periods/"+marchID+"/reopen", nil, http.StatusOK)
		}()

		admin.expectError(http.MethodPost, "/finance/credit-notes/"+noteID+"/refunds", map[string]any{
			"refund_date": refundDate, "amount": "50.00", "method": "bank",
		}, http.StatusConflict, "PERIOD_CLOSED")

		after := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		for _, key := range []string{"refunded_amount", "refunded_base_amount", "remaining_amount", "remaining_base_amount", "status"} {
			if text(after, key) != text(before, key) {
				t.Fatalf("credit note %s after rejected refund = %q, before = %q", key, text(after, key), text(before, key))
			}
		}
		if refunds := stage3JObjects(t, after, "refunds"); len(refunds) != 0 {
			t.Fatalf("refund rows after rejected refund = %d, want 0", len(refunds))
		}
		vouchers := listItems(admin,
			"/finance/vouchers?voucher_type=credit_note_refund&start_date="+marchStart+"&end_date="+marchEnd+"&page_size=200",
			http.StatusOK,
		)
		if len(vouchers) != 0 {
			t.Fatalf("credit_note_refund vouchers in closed period = %d, want 0", len(vouchers))
		}
	})

	// Credit application must reduce the base outstanding used by period-end FX
	// revaluation; unused credit notes themselves are not revalued.
	t.Run("fx revaluation after credit application", func(t *testing.T) {
		year := cleanRevaluationYear(t, admin)
		ensureTestYear(t, admin, year)
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		// Use an isolated partner so ambient open USD receivables from earlier
		// tests cannot join this customer's revaluation lines.
		fxCustomer := admin.mustPost("/masterdata/partners", map[string]any{
			"code": fmt.Sprintf("CUS-FX-3J-%d", time.Now().UnixNano()),
			"name": "Stage 3J FX Customer",
			"type": "customer",
		}, http.StatusCreated)
		fxCustomerID := text(fxCustomer, "id")
		invoiceDate := fmt.Sprintf("%04d-01-15", year)
		janStart := fmt.Sprintf("%04d-01-01", year)
		closeDate := fmt.Sprintf("%04d-01-31", year)

		_, shipmentAID, shipmentALineID, invoiceAID := stage3JCreateShippedInvoice(
			t, admin, fxCustomerID, warehouseID, materialID, vat0ID,
			"1", "100.00", "USD", "7.2", invoiceDate, "stage3j-fx-revaluation-a",
		)
		stage3JPayInvoice(t, admin, "receipt", fxCustomerID, invoiceAID, "USD", "100.00", "7.2", invoiceDate, "stage3j-fx-revaluation-a-payment")
		returned := postReturn(t, admin, shipmentAID, shipmentALineID, "1", invoiceDate, "stage3j-fx-revaluation-a-return")
		noteID := text(returned, "credit_note_id")
		trackNote(noteID)
		note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3JExpect(t, note, "amount", "100.0000")
		stage3JExpect(t, note, "base_amount", "720.0000")
		stage3JExpect(t, note, "applied_amount", "0.0000")
		stage3JExpect(t, note, "remaining_amount", "100.0000")
		stage3JExpect(t, note, "status", "open")

		_, _, _, invoiceBID := stage3JCreateShippedInvoice(
			t, admin, fxCustomerID, warehouseID, materialID, vat0ID,
			"2", "100.00", "USD", "7.2", invoiceDate, "stage3j-fx-revaluation-b",
		)
		applied := stage3JAllocate(t, admin, noteID, invoiceDate, stage3JAllocation{invoiceBID, "100.00"})
		stage3JExpect(t, applied, "applied_amount", "100.0000")
		stage3JExpect(t, applied, "applied_base_amount", "720.0000")
		stage3JExpect(t, applied, "remaining_amount", "0.0000")
		stage3JExpect(t, applied, "remaining_base_amount", "0.0000")
		stage3JExpect(t, applied, "status", "applied")
		invoiceB := admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
		stage3JExpect(t, invoiceB, "credited_amount", "100.0000")
		stage3JExpect(t, invoiceB, "base_credited_amount", "720.0000")
		stage3JExpect(t, invoiceB, "outstanding_amount", "100.0000")
		stage3JExpect(t, invoiceB, "base_revalued_amount", "0.0000")
		stage3JExpect(t, invoiceB, "status", "partially_paid")
		baseOutstanding := number(t, invoiceB, "base_total_amount") - number(t, invoiceB, "base_credited_amount")
		if !nearlyEqual(baseOutstanding, 720) {
			t.Fatalf("invoice B base outstanding after credit application = %v, want 720", baseOutstanding)
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
		januaryID := text(january, "id")
		if text(january, "status") == "closed" {
			admin.mustPost("/finance/periods/"+januaryID+"/reopen", nil, http.StatusOK)
		}
		admin.mustPost("/finance/periods/"+januaryID+"/close", nil, http.StatusOK)
		defer func() {
			admin.mustPost("/finance/periods/"+januaryID+"/reopen", nil, http.StatusOK)
		}()

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
		findLineByAccountPartner(t, revaluationLines, "1122", fxCustomerID)
		findLineByAccountPartner(t, revaluationLines, "660302", fxCustomerID)
		if got := stage3JSumPostingLines(t, revaluationLines, "1122", fxCustomerID, "debit"); !nearlyEqual(got, 10) {
			t.Fatalf("FX revaluation AR debit after credit application = %v, want 10", got)
		}
		if got := stage3JSumPostingLines(t, revaluationLines, "660302", fxCustomerID, "credit"); !nearlyEqual(got, 10) {
			t.Fatalf("FX revaluation gain credit after credit application = %v, want 10", got)
		}
		revalued := admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
		stage3JExpect(t, revalued, "base_revalued_amount", "10.0000")
	})

	t.Run("invoice detail lists credit note allocations", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, noteCode := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"50.00", "CNY", "", today, "stage3j-invoice-detail-note",
		)
		trackNote(noteID)
		_, _, _, invoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3j-invoice-detail-target",
		)
		stage3JAllocate(t, admin, noteID, today, stage3JAllocation{invoiceID, "50.00"})

		detail := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		allocations := stage3JObjects(t, detail, "credit_note_allocations")
		if len(allocations) != 1 {
			t.Fatalf("invoice credit note allocations = %d, want 1", len(allocations))
		}
		allocation := allocations[0]
		stage3JExpect(t, allocation, "credit_note_id", noteID)
		stage3JExpect(t, allocation, "credit_note_code", noteCode)
		stage3JExpect(t, allocation, "credit_type", "sales_return")
		stage3JExpect(t, allocation, "note_date", today)
		stage3JExpect(t, allocation, "invoice_id", invoiceID)
		stage3JExpect(t, allocation, "amount", "50.0000")
		stage3JExpect(t, allocation, "base_amount", "50.0000")
		stage3JExpect(t, allocation, "allocation_date", today)

		_, _, _, zeroInvoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3j-invoice-detail-zero",
		)
		zeroDetail := admin.mustGet("/finance/invoices/"+zeroInvoiceID, http.StatusOK)
		if allocations := stage3JObjects(t, zeroDetail, "credit_note_allocations"); len(allocations) != 0 {
			t.Fatalf("invoice without credit notes allocations = %d, want 0", len(allocations))
		}
	})

	t.Run("permissions", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "CNY", "", today, "stage3j-permissions",
		)
		trackNote(noteID)
		_, _, _, targetInvoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "50.00", true, "stage3j-permissions-target",
		)
		applyBody := stage3JAllocationBody(today, targetInvoiceID, "10.00")
		refundBody := map[string]any{
			"refund_date": today,
			"amount":      "10.00",
			"method":      "bank",
		}

		unauthorized := newClient(t)
		unauthorized.expectError(http.MethodPost, "/finance/credit-notes/"+noteID+"/allocations", applyBody, http.StatusUnauthorized, "UNAUTHORIZED")
		unauthorized.expectError(http.MethodPost, "/finance/credit-notes/"+noteID+"/refunds", refundBody, http.StatusUnauthorized, "UNAUTHORIZED")
		unauthorized.expectError(http.MethodGet, "/finance/credit-notes/"+noteID+"/allocation-candidates", nil, http.StatusUnauthorized, "UNAUTHORIZED")

		dept := newClient(t).login("demo_dept", env("TEST_DEMO_PASSWORD", "Demo@123456"))
		dept.expectError(http.MethodPost, "/finance/credit-notes/"+noteID+"/allocations", applyBody, http.StatusForbidden, "FORBIDDEN")
		dept.expectError(http.MethodPost, "/finance/credit-notes/"+noteID+"/refunds", refundBody, http.StatusForbidden, "FORBIDDEN")
		dept.expectError(http.MethodGet, "/finance/credit-notes/"+noteID+"/allocation-candidates", nil, http.StatusForbidden, "FORBIDDEN")

		status, payload := dept.request(http.MethodGet, "/finance/credit-notes?page_size=200", nil, dept.token)
		switch status {
		case http.StatusForbidden:
			if payload.Code != "FORBIDDEN" {
				t.Fatalf("demo_dept credit note list code = %q, want FORBIDDEN", payload.Code)
			}
		case http.StatusOK:
			rawItems, _ := payload.Data["items"].([]any)
			for _, raw := range rawItems {
				row, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				for _, stage3JNoteID := range stage3JNoteIDs {
					if text(row, "id") == stage3JNoteID {
						t.Fatalf("demo_dept can see Stage 3J credit note %s outside its scope", stage3JNoteID)
					}
				}
			}
		default:
			t.Fatalf("demo_dept credit note list status = %d, want 200 or 403, payload = %v", status, payload)
		}
	})

	t.Run("concurrent apply and refund", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		t.Run("concurrent applications do not double credit", func(t *testing.T) {
			_, noteID, _ := stage3JCreateOpenCreditNote(
				t, admin, customerID, warehouseID, materialID, vat0ID,
				"50.00", "CNY", "", today, "stage3j-concurrent-apply",
			)
			trackNote(noteID)
			invoiceIDs := make([]string, 0, 3)
			for _, remark := range []string{"stage3j-concurrent-apply-b", "stage3j-concurrent-apply-c", "stage3j-concurrent-apply-d"} {
				_, _, _, invoiceID, _ := createShippedOrder(
					t, admin, customerID, warehouseID, materialID, vat0ID,
					"1", "50.00", true, remark,
				)
				invoiceIDs = append(invoiceIDs, invoiceID)
			}
			requests := make([]stage3JRequest, 0, len(invoiceIDs))
			for _, invoiceID := range invoiceIDs {
				requests = append(requests, stage3JRequest{
					path: "/finance/credit-notes/" + noteID + "/allocations",
					body: stage3JAllocationBody(today, invoiceID, "50.00"),
				})
			}
			results := stage3JRunConcurrent(admin, requests)
			successes := 0
			conflicts := 0
			// Full consumption leaves the note applied, so losing contenders see
			// CREDIT_NOTE_CLOSED rather than a remaining-amount OVER_APPLY.
			for i, result := range results {
				if result.err != nil {
					t.Fatalf("concurrent application %d failed: %v", i, result.err)
				}
				switch result.status {
				case http.StatusOK:
					successes++
				case http.StatusConflict:
					if result.code != "CREDIT_NOTE_CLOSED" {
						t.Fatalf("concurrent application loser code = %q, want CREDIT_NOTE_CLOSED", result.code)
					}
					conflicts++
				default:
					t.Fatalf("concurrent application %d status = %d, code = %q", i, result.status, result.code)
				}
			}
			if successes != 1 || conflicts != 2 {
				t.Fatalf("concurrent applications success/conflicts = %d/%d, want 1/2", successes, conflicts)
			}

			note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
			stage3JExpect(t, note, "applied_amount", "50.0000")
			stage3JExpect(t, note, "remaining_amount", "0.0000")
			stage3JExpect(t, note, "status", "applied")
			if allocations := stage3JObjects(t, note, "allocations"); len(allocations) != 1 {
				t.Fatalf("concurrent application rows = %d, want 1", len(allocations))
			}
			creditedTotal := 0.0
			paidInvoices := 0
			for _, invoiceID := range invoiceIDs {
				invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
				creditedTotal += number(t, invoice, "credited_amount")
				if text(invoice, "status") == "paid" {
					paidInvoices++
				}
			}
			if !nearlyEqual(creditedTotal, 50) {
				t.Fatalf("concurrent application invoice credited total = %v, want 50", creditedTotal)
			}
			if paidInvoices != 1 {
				t.Fatalf("concurrent applications paid invoice count = %d, want 1", paidInvoices)
			}
		})

		t.Run("concurrent refunds do not double refund", func(t *testing.T) {
			_, noteID, _ := stage3JCreateOpenCreditNote(
				t, admin, customerID, warehouseID, materialID, vat0ID,
				"50.00", "CNY", "", today, "stage3j-concurrent-refund",
			)
			trackNote(noteID)
			body := map[string]any{
				"refund_date": today,
				"amount":      "50.00",
				"method":      "bank",
			}
			results := stage3JRunConcurrent(admin, []stage3JRequest{
				{path: "/finance/credit-notes/" + noteID + "/refunds", body: body},
				{path: "/finance/credit-notes/" + noteID + "/refunds", body: body},
				{path: "/finance/credit-notes/" + noteID + "/refunds", body: body},
			})
			successes := 0
			conflicts := 0
			// Full consumption leaves the note applied, so losing contenders see
			// CREDIT_NOTE_CLOSED rather than a remaining-amount OVER_REFUND.
			for i, result := range results {
				if result.err != nil {
					t.Fatalf("concurrent refund %d failed: %v", i, result.err)
				}
				switch result.status {
				case http.StatusOK:
					successes++
				case http.StatusConflict:
					if result.code != "CREDIT_NOTE_CLOSED" {
						t.Fatalf("concurrent refund loser code = %q, want CREDIT_NOTE_CLOSED", result.code)
					}
					conflicts++
				default:
					t.Fatalf("concurrent refund %d status = %d, code = %q", i, result.status, result.code)
				}
			}
			if successes != 1 || conflicts != 2 {
				t.Fatalf("concurrent refunds success/conflicts = %d/%d, want 1/2", successes, conflicts)
			}

			note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
			stage3JExpect(t, note, "refunded_amount", "50.0000")
			stage3JExpect(t, note, "remaining_amount", "0.0000")
			stage3JExpect(t, note, "status", "applied")
			refunds := stage3JObjects(t, note, "refunds")
			if len(refunds) != 1 {
				t.Fatalf("concurrent refund rows = %d, want 1", len(refunds))
			}
			if got := number(t, refunds[0], "amount"); !nearlyEqual(got, 50) {
				t.Fatalf("concurrent refund amount = %v, want 50", got)
			}
		})

		t.Run("concurrent application and refund serialize", func(t *testing.T) {
			_, noteID, _ := stage3JCreateOpenCreditNote(
				t, admin, customerID, warehouseID, materialID, vat0ID,
				"100.00", "USD", "7.2", today, "stage3j-concurrent-mixed",
			)
			trackNote(noteID)
			_, _, _, invoiceBID := stage3JCreateShippedInvoice(
				t, admin, customerID, warehouseID, materialID, vat0ID,
				"1", "50.00", "USD", "7.2", today, "stage3j-concurrent-mixed-b",
			)
			results := stage3JRunConcurrent(admin, []stage3JRequest{
				{path: "/finance/credit-notes/" + noteID + "/allocations", body: stage3JAllocationBody(today, invoiceBID, "50.00")},
				{path: "/finance/credit-notes/" + noteID + "/refunds", body: map[string]any{
					"refund_date": today, "amount": "50.00", "exchange_rate": "7.3", "method": "bank",
				}},
			})
			for i, result := range results {
				if result.err != nil {
					t.Fatalf("mixed concurrent request %d failed: %v", i, result.err)
				}
				if result.status != http.StatusOK {
					t.Fatalf("mixed concurrent request %d status = %d, code = %q", i, result.status, result.code)
				}
			}

			note := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
			stage3JExpect(t, note, "applied_amount", "50.0000")
			stage3JExpect(t, note, "applied_base_amount", "360.0000")
			stage3JExpect(t, note, "refunded_amount", "50.0000")
			stage3JExpect(t, note, "refunded_base_amount", "360.0000")
			stage3JExpect(t, note, "remaining_amount", "0.0000")
			stage3JExpect(t, note, "remaining_base_amount", "0.0000")
			stage3JExpect(t, note, "status", "applied")
			appliedAmount := number(t, note, "applied_amount")
			refundedAmount := number(t, note, "refunded_amount")
			remainingAmount := number(t, note, "remaining_amount")
			if !nearlyEqual(appliedAmount+refundedAmount+remainingAmount, number(t, note, "amount")) {
				t.Fatalf("credit note amount invariant broken: %v + %v + %v != %v",
					appliedAmount, refundedAmount, remainingAmount, number(t, note, "amount"))
			}
			appliedBase := number(t, note, "applied_base_amount")
			refundedBase := number(t, note, "refunded_base_amount")
			remainingBase := number(t, note, "remaining_base_amount")
			if !nearlyEqual(appliedBase+refundedBase+remainingBase, number(t, note, "base_amount")) {
				t.Fatalf("credit note base invariant broken: %v + %v + %v != %v",
					appliedBase, refundedBase, remainingBase, number(t, note, "base_amount"))
			}
			allocations := stage3JObjects(t, note, "allocations")
			if len(allocations) != 1 {
				t.Fatalf("mixed concurrent allocation rows = %d, want 1", len(allocations))
			}
			stage3JExpect(t, allocations[0], "invoice_id", invoiceBID)
			stage3JExpect(t, allocations[0], "base_amount", "360.0000")
			refunds := stage3JObjects(t, note, "refunds")
			if len(refunds) != 1 {
				t.Fatalf("mixed concurrent refund rows = %d, want 1", len(refunds))
			}
			stage3JExpect(t, refunds[0], "base_amount", "360.0000")
			stage3JExpect(t, refunds[0], "cash_base_amount", "365.0000")
			stage3JExpect(t, refunds[0], "fx_adjustment", "5.0000")
			invoiceB := admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
			stage3JExpect(t, invoiceB, "credited_amount", "50.0000")
			stage3JExpect(t, invoiceB, "base_credited_amount", "360.0000")
			stage3JExpect(t, invoiceB, "status", "paid")
		})
	})
}

func stage3JAllocationBody(date, invoiceID, amount string) map[string]any {
	return map[string]any{
		"allocation_date": date,
		"allocations": []map[string]any{
			{"invoice_id": invoiceID, "amount": amount},
		},
	}
}

func stage3JExpect(t *testing.T, m map[string]any, key, want string) {
	t.Helper()
	if got := text(m, key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}

func stage3JObjects(t *testing.T, data map[string]any, key string) []map[string]any {
	t.Helper()
	raw, ok := data[key].([]any)
	if !ok {
		t.Fatalf("field %s = %v, want array", key, data[key])
	}
	items := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		item, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("field %s entry = %v, want object", key, entry)
		}
		items = append(items, item)
	}
	return items
}

func stage3JPayInvoice(
	t *testing.T, admin *client,
	paymentType, partnerID, invoiceID, currency, amount, exchangeRate, date, reference string,
) map[string]any {
	t.Helper()
	body := map[string]any{
		"payment_type": paymentType,
		"partner_id":   partnerID,
		"payment_date": date,
		"currency":     currency,
		"amount":       amount,
		"method":       "bank",
		"reference":    reference,
	}
	if exchangeRate != "" {
		body["exchange_rate"] = exchangeRate
	}
	payment := admin.mustPost("/finance/payments", body, http.StatusCreated)
	admin.mustPost("/finance/payments/"+text(payment, "id")+"/allocate", map[string]any{
		"allocations": []map[string]any{
			{"invoice_id": invoiceID, "amount": amount},
		},
	}, http.StatusOK)
	return payment
}

func stage3JCreateShippedInvoice(
	t *testing.T, admin *client,
	customerID, warehouseID, materialID, taxID, quantity, unitPrice, currency, exchangeRate, date, remark string,
) (string, string, string, string) {
	t.Helper()
	orderBody := map[string]any{
		"customer_id":        customerID,
		"warehouse_id":       warehouseID,
		"order_date":         date,
		"currency":           currency,
		"price_includes_tax": true,
		"remark":             remark,
		"lines": []map[string]any{
			{"material_id": materialID, "quantity": quantity, "unit_price": unitPrice, "tax_rate_id": taxID},
		},
	}
	if exchangeRate != "" {
		orderBody["exchange_rate"] = exchangeRate
	}
	order := admin.mustPost("/sales/orders", orderBody, http.StatusCreated)
	orderID := text(order, "id")
	admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
	admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
	detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
	lines, _ := detail["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("sales order %s lines = %d, want 1", orderID, len(lines))
	}
	shipBody := map[string]any{
		"shipment_date": date,
		"remark":        remark + "-ship",
		"lines": []map[string]any{
			{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": quantity},
		},
	}
	if exchangeRate != "" {
		shipBody["exchange_rate"] = exchangeRate
	}
	shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", shipBody, http.StatusCreated)
	shipmentID := text(shipment, "id")
	shipmentLines, _ := shipment["lines"].([]any)
	if len(shipmentLines) != 1 {
		t.Fatalf("sales shipment %s lines = %d, want 1", shipmentID, len(shipmentLines))
	}
	return orderID, shipmentID, text(shipmentLines[0].(map[string]any), "id"), text(shipment, "invoice_id")
}

func stage3JCreateOpenCreditNote(
	t *testing.T, admin *client,
	customerID, warehouseID, materialID, taxID, unitPrice, currency, exchangeRate, date, remark string,
) (string, string, string) {
	t.Helper()
	_, shipmentID, shipmentLineID, invoiceID := stage3JCreateShippedInvoice(
		t, admin, customerID, warehouseID, materialID, taxID,
		"1", unitPrice, currency, exchangeRate, date, remark,
	)
	stage3JPayInvoice(t, admin, "receipt", customerID, invoiceID, currency, unitPrice, exchangeRate, date, remark+"-payment")
	returned := postReturn(t, admin, shipmentID, shipmentLineID, "1", date, remark+"-return")
	noteID := text(returned, "credit_note_id")
	noteCode := text(returned, "credit_note_code")
	if noteID == "" || noteCode == "" {
		t.Fatalf("credit note id/code missing: %v", returned)
	}
	return invoiceID, noteID, noteCode
}

func stage3JAllocate(t *testing.T, admin *client, noteID, date string, entries ...stage3JAllocation) map[string]any {
	t.Helper()
	rows := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, map[string]any{"invoice_id": entry.invoiceID, "amount": entry.amount})
	}
	return admin.mustPost("/finance/credit-notes/"+noteID+"/allocations", map[string]any{
		"allocation_date": date,
		"allocations":     rows,
	}, http.StatusOK)
}

func stage3JRefund(
	t *testing.T, admin *client,
	noteID, date, amount, exchangeRate, method, reference string,
) map[string]any {
	t.Helper()
	body := map[string]any{
		"refund_date": date,
		"amount":      amount,
		"method":      method,
		"reference":   reference,
	}
	if exchangeRate != "" {
		body["exchange_rate"] = exchangeRate
	}
	return admin.mustPost("/finance/credit-notes/"+noteID+"/refunds", body, http.StatusOK)
}

func stage3JRefundVoucherLines(t *testing.T, admin *client, refund map[string]any) []any {
	t.Helper()
	code := text(refund, "code")
	if code == "" {
		t.Fatalf("refund code is empty: %v", refund)
	}
	voucher := findVoucherBySource(admin, code)
	if voucher == nil {
		t.Fatalf("refund voucher for %s not found", code)
	}
	stage3JExpect(t, voucher, "voucher_type", "credit_note_refund")
	if got := text(voucher, "source_id"); got != text(refund, "id") {
		t.Fatalf("refund voucher source_id = %s, want %s", got, text(refund, "id"))
	}
	detail := admin.mustGet("/finance/vouchers/"+text(voucher, "id"), http.StatusOK)
	lines, _ := detail["lines"].([]any)
	return lines
}

func stage3JLookupLineByAccount(lines []any, accountCode string) map[string]any {
	for _, raw := range lines {
		line, ok := raw.(map[string]any)
		if ok && text(line, "account_code") == accountCode {
			return line
		}
	}
	return nil
}

func stage3JSumPostingLines(t *testing.T, lines []any, accountCode, partnerID, field string) float64 {
	t.Helper()
	total := 0.0
	for _, raw := range lines {
		line, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("voucher line payload = %v", raw)
		}
		if text(line, "account_code") == accountCode && text(line, "partner_id") == partnerID {
			total += number(t, line, field)
		}
	}
	return total
}

func stage3JRunConcurrent(c *client, requests []stage3JRequest) []stage3JHTTPResult {
	results := make([]stage3JHTTPResult, len(requests))
	start := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(len(requests))
	done.Add(len(requests))
	for i := range requests {
		go func(index int) {
			defer done.Done()
			ready.Done()
			<-start
			status, code, err := stage3JRawPost(c, requests[index].path, requests[index].body)
			results[index] = stage3JHTTPResult{status: status, code: code, err: err}
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()
	return results
}

func stage3JRawPost(c *client, path string, body any) (int, string, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+apiPrefix+path, bytes.NewReader(raw))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := stage3JHTTPClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", fmt.Errorf("read response body: %w", err)
	}
	if len(data) == 0 {
		return resp.StatusCode, "", fmt.Errorf("empty response body")
	}
	var payload apiPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return resp.StatusCode, "", fmt.Errorf("decode response body: %w", err)
	}
	return resp.StatusCode, payload.Code, nil
}
