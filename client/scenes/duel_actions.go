package scenes

import (
	"encoding/binary"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// 决斗盘上的点击语义 —— 点一张卡该给服务器回什么，取决于当前正在等待哪条消息。
// 这里不碰任何渲染库：换 GUI 时这部分应当原样存活。

// selectCardClick 处理选卡框里的一次点击。
//
// 「凑数值」的两种选择（MSG_SELECT_SUM 同调/超量素材等、MSG_SELECT_TRIBUTE 祭品）
// 要走 ToggleSumPick：每点一下都得重算剩下哪些卡还凑得出来。其余选择只是简单翻转。
func selectCardClick(card *client.ClientCard) {
	switch client.MainGame.DInfo.CurMsg {
	case network.MSG_SELECT_SUM, network.MSG_SELECT_TRIBUTE:
		client.MainGame.DField.ToggleSumPick(card, client.MainGame.DInfo.CurMsg)
	default:
		if !card.IsSelectable {
			return
		}
		card.IsSelected = !card.IsSelected
	}
	client.MainGame.FieldRev.Bump()
}

func isPlaceSelectable(player int, location uint8, sequence int) bool {
	curMsg := client.MainGame.DInfo.CurMsg
	if curMsg != network.MSG_SELECT_PLACE && curMsg != network.MSG_SELECT_DISFIELD {
		return false
	}
	field := client.MainGame.DField.SelectableField
	var bit uint32
	switch {
	case player == 0 && location == 0x04:
		bit = 1 << sequence
	case player == 0 && location == 0x08:
		bit = 1 << (sequence + 8)
	case player == 1 && location == 0x04:
		bit = 1 << (sequence + 16)
	case player == 1 && location == 0x08:
		bit = 1 << (sequence + 24)
	}
	return field&bit != 0
}

func onCardClick(card *client.ClientCard, player int, location uint8, sequence int) {
	dialog := client.MainGame.Dialog
	if dialog.Visible && dialog.Type == client.DialogCardSelect {
		selectCardClick(card)
		return
	}

	curMsg := client.MainGame.DInfo.CurMsg
	switch curMsg {
	case network.MSG_SELECT_PLACE, network.MSG_SELECT_DISFIELD:
		onEmptySlotClick(player, location, sequence)
		return

	case network.MSG_SELECT_COUNTER:
		if !card.IsSelectable {
			return
		}
		card.OpParam--
		if card.OpParam&0xffff == 0 {
			card.IsSelectable = false
		}
		df := client.MainGame.DField
		df.SelectCounterCount--
		if df.SelectCounterCount == 0 {
			resp := make([]byte, len(df.SelectableCards)*2)
			for i, c := range df.SelectableCards {
				val := uint16((c.OpParam >> 16) - (c.OpParam & 0xffff))
				binary.LittleEndian.PutUint16(resp[i*2:], val)
			}
			client.Client.SetResponseB(resp)
			client.Client.SendResponse()
			df.ClearSelect()
		}
		client.MainGame.FieldRev.Bump()
		return

	case network.MSG_SELECT_IDLECMD, network.MSG_SELECT_BATTLECMD:
		if card.CmdFlag != 0 {
			handleCmdClick(card, curMsg)
		}
	}
}

func onEmptySlotClick(player int, location uint8, sequence int) {
	curMsg := client.MainGame.DInfo.CurMsg
	if curMsg != network.MSG_SELECT_PLACE && curMsg != network.MSG_SELECT_DISFIELD {
		return
	}
	if !isPlaceSelectable(player, location, sequence) {
		return
	}
	client.MainGame.DField.SelectableField = 0
	client.Client.SetResponseB([]byte{uint8(player), location, uint8(sequence)})
	client.Client.SendResponse()
	client.MainGame.FieldRev.Bump()
}

// handleCmdClick 收集这张卡当前可做的操作：只有一种就直接回应，多种则弹选择框。
func handleCmdClick(card *client.ClientCard, curMsg int16) {
	df := client.MainGame.DField
	var options []string
	var resps []int32

	add := func(list []*client.ClientCard, label string, code int) {
		for i, c := range list {
			if c == card {
				options = append(options, label)
				resps = append(resps, int32((i<<16)+code))
			}
		}
	}
	addActivatable := func(code int) {
		for i, c := range df.ActivatableCards {
			if c != card {
				continue
			}
			if df.ActivatableDescs[i][1]&client.EDESCOperation != 0 {
				continue
			}
			options = append(options, "发动")
			resps = append(resps, int32((i<<16)+code))
		}
	}

	switch curMsg {
	case network.MSG_SELECT_IDLECMD:
		add(df.SummonableCards, "通常召唤", 0)
		add(df.SPSummonableCards, "特殊召唤", 1)
		add(df.ReposableCards, "改变表示形式", 2)
		add(df.MSetableCards, "盖放怪兽", 3)
		add(df.SSetableCards, "盖放魔陷", 4)
		addActivatable(5)
	case network.MSG_SELECT_BATTLECMD:
		addActivatable(0)
		add(df.AttackableCards, "攻击", 1)
	}

	switch len(options) {
	case 0:
	case 1:
		df.ClearCommandFlag()
		client.Client.SetResponseI(resps[0])
		client.Client.SendResponse()
		client.MainGame.FieldRev.Bump()
	default:
		client.MainGame.Dialog.ShowCmdSelect(options, resps)
		client.MainGame.FieldRev.Bump()
	}
}

// sendCmd 是指令栏按钮的统一动作：清掉命令标记并回一个整数。
func sendCmd(resp int32) {
	client.MainGame.DField.ClearCommandFlag()
	client.Client.SetResponseI(resp)
	client.Client.SendResponse()
	client.MainGame.FieldRev.Bump()
}
