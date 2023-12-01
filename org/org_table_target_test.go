package org

import (
	"testing"
)

func ValPos(sr, sc, er, ec int, tt FormulaTarget, t *testing.T) {
	if tt.Start.Row != sr && tt.Start.Col != sc {
		t.Errorf("Failed Start did not parse correctly: %v\n", tt)
	}
	if tt.End.Row != er && tt.End.Col != ec {
		t.Errorf("Failed End did not parse correctly: %v\n", tt)
	}
}

func TestOrgFormulaTarget(t *testing.T) {
	tt := FormulaTarget{Raw: "@3$3"}
	tt.Process()
	ValPos(3, 3, 3, 3, tt, t)
	tt = FormulaTarget{Raw: "@1$2..@3$4"}
	tt.Process()
	ValPos(1, 2, 3, 4, tt, t)
	tt = FormulaTarget{Raw: "$5"}
	tt.Process()
	ValPos(-1, 5, -1, 5, tt, t)
	tt = FormulaTarget{Raw: "@6"}
	tt.Process()
	ValPos(6, -1, 6, -1, tt, t)
}
