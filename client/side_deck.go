package client

// 换副卡组（match 第二、三局开始前）。
//
// 规则很窄：只能在主卡组、额外卡组、副卡组之间**挪**牌，不能增删。
// 提交时三边张数必须与换牌前完全一致 —— 服务端 LoadSide 会再验一次
// （比对每种卡号的总张数，见 core/duel/deck_manager.go），对不上直接判卡组错误。
//
// 这一整块此前完全没有：客户端收到 STOC_CHANGE_SIDE 只是把 IsSiding 置真就没了下文，
// 既不切界面也不回传卡组，服务端在那边等 CTOS_UPDATE_DECK，第二局就此卡死。

// SideMove 把一张卡从一个区挪到另一个区。
// from/to 用 0=主卡组 1=额外卡组 2=副卡组；index 是它在 from 区里的下标。
// 返回是否真的挪动了。
func (dm *DeckManager) SideMove(from, to, index int) bool {
	if from == to {
		return false
	}
	src := dm.sideList(from)
	dst := dm.sideList(to)
	if src == nil || dst == nil || index < 0 || index >= len(*src) {
		return false
	}
	code := (*src)[index]
	*src = append((*src)[:index], (*src)[index+1:]...)
	*dst = append(*dst, code)
	return true
}

// sideList 返回指向卡组三部分之一的指针，便于原地增删。
func (dm *DeckManager) sideList(which int) *[]uint32 {
	switch which {
	case 0:
		return &dm.CurrentDeck.Main
	case 1:
		return &dm.CurrentDeck.Extra
	case 2:
		return &dm.CurrentDeck.Side
	}
	return nil
}

// SideCountsMatch 判断当前三边张数是否与换牌前一致 —— 提交的前提。
// 对应 C++ deck_con.cpp 里 BUTTON_SIDE_OK 的那三个 pre_* 比较。
func (g *Game) SideCountsMatch() bool {
	d := &g.DeckMgr.CurrentDeck
	return len(d.Main) == g.SidePreMain &&
		len(d.Extra) == g.SidePreExtra &&
		len(d.Side) == g.SidePreSide
}

// SideCountHint 用一句话说明当前哪边多了哪边少了，直接显示给玩家。
// 张数不对时按「确定」只会被服务端拒绝，不如提前讲清楚。
func (g *Game) SideCountHint() string {
	d := &g.DeckMgr.CurrentDeck
	for _, part := range []struct {
		name string
		got  int
		want int
	}{
		{"主卡组", len(d.Main), g.SidePreMain},
		{"额外卡组", len(d.Extra), g.SidePreExtra},
		{"副卡组", len(d.Side), g.SidePreSide},
	} {
		if part.got != part.want {
			return partHint(part.name, part.got, part.want)
		}
	}
	return ""
}

func partHint(name string, got, want int) string {
	if got > want {
		return name + "多了 " + itoa(got-want) + " 张"
	}
	return name + "少了 " + itoa(want-got) + " 张"
}
