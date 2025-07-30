package duel

import (
	cmap "github.com/orcaman/concurrent-map/v2"
)

// Room 表示一个游戏房间
type Room struct {
	ID       string // 房间ID
	Name     string // 房间名称
	DuelMode IDuelMode
	Players  cmap.ConcurrentMap[string, IPlayer] // 玩家列表，key是玩家ID
}

// Player 表示房间中的一个玩家
type Player struct {
	ID        string // 玩家ID
	JoinOrder int    // 加入顺序
	Name      string // 玩家名称
	IsHost    bool   // 是否是房主
}

// NewRoom 创建一个新房间
func NewRoom(id string) *Room {

	r := &Room{
		ID:      id,
		Players: cmap.New[IPlayer](),
	}

	return r
}

// AddPlayer 添加玩家到房间
func (r *Room) AddPlayer(player IPlayer) {
	setSuccess := r.Players.SetIfAbsent(player.GetID(), player)
	if setSuccess {
		return
	}
	p, exist := r.Players.Get(player.GetID())
	if exist {
		r.Players.Remove(player.GetID())
		p.Disconnect()
	}
	r.AddPlayer(player)
}

// RemovePlayer 从房间移除玩家
func (r *Room) RemovePlayer(playerID string) {
	r.Players.Remove(playerID)
}
