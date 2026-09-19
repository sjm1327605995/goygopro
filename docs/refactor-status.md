# 重构目标与完成情况

> 更新时间：2026-09-17。本文档跟踪本轮"前后端整理重构"的目标、执行顺序与完成状态。
> 验证基线命令：`go build ./... && go vet ./... && go test -count=1 ./...`、`cd frontend && ./node_modules/.bin/tsc --noEmit && ./node_modules/.bin/vite build && ./node_modules/.bin/vite build --config vite.smoke.config.js`，外加 12 个浏览器冒烟页。

## 总览

| # | 阶段 | 状态 | 说明 |
|---|------|------|------|
| P0a | Go 死代码清理（core/duel、core/utils、protocol） | ✅ 完成 | 13 项删除，2 项验证后保留 |
| P0b | Wails 层死代码清理 + 删 cmd/query、cmd/extractscripts | ✅ 完成 | 见明细 |
| P0c | 前端死代码 + smoke-dist 出 git | ✅ 完成 | 见明细 |
| P1 | 删除根目录 server.go（第三份服务器实现） | ✅ 完成 | 入口统一为 cmd/server + core/duel/server_engine.go |
| P2 | 抽取 single/tag 公共基类 | ✅ 完成 | 两文件共 -699 行，新增 604 行共享代码 |
| P3a | wails STOC/config 表驱动化 | ✅ 完成 | duel_client.go 529→330 行 |
| P3b | Analyze 分发表驱动化（single/tag 各 77 case） | ✅ 完成 | 收尾修正测试 mock，见下方明细 |
| P4 | 并发收口（锁/race/错误码/重连/句柄泄漏） | ✅ 完成 | 见明细 |
| P5 | 前端结构重构 | ✅ 完成（5/5） | PromptHost 拆分 + useCardSelection + reducer 拆表 + field3d 拆分 + field3D strict 化 |
| — | 全量回归验证 | ✅ 完成 | Go 门禁 + 前端 tsc/双构建 + 12 冒烟页（344 项检查）全绿 |

## 已完成明细

### P0a Go 死代码清理（验证：build/vet/test 全绿）

已删除：
- `core/duel/game.go`（空文件）
- `core/duel/replay.go`：`Rewind` / `IsReplaying` / `DeleteReplay` / `RenameReplay`（cmd/wails 已用 `os.Remove` 独立实现）
- `core/utils/utils.go`：`Buffer` 全套、`CopyWStrRef`（保留 `NullTerminate`、`Wcscmp`）
- `core/utils/reader.go`：`SubSlicesOffset`（逻辑内联进唯一调用方 `SubSlices`）
- `room_manager.go`：`Manager.CurrentRoom`
- `deck_manager.go`：`GetLFListName`
- `packet_router.go`：`BindWithSize`、`GameAction/DuelAction/LobbyAction/BindAction`、`groups`/`basePath` 死字段
- `packet_context.go`：`HasError/IsAborted/MustGet/SendPacketError`，`GetPayload` 内联进 `MustPayload`
- `packet_errors.go`：`ErrObserverNotAllowed`
- `duel_player.go`：`DuelPlayer.Disconnect`
- `proto.go`：`SimpleCodec.Encode` 空桩（确认无接口满足关系）
- `tag_duel.go`：`NewTagDuel`（改用构造函数，见 P2）
- `duel_mode.go`：`CheckMessageSize`（与 `CheckMsgSize` 重复）
- `protocol/network/utils.go`：`BufferIO/NewBufferIO`
- `single_duel.go`：`// last_replay.Flush();` 注释残留

验证后保留（实为活代码）：
- `Replay.Reset/ReadData/ReadInt32/ReadName/ReadInfo`：`OpenReplay` 内部调用链
- `YGOBuffer.Unpack`：被 `cmd/wails/duel_client.go` 调用

### P0b Wails 层死代码清理（验证：build/vet/test 全绿）

