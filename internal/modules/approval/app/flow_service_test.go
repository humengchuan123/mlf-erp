package app

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
	"github.com/shopspring/decimal"
)

func testContext() context.Context {
	ctx := tenant.With(context.Background(), tenant.Info{ID: "tenant-1", OrgID: "org-1"})
	return auth.WithActor(ctx, auth.Actor{
		UserID:    "actor-1",
		TenantID:  "tenant-1",
		OrgID:     "org-1",
		DataScope: "all",
	})
}

func flowKey(tenantID, id string) string {
	return tenantID + "/" + id
}

func requestKey(tenantID, id string) string {
	return tenantID + "/" + id
}

func pendingKey(tenantID string, documentType domain.DocumentType, documentID string) string {
	return tenantID + "/" + string(documentType) + "/" + documentID
}

func cloneFlow(flow *domain.Flow) *domain.Flow {
	if flow == nil {
		return nil
	}
	copied := *flow
	copied.Steps = append([]domain.FlowStep(nil), flow.Steps...)
	return &copied
}

func cloneRequest(request *domain.Request) *domain.Request {
	if request == nil {
		return nil
	}
	copied := *request
	copied.Steps = append([]domain.FlowStep(nil), request.Steps...)
	copied.Actions = append([]domain.Action(nil), request.Actions...)
	return &copied
}

type fakeFlowRepo struct {
	flows           map[string]*domain.Flow
	requestedFlows  map[string]bool
	matching        *domain.Flow
	findMatchingErr error
	createErr       error
	updateErr       error
	deleteErr       error
	listErr         error
	getErr          error
	hasRequestsErr  error
	created         []*domain.Flow
	updated         []*domain.Flow
	deleted         []string
	updateCalls     int
	deleteCalls     int
	lastMatchTenant string
	lastMatchOrg    string
	lastMatchType   domain.DocumentType
	lastMatchAmount decimal.Decimal
}

func newFakeFlowRepo() *fakeFlowRepo {
	return &fakeFlowRepo{
		flows:          make(map[string]*domain.Flow),
		requestedFlows: make(map[string]bool),
	}
}

