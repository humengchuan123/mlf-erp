package domain

import "errors"

var (
	ErrNotFound      = errors.New("approval not found")
	ErrInvalidFlow   = errors.New("invalid approval flow")
	ErrFlowInUse     = errors.New("approval flow has requests")
	ErrPendingExists = errors.New("pending approval already exists")
	ErrNoPending     = errors.New("no pending approval")
	ErrNotApprover   = errors.New("not the current approver")
	ErrSelfApproval  = errors.New("cannot approve own request")
	ErrAlreadyActed  = errors.New("approval step already acted")
	ErrNoApprover    = errors.New("no eligible approver")
	ErrInvalidStatus = errors.New("invalid approval status")
)
