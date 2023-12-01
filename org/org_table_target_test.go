package org

import (
	"testing"
)

func TestOrgFormulaTarget(t *testing.T) {
	tt := FormulaTarget{Raw: "@3$3"}
	tt.Process()
	if tt.Start.Row != 3 && tt.Start.Col != 3 {
		t.Errorf("Failed Start did not parse correctly: %v\n", tt)
	}
	if tt.End.Row != 3 && tt.End.Col != 3 {
		t.Errorf("Failed End did not parse correctly: %v\n", tt)
	}
}
