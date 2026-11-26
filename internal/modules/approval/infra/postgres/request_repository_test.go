package postgres

import (
	"testing"

	"github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
)

func TestNextStepAndSkippedSkipsRequesterUserSteps(t *testing.T) {
	steps := []domain.FlowStep{
		{StepNo: 1, ApproverType: domain.ApproverRole, ApproverID: "role-1"},
		{StepNo: 4, ApproverType: domain.ApproverUser, ApproverID: "requester"},
		{StepNo: 2, ApproverType: domain.ApproverUser, ApproverID: "requester"},
		{StepNo: 3, ApproverType: domain.ApproverUser, ApproverID: "finance"},
	}

	next, skipped, ok := nextStepAndSkipped(steps, 1, "requester")
	if !ok {
		t.Fatal("nextStepAndSkipped() = no step, want step 3")
	}
	if next.StepNo != 3 || next.ApproverID != "finance" {
		t.Fatalf("nextStepAndSkipped() = step %d approver %q, want step 3 finance", next.StepNo, next.ApproverID)
	}
	if len(skipped) != 2 || skipped[0].StepNo != 2 || skipped[1].StepNo != 4 {
		t.Fatalf("nextStepAndSkipped() skipped = %+v, want steps 2 and 4 in order", skipped)
	}
}

func TestNextStepAndSkippedDoesNotSkipRequesterRoleStep(t *testing.T) {
	steps := []domain.FlowStep{
		{StepNo: 1, ApproverType: domain.ApproverUser, ApproverID: "manager"},
		{StepNo: 2, ApproverType: domain.ApproverRole, ApproverID: "requester-role"},
	}

	next, skipped, ok := nextStepAndSkipped(steps, 1, "requester")
	if !ok {
		t.Fatal("nextStepAndSkipped() = no step, want role step 2")
	}
	if next.StepNo != 2 {
		t.Fatalf("nextStepAndSkipped() step = %d, want 2", next.StepNo)
	}
	if len(skipped) != 0 {
		t.Fatalf("nextStepAndSkipped() skipped = %+v, want none", skipped)
	}
}

func TestNextStepAndSkippedNoneAfterSkippingSelf(t *testing.T) {
	steps := []domain.FlowStep{
		{StepNo: 1, ApproverType: domain.ApproverRole, ApproverID: "role-1"},
		{StepNo: 2, ApproverType: domain.ApproverUser, ApproverID: "requester"},
	}

	next, skipped, ok := nextStepAndSkipped(steps, 1, "requester")
	if ok {
		t.Fatalf("nextStepAndSkipped() = step %d, want none", next.StepNo)
	}
	if len(skipped) != 1 || skipped[0].StepNo != 2 {
		t.Fatalf("nextStepAndSkipped() skipped = %+v, want step 2", skipped)
	}
}

func TestNextStepAndSkippedChoosesLowestStep(t *testing.T) {
	steps := []domain.FlowStep{
		{StepNo: 3, ApproverType: domain.ApproverUser, ApproverID: "finance"},
		{StepNo: 1, ApproverType: domain.ApproverUser, ApproverID: "manager"},
		{StepNo: 2, ApproverType: domain.ApproverUser, ApproverID: "director"},
	}

	next, skipped, ok := nextStepAndSkipped(steps, 1, "requester")
	if !ok {
		t.Fatal("nextStepAndSkipped() = no step, want step 2")
	}
	if next.StepNo != 2 {
		t.Fatalf("nextStepAndSkipped() step = %d, want 2", next.StepNo)
	}
	if len(skipped) != 0 {
		t.Fatalf("nextStepAndSkipped() skipped = %+v, want none", skipped)
	}
}

func TestNextStepAndSkippedNoFutureSteps(t *testing.T) {
	steps := []domain.FlowStep{
		{StepNo: 1, ApproverType: domain.ApproverUser, ApproverID: "manager"},
	}

	next, skipped, ok := nextStepAndSkipped(steps, 1, "requester")
	if ok {
		t.Fatalf("nextStepAndSkipped() = step %d, want none", next.StepNo)
	}
	if len(skipped) != 0 {
		t.Fatalf("nextStepAndSkipped() skipped = %+v, want none", skipped)
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
