package app

import (
	"errors"
	"testing"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
	"github.com/shopspring/decimal"
)

func requestFlow() *domain.Flow {
	return &domain.Flow{
		ID:              "flow-1",
		TenantID:        "tenant-1",
		OrgID:           "org-1",
		Name:            "Sales approval",
		DocumentType:    domain.DocumentSalesOrder,
		ConditionType:   domain.ConditionAlways,
		ThresholdAmount: decimal.Zero,
		Active:          true,
		Steps: []domain.FlowStep{
			{ID: "fs-1", TenantID: "tenant-1", FlowID: "flow-1", StepNo: 1, Name: "Sales manager", ApproverType: domain.ApproverRole, ApproverID: "role-1"},
			{ID: "fs-2", TenantID: "tenant-1", FlowID: "flow-1", StepNo: 2, Name: "Requester", ApproverType: domain.ApproverUser, ApproverID: "user-self"},
			{ID: "fs-3", TenantID: "tenant-1", FlowID: "flow-1", StepNo: 3, Name: "Finance", ApproverType: domain.ApproverUser, ApproverID: "user-other"},
		},
	}
}

func pendingRequest() *domain.Request {
	return &domain.Request{
		ID:           "req-1",
		TenantID:     "tenant-1",
		OrgID:        "org-1",
		DocumentType: domain.DocumentSalesOrder,
		DocumentID:   "doc-1",
		DocumentCode: "SO-001",
		FlowID:       "flow-1",
		Status:       domain.RequestPending,
		CurrentStep:  1,
		RequestedBy:  "user-self",
		RequestedAt:  time.Unix(1700000000, 0),
		Amount:       decimal.NewFromInt(125),
		BaseAmount:   decimal.NewFromInt(100),
		Currency:     "USD",
		CreatedAt:    time.Unix(1700000000, 0),
		UpdatedAt:    time.Unix(1700000000, 0),
	}
}

func startInput() domain.StartInput {
	return domain.StartInput{
		TenantID:     "tenant-1",
		OrgID:        "org-1",
		DocumentType: domain.DocumentSalesOrder,
		DocumentID:   "doc-1",
		DocumentCode: "SO-001",
		RequestedBy:  "user-self",
		Amount:       decimal.RequireFromString("125.5000"),
		BaseAmount:   decimal.RequireFromString("100.2500"),
		Currency:     "USD",
	}
}

func TestRequestServiceStartCreatesPendingRequest(t *testing.T) {
	flowRepo := newFakeFlowRepo()
	flowRepo.matching = requestFlow()
	requestRepo := newFakeRequestRepo()
	service := NewRequestService(flowRepo, requestRepo)

	started, err := service.Start(testContext(), startInput())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !started {
		t.Fatal("Start() = false, want true")
	}
	if requestRepo.createCalls != 1 {
		t.Fatalf("Create() calls = %d, want 1", requestRepo.createCalls)
	}
	request := requestRepo.requests[requestKey("tenant-1", requestRepo.pending[pendingKey("tenant-1", domain.DocumentSalesOrder, "doc-1")])]
	if request == nil {
		t.Fatal("pending request was not stored")
	}
	if request.ID == "" {
		t.Fatal("request ID is empty")
	}
	if request.TenantID != "tenant-1" || request.OrgID != "org-1" {
		t.Fatalf("tenant/org = %q/%q, want tenant-1/org-1", request.TenantID, request.OrgID)
	}
	if request.DocumentType != domain.DocumentSalesOrder || request.DocumentID != "doc-1" || request.DocumentCode != "SO-001" {
		t.Fatalf("document fields not preserved: %+v", request)
	}
	if request.FlowID != "flow-1" {
		t.Fatalf("FlowID = %q, want flow-1", request.FlowID)
	}
	if request.Status != domain.RequestPending {
		t.Fatalf("Status = %q, want pending", request.Status)
	}
	if request.CurrentStep != 1 {
		t.Fatalf("CurrentStep = %d, want 1", request.CurrentStep)
	}
	if request.RequestedBy != "user-self" {
		t.Fatalf("RequestedBy = %q, want user-self", request.RequestedBy)
	}
	if request.RequestedAt.IsZero() {
		t.Fatal("RequestedAt was not set")
	}
	if !request.Amount.Equal(decimal.RequireFromString("125.5000")) {
		t.Fatalf("Amount = %s, want 125.5000", request.Amount)
	}
	if !request.BaseAmount.Equal(decimal.RequireFromString("100.2500")) {
		t.Fatalf("BaseAmount = %s, want 100.2500", request.BaseAmount)
	}
	if request.Currency != "USD" {
		t.Fatalf("Currency = %q, want USD", request.Currency)
	}
	if len(request.Steps) != 3 {
		t.Fatalf("steps = %d, want all 3 flow steps", len(request.Steps))
	}
	if request.Steps[0].ID != "fs-1" || request.Steps[1].ID != "fs-2" || request.Steps[2].ID != "fs-3" {
		t.Fatalf("unexpected steps: %+v", request.Steps)
	}
	if len(request.Actions) != 1 {
		t.Fatalf("actions = %d, want 1 skip action", len(request.Actions))
	}
	skip := request.Actions[0]
	if skip.Action != domain.ActionSkip || skip.StepNo != 2 {
		t.Fatalf("skip action = %+v, want skip for step 2", skip)
	}
	if skip.ActorID != "user-self" || skip.Comment != "applicant step skipped" {
		t.Fatalf("skip action actor/comment = %q/%q", skip.ActorID, skip.Comment)
	}
	if flowRepo.lastMatchTenant != "tenant-1" || flowRepo.lastMatchOrg != "org-1" {
		t.Fatalf("FindMatching() tenant/org = %q/%q", flowRepo.lastMatchTenant, flowRepo.lastMatchOrg)
	}
	if flowRepo.lastMatchType != domain.DocumentSalesOrder {
		t.Fatalf("FindMatching() document type = %q", flowRepo.lastMatchType)
	}
	if !flowRepo.lastMatchAmount.Equal(decimal.RequireFromString("100.2500")) {
		t.Fatalf("FindMatching() amount = %s, want base amount", flowRepo.lastMatchAmount)
	}
}