func (r *fakeFlowRepo) List(_ context.Context, tenantID string, documentType domain.DocumentType, active *bool) ([]*domain.Flow, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	result := make([]*domain.Flow, 0)
	for _, flow := range r.flows {
		if flow.TenantID != tenantID {
			continue
		}
		if documentType != "" && flow.DocumentType != documentType {
			continue
		}
		if active != nil && flow.Active != *active {
			continue
		}
		result = append(result, cloneFlow(flow))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (r *fakeFlowRepo) Get(_ context.Context, tenantID, id string) (*domain.Flow, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	flow, ok := r.flows[flowKey(tenantID, id)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneFlow(flow), nil
}

func (r *fakeFlowRepo) Create(_ context.Context, flow *domain.Flow) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.flows[flowKey(flow.TenantID, flow.ID)] = cloneFlow(flow)
	r.created = append(r.created, cloneFlow(flow))
	return nil
}

func (r *fakeFlowRepo) Update(_ context.Context, flow *domain.Flow) error {
	r.updateCalls++
	if r.updateErr != nil {
		return r.updateErr
	}
	if _, ok := r.flows[flowKey(flow.TenantID, flow.ID)]; !ok {
		return domain.ErrNotFound
	}
	r.flows[flowKey(flow.TenantID, flow.ID)] = cloneFlow(flow)
	r.updated = append(r.updated, cloneFlow(flow))
	return nil
}

func (r *fakeFlowRepo) Delete(_ context.Context, tenantID, id string) error {
	r.deleteCalls++
	if r.deleteErr != nil {
		return r.deleteErr
	}
	key := flowKey(tenantID, id)
	if _, ok := r.flows[key]; !ok {
		return domain.ErrNotFound
	}
	delete(r.flows, key)
	r.deleted = append(r.deleted, id)
	return nil
}

func (r *fakeFlowRepo) FindMatching(_ context.Context, tenantID, orgID string, documentType domain.DocumentType, baseAmount decimal.Decimal) (*domain.Flow, error) {
	r.lastMatchTenant = tenantID
	r.lastMatchOrg = orgID
	r.lastMatchType = documentType
	r.lastMatchAmount = baseAmount
	if r.findMatchingErr != nil {
		return nil, r.findMatchingErr
	}
	return cloneFlow(r.matching), nil
}

func (r *fakeFlowRepo) HasRequests(_ context.Context, tenantID, flowID string) (bool, error) {
	if r.hasRequestsErr != nil {
		return false, r.hasRequestsErr
	}
	return r.requestedFlows[flowKey(tenantID, flowID)], nil
}

type fakeApproverValidator struct {
	roles   map[string]bool
	users   map[string]bool
	roleErr error
	userErr error
}

func newFakeApproverValidator() *fakeApproverValidator {
	return &fakeApproverValidator{
		roles: map[string]bool{flowKey("tenant-1", "role-1"): true},
		users: map[string]bool{flowKey("tenant-1", "user-1"): true},
	}
}

func (v *fakeApproverValidator) RoleExists(_ context.Context, tenantID, roleID string) (bool, error) {
	if v.roleErr != nil {
		return false, v.roleErr
	}
	return v.roles[flowKey(tenantID, roleID)], nil
}

func (v *fakeApproverValidator) UserExists(_ context.Context, tenantID, userID string) (bool, error) {
	if v.userErr != nil {
		return false, v.userErr
	}
	return v.users[flowKey(tenantID, userID)], nil
}

type approveCall struct {
	tenantID  string
	requestID string
	actor     domain.Actor
	comment   string
}

type rejectCall struct {
	tenantID  string
	requestID string
	actor     domain.Actor
	comment   string
}

type cancelCall struct {
	tenantID     string
	documentType domain.DocumentType
	documentID   string
	actorID      string
	reason       string
}

type fakeRequestRepo struct {
	requests          map[string]*domain.Request
	pending           map[string]string
	createErr         error
	getErr            error
	listErr           error
	listPendingErr    error
	getPendingErr     error
	approveFinal      bool
	approveErr        error
	rejectErr         error
	cancelErr         error
	createCalls       int
	listPendingCalls  int
	approveCalls      []approveCall
	rejectCalls       []rejectCall
	cancelCalls       []cancelCall
	listPendingResult []*domain.Request
	lastListFilter    scope.Filter
	lastListPage      int
	lastListPageSize  int
}

func newFakeRequestRepo() *fakeRequestRepo {
	return &fakeRequestRepo{
		requests: make(map[string]*domain.Request),
		pending:  make(map[string]string),
	}
}

func (r *fakeRequestRepo) Create(_ context.Context, request *domain.Request) error {
	r.createCalls++
	if r.createErr != nil {
		return r.createErr
	}
	key := pendingKey(request.TenantID, request.DocumentType, request.DocumentID)
	if _, ok := r.pending[key]; ok {
		return domain.ErrPendingExists
	}
	r.requests[requestKey(request.TenantID, request.ID)] = cloneRequest(request)
	if request.Status == domain.RequestPending {
		r.pending[key] = request.ID
	}
	return nil
}

func (r *fakeRequestRepo) Get(_ context.Context, filter scope.Filter, tenantID, id string) (*domain.Request, error) {
	r.lastListFilter = filter
	if r.getErr != nil {
		return nil, r.getErr
	}
	request, ok := r.requests[requestKey(tenantID, id)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneRequest(request), nil
}

func (r *fakeRequestRepo) List(_ context.Context, filter scope.Filter, tenantID string, documentType domain.DocumentType, documentID, status string, page, pageSize int) ([]*domain.Request, int, error) {
	r.lastListFilter = filter
	r.lastListPage = page
	r.lastListPageSize = pageSize
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	result := make([]*domain.Request, 0)
	for _, request := range r.requests {
		if request.TenantID != tenantID {
			continue
		}
		if documentType != "" && request.DocumentType != documentType {
			continue
		}
		if documentID != "" && request.DocumentID != documentID {
			continue
		}
		if status != "" && string(request.Status) != status {
			continue
		}
		result = append(result, cloneRequest(request))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	total := len(result)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return result[start:end], total, nil
}

func (r *fakeRequestRepo) ListPending(_ context.Context, filter scope.Filter, tenantID string, _ domain.Actor) ([]*domain.Request, error) {
	r.listPendingCalls++
	r.lastListFilter = filter
	if r.listPendingErr != nil {
		return nil, r.listPendingErr
	}
	if r.listPendingResult != nil {
		result := make([]*domain.Request, 0, len(r.listPendingResult))
		for _, request := range r.listPendingResult {
			result = append(result, cloneRequest(request))
		}
		return result, nil
	}
	result := make([]*domain.Request, 0)
	for _, request := range r.requests {
		if request.TenantID == tenantID && request.Status == domain.RequestPending {
			result = append(result, cloneRequest(request))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (r *fakeRequestRepo) GetPendingByDocument(_ context.Context, tenantID string, documentType domain.DocumentType, documentID string) (*domain.Request, error) {
	if r.getPendingErr != nil {
		return nil, r.getPendingErr
	}
	id, ok := r.pending[pendingKey(tenantID, documentType, documentID)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	request, ok := r.requests[requestKey(tenantID, id)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneRequest(request), nil
}

func (r *fakeRequestRepo) Approve(_ context.Context, tenantID, requestID string, actor domain.Actor, comment string) (bool, error) {
	r.approveCalls = append(r.approveCalls, approveCall{
		tenantID: tenantID, requestID: requestID, actor: actor, comment: comment,
	})
	if r.approveErr != nil {
		return false, r.approveErr
	}
	return r.approveFinal, nil
}

func (r *fakeRequestRepo) Reject(_ context.Context, tenantID, requestID string, actor domain.Actor, comment string) error {
	r.rejectCalls = append(r.rejectCalls, rejectCall{
		tenantID: tenantID, requestID: requestID, actor: actor, comment: comment,
	})
	return r.rejectErr
}

func (r *fakeRequestRepo) CancelByDocument(_ context.Context, tenantID string, documentType domain.DocumentType, documentID, actorID, reason string) error {
	r.cancelCalls = append(r.cancelCalls, cancelCall{
		tenantID: tenantID, documentType: documentType, documentID: documentID, actorID: actorID, reason: reason,
	})
	if r.cancelErr != nil {
		return r.cancelErr
	}
	return nil
}

func (r *fakeRequestRepo) addPending(request *domain.Request) {
	r.requests[requestKey(request.TenantID, request.ID)] = cloneRequest(request)
	r.pending[pendingKey(request.TenantID, request.DocumentType, request.DocumentID)] = request.ID
}

func validFlowInput() FlowInput {
	return FlowInput{
		Name:            "Sales order amount approval",
		DocumentType:    domain.DocumentSalesOrder,
		OrgID:           "org-1",
		ConditionType:   domain.ConditionAmountGTE,
		ThresholdAmount: decimal.NewFromInt(1000),
		Priority:        10,
		Active:          true,
		Steps: []FlowStepInput{
			{StepNo: 1, Name: "Sales manager", ApproverType: domain.ApproverRole, ApproverID: "role-1"},
			{StepNo: 2, Name: "Finance manager", ApproverType: domain.ApproverUser, ApproverID: "user-1"},
		},
	}
}

func existingFlow() *domain.Flow {
	now := time.Unix(1700000000, 0)
	return &domain.Flow{
		ID:              "flow-1",
		TenantID:        "tenant-1",
		OrgID:           "org-1",
		Name:            "Existing flow",
		DocumentType:    domain.DocumentSalesOrder,
		ConditionType:   domain.ConditionAlways,
		ThresholdAmount: decimal.Zero,
		Active:          true,
		CreatedBy:       "creator-1",
		UpdatedBy:       "creator-1",
		CreatedAt:       now,
		UpdatedAt:       now,
		Steps: []domain.FlowStep{
			{ID: "step-1", TenantID: "tenant-1", FlowID: "flow-1", StepNo: 1, Name: "Approver", ApproverType: domain.ApproverRole, ApproverID: "role-1", CreatedAt: now},
		},
	}
}

func seedFlow(repo *fakeFlowRepo, flow *domain.Flow) {
	repo.flows[flowKey(flow.TenantID, flow.ID)] = cloneFlow(flow)
}

func TestFlowServiceCreateValidFlow(t *testing.T) {
	repo := newFakeFlowRepo()
	service := NewFlowService(repo, newFakeApproverValidator())

	flow, err := service.Create(testContext(), validFlowInput())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if flow.ID == "" {
		t.Fatal("Create() flow ID is empty")
	}
	if flow.TenantID != "tenant-1" {
		t.Fatalf("TenantID = %q, want tenant-1", flow.TenantID)
	}
	if flow.CreatedBy != "actor-1" || flow.UpdatedBy != "actor-1" {
		t.Fatalf("audit users = %q/%q, want actor-1", flow.CreatedBy, flow.UpdatedBy)
	}
	if flow.CreatedAt.IsZero() || flow.UpdatedAt.IsZero() {
		t.Fatal("Create() did not set timestamps")
	}
	if len(flow.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(flow.Steps))
	}
	for _, step := range flow.Steps {
		if step.ID == "" || step.FlowID != flow.ID || step.TenantID != "tenant-1" {
			t.Fatalf("step not linked to flow: %+v", step)
		}
	}
	if _, ok := repo.flows[flowKey("tenant-1", flow.ID)]; !ok {
		t.Fatal("flow was not persisted")
	}
}

func TestFlowServiceCreateValidatesInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FlowInput)
	}{
		{name: "missing steps", mutate: func(input *FlowInput) { input.Steps = nil }},
		{name: "non sequential steps", mutate: func(input *FlowInput) {
			input.Steps = []FlowStepInput{
				{StepNo: 1, Name: "First", ApproverType: domain.ApproverRole, ApproverID: "role-1"},
				{StepNo: 3, Name: "Third", ApproverType: domain.ApproverUser, ApproverID: "user-1"},
			}
		}},
		{name: "step numbers start at zero", mutate: func(input *FlowInput) {
			input.Steps[0].StepNo = 0
		}},
		{name: "duplicate step numbers", mutate: func(input *FlowInput) {
			input.Steps[1].StepNo = 1
		}},
		{name: "invalid approver type", mutate: func(input *FlowInput) {
			input.Steps[0].ApproverType = "group"
		}},
		{name: "missing approver", mutate: func(input *FlowInput) {
			input.Steps[0].ApproverID = ""
		}},
		{name: "unknown role", mutate: func(input *FlowInput) {
			input.Steps[0].ApproverID = "role-missing"
		}},
		{name: "unknown user", mutate: func(input *FlowInput) {
			input.Steps[1].ApproverID = "user-missing"
		}},
		{name: "negative threshold", mutate: func(input *FlowInput) {
			input.ThresholdAmount = decimal.NewFromInt(-1)
		}},
		{name: "invalid document type", mutate: func(input *FlowInput) {
			input.DocumentType = "invoice"
		}},
		{name: "invalid condition type", mutate: func(input *FlowInput) {
			input.ConditionType = "amount_lt"
		}},
		{name: "missing name", mutate: func(input *FlowInput) {
			input.Name = " "
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeFlowRepo()
			service := NewFlowService(repo, newFakeApproverValidator())
			input := validFlowInput()
			tt.mutate(&input)

			if _, err := service.Create(testContext(), input); !errors.Is(err, domain.ErrInvalidFlow) {
				t.Fatalf("Create() error = %v, want ErrInvalidFlow", err)
			}
			if len(repo.created) != 0 {
				t.Fatal("invalid flow was persisted")
			}
		})
	}
}

func TestFlowServiceCreatePropagatesValidatorError(t *testing.T) {
	validator := newFakeApproverValidator()
	validator.roleErr = errors.New("role lookup failed")
	service := NewFlowService(newFakeFlowRepo(), validator)

	_, err := service.Create(testContext(), validFlowInput())
	if !errors.Is(err, validator.roleErr) {
		t.Fatalf("Create() error = %v, want validator error", err)
	}
}

func TestFlowServiceUpdateBlockedByRequestHistory(t *testing.T) {
	repo := newFakeFlowRepo()
	seedFlow(repo, existingFlow())
	repo.requestedFlows[flowKey("tenant-1", "flow-1")] = true
	service := NewFlowService(repo, newFakeApproverValidator())

	_, err := service.Update(testContext(), "flow-1", validFlowInput())
	if !errors.Is(err, domain.ErrFlowInUse) {
		t.Fatalf("Update() error = %v, want ErrFlowInUse", err)
	}
	if repo.updateCalls != 0 {
		t.Fatal("Update() persisted a flow with request history")
	}
}

func TestFlowServiceDeleteBlockedByRequestHistory(t *testing.T) {
	repo := newFakeFlowRepo()
	seedFlow(repo, existingFlow())
	repo.requestedFlows[flowKey("tenant-1", "flow-1")] = true
	service := NewFlowService(repo, newFakeApproverValidator())

	err := service.Delete(testContext(), "flow-1")
	if !errors.Is(err, domain.ErrFlowInUse) {
		t.Fatalf("Delete() error = %v, want ErrFlowInUse", err)
	}
	if repo.deleteCalls != 0 {
		t.Fatal("Delete() removed a flow with request history")
	}
}

func TestFlowServiceUpdateSuccess(t *testing.T) {
	repo := newFakeFlowRepo()
	seedFlow(repo, existingFlow())
	service := NewFlowService(repo, newFakeApproverValidator())
	input := validFlowInput()
	input.Name = "Updated flow"

	flow, err := service.Update(testContext(), "flow-1", input)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if flow.Name != "Updated flow" {
		t.Fatalf("Name = %q, want Updated flow", flow.Name)
	}
	if flow.CreatedBy != "creator-1" || !flow.CreatedAt.Equal(existingFlow().CreatedAt) {
		t.Fatalf("creation metadata not preserved: %+v", flow)
	}
	if flow.UpdatedBy != "actor-1" {
		t.Fatalf("UpdatedBy = %q, want actor-1", flow.UpdatedBy)
	}
	for _, step := range flow.Steps {
		if step.FlowID != "flow-1" {
			t.Fatalf("step FlowID = %q, want flow-1", step.FlowID)
		}
	}
	if len(repo.updated) != 1 {
		t.Fatalf("updated flows = %d, want 1", len(repo.updated))
	}
}

func TestFlowServiceDeleteSuccess(t *testing.T) {
	repo := newFakeFlowRepo()
	seedFlow(repo, existingFlow())
	service := NewFlowService(repo, newFakeApproverValidator())

	if err := service.Delete(testContext(), "flow-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok := repo.flows[flowKey("tenant-1", "flow-1")]; ok {
		t.Fatal("flow was not deleted")
	}
}

func TestFlowServiceUpdateMissingFlow(t *testing.T) {
	service := NewFlowService(newFakeFlowRepo(), newFakeApproverValidator())

	_, err := service.Update(testContext(), "missing", validFlowInput())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Update() error = %v, want ErrNotFound", err)
	}
}
