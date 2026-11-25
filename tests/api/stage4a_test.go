//go:build integration

package api_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"
)

type stage4AResult struct {
	status int
	code   string
	data   map[string]any
	err    error
}

func TestStage4A(t *testing.T) {
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
	vat0 := findItem(listItems(admin, "/masterdata/tax-rates?keyword=VAT0&page_size=20", http.StatusOK), "code", "VAT0")
	if customer == nil || supplier == nil || warehouse == nil || material == nil || vat0 == nil {
		t.Fatal("demo master data missing")
	}
	customerID := text(customer, "id")
	supplierID := text(supplier, "id")
	warehouseID := text(warehouse, "id")
	materialID := text(material, "id")
	vat0ID := text(vat0, "id")

	ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)

	t.Run("payment create replay", func(t *testing.T) {
		reference := stage4AUnique("stage4a-payment")
		key := stage4AUnique("stage4a-payment-key")
		body := map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": today,
			"currency":     "CNY",
			"amount":       "100.00",
			"method":       "bank",
			"reference":    reference,
			"remark":       reference,
		}

		status, first := admin.postWithHeaders("/finance/payments", body, stage4AHeaders(key))
		if status != http.StatusCreated {
			t.Fatalf("first create payment status = %d, want %d, payload = %v", status, http.StatusCreated, first)
		}
		paymentID := text(first.Data, "id")
		if paymentID == "" {
			t.Fatalf("first create payment returned empty id: %v", first)
		}

		status, replay := admin.postWithHeaders("/finance/payments", body, stage4AHeaders(key))
		if status != http.StatusCreated {
			t.Fatalf("replayed create payment status = %d, want %d, payload = %v", status, http.StatusCreated, replay)
		}
		if got := text(replay.Data, "id"); got != paymentID {
			t.Fatalf("replayed payment id = %q, want %q", got, paymentID)
		}

		conflictBody := map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": today,
			"currency":     "CNY",
			"amount":       "101.00",
			"method":       "bank",
			"reference":    reference,
			"remark":       reference,
		}
		status, conflict := admin.postWithHeaders("/finance/payments", conflictBody, stage4AHeaders(key))
		if status != http.StatusConflict || conflict.Code != "IDEMPOTENCY_CONFLICT" {
			t.Fatalf("different amount status/code = %d/%q, want 409 IDEMPOTENCY_CONFLICT, payload = %v", status, conflict.Code, conflict)
		}

		payments := listItems(admin, "/finance/payments?keyword="+url.QueryEscape(reference)+"&page_size=200", http.StatusOK)
		count := 0
		var foundID string
		for _, payment := range payments {
			if text(payment, "reference") == reference {
				count++
				foundID = text(payment, "id")
			}
		}
		if count != 1 {
			t.Fatalf("payments with reference %q = %d, want 1", reference, count)
		}
		if foundID != paymentID {
			t.Fatalf("listed payment id = %q, want %q", foundID, paymentID)
		}
	})

	t.Run("allocate replay", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, _, _, invoiceID := stage3JCreateShippedInvoice(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", "CNY", "", today, stage4AUnique("stage4a-allocate-invoice"),
		)
		payment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "100.00", today, stage4AUnique("stage4a-allocate-payment"))
		paymentID := text(payment, "id")
		key := stage4AUnique("stage4a-allocate-key")
		body := map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": invoiceID, "amount": "100.00"},
			},
		}

		status, first := admin.postWithHeaders("/finance/payments/"+paymentID+"/allocate", body, stage4AHeaders(key))
		if status != http.StatusOK {
			t.Fatalf("first allocate status = %d, want %d, payload = %v", status, http.StatusOK, first)
		}

		status, replay := admin.postWithHeaders("/finance/payments/"+paymentID+"/allocate", body, stage4AHeaders(key))
		if status != http.StatusOK {
			t.Fatalf("replayed allocate status = %d, want %d, payload = %v", status, http.StatusOK, replay)
		}
		stage4AExpectReplay(t, first.Data, replay.Data)

		detail := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		allocations := stage3JObjects(t, detail, "allocations")
		if len(allocations) != 1 {
			t.Fatalf("payment allocations after replay = %d, want 1", len(allocations))
		}
		if got := text(allocations[0], "invoice_id"); got != invoiceID {
			t.Fatalf("allocation invoice_id = %q, want %q", got, invoiceID)
		}
		if got := number(t, detail, "allocated_amount"); !nearlyEqual(got, 100) {
			t.Fatalf("payment allocated_amount = %v, want 100", got)
		}
	})

	t.Run("unallocate replay", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, _, _, invoiceID := stage3JCreateShippedInvoice(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", "CNY", "", today, stage4AUnique("stage4a-unallocate-invoice"),
		)
		payment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "100.00", today, stage4AUnique("stage4a-unallocate-payment"))
		paymentID := text(payment, "id")
		stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "100.00"})

		key := stage4AUnique("stage4a-unallocate-key")
		reason := stage4AUnique("stage4a-unallocate-reason")
		body := stage3KUnallocateBody([]string{invoiceID}, today, reason)

		status, first := admin.postWithHeaders("/finance/payments/"+paymentID+"/unallocate", body, stage4AHeaders(key))
		if status != http.StatusOK {
			t.Fatalf("first unallocate status = %d, want %d, payload = %v", status, http.StatusOK, first)
		}

		status, replay := admin.postWithHeaders("/finance/payments/"+paymentID+"/unallocate", body, stage4AHeaders(key))
		if status != http.StatusOK {
			t.Fatalf("replayed unallocate status = %d, want %d, payload = %v", status, http.StatusOK, replay)
		}
		stage4AExpectReplay(t, first.Data, replay.Data)

		detail := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		if allocations := stage3JObjects(t, detail, "allocations"); len(allocations) != 0 {
			t.Fatalf("payment allocations after unallocate replay = %d, want 0", len(allocations))
		}
		reversals := stage3JObjects(t, detail, "reversals")
		if len(reversals) != 1 {
			t.Fatalf("payment reversals after replay = %d, want 1", len(reversals))
		}
		if got := text(reversals[0], "invoice_id"); got != invoiceID {
			t.Fatalf("reversal invoice_id = %q, want %q", got, invoiceID)
		}
		if got := text(reversals[0], "reason"); got != reason {
			t.Fatalf("reversal reason = %q, want %q", got, reason)
		}
	})

	t.Run("refund replay", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "CNY", "", today, stage4AUnique("stage4a-refund-note"),
		)
		key := stage4AUnique("stage4a-refund-key")
		body := map[string]any{
			"refund_date": today,
			"amount":      "40.00",
			"method":      "bank",
			"reference":   stage4AUnique("stage4a-refund"),
		}

		status, first := admin.postWithHeaders("/finance/credit-notes/"+noteID+"/refunds", body, stage4AHeaders(key))
		if status != http.StatusOK {
			t.Fatalf("first refund status = %d, want %d, payload = %v", status, http.StatusOK, first)
		}
		firstRefundID := stage4ARefundID(t, first.Data)
		if got := number(t, first.Data, "refunded_amount"); !nearlyEqual(got, 40) {
			t.Fatalf("refunded_amount after first refund = %v, want 40", got)
		}

		status, replay := admin.postWithHeaders("/finance/credit-notes/"+noteID+"/refunds", body, stage4AHeaders(key))
		if status != http.StatusOK {
			t.Fatalf("replayed refund status = %d, want %d, payload = %v", status, http.StatusOK, replay)
		}
		if got := stage4ARefundID(t, replay.Data); got != firstRefundID {
			t.Fatalf("replayed refund id = %q, want %q", got, firstRefundID)
		}
		if got := number(t, replay.Data, "refunded_amount"); !nearlyEqual(got, 40) {
			t.Fatalf("refunded_amount after replay = %v, want 40", got)
		}

		detail := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		if got := number(t, detail, "refunded_amount"); !nearlyEqual(got, 40) {
			t.Fatalf("credit note refunded_amount = %v, want 40", got)
		}
		if refunds := stage3JObjects(t, detail, "refunds"); len(refunds) != 1 {
			t.Fatalf("credit note refund rows = %d, want 1", len(refunds))
		}

		conflictBody := map[string]any{
			"refund_date": today,
			"amount":      "50.00",
			"method":      "bank",
			"reference":   stage4AUnique("stage4a-refund-conflict"),
		}
		status, conflict := admin.postWithHeaders("/finance/credit-notes/"+noteID+"/refunds", conflictBody, stage4AHeaders(key))
		if status != http.StatusConflict || conflict.Code != "IDEMPOTENCY_CONFLICT" {
			t.Fatalf("different refund amount status/code = %d/%q, want 409 IDEMPOTENCY_CONFLICT, payload = %v", status, conflict.Code, conflict)
		}

		detail = admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		if got := number(t, detail, "refunded_amount"); !nearlyEqual(got, 40) {
			t.Fatalf("credit note refunded_amount after conflict = %v, want 40", got)
		}
		if refunds := stage3JObjects(t, detail, "refunds"); len(refunds) != 1 {
			t.Fatalf("credit note refund rows after conflict = %d, want 1", len(refunds))
		}
	})

	t.Run("concurrent refund same key", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, noteID, _ := stage3JCreateOpenCreditNote(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"100.00", "CNY", "", today, stage4AUnique("stage4a-concurrent-refund-note"),
		)
		key := stage4AUnique("stage4a-concurrent-refund-key")
		body := map[string]any{
			"refund_date": today,
			"amount":      "30.00",
			"method":      "bank",
			"reference":   stage4AUnique("stage4a-concurrent-refund"),
		}

		results := stage4AConcurrentPost(admin, "/finance/credit-notes/"+noteID+"/refunds", body, key)
		successes := 0
		var refundIDs []string
		for index, result := range results {
			if result.err != nil {
				t.Fatalf("concurrent refund request %d failed: %v", index, result.err)
			}
			switch result.status {
			case http.StatusOK:
				successes++
				refunds := stage3JObjects(t, result.data, "refunds")
				if len(refunds) != 1 {
					t.Fatalf("concurrent refund request %d refund rows = %d, want 1", index, len(refunds))
				}
				refundIDs = append(refundIDs, text(refunds[0], "id"))
			case http.StatusConflict:
				if result.code != "IDEMPOTENCY_IN_PROGRESS" && result.code != "IDEMPOTENCY_CONFLICT" {
					t.Fatalf("concurrent refund request %d code = %q, want IDEMPOTENCY_IN_PROGRESS or IDEMPOTENCY_CONFLICT", index, result.code)
				}
			default:
				t.Fatalf("concurrent refund request %d status = %d, want 200 or 409", index, result.status)
			}
		}
		if successes == 0 {
			t.Fatal("concurrent refund produced no successful response")
		}
		for _, id := range refundIDs {
			if id != refundIDs[0] {
				t.Fatalf("concurrent refund ids = %v, want a single refund id", refundIDs)
			}
		}

		detail := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		refunds := stage3JObjects(t, detail, "refunds")
		if len(refunds) != 1 {
			t.Fatalf("credit note refund rows after concurrent requests = %d, want 1", len(refunds))
		}
		if got := number(t, detail, "refunded_amount"); !nearlyEqual(got, 30) {
			t.Fatalf("credit note refunded_amount after concurrent requests = %v, want 30", got)
		}
	})

	t.Run("sales ship replay", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":        customerID,
			"warehouse_id":       warehouseID,
			"order_date":         today,
			"currency":           "CNY",
			"price_includes_tax": true,
			"remark":             stage4AUnique("stage4a-ship-order"),
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "10.00", "tax_rate_id": vat0ID},
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
		lineID := text(lines[0].(map[string]any), "id")

		key := stage4AUnique("stage4a-ship-key")
		body := map[string]any{
			"shipment_date": today,
			"remark":        stage4AUnique("stage4a-ship"),
			"lines": []map[string]any{
				{"order_line_id": lineID, "quantity": "1"},
			},
		}

		status, first := admin.postWithHeaders("/sales/orders/"+orderID+"/ship", body, stage4AHeaders(key))
		if status != http.StatusCreated {
			t.Fatalf("first ship status = %d, want %d, payload = %v", status, http.StatusCreated, first)
		}
		shipmentID := text(first.Data, "id")
		if shipmentID == "" {
			t.Fatalf("first ship returned empty shipment id: %v", first)
		}

		status, replay := admin.postWithHeaders("/sales/orders/"+orderID+"/ship", body, stage4AHeaders(key))
		if status != http.StatusCreated {
			t.Fatalf("replayed ship status = %d, want %d, payload = %v", status, http.StatusCreated, replay)
		}
		if got := text(replay.Data, "id"); got != shipmentID {
			t.Fatalf("replayed shipment id = %q, want %q", got, shipmentID)
		}

		shipments := listItems(admin, "/sales/shipments?order_id="+orderID+"&page_size=100", http.StatusOK)
		if len(shipments) != 1 {
			t.Fatalf("shipments for order %s = %d, want 1", orderID, len(shipments))
		}
		if got := text(shipments[0], "id"); got != shipmentID {
			t.Fatalf("listed shipment id = %q, want %q", got, shipmentID)
		}
	})

	t.Run("purchase receive replay", func(t *testing.T) {
		order := admin.mustPost("/purchase/orders", map[string]any{
			"supplier_id":        supplierID,
			"warehouse_id":       warehouseID,
			"order_date":         today,
			"currency":           "CNY",
			"price_includes_tax": false,
			"remark":             stage4AUnique("stage4a-receive-order"),
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "10.00"},
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
		lineID := text(lines[0].(map[string]any), "id")

		key := stage4AUnique("stage4a-receive-key")
		body := map[string]any{
			"receipt_date": today,
			"remark":       stage4AUnique("stage4a-receive"),
			"lines": []map[string]any{
				{"order_line_id": lineID, "quantity": "1"},
			},
		}

		status, first := admin.postWithHeaders("/purchase/orders/"+orderID+"/receive", body, stage4AHeaders(key))
		if status != http.StatusCreated {
			t.Fatalf("first receive status = %d, want %d, payload = %v", status, http.StatusCreated, first)
		}
		receiptID := text(first.Data, "id")
		if receiptID == "" {
			t.Fatalf("first receive returned empty receipt id: %v", first)
		}

		status, replay := admin.postWithHeaders("/purchase/orders/"+orderID+"/receive", body, stage4AHeaders(key))
		if status != http.StatusCreated {
			t.Fatalf("replayed receive status = %d, want %d, payload = %v", status, http.StatusCreated, replay)
		}
		if got := text(replay.Data, "id"); got != receiptID {
			t.Fatalf("replayed receipt id = %q, want %q", got, receiptID)
		}

		receipts := listItems(admin, "/purchase/receipts?order_id="+orderID+"&page_size=100", http.StatusOK)
		if len(receipts) != 1 {
			t.Fatalf("receipts for order %s = %d, want 1", orderID, len(receipts))
		}
		if got := text(receipts[0], "id"); got != receiptID {
			t.Fatalf("listed receipt id = %q, want %q", got, receiptID)
		}
	})

	t.Run("missing key still works", func(t *testing.T) {
		reference := stage4AUnique("stage4a-no-key-payment")
		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": today,
			"currency":     "CNY",
			"amount":       "10.00",
			"method":       "bank",
			"reference":    reference,
			"remark":       reference,
		}, http.StatusCreated)
		if text(payment, "id") == "" {
			t.Fatalf("payment without idempotency key returned empty id: %v", payment)
		}
	})
}

