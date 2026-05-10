package duel

import (
	cmap "github.com/orcaman/concurrent-map/v2"
)

// Manager 房间管理器
// C++ 语义：NetServer 直接持有全局唯一的 duel_mode 指针。
// Go 中保留 Manager 以支持索引，但 Room 本身只是 DuelMode 的薄包装。
type Manager struct {
	rooms cmap.ConcurrentMap[string, *Room]
}

var DefaultManager = NewManager()

// NewManager 创建一个新的房间管理器
func NewManager() *Manager {
	return &Manager{
		rooms: cmap.New[*Room](),
	}
}

// CreateRoom 创建房间
// C++ 对应：NetServer::HandleCTOSPacket 中 CTOS_CREATE_GAME 分支。
// 如果房间已存在，返回已存在的房间和 false。
func (m *Manager) CreateRoom(roomID string, mode IDuelMode) (*Room, bool) {
	room, has := m.rooms.Get(roomID)
	if has {
		return room, false
	}
	room = NewRoom(roomID)
	room.DuelMode = mode
	if m.rooms.SetIfAbsent(roomID, room) {
		return room, true
	}
	// 并发冲突，重新获取
	return m.rooms.Get(roomID)
}

// GetRoom 获取已有房间
// C++ 对应：NetServer::HandleCTOSPacket 中 CTOS_JOIN_GAME 分支。
// 房间不存在时返回 nil, false。
func (m *Manager) GetRoom(roomID string) (*Room, bool) {
	return m.rooms.Get(roomID)
}

// RemoveRoom 移除房间索引
// C++ 中 NetServer 不主动销毁 duel_mode，由 EndDuel 或进程退出处理。
// 此处提供显式移除入口，供 DuelMode.LeaveGame / EndDuel 等场景调用。
func (m *Manager) RemoveRoom(roomID string) {
	m.rooms.Remove(roomID)
}

// CurrentRoom 获取当前第一个可用房间
// C++ 语义：NetServer::duel_mode 是唯一的房间。Go 中多房间时返回第一个。
func (m *Manager) CurrentRoom() *Room {
	for _, room := range m.rooms.Items() {
		return room
	}
	return nil
}

// AllRooms 获取所有房间
func (m *Manager) AllRooms() map[string]*Room {
	return m.rooms.Items()
}

// RoomCount 返回当前房间数量
func (m *Manager) RoomCount() int {
	return m.rooms.Count()
}
