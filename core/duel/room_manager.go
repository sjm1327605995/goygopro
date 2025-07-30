package duel

import (
	cmap "github.com/orcaman/concurrent-map/v2"
)

// Manager 房间管理器
type Manager struct {
	rooms cmap.ConcurrentMap[string, *Room] // 房间列表，key是房间ID
}

var DefaultManager = NewManager()

// NewManager 创建一个新的房间管理器
func NewManager() *Manager {

	return &Manager{
		rooms: cmap.New[*Room](),
	}
}

type IPlayer interface {
	GetID() string
	SetID(string)
	Disconnect()
}

// JoinRoom 玩家加入房间
func (m *Manager) JoinRoom(roomID string, player IPlayer) (room *Room, isHost bool) {
	room, has := m.rooms.Get(roomID)
	if has {
		room.AddPlayer(player)
		return room, false
	}
	room = NewRoom(roomID)
	room.DuelMode = &SingleDuel{Observers: make(map[string]*DuelPlayer)}
	addSuccess := m.rooms.SetIfAbsent(roomID, room)
	if addSuccess {
		return room, true
	}
	return m.JoinRoom(roomID, player)
}

// LeaveRoom 玩家离开房间
func (m *Manager) LeaveRoom(roomID string, playerID string) {
	room, exist := m.rooms.Get(roomID)
	if !exist {
		return
	}
	//player, existPlayer := room.Players.Get(playerID)
	//if !existPlayer {
	//	return
	//}

	room.RemovePlayer(playerID)
	if room.Players.Count() == 0 {
		m.rooms.Remove(roomID)
	}
}
