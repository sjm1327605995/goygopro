package duel

import (
	"sync"

	cmap "github.com/orcaman/concurrent-map/v2"
)

// Manager 房间管理器
// C++ 语义：NetServer 直接持有全局唯一的 duel_mode 指针。
// Go 中保留 Manager 以支持索引，但 Room 本身只是 DuelMode 的薄包装。
type Manager struct {
	rooms cmap.ConcurrentMap[string, *Room]
	// createMu 串行化单房间约束下的建房流程（RoomCount 预检 + SetIfAbsent）。
	// SetIfAbsent 只对相同 roomID 原子，两个不同密码的并发 CREATE_GAME
	// 都可能通过裸的 RoomCount()==0 预检而各自建成房间。
	createMu sync.Mutex
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
// 注意：本方法不保证「服务器最多一个房间」，需要该约束时用 CreateRoomIfEmpty。
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

// CreateRoomIfEmpty 在服务器尚无房间时创建房间，否则返回 (nil, false)。
// 这是单房间约束的原子入口：预检与写入在同一临界区内，并发 CREATE_GAME
// 只有第一个能成功（C++ 中 NetServer 只有全局唯一 duel_mode，天然单房间）。
func (m *Manager) CreateRoomIfEmpty(roomID string, mode IDuelMode) (*Room, bool) {
	m.createMu.Lock()
	defer m.createMu.Unlock()
	if m.rooms.Count() > 0 {
		return nil, false
	}
	room := NewRoom(roomID)
	room.DuelMode = mode
	if m.rooms.SetIfAbsent(roomID, room) {
		return room, true
	}
	// 理论上不可达（Count==0 时无同 ID 房间），防御性重查
	room, _ = m.rooms.Get(roomID)
	return room, false
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

// AllRooms 获取所有房间
func (m *Manager) AllRooms() map[string]*Room {
	return m.rooms.Items()
}

// RoomCount 返回当前房间数量
func (m *Manager) RoomCount() int {
	return m.rooms.Count()
}
