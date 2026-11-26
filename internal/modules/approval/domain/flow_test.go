package domain

import (
	"errors"
	"testing"
)

func TestResolveApprovalStepsNoSteps(t *testing.T) {
	if _, err := ResolveApprovalSteps(nil, "user-1"); !errors.Is(err, ErrNoApprover) {
		t.Fatalf("ResolveApprovalSteps() error = %v, want ErrNoApprover", err)
	}
	if _, err := ResolveApprovalSteps([]FlowStep{}, "user-1"); !errors.Is(err, ErrNoApprover) {
		t.Fatalf("ResolveApprovalSteps() error = %v, want ErrNoApprover", err)
	}
}

func TestResolveApprovalStepsAllSelf(t *testing.T) {
	steps := []FlowStep{
		{StepNo: 1, ApproverType: ApproverUser, ApproverID: "requester"},
		{StepNo: 2, ApproverType: ApproverUser, ApproverID: "requester"},
	}

	if _, err := ResolveApprovalSteps(steps, "requester"); !errors.Is(err, ErrNoApprover) {
		t.Fatalf("ResolveApprovalSteps() error = %v, want ErrNoApprover", err)
	}
}

func TestResolveApprovalStepsMixedSorted(t *testing.T) {
	steps := []FlowStep{
		{ID: "fs-4", StepNo: 4, ApproverType: ApproverUser, ApproverID: "requester"},
		{ID: "fs-1", StepNo: 1, ApproverType: ApproverUser, ApproverID: "requester"},
		{ID: "fs-3", StepNo: 3, ApproverType: ApproverRole, ApproverID: "role-finance"},
		{ID: "fs-2", StepNo: 2, ApproverType: ApproverUser, ApproverID: "user-other"},
	}

	resolved, err := ResolveApprovalSteps(steps, "requester")
	if err != nil {
		t.Fatalf("ResolveApprovalSteps() error = %v", err)
	}
	if resolved.CurrentStep != 2 {
		t.Fatalf("CurrentStep = %d, want 2", resolved.CurrentStep)
	}
	if len(resolved.Skipped) != 2 {
		t.Fatalf("Skipped = %d steps, want 2", len(resolved.Skipped))
	}
	if resolved.Skipped[0].ID != "fs-1" || resolved.Skipped[1].ID != "fs-4" {
		t.Fatalf("Skipped = %+v, want steps 1 and 4 in order", resolved.Skipped)
	}
}

func TestResolveApprovalStepsRetainsRequesterRoleStep(t *testing.T) {
	steps := []FlowStep{
		{ID: "fs-1", StepNo: 1, ApproverType: ApproverRole, ApproverID: "requester-role"},
		{ID: "fs-2", StepNo: 2, ApproverType: ApproverUser, ApproverID: "requester"},
	}

	resolved, err := ResolveApprovalSteps(steps, "requester")
	if err != nil {
		t.Fatalf("ResolveApprovalSteps() error = %v", err)
	}
	if resolved.CurrentStep != 1 {
		t.Fatalf("CurrentStep = %d, want 1 (role step retained)", resolved.CurrentStep)
	}
	if len(resolved.Skipped) != 1 || resolved.Skipped[0].ID != "fs-2" {
		t.Fatalf("Skipped = %+v, want only user step fs-2", resolved.Skipped)
	}
}