func TestRequestServiceStartSkipsSelfStepAndStartsAtNextStep(t *testing.T) {
	flow := requestFlow()
	flow.Steps = []domain.FlowStep{
		{ID: "fs-1", TenantID: "tenant-1", FlowID: "flow-1", StepNo: 1, Name: "Requester", ApproverType: domain.ApproverUser, ApproverID: "user-self"},
		{ID: "fs-2", TenantID: "tenant-1", FlowID: "flow-1", StepNo: 2, Name: "Manager", ApproverType: domain.ApproverRole, ApproverID: "role-1"},
	}
	flowRepo := newFakeFlowRepo()
	flowRepo.matching = flow
	requestRepo := newFakeRequestRepo()
	service := NewRequestService(flowRepo, requestRepo)

	started, err := service.Start(testContext(), startInput())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !started {
		t.Fatal("Start() = false, want true")
	}
	request := requestRepo.requests[requestKey("tenant-1", requestRepo.pending[pendingKey("tenant-1", domain.DocumentSalesOrder, "doc-1")])]
	if request == nil {
		t.Fatal("request was not stored")
	}
	if request.CurrentStep != 2 {
		t.Fatalf("CurrentStep = %d, want 2", request.CurrentStep)
	}
	if len(request.Steps) != 2 || request.Steps[0].ID != "fs-1" || request.Steps[1].ID != "fs-2" {
		t.Fatalf("unexpected steps: %+v", request.Steps)
	}
	if len(request.Actions) != 1 || request.Actions[0].Action != domain.ActionSkip || request.Actions[0].StepNo != 1 {
		t.Fatalf("skip actions = %+v, want skip for step 1", request.Actions)
	}
}

func TestRequestServiceStartAllStepsSelf(t *testing.T) {
	flow := requestFlow()
	flow.Steps = []domain.FlowStep{
		{ID: "fs-1", TenantID: "tenant-1", FlowID: "flow-1", StepNo: 1, Name: "Requester", ApproverType: domain.ApproverUser, ApproverID: "user-self"},
	}
	flowRepo := newFakeFlowRepo()
	flowRepo.matching = flow
	requestRepo := newFakeRequestRepo()
	service := NewRequestService(flowRepo, requestRepo)

	started, err := service.Start(testContext(), startInput())
	if !errors.Is(err, domain.ErrNoApprover) {
		t.Fatalf("Start() error = %v, want ErrNoApprover", err)
	}
	if started {
		t.Fatal("Start() = true, want false")
	}
	if requestRepo.createCalls != 0 {
		t.Fatal("request was created without an eligible approver")
	}
}

func TestRequestServiceStartNoMatchingFlow(t *testing.T) {
	flowRepo := newFakeFlowRepo()
	requestRepo := newFakeRequestRepo()
	service := NewRequestService(flowRepo, requestRepo)

	started, err := service.Start(testContext(), startInput())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started {
		t.Fatal("Start() = true, want false")
	}
	if requestRepo.createCalls != 0 {
		t.Fatal("request was created without a matching flow")
	}
	if flowRepo.lastMatchTenant != "tenant-1" {
		t.Fatalf("FindMatching() tenant = %q, want tenant-1", flowRepo.lastMatchTenant)
	}
}