- `game_msg_skip.go`：删 `skipQueryBlobList` 与 MSG_AI_NAME/MSG_SHOW_HINT/MSG_RELOAD_FIELD 三个 unreachable case
- `duel_client.go`：删 `ctx/SetContext`、`stopChan`、`playerType` 死字段
- `app.go`：删 `DisconnectServer`、`SendResponseB` 绑定（`routeResponseB` 保留）
- `config.go`：`LoadConfig` 降为非导出 `loadConfig`
- `ocgcore/api.go`：删 `YGOCardData`、`GoString`、`OCGApi.buffer`
- `ocgcore/duel.go`：删 `QueryFieldCardDef`、`QueryCardDef`
- 整目录删除：`cmd/query/`、`cmd/extractscripts/`
- 前端同步：删 `wails_bridge.ts` 的 `sendResponseB` 方法

### P0c 前端死代码 + 仓库卫生（验证：tsc + 双构建通过）

- 删 `frontend/smoke/*.ps1` 两个调试残留
- `CardPreviewPanel.typeLabel` 去 export（实为模块内使用，非死代码）
- `constants.ts` 删 4 个死常量：`TYPE_TRAPMONSTER`、`OPCODE_OPERATORS`、`OPCODE_ISCODE`、`PHINT_DESC_REMOVE`
- `frontend/smoke-dist/`（83 个被跟踪的构建产物）移出 git：加入 `.gitignore` + `git rm -r --cached`（磁盘文件保留）
- `vite.config.js` 加 manualChunks，three 独立 chunk（主 chunk 880kB→359kB）

### P1 统一服务器入口

- 删除根目录 `server.go`（与 `core/duel/server_engine.go`、`cmd/server` 三足鼎立且行为分叉：连接归零杀服、断线玩家不移出房间）
- 确认 module 根无其他 .go 依赖、文档/构建无 `go run .` 引用

### P2 single/tag 公共基类抽取（验证：go test 全程绿）

| 文件 | 前 | 后 |
|------|-----|-----|
| single_duel.go | 2035 | 1676 |
| tag_duel.go | 1933 | 1593 |
| duel_common.go（新增） | 0 | 474 |
| duel_refresh.go（新增） | 0 | 130 |

- `duelRoom` 差异接口（14 个钩子）+ 广播/响应/计时/大厅共享函数
- `DuelMode` 上移 8 个共用字段；Refresh 全套、Process/DuelEndProc、WaitforResponse/TimeConfirm、EndDuel、大厅管理（Chat/Ready/Kick/HandResult/ToObserver 等）单份化
- 刻意保留的原版差异（写成钩子）：single `clen < LEN_HEADER` vs tag `<=`、tag 不向观察者发 TIME_LIMIT、single LeaveGame 终局包 proto 用 MSG_WIN（原版疑似 bug 按原样保留）、tag 密码错误不断开、single GetResponse 截断响应等
- 未抽取：`Analyze`（留 P3b）、`JoinGame` 入座主体、`StartDuel`、`TPResult`、`RefreshSingle(Def)`、`Surrender` 主体

### P3a wails STOC/config 表驱动化（验证：go test 全绿，含 events_parity_test）

- 新增 `stoc_bindings.go`（178 行）：19 个 STOC 条目入表（2 直发 + 7 decorate + 3 特判 + 7 无包体），字段映射逐字保留
- 新增 `stoc_handlers.go`（98 行）：查表分发；**未知 STOC 包从静默丢弃改为记日志丢弃**（唯一的刻意行为改进）
- `duel_client.go` 529→330 行
- `config.go` 84 行 switch → `configSetters` 反射表（conf tag 成为键名唯一事实源）；修正 `DefaultOT` 的 conf tag 误拼
- `events_parity_test.go` 扫描名单加入 stoc_bindings.go（防止事件名悄悄脱离对齐基准）

### P3b Analyze 分发表驱动化（验证：go test ./... 全绿）

- 新增 `engine_msg_layout.go`（逐消息字段布局为唯一事实源，`skipLayout` 与 `walkEngineMessage` 共用）、`analyze_shared.go`（`analyzeHandler` 表 + `runAnalyze` 驱动循环 + 全部共享 handler）
- `single_duel.go` / `tag_duel.go` 的 `Analyze` 约 77 case switch → `runAnalyze(s, {single,tag}AnalyzeTable, ...)`；模式特有消息（`MSG_NEW_TURN`/`MSG_MATCH_KILL`/`MSG_TAG_SWAP`/单边的 `MSG_HAND_RES`）留在各模式 `*AnalyzeTable` 追加，玩家路由差异经 `duelRoom` 钩子吸收
- 新增 `analyze_layout_test.go`：`TestAnalyzeLayoutMatchesBatchWalker`（每个 handler 消费的字节数与布局表走查一致，消除 message_scan 与 Analyze 各写一份布局的分叉）、`TestAnalyzeTableModeSpecifics`（模式表该有/不该有的条目）
- 收尾修正：原 fake `DuelPlayer` 连接为 nil 触发 `Write` 空指针 → `stubConn`（嵌入 `gnet.Conn` 补齐接口 + 覆盖 `Write` 吞写）；带 `m.(*SingleDuel)` 断言的 `MSG_MATCH_KILL` 改用真实 `*SingleDuel` 走查而非 fake room

