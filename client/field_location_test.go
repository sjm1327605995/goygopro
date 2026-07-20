package client

import "testing"

// 场地各区的中心坐标在 C++ 里不是常量，而是从 materials.cpp 的四边形顶点算出来的：
// GetCardLocation 取 (v[0].X+v[1].X)/2 与 (v[0].Y+v[2].Y)/2（见 gframe/client_field.cpp）。
// Go 这边为省事把结果抄成了常量 —— 抄就会错：额外怪兽区两处都把 2.85 写成了 2.95
// （约 10px），肉眼看不出来，卡就是差一点点没坐在格子中间。
//
// 所以这里把 C++ 的**顶点原始数据**抄过来，按同样的公式反算，与 zoneCenter 比对。
// 抄顶点比抄结果可靠：顶点是带规律的字面量（1.2f + i*1.1f），错了一眼能看出来。
func TestZoneCenterMatchesCppVertices(t *testing.T) {
	MainGame.DInfo.DuelRule = 5 // 新大师规则，rule = 1

	// materials.cpp: SetS3DVertex(v, x1, y1, x2, y2, ...)
	// 中心 = ((x1+x2)/2, (y1+y2)/2)
	center := func(x1, y1, x2, y2 float32) (float32, float32) {
		return (x1 + x2) / 2, (y1 + y2) / 2
	}

	type want struct {
		name      string
		controler int
		location  uint8
		sequence  int
		wx, wy    float32
	}
	var cases []want

	// vFieldMzone[0][i]，i<5: (1.2+i*1.1, 0.8) .. (2.3+i*1.1, 2.0)
	for i := 0; i < 5; i++ {
		x, y := center(1.2+float32(i)*1.1, 0.8, 2.3+float32(i)*1.1, 2.0)
		cases = append(cases, want{"p0 怪兽区", 0, 0x04, i, x, y})
	}
	// vFieldMzone[0][5] / [6]：额外怪兽区
	x, y := center(2.3, -0.6, 3.4, 0.6)
	cases = append(cases, want{"p0 额外怪兽区左", 0, 0x04, 5, x, y})
	x, y = center(4.5, -0.6, 5.6, 0.6)
	cases = append(cases, want{"p0 额外怪兽区右", 0, 0x04, 6, x, y})

	// vFieldMzone[1][i]，i<5: (6.7-i*1.1, -0.8) .. (5.6-i*1.1, -2.0)
	for i := 0; i < 5; i++ {
		x, y := center(6.7-float32(i)*1.1, -0.8, 5.6-float32(i)*1.1, -2.0)
		cases = append(cases, want{"p1 怪兽区", 1, 0x04, i, x, y})
	}
	x, y = center(5.6, 0.6, 4.5, -0.6)
	cases = append(cases, want{"p1 额外怪兽区左", 1, 0x04, 5, x, y})
	x, y = center(3.4, 0.6, 2.3, -0.6)
	cases = append(cases, want{"p1 额外怪兽区右", 1, 0x04, 6, x, y})

	for _, c := range cases {
		gotX, gotY := zoneCenter(c.controler, c.location, c.sequence, 1)
		wantX, wantY := worldToScreen(c.wx, c.wy)
		if !closeEnough(gotX, wantX) || !closeEnough(gotY, wantY) {
			t.Errorf("%s[%d]: zoneCenter = (%.1f, %.1f)，按 C++ 顶点算应为 (%.1f, %.1f)",
				c.name, c.sequence, gotX, gotY, wantX, wantY)
		}
	}
}

// 怪兽区与魔陷区上下相邻、间距应当一致（世界坐标里差 1.2）。
// 这条抓的是「某一格被抄错」——单看一个数看不出来，看间距立刻暴露。
func TestZoneRowsEvenlySpaced(t *testing.T) {
	MainGame.DInfo.DuelRule = 5

	for player := 0; player < 2; player++ {
		var prevX float32
		for seq := 0; seq < 5; seq++ {
			mx, my := zoneCenter(player, 0x04, seq, 1)
			sx, sy := zoneCenter(player, 0x08, seq, 1)

			if mx != sx {
				t.Errorf("p%d seq%d: 怪兽区 x=%.1f 与魔陷区 x=%.1f 不对齐（应在同一列）",
					player, seq, mx, sx)
			}
			// 怪兽区在魔陷区更靠中央的一侧
			gap := sy - my
			if player == 1 {
				gap = my - sy
			}
			if !closeEnough(gap, 1.2*scaleY) {
				t.Errorf("p%d seq%d: 怪兽区与魔陷区行距 = %.1fpx, 应为 %.1fpx",
					player, seq, gap, 1.2*scaleY)
			}
			if seq > 0 {
				step := mx - prevX
				wantStep := float32(1.1 * scaleX)
				if player == 1 {
					wantStep = -wantStep
				}
				if !closeEnough(step, wantStep) {
					t.Errorf("p%d seq%d: 与上一格的列距 = %.1fpx, 应为 %.1fpx",
						player, seq, step, wantStep)
				}
			}
			prevX = mx
		}
	}
}

func closeEnough(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.5 // 亚像素误差无所谓，抓的是抄错的那 10px
}
