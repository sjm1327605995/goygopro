# Packet Router 设计方案

## 1. 问题诊断

当前 `HandleCTOSPacket` 的痛点：

```go
func (d *DuelPlayer) HandleCTOSPacket(data []byte) {
    pktType := data[0]
    pData := data[1:]
    // ... 状态检查重复 3 次
    switch pktType {
    case network.CTOS_RESPONSE:
        if d.Game == nil { return }  // 重复检查
        d.Game.GetResponse(d, pData)
    case network.CTOS_HAND_RESULT:
        if d.Game == nil { return }
        if len(pData) < int(binary.Size(protocol.CTOSHandResult{})) { return }  // 重复验证
        var pkt protocol.CTOSHandResult
        restruct.Unpack(pData, binary.LittleEndian, &pkt)  // 重复解析
        d.Game.HandResult(d, pkt.Res)
    // ... 20+ 个 case
    }
}
```

**硬编码问题**：
- switch-case 无法扩展，新增协议必须改核心文件
- 每个 case 重复 `nil` 检查、长度验证、二进制解析
- 错误处理不统一（panic / return / fmt.Println 混用）
- 无法给一类消息统一加拦截器（如"必须已登录"、"必须在游戏中"）

## 2. 核心设计

参考 Gin 的 `Router → Middleware → Handler → Context` 模式：

```
┌─────────────────────────────────────────────┐
│  PacketRouter (路由注册中心)                    │
│  router.Register(CTOS_JOIN_GAME, handlers...)│
└─────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│  Middleware Chain (中间件链)                  │
│  ValidateLength → AuthCheck → GameCheck     │
│  → UnpackStruct → YourHandler               │
└─────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│  PacketContext (请求上下文)                   │
│  ctx.PktType, ctx.Player, ctx.Game,         │
│  ctx.Bind(&struct), ctx.Error(), ctx.Reply() │
└─────────────────────────────────────────────┘
```

## 3. API 设计（目标形态）

```go
// 初始化路由器
router := NewPacketRouter()

// 注册中间件（全局）
router.Use(RecoverMiddleware, StateMiddleware)

// 注册带自动解析的路由
router.Register(network.CTOS_PLAYER_INFO, BindMiddleware(protocol.CTOSPlayerInfo{}), HandlePlayerInfo)
router.Register(network.CTOS_HAND_RESULT, RequireGame, BindMiddleware(protocol.CTOSHandResult{}), HandleHandResult)
router.Register(network.CTOS_JOIN_GAME, BindMiddleware(protocol.CTOSJoinGame{}), HandleJoinGame)
router.Register(network.CTOS_CREATE_GAME, BindMiddleware(protocol.CTOSCreateGame{}), HandleCreateGame)

// 处理器签名
func HandlePlayerInfo(ctx *PacketContext) {
    pkt := ctx.GetPayload().(*protocol.CTOSPlayerInfo)
    player := ctx.Player
    // ... 业务逻辑
}

func HandleHandResult(ctx *PacketContext) {
    pkt := ctx.GetPayload().(*protocol.CTOSHandResult)
    ctx.Game.HandResult(ctx.Player, pkt.Res)
}
```

## 4. 中间件清单

| 中间件 | 功能 | 适用场景 |
|--------|------|----------|
| `RecoverMiddleware` | catch panic，记录日志 | 全局 |
| `StateMiddleware` | 检查 `d.State` 合法性 | 全局 |
| `RequireGame` | 检查 `d.Game != nil` | 需要在对局中的操作 |
| `RequireDuel` | 检查 `d.Game.OCGDuel() != nil` | 需要决斗已开始的操作 |
| `RequireLobby` | 检查 `d.Game.BaseMode().Duel == nil` | 大厅阶段操作 |
| `ValidateLength(n)` | 检查 `len(pData) >= n` | 固定长度消息 |
| `BindMiddleware(T)` | 自动 `restruct.Unpack` 到 `*T` | 所有有结构体的消息 |

## 5. 错误处理统一化

```go
type PacketError struct {
    Code    uint8  // 协议错误码
    Message string // 描述
    Cause   error  // 原始错误
}

func (ctx *PacketContext) Error(code uint8, msg string) {
    ctx.errorCode = code
    ctx.errorMsg = msg
    ctx.Abort()
}
```

## 6. 文件结构

```
core/duel/packet/
├── router.go        # 路由器核心
├── context.go       # PacketContext
├── middleware.go    # 通用中间件（Recover、Auth、Bind等）
├── handlers.go      # 从 duel_palyer.go 迁移来的处理器
└── errors.go        # 统一错误类型
```

## 7. 迁移策略

1. **新建 packet 包**，实现 Router + Context + Middleware
2. **重写 duel_palyer.go** 的 `HandleCTOSPacket` 为 `router.Dispatch(d, data)`
3. **逐个迁移 case** 为 Handler 函数，放到 `handlers.go`
4. **验证编译通过后**，删除旧的 switch-case

## 8. 对比：旧 vs 新

**旧代码**（CTOS_HAND_RESULT，18 行）：
```go
case network.CTOS_HAND_RESULT:
    if d.Game == nil { return }
    if len(pData) < int(binary.Size(protocol.CTOSHandResult{})) { return }
    var pkt protocol.CTOSHandResult
    restruct.Unpack(pData, binary.LittleEndian, &pkt)
    d.Game.HandResult(d, pkt.Res)
```

**新代码**（注册 + Handler，6 行）：
```go
// 注册
router.Register(network.CTOS_HAND_RESULT, RequireGame, Bind(&protocol.CTOSHandResult{}), HandleHandResult)

// 处理
func HandleHandResult(ctx *packet.Context) {
    pkt := ctx.MustPayload().(*protocol.CTOSHandResult)
    ctx.Game.HandResult(ctx.Player, pkt.Res)
}
```

---

**收益**：
- 消除 switch-case，新增协议只需一行注册
- 消除 `if d.Game == nil` 重复代码（中间件统一处理）
- 消除手动 `restruct.Unpack`（Bind 中间件自动处理）
- 消除长度验证（ValidateLength 中间件）
- 错误处理统一，panic 被 Recover 捕获
- 可测试：Handler 接收 Context，易于单元测试
