package domain

import (
	"context"

	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
	"github.com/shopspring/decimal"
)

type FlowRepository interface {
	List(ctx context.Context, tenantID string, documentType DocumentType, active *bool) ([]*Flow, error)
	Get(ctx context.Context, tenantID, id string) (*Flow, error)
	Create(ctx context.Context, flow *Flow) error
	Update(ctx context.Context, flow *Flow) error
	Delete(ctx context.Context, tenantID, id string) error
	FindMatching(ctx context.Context, tenantID, orgID string, documentType DocumentType, baseAmount decimal.Decimal) (*Flow, error)
	HasRequests(ctx context.Context, tenantID, flowID string) (bool, error)
}

type RequestRepository interface {
	Create(ctx context.Context, request *Request) error
	Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Request, error)
	List(ctx context.Context, filter scope.Filter, tenantID string, documentType DocumentType, documentID, status string, page, pageSize int) ([]*Request, int, error)
	ListPending(ctx context.Context, filter scope.Filter, tenantID string, actor Actor) ([]*Request, error)
	GetPendingByDocument(ctx context.Context, tenantID string, documentType DocumentType, documentID string) (*Request, error)
	Approve(ctx context.Context, tenantID, requestID string, actor Actor, comment string) (bool, error)
	Reject(ctx context.Context, tenantID, requestID string, actor Actor, comment string) error
	CancelByDocument(ctx context.Context, tenantID string, documentType DocumentType, documentID, actorID, reason string) error
}

type ApproverValidator interface {
	RoleExists(ctx context.Context, tenantID, roleID string) (bool, error)
	UserExists(ctx context.Context, tenantID, userID string) (bool, error)
}
