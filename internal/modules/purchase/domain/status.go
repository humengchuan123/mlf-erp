package domain

type Status string

const (
	StatusDraft             Status = "draft"
	StatusSubmitted         Status = "submitted"
	StatusApproved          Status = "approved"
	StatusPartiallyReceived Status = "partially_received"
	StatusReceived          Status = "received"
	StatusClosed            Status = "closed"
	StatusCancelled         Status = "cancelled"
)

func (s Status) CanEdit() bool {
	return s == StatusDraft
}

func (s Status) CanSubmit() bool {
	return s == StatusDraft
}

func (s Status) CanApprove() bool {
	return s == StatusSubmitted
}

func (s Status) CanCancel() bool {
	return s == StatusDraft || s == StatusSubmitted || s == StatusApproved
}

func (s Status) CanReceive() bool {
	return s == StatusApproved || s == StatusPartiallyReceived
}
