package duel

import "testing"

// TestRpsBeatsMatchesCppOrChain 锁定 rpsBeats 与原版 C++ 三重或式
//（single_duel.cpp HandResult：(0==1&&1==2)||(0==2&&1==3)||(0==3&&1==1)）
// 在全部 9 种手势组合上等价，防止取模改写偏离原版语义。
func TestRpsBeatsMatchesCppOrChain(t *testing.T) {
	cpp := func(r0, r1 uint8) bool {
		return (r0 == 1 && r1 == 2) || (r0 == 2 && r1 == 3) || (r0 == 3 && r1 == 1)
	}
	for a := uint8(1); a <= 3; a++ {
		for b := uint8(1); b <= 3; b++ {
			if got, want := rpsBeats(a, b), cpp(a, b); got != want {
				t.Fatalf("rpsBeats(%d,%d)=%v, C++ 或式=%v", a, b, got, want)
			}
		}
	}
	// 自反性：任何手势不克自己（平局由 handResult 的 r0==r1 分支处理）。
	for g := uint8(1); g <= 3; g++ {
		if rpsBeats(g, g) {
			t.Fatalf("rpsBeats(%d,%d) 应为 false", g, g)
		}
	}
}