func stage4AUnique(prefix string) string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return prefix + "-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func stage4AHeaders(key string) map[string]string {
	return map[string]string{"Idempotency-Key": key}
}

func stage4AExpectReplay(t *testing.T, first, second map[string]any) {
	t.Helper()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("replayed response differs\nfirst:  %v\nsecond: %v", first, second)
	}
}

func stage4ARefundID(t *testing.T, note map[string]any) string {
	t.Helper()
	refunds := stage3JObjects(t, note, "refunds")
	if len(refunds) != 1 {
		t.Fatalf("refund rows = %d, want 1 (payload = %v)", len(refunds), note)
	}
	id := text(refunds[0], "id")
	if id == "" {
		t.Fatalf("refund id is empty: %v", refunds[0])
	}
	return id
}

func stage4AConcurrentPost(c *client, path string, body any, key string) []stage4AResult {
	const concurrency = 2
	results := make([]stage4AResult, concurrency)
	headers := stage4AHeaders(key)
	start := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(concurrency)
	done.Add(concurrency)
	for index := range results {
		go func(index int) {
			defer done.Done()
			ready.Done()
			<-start
			results[index] = stage4ARawPost(c, path, body, headers)
		}(index)
	}
	ready.Wait()
	close(start)
	done.Wait()
	return results
}

func stage4ARawPost(c *client, path string, body any, headers map[string]string) stage4AResult {
	raw, err := json.Marshal(body)
	if err != nil {
		return stage4AResult{err: fmt.Errorf("marshal request body: %w", err)}
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+apiPrefix+path, bytes.NewReader(raw))
	if err != nil {
		return stage4AResult{err: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := stage3JHTTPClient.Do(req)
	if err != nil {
		return stage4AResult{err: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return stage4AResult{status: resp.StatusCode, err: fmt.Errorf("read response body: %w", err)}
	}
	var payload apiPayload
	if len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			return stage4AResult{status: resp.StatusCode, err: fmt.Errorf("decode response body: %w", err)}
		}
	}
	return stage4AResult{status: resp.StatusCode, code: payload.Code, data: payload.Data}
}