### P4 并发收口（验证：build/vet/test 全绿）

1. **create/join 显式房间锁** — `HandleCreateGame`/`HandleJoinGame` 的 `JoinGame` 调用前加 `room.DuelMode.BaseMode().Mu`。这两条路由未挂 `RoomLockMiddleware`（`RequireNotInGame` 保证进来时 `c.Game()==nil`，该中间件是 no-op），但 `JoinGame` 写房间共享玩家表，`--multicore=true` 下需显式持锁。
2. **`WailsDuelClient` 重连代际守卫** — `running bool` → `atomic.Bool`（消掉无锁读写）；新增 `gen uint64` 连接代际，`readLoop` 固定捕获 `conn+gen`，退出走 `teardown(conn, gen)`（代际不匹配即 no-op）。修掉了重连后旧 readLoop 误 `Disconnect` 新连接的隐患。
3. **`StartSingle` 句柄泄漏** — `PrepareWithOpt` 失败、双重检查 `a.single != nil` 两条提前返回路径补 `ss.Duel.End()`（`Run` 里 `defer d.End()` 在两条路径都不触发）。
4. **`errorHandler` 数据竞争** — `ocgcore.Duel` 加 `errorMu`，`SetErrorHandler`/`OnMessage` 读写都加锁，handler 在锁外调用（防回调里再触 Duel 死锁）。
5. **`SendError` 对齐 `ERRMSG_*`** — 内部码（0x10-0x18）不再直接塞进 `STOC_ERROR_MSG.Msg`；新增独立码 `ErrJoinFailed(0x18)`，`errmsgCode` 把它映射成 `ERRMSG_JOINERROR`，其余协议违例类错误（原版 netserver 同样丢弃不回复）不再发包。
6. **`StartLocalServer` 接配置 + 不吞错** — `port<=0` 回退 `loadConfig().ServerPort`（再兜底 7911）；`StartDuelServer` 的错误经 `errCh` + 500ms 探测上报，不再 `_ =` 静默丢弃。
7. **验证** — 本机无 cgo 跑不了 `-race`（见备注），改以确定性单测锁定新行为：`TestTeardownGenerationGuard`（重连不误断新连接）、`TestErrmsgCode`/`TestSendErrorWireLayout`/`TestSendErrorNoClientMsg`（STOC_ERROR_MSG 线格式与 ERRMSG_* 映射）；全量 `go test` 通过。

### P5（第 1、2 项）PromptHost 拆分 + useCardSelection（验证：tsc + 双构建 + 冒烟全绿）

1. **PromptHost 拆 `components/prompts/`** — 779 行、11 个内嵌组件拆成 12 个子文件：`CardTile.tsx`（含 `SelectCard` 公共形状）、`useCardSelection.ts`、`YesNoModal.tsx`、`CardSelectModal.tsx`、`PositionModal.tsx`、`RpsModal.tsx`、`CounterModal.tsx`、`SumSelectModal.tsx`、`SortModal.tsx`、`BitmaskModal.tsx`、`OptionListModal.tsx`、`AnnounceCardModal.tsx`；`PromptHost.tsx` 缩到只剩事件路由（779→~300 行）。所有 id/class/data-attr 契约逐字保留（`#modal-btn-yes`、`.select-card-item`、`.sum-card`、`.sort-card`、`.counter-inc`、`.mask-opt`、`.num-opt`、`#announce-card-input` 等）。
2. **抽 `useCardSelection(min,max)`** — 收敛 CardSelectModal/SumSelectModal/SortModal 三处重复的「点选/取消、到 max 上限不再新增」状态机；`selected` 保持点击顺序（SortModal 依赖它生成置换），返回 `minMet`（CardSelect 确认钮 disabled、SumSelect 的 countOk 都复用）。
3. **验证** — `tsc --noEmit` + `vite build` + `vite build --config vite.smoke.config.js` 全绿；无头冒烟 `prompts_smoke` 67/67、`practice_smoke` 25/25、`stage_smoke` 32/32（重点页）。

