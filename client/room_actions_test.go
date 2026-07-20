package client

import (
	"testing"

	"github.com/sjm1327605995/goygopro/protocol/network"
)

// 这几个动作服务端一直支持，客户端却从来没发过：投降、正常离开、取消准备、
// 观众转回决斗者、踢人。于是玩家做不了这些事 —— 决斗中无法认输，退出只能粗暴断连，
// 准备之后无法反悔，转成观众就再也回不去。
//
// 这类交互唯一值得断言的就是「点了之后到底发出了什么包」，所以用 sendHook 录下来。

type sentPacket struct {
	proto   uint8
	payload []byte
}

func captureSends(t *testing.T) *[]sentPacket {
	t.Helper()
	var got []sentPacket
	sendHook = func(proto uint8, payload []byte) {
		got = append(got, sentPacket{proto: proto, payload: append([]byte(nil), payload...)})
	}
	t.Cleanup(func() { sendHook = nil })
	return &got
}

func TestRoomActionsSendCorrectPackets(t *testing.T) {
	for _, tc := range []struct {
		name string
		do   func()
		want uint8
	}{
		{"投降", Client.Surrender, network.CTOS_SURRENDER},
		{"离开房间", Client.LeaveGame, network.CTOS_LEAVE_GAME},
		{"取消准备", Client.NotReady, network.CTOS_HS_NOTREADY},
		{"回到决斗席", Client.ToDuelist, network.CTOS_HS_TODUELIST},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := captureSends(t)
			tc.do()

			if len(*got) != 1 {
				t.Fatalf("发出了 %d 个包, want 1", len(*got))
			}
			if (*got)[0].proto != tc.want {
				t.Errorf("包类型 = %d, want %d", (*got)[0].proto, tc.want)
			}
		})
	}
}

// 踢人要带上被踢的位置，否则服务端不知道踢谁。
func TestKickCarriesPosition(t *testing.T) {
	got := captureSends(t)

	Client.Kick(2)

	if len(*got) != 1 {
		t.Fatalf("发出了 %d 个包, want 1", len(*got))
	}
	p := (*got)[0]
	if p.proto != network.CTOS_HS_KICK {
		t.Errorf("包类型 = %d, want CTOS_HS_KICK", p.proto)
	}
	if len(p.payload) != 1 || p.payload[0] != 2 {
		t.Errorf("负载 = %v, want [2]（被踢的位置）", p.payload)
	}
}

// 点「准备」必须先发卡组再发 READY：服务端要靠 CTOS_UPDATE_DECK 才能校验并开局，
// 顺序反了服务端会先收到 READY 而手上没有卡组。
func TestReadySendsDeckFirst(t *testing.T) {
	got := captureSends(t)
	MainGame.DeckMgr.CurrentDeck = Deck{Main: []uint32{1, 2, 3}}

	Client.SendUpdateDeck(&MainGame.DeckMgr.CurrentDeck)
	Client.SendPacketToServer(network.CTOS_HS_READY)

	if len(*got) != 2 {
		t.Fatalf("发出了 %d 个包, want 2", len(*got))
	}
	if (*got)[0].proto != network.CTOS_UPDATE_DECK {
		t.Errorf("第一个包 = %d, 应当先发卡组 CTOS_UPDATE_DECK", (*got)[0].proto)
	}
	if (*got)[1].proto != network.CTOS_HS_READY {
		t.Errorf("第二个包 = %d, want CTOS_HS_READY", (*got)[1].proto)
	}
}
