package postgres

import (
	"strings"
	"testing"

	"github.com/mlf-erp/mlf-erp/internal/platform/scope"
)

func TestAppendScopeFailsClosedWhenIdentityIsMissing(t *testing.T) {
	tests := []struct {
		name      string
		filter    scope.Filter
		wantDeny  bool
		wantArgs  int
		wantInSQL string
	}{
		{name: "all", filter: scope.Filter{DataScope: "all"}, wantArgs: 0},
		{name: "self without user", filter: scope.Filter{DataScope: "self"}, wantDeny: true, wantArgs: 0},
		{name: "dept without path", filter: scope.Filter{DataScope: "dept"}, wantDeny: true, wantArgs: 0},
		{name: "company without path", filter: scope.Filter{DataScope: "company"}, wantDeny: true, wantArgs: 0},
		{
			name:      "self with user",
			filter:    scope.Filter{DataScope: "self", UserID: "u1"},
			wantArgs:  1,
			wantInSQL: "p.created_by = $1",
		},
		{
			name:      "dept with path",
			filter:    scope.Filter{DataScope: "dept", OrgPath: "/root/dept"},
			wantArgs:  1,
			wantInSQL: "o.path = $1",
		},
		{
			name:      "company with path",
			filter:    scope.Filter{DataScope: "company", CompanyPath: "/root"},
			wantArgs:  1,
			wantInSQL: "o.path = $1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var where string
			var args []any
			appendScope(&where, &args, tt.filter)

			if tt.wantDeny {
				if !strings.Contains(where, "1 = 0") {
					t.Fatalf("appendScope() = %q, want fail-closed condition", where)
				}
				if len(args) != 0 {
					t.Fatalf("appendScope() args = %v, want none", args)
				}
				return
			}

			if tt.wantArgs != len(args) {
				t.Fatalf("appendScope() args = %d, want %d", len(args), tt.wantArgs)
			}
			if tt.wantInSQL != "" && !strings.Contains(where, tt.wantInSQL) {
				t.Fatalf("appendScope() = %q, want substring %q", where, tt.wantInSQL)
			}
		})
	}
}