func TestRequestServiceStartPendingExists(t *testing.T) {
	flowRepo := newFakeFlowRepo()
	flowRepo.matching = requestFlow()
	requestRepo := newFakeRequestRepo()
	requestRepo.addPending(pendingRequest())
	service := NewRequestService(flowRepo, requestRepo)

	started, err := service.Start(testContext(), startInput())
	if !errors.Is(err, domain.ErrPendingExists) {
		t.Fatalf("Start() error = %v, want ErrPendingExists", err)
	}
	if started {
		t.Fatal("Start() = true, want false")
	}
}

func TestRequestServiceStartUsesContextTenantWhenInputTenantEmpty(t *testing.T) {
	flowRepo := newFakeFlowRepo()
	flowRepo.matching = requestFlow()
	requestRepo := newFakeRequestRepo()
	service := NewRequestService(flowRepo, requestRepo)
	input := startInput()
	input.TenantID = ""

	started, err := service.Start(testContext(), input)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !started {
		t.Fatal("Start() = false, want true")
	}
	if flowRepo.lastMatchTenant != "tenant-1" {
		t.Fatalf("FindMatching() tenant = %q, want tenant-1", flowRepo.lastMatchTenant)
	}
	if requestRepo.createCalls != 1 {
		t.Fatalf("Create() calls = %d, want 1", requestRepo.createCalls)
	}
}

func TestRequestServicePending(t *testing.T) {
	requestRepo := newFakeRequestRepo()
	service := NewRequestService(newFakeFlowRepo(), requestRepo)

	pending, err := service.Pending(testContext(), domain.DocumentSalesOrder, "doc-1")
	if err != nil {
		t.Fatalf("Pending() error = %v", err)
	}
	if pending {
		t.Fatal("Pending() = true, want false")
	}

	requestRepo.addPending(pendingRequest())
	pending, err = service.Pending(testContext(), domain.DocumentSalesOrder, "doc-1")
	if err != nil {
		t.Fatalf("Pending() error = %v", err)
	}
	if !pending {
		t.Fatal("Pending() = false, want true")
	}
}

func TestRequestServiceApproveDelegates(t *testing.T) {
	for _, final := range []bool{true, false} {
		name := "not final"
		if final {
			name = "final"
		}
		t.Run(name, func(t *testing.T) {
			requestRepo := newFakeRequestRepo()
			requestRepo.addPending(pendingRequest())
			requestRepo.approveFinal = final
			service := NewRequestService(newFakeFlowRepo(), requestRepo)
			actor := domain.Actor{UserID: "actor-1", IsSuperAdmin: true}

			got, err := service.Approve(testContext(), domain.DocumentSalesOrder, "doc-1", actor, "looks good")
			if err != nil {
				t.Fatalf("Approve() error = %v", err)
			}
			if got != final {
				t.Fatalf("Approve() = %v, want %v", got, final)
			}
			if len(requestRepo.approveCalls) != 1 {
				t.Fatalf("approve calls = %d, want 1", len(requestRepo.approveCalls))
			}
			call := requestRepo.approveCalls[0]
			if call.tenantID != "tenant-1" || call.requestID != "req-1" {
				t.Fatalf("unexpected approve call: %+v", call)
			}
			if call.actor != actor || call.comment != "looks good" {
				t.Fatalf("unexpected actor/comment: %+v", call)
			}
		})
	}
}

func TestRequestServiceApproveNoPending(t *testing.T) {
	requestRepo := newFakeRequestRepo()
	service := NewRequestService(newFakeFlowRepo(), requestRepo)

	_, err := service.Approve(testContext(), domain.DocumentSalesOrder, "doc-1", domain.Actor{UserID: "actor-1"}, "")
	if !errors.Is(err, domain.ErrNoPending) {
		t.Fatalf("Approve() error = %v, want ErrNoPending", err)
	}
	if len(requestRepo.approveCalls) != 0 {
		t.Fatal("repository Approve was called without a pending request")
	}
}