### P5（第 3 项）reducer applyEvent 拆 handler 表（验证：tsc + 双构建 + 冒烟全绿）

- `domain/reducer.ts` 的 `applyEvent`（约 60 个 case 的 switch）拆成 8 张按事件族分组的 handler 表：`lifecycleHandlers`（开局/进房/终局/计时/win）、`lpHandlers`、`boardHandlers`（召唤/盖卡/移动/update_* 等 board 同步）、`counterHandlers`、`phaseHandlers`（阶段/回合/按钮提示）、`singleModeHandlers`（AI/谜题/提示）、`cardHandlers`（抽卡/确认/洗切/选中/装备）、`logHandlers`（连锁/战斗/掷币等纯日志）。
- `applyEvent` 收缩为：拼 `event` 字段 → `HINT_CLEARING` clearHint → `HANDLERS[ev.event]` 查表分发（未命中返回原 state，等价原 `default`）。handler 签名 `(state, ev)` 中 `ev: any`（跨 handler 不再享受 switch 逐 case 的收窄），原 `update_data` 里的 `ev.cards.map((q) => …)` 补 `q: CardQuery | null` 显式标注。
- 每个 case body 逐字保留（含注释与 `withLog` 调用）；共享 handler 收敛（三种召唤 → `applySummoning`、增删指示物 → `applyCounter`、连锁结束/卡角标 → `noop`）。
- **验证** — `tsc --noEmit` + 双构建全绿；无头冒烟 `practice_smoke` 25/25、`prompts_smoke` 67/67、`stage_smoke` 32/32 全绿。

### P5（第 4 项）field3d 拆 geometry/textures/anim（验证：tsc + 双构建 + 冒烟全绿）

- `duel/field3d.ts` 1383 行拆成 3 个伴生文件，主文件降到 ~600 行：
  - `field3d_geometry.ts`：`CARD_WIDTH/HEIGHT/DEPTH` + `ZONE_COORDS`（区域坐标与卡尺寸的单一事实源）。抽出成独立模块是为了让 anim 侧能无循环地引用 `ZONE_COORDS`。
  - `field3d_textures.ts`：`generateCardTexture(self, code, info)`（程序化卡面绘制 + 异步真卡图叠层 + 纹理缓存）。
  - `field3d_anim.ts`：9 个卡动画/粒子特效（`animateDrawCard/Summon/SetCard/Reposition/Attack/Destroy`、`createShockwave/createHitSparks/cameraShake`）。
- 抽出去的实现一律以 `DuelField3D` 实例为第一参 `self`，只 `import type` 主类（textures 与 anim 都只对主类用类型导入、anim 再 `ZONE_COORDS` 从 geometry 引入），**不构成运行时循环导入**；主类保留同名 public 方法做薄委托，`duel_manager.ts` / `chain_visualizer.ts` 的调用契约（含 `createShockwave(x,z,color)`、`cameraShake(intensity,duration)`）逐字不变。
- 每个方法体逐字搬移（含注释、`if (!targetCoord) return` 边界守卫、`soundManager`/`TWEEN` 调用），仅 `this.` → `self.`。
- **验证** — `tsc --noEmit` + 双构建全绿（prod 150 模块，+3 新文件）；无头冒烟 `stage_smoke` 32/32、`practice_smoke` 25/25、`prompts_smoke` 67/67 全绿。

### P5（第 5 项）field3D strict 化（验证：tsc + 双构建 + 冒烟全绿）

