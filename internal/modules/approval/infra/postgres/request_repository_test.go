package postgres

import (
	"testing"

	"github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
)

func TestNextApproverStepSkipsRequesterUserStep(t *testing.T) {
	steps := []domain.FlowStep{
		{StepNo: 1, ApproverType: domain.ApproverRole, ApproverID: "role-1"},
		{StepNo: 2, ApproverType: domain.ApproverUser, ApproverID: "requester"},
		{StepNo: 3, ApproverType: domain.ApproverUser, ApproverID: "finance"},
	}

	next, ok := nextApproverStep(steps, 1, "requester")
	if !ok {
		t.Fatal("nextApproverStep() = no step, want step 3")
	}
	if next.StepNo != 3 || next.ApproverID != "finance" {
		t.Fatalf("nextApproverStep() = step %d approver %q, want step 3 finance", next.StepNo, next.ApproverID)
	}
}

func TestNextApproverStepDoesNotSkipRequesterRoleStep(t *testing.T) {
	steps := []domain.FlowStep{
		{StepNo: 1, ApproverType: domain.ApproverUser, ApproverID: "manager"},
		{StepNo: 2, ApproverType: domain.ApproverRole, ApproverID: "requester-role"},
	}

	next, ok := nextApproverStep(steps, 1, "requester")
	if !ok {
		t.Fatal("nextApproverStep() = no step, want role step 2")
	}
	if next.StepNo != 2 {
		t.Fatalf("nextApproverStep() step = %d, want 2", next.StepNo)
	}
}

func TestNextApproverStepNoneAfterSkippingSelf(t *testing.T) {
	steps := []domain.FlowStep{
		{StepNo: 1, ApproverType: domain.ApproverRole, ApproverID: "role-1"},
		{StepNo: 2, ApproverType: domain.ApproverUser, ApproverID: "requester"},
	}

	next, ok := nextApproverStep(steps, 1, "requester")
	if ok {
		t.Fatalf("nextApproverStep() = step %d, want none", next.StepNo)
	}
}

func TestNextApproverStepChoosesLowestStep(t *testing.T) {
	steps := []domain.FlowStep{
		{StepNo: 3, ApproverType: domain.ApproverUser, ApproverID: "finance"},
		{StepNo: 1, ApproverType: domain.ApproverUser, ApproverID: "manager"},
		{StepNo: 2, ApproverType: domain.ApproverUser, ApproverID: "director"},
	}

	next, ok := nextApproverStep(steps, 1, "requester")
	if !ok {
		t.Fatal("nextApproverStep() = no step, want step 2")
	}
	if next.StepNo != 2 {
		t.Fatalf("nextApproverStep() step = %d, want 2", next.StepNo)
	}
}

func TestFindFlowStep(t *testing.T) {
	steps := []domain.FlowStep{
		{StepNo: 1, Name: "manager"},
		{StepNo: 2, Name: "finance"},
	}

	step, ok := findFlowStep(steps, 2)
	if !ok || step.Name != "finance" {
		t.Fatalf("findFlowStep() = %+v, %v, want step 2 finance", step, ok)
	}
	if _, ok := findFlowStep(steps, 3); ok {
		t.Fatal("findFlowStep() found a missing step")
	}
}
