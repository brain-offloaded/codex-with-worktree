package app

import (
	"reflect"
	"testing"
)

func TestPlanCommand_DefaultPassthrough(t *testing.T) {
	plan := PlanCommand([]string{"exec", "fix this"})
	if plan.Mode != ModePickAndExec {
		t.Fatalf("unexpected mode: %v", plan.Mode)
	}
	want := []string{"exec", "fix this"}
	if !reflect.DeepEqual(plan.CodexArgs, want) {
		t.Fatalf("unexpected args: %#v", plan.CodexArgs)
	}
}

func TestPlanCommand_VersionBypassesPicker(t *testing.T) {
	plan := PlanCommand([]string{"--version"})
	if plan.Mode != ModeDirectCodex {
		t.Fatalf("unexpected mode: %v", plan.Mode)
	}
}

func TestPlanCommand_ListIsInternal(t *testing.T) {
	plan := PlanCommand([]string{"list"})
	if plan.Mode != ModeInternal {
		t.Fatalf("unexpected mode: %v", plan.Mode)
	}
	if plan.InternalCommand != "list" {
		t.Fatalf("unexpected command: %q", plan.InternalCommand)
	}
}
