package domain

type Status string

const (
	StatusDraft            Status = "draft"
	StatusSubmitted        Status = "submitted"
	StatusApproved         Status = "approved"
	StatusPartiallyShipped Status = "partially_shipped"
	StatusShipped          Status = "shipped"
	StatusCancelled        Status = "cancelled"
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

func (s Status) CanShip() bool {
	return s == StatusApproved || s == StatusPartiallyShipped
}