func TestRequestServiceRejectDelegates(t *testing.T) {
	requestRepo := newFakeRequestRepo()
	requestRepo.addPending(pendingRequest())
	service := NewRequestService(newFakeFlowRepo(), requestRepo)
	actor := domain.Actor{UserID: "actor-1"}

	if err := service.Reject(testContext(), domain.DocumentSalesOrder, "doc-1", actor, "missing invoice"); err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if len(requestRepo.rejectCalls) != 1 {
		t.Fatalf("reject calls = %d, want 1", len(requestRepo.rejectCalls))
	}
	call := requestRepo.rejectCalls[0]
	if call.tenantID != "tenant-1" || call.requestID != "req-1" {
		t.Fatalf("unexpected reject call: %+v", call)
	}
	if call.actor != actor || call.comment != "missing invoice" {
		t.Fatalf("unexpected actor/comment: %+v", call)
	}
}

func TestRequestServiceRejectNoPending(t *testing.T) {
	requestRepo := newFakeRequestRepo()
	service := NewRequestService(newFakeFlowRepo(), requestRepo)

	err := service.Reject(testContext(), domain.DocumentSalesOrder, "doc-1", domain.Actor{UserID: "actor-1"}, "no")
	if !errors.Is(err, domain.ErrNoPending) {
		t.Fatalf("Reject() error = %v, want ErrNoPending", err)
	}
	if len(requestRepo.rejectCalls) != 0 {
		t.Fatal("repository Reject was called without a pending request")
	}
}

func TestRequestServiceCancelDelegates(t *testing.T) {
	requestRepo := newFakeRequestRepo()
	requestRepo.addPending(pendingRequest())
	service := NewRequestService(newFakeFlowRepo(), requestRepo)

	if err := service.Cancel(testContext(), domain.DocumentSalesOrder, "doc-1", "actor-1", "customer cancelled"); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if len(requestRepo.cancelCalls) != 1 {
		t.Fatalf("cancel calls = %d, want 1", len(requestRepo.cancelCalls))
	}
	call := requestRepo.cancelCalls[0]
	if call.tenantID != "tenant-1" || call.documentType != domain.DocumentSalesOrder || call.documentID != "doc-1" {
		t.Fatalf("unexpected cancel call: %+v", call)
	}
	if call.actorID != "actor-1" || call.reason != "customer cancelled" {
		t.Fatalf("unexpected actor/reason: %+v", call)
	}
}

func TestRequestServiceCancelNoPendingIsNoOp(t *testing.T) {
	requestRepo := newFakeRequestRepo()
	requestRepo.cancelErr = domain.ErrNoPending
	service := NewRequestService(newFakeFlowRepo(), requestRepo)

	if err := service.Cancel(testContext(), domain.DocumentSalesOrder, "doc-1", "actor-1", "cancelled"); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	if len(requestRepo.cancelCalls) != 1 {
		t.Fatal("repository CancelByDocument was not called")
	}
}

func TestRequestServiceCancelPropagatesError(t *testing.T) {
	requestRepo := newFakeRequestRepo()
	requestRepo.cancelErr = errors.New("database failed")
	service := NewRequestService(newFakeFlowRepo(), requestRepo)

	if err := service.Cancel(testContext(), domain.DocumentSalesOrder, "doc-1", "actor-1", "cancelled"); !errors.Is(err, requestRepo.cancelErr) {
		t.Fatalf("Cancel() error = %v, want repository error", err)
	}
}

func TestRequestServiceGetAndListDelegate(t *testing.T) {
	requestRepo := newFakeRequestRepo()
	requestRepo.addPending(pendingRequest())
	service := NewRequestService(newFakeFlowRepo(), requestRepo)

	request, err := service.Get(testContext(), "req-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if request.ID != "req-1" {
		t.Fatalf("Get() ID = %q, want req-1", request.ID)
	}
	if requestRepo.lastListFilter.TenantID != "tenant-1" || requestRepo.lastListFilter.UserID != "actor-1" {
		t.Fatalf("Get() scope = %+v", requestRepo.lastListFilter)
	}

	requests, total, err := service.List(testContext(), domain.DocumentSalesOrder, "doc-1", string(domain.RequestPending), 1, 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 || len(requests) != 1 {
		t.Fatalf("List() total/len = %d/%d, want 1/1", total, len(requests))
	}
	if requestRepo.lastListPage != 1 || requestRepo.lastListPageSize != 10 {
		t.Fatalf("List() page/size = %d/%d, want 1/10", requestRepo.lastListPage, requestRepo.lastListPageSize)
	}

	pending, err := service.ListPending(testContext(), domain.Actor{UserID: "actor-1"})
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(pending) != 1 || pending[0].ID != "req-1" {
		t.Fatalf("ListPending() = %+v, want req-1", pending)
	}
	if requestRepo.listPendingCalls != 1 {
		t.Fatalf("ListPending() calls = %d, want 1", requestRepo.listPendingCalls)
	}
}