- `duel_manager.ts` 的 `field3D: any` → `field3D: DuelField3D`（`import type { DuelField3D } from './field3d.ts'`，仅类型导入、不构成运行时循环）；构造参数 `constructor(field3D: DuelField3D, …)` 同改。棘轮最后一步的 stale 桥接注释一并删除。
- 连带收敛：`chainVisualizer` 在 `DuelField3D` 上声明为 `ChainVisualizer | null`，`duel_manager.ts` 里 4 处 `this.field3D.chainVisualizer.*`（`addChainLink`/`highlightSolvingLink`/`removeSolvingLink`/`clearChain`）调用补 `?.`——运行期恒非空、行为不变，仅满足 strict null-check。
- 此前被 `any` 掩盖的 `this.field3D.*` 全部方法（`setOpponentHandCount`/`animateDrawCard`/`animateSummon`/`animateSetCard`/`placeCard`/`moveCard`/`removeFromSlot`/`meshAt`/`addRelationLine`/`removeRelationLine`/`applyCounterDelta`/`flashCards`/`clearBoard`/`cameraShake`/`flipSzoneCard`/`cardRotationFor`/`positionStateFor` 等）经 tsc 逐一对上 `DuelField3D` 的公开签名，无一需回退 `any`。
- **验证** — `tsc --noEmit` + 双构建全绿；无头冒烟 `stage_smoke` 32/32、`practice_smoke` 25/25、`prompts_smoke` 67/67 全绿。

## 执行清单（全部完成）

### P5 前端结构重构（按性价比排序）

1. ✅ `PromptHost.tsx`（779 行，9 个内嵌 Modal）拆 `components/prompts/` 子文件
2. ✅ 抽 `useCardSelection(min,max)` hook，收敛 CardSelect/SumSelect/Sort 三处重复状态机（约 60 行）
3. ✅ `domain/reducer.ts` 的 `applyEvent` switch 按事件族拆成 8 张 handler 表（见下方明细）
4. ✅ `duel/field3d.ts`（1383 行）拆 `field3d_geometry.ts` + `field3d_textures.ts` + `field3d_anim.ts`（见下方明细）
5. ✅ 消灭 `duel_manager.ts` 的 `field3D: any`（typed 为 `DuelField3D`，见下方明细）
- 每步验证：tsc + 双构建 + 12 冒烟页（重点 prompts/practice/stage）

### 最终门禁（✅ 2026-09-17 全绿）

- `go build ./... && go vet ./... && go test -count=1 ./...` 全绿（cmd/wails 21.6s、core/duel 21.4s、protocol 0.5s，余者无测试文件）
- 前端 `tsc --noEmit` + `vite build` + `vite build --config vite.smoke.config.js` 全绿
- 12 冒烟页全绿（344 项检查）：prompts 67、practice 25、stage 32、spec 11、settings 17、theater 38、single 19、menu 7、deck 34、lobby 33、widgets 49、replay 12
- （建议）整理后提交一个 commit 批次

## 备注

- 全程未 git commit，所有改动在工作区；smoke-dist 已从索引移除（`git rm -r --cached`）
- 本机无 cgo，`go test -race` 不可用；锁语义靠人工比对 `RoomLockMiddleware` 确认
- 审计发现的"房间生命周期语义"（去单房限制、host 离开不关服、空房回收）改动产品行为较大，未纳入 P4，建议单独立项并配双客户端 e2e 验证

## 追加：双客户端完整对战 e2e（2026-09-19）

- 新增 `cmd/wails/netplay_duel_test.go::TestTwoClientsAutoPlayFullDuel`：两个真实 TCP 客户端（WailsDuelClient 全事件链）经 `StartLocalServer` 建房/加入后，由自动驾驶按前端事件语义逐一应答引擎提示（time_limit 心跳、idlecmd 召唤、select_place 落点、battlecmd 攻击、select_chain 放弃等），用 40 张 4 星白板（基因狼人 69247929）打完一整局：通常召唤→战斗→伤害扣 LP→一方 LP 归零 MSG_WIN，断言双方看到一致的胜者且收到 duel_end/replay。`-count=5` 连跑稳定。
- 顺带修复真实 bug：`engine_bindings.go` 的 `decorateSelectPlace` 此前把引擎的**禁用区域**位掩码（`selectable_field = ~flag`）当成可选区域列表发出，前端自动落点会选中 EMZ/SZONE 等非法区域 → MSG_RETRY 被静默跳过 → 对局卡死。现已取反为真正的可选区域，并按 SELECT_DISFIELD 需要给每个 zone 带上归属方 `player`（含对手区域高位段）；前端 `duel_manager.ts` 的自动落点改用 `zone.player ?? data.player`。
- 验证：`go build/vet/test ./...` 全绿（含 netplay 两个 e2e），前端 `tsc --noEmit` + 双 vite 构建全绿。
