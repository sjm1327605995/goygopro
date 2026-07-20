package client

import "github.com/sjm1327605995/goygopro/protocol/network"

// 「取消 / 完成」——决斗中各种选择的退出口。翻译自 C++ event_handler.cpp 的
// ClientField::CancelOrFinish，剥掉窗口显隐，只留下「这一步该给服务器回什么」。
//
// 此前 Go 侧完全没有这套东西，最要命的是连锁：服务器每问一次「要不要连锁」，
// 客户端只列出可连锁的卡，没有「不连锁」这个出口 —— 对方每发动一个效果，
// 玩家都被迫连锁一张。决斗根本没法正常进行。

// CancelResult 说明这次取消做了什么，方便界面决定要不要关掉弹窗。
type CancelResult int

const (
	CancelNothing   CancelResult = iota // 当前状态下取消没有意义
	CancelResponded                     // 已经回了服务器，弹窗可以关掉
)

// CancelOrFinish 处理一次「取消/完成」。返回是否真的回应了服务器。
func (cf *ClientField) CancelOrFinish(curMsg int16) CancelResult {
	switch curMsg {
	case network.MSG_SELECT_YESNO, network.MSG_SELECT_EFFECTYN:
		// 问句一律按「否」回答
		Client.SetResponseI(0)
		Client.SendResponse()
		return CancelResponded

	case network.MSG_SELECT_CARD, network.MSG_SELECT_TRIBUTE:
		// 一张没选而且允许取消 —— 整个选择作罢
		if len(cf.SelectedCards) == 0 {
			if !cf.SelectCancelable {
				return CancelNothing
			}
			Client.SetResponseI(-1)
			Client.SendResponse()
			cf.ClearSelect()
			return CancelResponded
		}
		// 已经选够了 —— 这时按钮的含义是「完成」
		if cf.SelectReady {
			cf.SetResponseSelectedCards()
			Client.SendResponse()
			cf.ClearSelect()
			return CancelResponded
		}
		return CancelNothing

	case network.MSG_SELECT_SUM:
		// 凑数值只有「完成」，没有中途取消
		if !cf.SelectReady {
			return CancelNothing
		}
		cf.SetResponseSelectedCards()
		Client.SendResponse()
		cf.ClearSelect()
		return CancelResponded

	case network.MSG_SELECT_UNSELECT_CARD:
		if !cf.SelectCancelable {
			return CancelNothing
		}
		Client.SetResponseI(-1)
		Client.SendResponse()
		cf.ClearSelect()
		return CancelResponded

	case network.MSG_SELECT_CHAIN:
		// 「不连锁」。强制连锁时没得选，只能连。
		if cf.ChainForced {
			return CancelNothing
		}
		Client.SetResponseI(-1)
		Client.SendResponse()
		cf.ClearChainSelect()
		return CancelResponded

	case network.MSG_SORT_CARD:
		// 放弃排序，让服务器按默认顺序处理
		Client.SetResponseI(-1)
		Client.SendResponse()
		cf.SortList = nil
		return CancelResponded

	case network.MSG_SELECT_PLACE:
		if !cf.SelectCancelable {
			return CancelNothing
		}
		// 回一个空位置表示放弃（对应 C++ 的 respbuf{LocalPlayer(0), 0, 0}）
		cf.SelectableField = 0
		Client.SetResponseB([]byte{uint8(MainGame.LocalPlayer(0)), 0, 0})
		Client.SendResponse()
		return CancelResponded
	}
	return CancelNothing
}

// CanCancel 表示当前这一步能不能取消 —— 界面据此决定要不要显示取消按钮。
// 与 CancelOrFinish 的判断保持一致，否则会出现「按钮在但点了没反应」。
func (cf *ClientField) CanCancel(curMsg int16) bool {
	switch curMsg {
	case network.MSG_SELECT_YESNO, network.MSG_SELECT_EFFECTYN, network.MSG_SORT_CARD:
		return true
	case network.MSG_SELECT_CARD, network.MSG_SELECT_TRIBUTE:
		return (len(cf.SelectedCards) == 0 && cf.SelectCancelable) || cf.SelectReady
	case network.MSG_SELECT_SUM:
		return cf.SelectReady
	case network.MSG_SELECT_UNSELECT_CARD, network.MSG_SELECT_PLACE:
		return cf.SelectCancelable
	case network.MSG_SELECT_CHAIN:
		return !cf.ChainForced
	}
	return false
}

// CancelLabel 是取消按钮该显示的字：选够了是「完成」，否则是「取消」。
// 连锁那步说「不连锁」比「取消」清楚得多。
func (cf *ClientField) CancelLabel(curMsg int16) string {
	switch curMsg {
	case network.MSG_SELECT_CHAIN:
		return "不连锁"
	case network.MSG_SELECT_CARD, network.MSG_SELECT_TRIBUTE, network.MSG_SELECT_SUM:
		if cf.SelectReady && len(cf.SelectedCards) > 0 {
			return "完成"
		}
	}
	return "取消"
}

// SetResponseSelectedCards 把已选的卡回给服务器。
// 回的是每张卡的 SelectSeq（服务器发下来时给的编号），不是它在列表里的下标 ——
// 凑数值那步两者并不相同（must_select 的卡编号都是 0）。
// 对应 C++ event_handler.cpp 的 ClientField::SetResponseSelectedCards。
func (cf *ClientField) SetResponseSelectedCards() {
	n := len(cf.SelectedCards)
	if n > 255 {
		n = 255
	}
	resp := make([]byte, 0, n+1)
	resp = append(resp, byte(n))
	for i := 0; i < n; i++ {
		resp = append(resp, byte(cf.SelectedCards[i].SelectSeq))
	}
	Client.SetResponseB(resp)
}
