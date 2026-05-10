package duel

// Room 表示一个游戏房间
// C++ 语义：YGOPro 原版中没有独立的 Room 概念，DuelMode 本身就是房间。
// Room 在 Go 中仅作为 RoomManager 的索引容器，不管理玩家、不持有房间元数据。
// 房间的名称、密码、配置等全部存放在 DuelMode（HostInfo/Name/Pass）中。
type Room struct {
	ID       string    // 房间ID（通常由密码决定）
	DuelMode IDuelMode // 房间的游戏逻辑实例
}

// NewRoom 创建一个新房间
func NewRoom(id string) *Room {
	return &Room{ID: id}
}
