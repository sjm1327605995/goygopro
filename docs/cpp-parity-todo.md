# C++ 原版对照差距 TODO Plan

> 来源：2024 年对 `source/ygopro`（C++ 原版客户端/服务端）与当前 Go+Web 实现（`core/duel`、`cmd/wails`、`frontend/src`）的逐模块核验。
> 状态图例：`[ ]` 未开始 / `[~]` 进行中 / `[x]` 完成
> 结论基线：**服务端协议层与原版 1:1 对齐；差距集中在前端表现层与外围功能。**

## P0 — 功能正确性（必修）

- [x] **Tag 模式计时器不重武装**
  - 位置：`core/duel/tag_duel.go:591`（`TagTimer` 非超时路径不 `event_add`）
  - 原版依据：`source/ygopro/gframe/tag_duel.cpp:1740-1741` 每秒重新武装
  - 后果：tag 模式计时器走一秒后停止，超时判负永不触发；代码注释"与原版一致"与实际不符
  - 验证：新增 tag 计时器测试（快进虚拟时钟断言超时判负触发）
  - 完成（2026-09）：`core/duel/tag_duel.go:594` 非超时路径补 `AfterFunc` 每秒重武装；
    修正 `duel_common.go:114` / `tag_duel.go:498` 的错误注释；新增
    `core/duel/tag_timer_test.go`（真实 1s 轮询断言 `timeElapsed` 持续推进 +
    超时路径端到端判负/收尾，替代虚拟时钟——`timerWheel` 为全局真实时钟无可注入点）。

- [x] **未知 MSG 的静默错位风险**
  - 位置：`core/duel/analyze_shared.go:39`（未命中布局表只消费 1 字节后 continue）
  - 补登布局：`MSG_SORT_CHAIN(21)`、`MSG_ANNOUNCE_CARD_FILTER(144)`、`MSG_REQUEST_DECK(8)`、`MSG_DUEL_WINNER(200)`、`MSG_CUSTOM_MSG(180)` 到 `engine_msg_layout.go`
  - 未知类型改为显式降级（记日志 + EndDuel），而非静默 continue
  - 验证：`analyze_layout_test.go` 补充新消息的布局一致性测试
  - 完成（2026-09）：五条消息已登记（SORT_CHAIN 布局同 SORT_CARD 且 response=true，
    依据 edo9300 上游 SortCard 处理器；其余四条为上游仅有号码定义、无引擎写入方，
    登记空体）；`runAnalyze` 未知类型改为显式降级（布局已知→按布局跳过并记日志；
    布局未知/win 型→记日志 + EndDuel 兜底）。wails 侧（`game_msg_skip.go`）本就
    显式报错，未新增绑定/事件，`events_parity_test.go` 保持绿色。

- [x] **`ocgcore/include/ocgapi.h` 与实际导出不同步**
  - 缺 `create_duel_v2`、`new_tag_card` 声明（Go 已注册使用，仅头文件旧）
  - 对齐上游 ocgcore 头文件
  - 完成（2026-09）：补 `create_duel_v2(uint32_t seed[8])`（对齐 Go `*[8]uint32`，
    上游为 `uint32_t seed_sequence[]`）与 `new_tag_card(intptr_t, uint32_t, uint8_t, uint8_t)`；
    20 个 purego 注册函数现与头文件一一对应。

## P1 — 对战操作体验（差距最大）

- [x] **MSG_SELECT_PLACE 落点选择 UI**
  - 现状：`duel/duel_manager.ts` 自动应答第一个空位；设置项 `automonsterpos/autospellpos/randompos` 无消费方
  - 目标：复刻原版——可选区域位掩码 → 场上虚线高亮（`field3d.ts` 渲染可选格），点击格子累计选位，支持取消；`automonsterpos` 开时才自动选位
  - 关联：MSG_SELECT_DISFIELD 同样无 UI
  - 验证：`stage_smoke`/`prompts_smoke` 增加 select_place 交互断言
  - 完成（2026-09）：Go `emitSelectPlace` 把 MSG_SELECT_PLACE/DISFIELD 的禁用位
    掩码解码成带区域归属的 zones 列表（disfield 标志区分两询问）；前端
    `duel_manager.beginPlaceSelect` 按原版 automonsterpos/autospellpos 区域
    判定 + 区内优先序 6,5,2,1,3,0,4（randompos 开则区内均匀随机）代答，否则
    进入点选模式；`field3d.setPlaceSelectZones` 画未选=蓝色虚线呼吸/
    已选=琥珀实线高亮，点格累计选位、再点已选格取消、选满按原版区域序
    （己 mzone→己 szone→对 mzone→对 szone，seq 升序）经 RespondSelectPlaces
    一次回传；右键可取消询问回 [player,0,0]。冒烟断言：自动落点优先级、
    disfield 从不自动、点选/取消/选满应答、多选未满不应答、右键撤销最近一次。

- [x] **field_disabled 禁用格渲染**
  - 现状：事件只存 store（`domain/reducer.ts`），3D 不画禁用格
  - 目标：`field3d.ts` 对禁用区域渲染红叉/遮罩（对齐原版 drawing.cpp）
  - 完成（2026-09）：`field3d.setFieldDisabled` 按与 decorateSelectPlace 同构的
    位域扫描禁用格，每格画两条白色对角线（drawing.cpp:210-241），
    stage/prompts 冒烟断言叉线数量/位置/清除。

- [x] **MSG_BATTLE 攻防对撞浮层**
  - 现状：`duel:battle` 事件无订阅者（`reducer.ts:780` 注释称 3D 消费，实为 noop）
  - 目标：伤害步骤显示双方 ATK/DEF 数值对撞浮层 + 战破结果
  - 完成（2026-09）：新增 `components/BattleOverlay.tsx`（挂到 DuelStage，回放
    同样展示）——双方攻守数值两侧对撞 → 战破结果行（直接攻击/双方战破），
    卡名按 seat→显示座从 store.board 解析，2.4s 淡出；Go decorateBattle 把
    26 字节体末位的 ocgcore 战破旗标 bd[0]/bd1 译为 attackerDestroyed/
    targetDestroyed（原 AttackerDirect/TargetDirect 字段名实为误称）。

- [x] **LP 数字滚动动画**
  - 现状：LP 直跳；`playLPTick` 音效已定义无人调用
  - 目标：LP 变化时数字滚动 + 接入 `playLPTick`
  - 完成（2026-09）：`PlayerPanel.useRollingLP` rAF 二次插值 700ms，滚动中按
    ~110ms 间隔调 playLPTick；血条宽度仍用真实 LP（CSS 0.4s transition）。
    stage 冒烟用 playLPTick 计数证明滚动发生（虚拟时间下中间值采样不稳）。

- [x] **决斗中右键语义**
  - 现状：只有左键 ActionPopup；右键无行为
  - 目标：右键 = 取消当前选择/关闭菜单/忽略连锁（对齐 `event_handler.cpp`），墓地/额外等区域右键快速查看列表
  - 完成（2026-09）：`field3d` contextmenu 监听 → 命中墓地/除外/额外/卡组堆区
    （盘面交点与堆区中心矩形粗匹配）发 ui:show_pile 打开 CardListOverlay
    （F1-F8 同源；卡组无列表、墓地禁查时不发）；点空处关 ActionPopup，
    select_place 点选模式下右键撤销最近一次已选、无可退且询问可取消
    （count=0）时回 [player,0,0]（原版 CancelOrFinish）。

## P2 — 模式与外围功能

- [x] **TAG 组队 UI 补全**
  - 位置：`frontend/src/components/Lobby.tsx`（seats 4 席、TAG Mode=2 渲染 4 行）、
    `RightControls.tsx`（投降标记局间复位）、`cmd/wails/engine_bindings.go`
    （MSG_TAG_SWAP → duel:tag_swap）
  - 原版依据：wHostPrepare 恒建 4 座（game.cpp:308-317）、NETPLAYER_TYPE_OBSERVER=7
    （network.h:256）、MSG_TAG_SWAP 重排 dField（duelclient.cpp:3782）
  - 完成（2026-09）：座位数组扩到 4（ seatCount 由 HostInfo.Mode 决定 2/4）；
    player_enter/change 支持 2/3 号位；观战判定从 selftype>1 修正为 >=7（原
    写法会把 TAG 队友当观战者）；canStart 按模式要求 2/4 席全准备；聊天身份
    按队色 0/2 蓝 1/3 红。teammate_surrender 在 duel_start/duel_end 复位，
    确认弹窗补「队友已请求投降」提示。MSG_TAG_SWAP 接入 engineBindings
    （tagSwapMsg + decorateTagSwap，变长体 pbuf 消费，手牌/额外码非当前
    操作者已被服务端抹零），前端 reducer 刷新 deck/extra 牌堆计数与本队
    手牌、duel_manager 同步对方手牌张数；场面由服务端紧随的
    RefreshMzone/Szone/Hand update_data 刷新（与原版一致，不强制动画）。
    冒烟：lobby_smoke 增 TAG 4 座/开始使能/队友位即决斗者/selftype7 观战
    断言 + Go tag_swap 布局测试（cmd/wails/tag_swap_test.go）。

- [x] **逐张换副卡组（siding）**
  - 位置：`frontend/src/components/SideDecking.tsx`（重写）、
    `frontend/src/duel/side_deck_state.ts`（新增，上局卡组缓存）
  - 原版依据：deck_con.cpp is_siding 右击互换 + push/pop 容量 +5、
    BUTTON_SIDE_OK 的 pre_mainc/extrac/sidec 校验（SysString 1410）、
    服务端 LoadSide 同卡数校验（deck_manager.go）
  - 完成（2026-09）：换侧起点 = 上次 CTOS_UPDATE_DECK 发送的卡组（Lobby
    准备时写入缓存，缓存缺失回退首个预存卡组）；主/额外↔副点击或拖拽逐张
    互换（副→主/额外按卡类型自动分流）；编辑期容量按原版放宽 +5；
    「完成」校验三区数量与上一局严格一致后发送。保留载入预存卡组
    （BUTTON_SIDE_RELOAD 语义）与还原。widgets_smoke 的确认握手断言保持绿。

- [x] **禁限卡表数据接入卡组编辑器**
  - 位置：`cmd/wails/app.go`（LFListContent 绑定）、
    `frontend/src/components/DeckBuilder.tsx`（角标 + check_limit）
  - 原版依据：deck_con.cpp check_limit（limit 默认 3、命中 lflist 取 0/1/2）、
    DrawThumb 左上 lim 贴图（drawing.cpp:1176-1198）、Initialize 按
    use_lflist/default_lflist 选表
  - 完成（2026-09）：Go 暴露 LFListContent(hash) → 卡码→0/1/2 映射（哈希 0/
    未知返回空表 = 无限制）；编辑器按原版配置语义选表（use_lflist=1 时用
    default_lflist 索引，否则 N/A），卡组格与搜索结果画 禁/限/准 角标，
    加入卡时按表校验同名上限（0 禁卡拦截、1/2 限张数、未命中 3）。工具行
    显示当前表名。冒烟：deck_smoke 增角标渲染 + 准限 2 张拦截断言 +
    Go LFListContent 测试（cmd/wails/lflist_content_test.go）。

- [x] **LAN 房间发现接入前端**
  - 位置：`cmd/wails/lan_discovery.go`（新增，RefreshHosts 绑定）、
    `frontend/src/components/Lobby.tsx`（lstHostList 接入 + 刷新按钮）
  - 原版依据：duelclient.cpp BeginRefreshHost/BroadcastReply（请求 7920、
    应答 7921、3s 窗口、(ip,port) 去重、identifier/version 过滤）、
    hoststr 行文案拼接、LISTBOX_LAN_HOST 回填主机信息
  - 完成（2026-09）：Go 侧 UDP 发现：:7921 收应答 + 按本地 IPv4 接口向
    255.255.255.255:7920 发 HostRequest（另补 127.0.0.1 一发，同机自发现；
    原版靠栈环回不保证），identifier/version 不符丢弃、(ip,port) 去重；
    联机窗打开自动刷一轮 + 「刷新主机」重刷（防重入守卫）。列表按原版
    [卡表][规则][模式][标准/自定义]房间名 渲染，点击行回填主机信息输入框。
    冒烟：lobby_smoke 增列表渲染/回填/刷新断言 + Go 端到端测试
    （cmd/wails/lan_discovery_test.go，127.0.0.1 假应答端，去重+版本过滤）。

## P3 — 表现层打磨

- [x] **BGM 系统**：设置项 `enable_music/music_volume/music_mode` 存在但无播放代码；8 类场景 BGM（menu/duel/deck/advantage/disadvantage/win/lose）
  - 完成（2026-09）：新增 `frontend/src/audio/bgm_manager.ts`——原版
    sound_manager.cpp PlayBGM 的合成版：七类场景各一套和弦进行
    （pad 和弦 + bass 脉冲 + 琶音，不同音型/速度/明亮度），music_mode=0
    走通用音型（BGM_ALL 语义），同场景不重排。三源接线在 App.tsx：
    路由 effect（menu/deck/duel）、duelStore 订阅（LP 优劣 →
    advantage/disadvantage、胜负 → win/lose）、设置 effect（开/关、音量、
    模式）。ctx suspended 期间跳过排程，手势解锁后接续。
    settings_smoke 增 12 条场景/设置断言。
- [x] **音效资源**：当前 Web Audio 合成 12 种；原版 40 种 wav；闲置函数 `playPhaseChange`、`playTrapActivate` 接入或移除
  - 完成（2026-09）：playPhaseChange 接入 duel:new_phase（原版 SOUND_PHASE
    与阶段横幅同触发点）；playTrapActivate 接入连锁处理音效——
    chainVisualizer 记录连锁卡 TYPE，chain_solving 时陷阱/永续·场地魔陷
    播陷阱警报、其余播魔法晶音。其余合成音维持现状。
- [x] **无人消费事件收尾**：`duel:chained`、`duel:summoned/spsummoned/flipsummoned`、`match_kill`（胜利卡图展示）、`story:ended`（残留转发，无订阅则删）
  - 完成（2026-09）：duel:chained → 最新连锁徽章顿点盖戳
    （chainVisualizer.stampLatestLink）+ 日志；三种 summoned →
    field3d.settleCard（怪兽槽位小冲击波+压实弹跳）+ SysString
    1604/1606/1608 日志；match_kill → VictoryOverlay 展示击杀卡图+卡名
    （widgets_smoke 断言）；stoc:waiting_side 提示条已有（reducer →
    HintBar），补 duel:start 复位 hint；story:ended 死转发从
    events_forward.ts 删除（Go 侧 story 模式早已移除）。
- [x] **showcard 特效补全**：当前 SpecOverlay 仅部分 case；`confirm_cards` 为简化闪光（`field3d.ts:816`）
  - 完成（2026-09）：SpecOverlay 补齐 showcard=4（confirm_cards 单卡/
    HINT 淡入揭示）、showcard=6（CHINT_TURN 回合数大字盖戳，卡图从
    store.board 换算）、duel:hint HINT_EFFECT/HINT_CARD → mask 揭示；
    field3d.flashCards 升级为揭示序列（盖卡翻开抖动+抬升+黄色高亮）。
    spec_smoke 增 fade/number/hint-reveal 断言。
- [x] **洗牌/掷币/猜拳动画**：shuffle_hand 手牌重排动画、猜拳双拳下落动画（原版 drawing.cpp）
  - 完成（2026-09）：HandDock 订阅 duel:shuffle_hand（本方）→ 手牌坞
    .shuffling 类逐卡错位抖动重排（practice_smoke 断言类名+新序）；
    duel:hand_res → showcard=100 双拳下落对碰（emoji 拳+碰撞光环）；
    toss_coin/toss_dice → 硬币翻转/骰子弹跳图标+保留 ACMessage 文本
    （spec_smoke 断言）。顺带修正 reducer 骰子显示 +1 偏差（ocgcore
    结果本就 1-6，原版直接显示原值）。

## P4 — 基础设施

- [x] **strings.conf 多语言字符串**：当前 Go 侧 ResolveDesc 兜底 + 前端硬编码，部分提示退化为通用文案
  - 完成（2026-09）：仓库根 strings.conf 的决斗常用子集（选择提示 500-570、
    区域 1000-1009、阶段/时点提示、1390/1409/1500-1512、1603-1622、!victory
    0x0-0x23）整理为 `frontend/src/domain/sys_strings.ts`（sysString /
    victoryString / PHASE_LABELS 唯一表）与 `cmd/wails/sys_strings.go`
    （ResolveDesc 系统 id 直接返回文案，未收录仍空串兜底）。接线：
    reducer（阶段名/等待/换侧/召唤/错过时点/掷币）、PhaseStrip（横幅
    与 reducer 共用 PHASE_LABELS）、duel_manager（落点提示 560/570）、
    SpecOverlay（掷币正反面）、VictoryOverlay（胜负原因行）。
- [x] **引擎日志分级**：`ocgcore/duel.go:OnMessage` 仅 fmt.Println
  - 完成（2026-09）：OnMessage 改走可配置 logger（默认 stderr，前缀
    [ocgcore]），新增 `SetLogOutput(io.Writer)`（nil 恢复 stderr）与
    `SetDebugLogging(bool)` 开关；新增 ocgcore/duel_test.go 锁定行为。
- [ ] **房间并发**：当前刻意限单房间对齐 C++（`packet_handlers.go:78`），RoomManager 已有多房间能力——若要做服务器形态再放开（需产品决策）
  - 维持现状（本次跳过，按单房间对齐 C++）

## 建议执行顺序

1. P0 全部（正确性，改动小）
2. P1 按 select_place → battle 浮层/LP 动画 → 右键菜单
3. P2 按 TAG UI → siding → 禁限表 → 房间发现
4. P3/P4 视精力安排

每项完成后跑：`go test ./core/duel/ ./cmd/wails/`（需 `YGO_REPLAY_DIR`）+ 前端相关冒烟（`stage_smoke`/`prompts_smoke`/`practice_smoke`）+ 生产 dist 重建。

## 复核追加修复（2024）

服务端对照原版 gframe 复核出的 6 项残留缺口，本轮全部修复：

- [x] **A. TAG 投降标记不随回合复位**：`core/duel/tag_duel.go` `analyzeNewTurn`
  只轮换 curPlayer 从不清 `surrender[4]`；原版 `tag_duel.cpp:920-922` 每次
  MSG_NEW_TURN 全置 false。已补复位循环，新增
  `core/duel/tag_surrender_test.go`（A 队友投降→新回合→B 队友投降不判负，
  同回合两人投降仍判负）。
- [x] **B. Single 计时银行永不扣减**：`core/duel/single_duel.go` `GetResponse`
  在扣减前 `timeElapsed=0`（恒减 0），且多写了原版没有的
  `lastResponse = dp.Type`。对照 `single_duel.cpp:1419-1423` 删除整块
  （lastResponse 已由 waitForResponse 记录，回写会破坏其「被等待者」语义），
  扣减后清零。新增 `core/duel/single_timer_test.go`。
- [x] **C. wails 建房不启动 LAN 广播应答**：`cmd/wails/app.go`
  `StartLocalServer` 现按 `cmd/server` 写法启动 `duel.StartBroadcast`
  （:7920 被占只记日志不影响建房）。`core/duel/broadcast.go` 新增
  `Running`/`EnsureBroadcast`（记录上次端口，建房时若广播已停则重启，
  对齐 `netserver.cpp:313` CTOS_CREATE_GAME 即 StartBroadcast），
  `packet_handlers.go` HandleCreateGame 挂钩。`StopListen` 连带停广播
  经核对与原版一致（`netserver.cpp:79-82` StopListen 内含 StopBroadcast，
  `single_duel.cpp:325` 的 StopBroadcast 注释掉正是因为冗余），维持现状并
  在 `duel_mode.go` 加注。
- [x] **D. 密码错误加入报"房间人数已满"**：`packet_handlers.go`
  HandleJoinGame 在 GetRoom 失败时，若服务器存在任意房间则回 JOINERROR
  code=1（原版 SysString 1404「密码错误」，前端 Lobby describeErrorMsg
  code=1 分支自动生效）；无房间才回 code=0。`packet_context.go` 新增
  `SendErrorWithCode`/`AbortWithErrorCode`，`packet_errors.go` 新增
  `ErrWrongRoomPassword`。新增 `core/duel/packet_handlers_test.go`。
  单房间语义不变。
- [x] **E. single 给观战者多发 STOC_TIME_LIMIT**：`single_duel.go`
  `timeLimitToObservers` 由 true 改 false（原版
  `single_duel.cpp:1451-1452` 只发 players[0]/[1]），并修正
  `duel_common.go` 接口注释（原注释错误声称原版 single 发观战者）。
- [x] **F. MSG_START 布局字节数**：`engine_msg_layout.go` `MSG_START`
  由 fixed(15) 改 fixed(18)（playertype1+duelrule1+lp8+deck/extra8，
  服务端合成 startbuf 19 字节含 opcode）。`protocol/message.go` StartMsg
  镜像定义本就 18 字节，一致无需改。

## HINT 系列补齐（2026-09，前端对照 duelclient.cpp:1084-1206）

- [x] **HINT_SELECTMSG（type 3）选择提示**：reducer 新增
  `selectHint`（原版 `DuelClient::select_hint`，duel:hint type 3 存入，
  新 hint 覆盖、duel:start 复位）与 `selectHintText`（选择进行中常驻
  文本，对应原版 stHintMsg 的 `提示(min-max)`）。消费方：
  PromptHost 的 select_card（含 tribute，Go 已并入 duel:select_card）/
  select_unselect（不 consume，对应原版 select_unselect_hint 跨询问
  持续）/select_option/select_sum/announce_race/attrib/card/number
  标题优先用 selectHint（sysString 表解析系统 id、ResolveDesc 解析
  desc id，格式 `提示(min-max)`）；duel_manager.beginPlaceSelect 的
  SELECT_PLACE 用 SysString 569+卡名、DISFIELD 用 desc 文本（自动
  代答路径同样 consume）。CardTooltip 在 selectHintText 非空时悬停
  首行附带选择提示。应答/落点结束即 disarm。
- [x] **HINT_OPSELECTED(4)/RACE(6)/ATTRIB(7)/CODE(8)/NUMBER(9)
  宣言展示**：Go 侧 `duel:hint` 本已全量转发（HintMsg type/player/
  data），无需改 Go。reducer 落日志（SysString 1510/1511/1512，种族
  属性用 constants.ts 新增 formatRace/formatAttribute，卡名走
  cardName 缓存）；SpecOverlay 弹 stACMessage 浮条（desc id 经
  ResolveDesc、卡名经 getCard 异步精解）。
- [x] **HINT_ZONE（type 11）区域高亮**：field3d 新增
  setHintZones/clearHintZones（绿色实线+浅填充，与 select_place 蓝
  虚线区分）；duel_manager 译位掩码（与 select_place 同位域，对方
  操作时高低 16bit 互换），0.8s（原版 WaitFrameSignal(40)）自动清
  除，新 hint/阶段切换/清盘时清除；reducer 按 duelclient.cpp:1174-
  1204 逐格译区域描述落日志（SysString 1510）。sys_strings.ts 补
  569/1081。
- 验证：prompts_smoke 增 selectHint 标题/消费/arm-disarm、宣言日志、
  HINT_ZONE 高亮/对方换位/阶段清除/自动清除断言；spec_smoke 增
  NUMBER/RACE/ATTRIB/CODE/OPSELECTED 浮条与 tooltip 选择提示断言。

## 观战/回放交换视角 + select_card 场上点选（2026-10，前端对照 game.cpp/event_handler.cpp/drawing.cpp）

- [x] **交换视角（btnSpectatorSwap game.cpp:913 / btnReplaySwap :901 →
  DuelClient::SwapField / ReplayMode::SwapField）**：store 新增
  `viewSwapped`（reducer.ts），`localSeat()` 结果随其翻转——所有 seat→
  显示座换算点自动跟随；`applyViewSwap()` 在切换时对调已累积的显示座
  数组（names/lp/piles/timer/board/turnPlayer/win/hand↔oppHand），保证
  前后事件应用连续。手牌改为双侧维护（`hand`/`oppHand`，draw/move/
  shuffle_hand/tag_swap/update_data 按显示座落写），交换后手牌坞看对侧
  ——在线对局对侧手牌码本就被服务端抹零，可见性严格遵循服务端数据。
  field3d 新增 `setViewSwapped()`：相机 tween 绕到场地另一侧（等效场地
  旋转 180°，卡牌朝向随观看侧自然正确），远侧手背行 z 随视角翻边
  （`setOpponentHandCount`→`setFarHandCount`，DuelManager 改双座
  handCounts 跟踪）。观战态 RightControls 出「切换视角」（SysString 1346，
  #spectator-swap-btn），回放剧场播控行出同款（#replay-swap-btn）。
  DuelManager 的 showHintZones/onPileRightClick/resyncFromStore 的
  seat↔显示座换算改走 bottomEngineSeat/seatToDisplay。
- [x] **select_card / select_unselect 场上点选（drawing.cpp:431-436
  DrawSelectionLine 黄框 + event_handler.cpp 直接点卡）**：reducer 新增
  共享选择态 `cardSelect`（kind/min/max/cancelable/cards/selected，
  mzone/szone 候选标 onField）；duel_manager 对 onField 候选布 3D 黄框
  高亮（未选虚线呼吸/已选实线填充，field3d `setCardSelectMarks`），点击
  raycast 命中卡 mesh 即点选/再点取消；非场上候选走弹窗，混合来源时
  弹窗（新 StoreCardSelectModal，选择集外置到 store）与场上高亮并存、
  经 store 同步；纯场上不开弹窗，由 #card-select-bar（原版 stHintMsg +
  btnCancelOrFinish 场形态）承担提示/完成/取消。**选满 max 立即自动
  应答**（顺带修复审计轻微项；含「已达 min 且可选卡全部被选中」分支，
  对齐 event_handler.cpp:1389-1398），应答出口收编到
  duel/card_select.ts。select_unselect 场上点中即应答合并下标。
  tribute（MSG_SELECT_TRIBUTE）经 Go 侧新增的 `tribute` 标志排除在
  选择态之外，保持原弹窗路径（CheckSelectTribute 求和校验不在本项
  范围）；sum/counter 等弹窗不变。
- 验证：prompts_smoke 新增 fieldSelect*/mixedSelect*/unselectField*/
  tribute*/observer swap 共 17 断言，既有 select_card 用例改按自动
  应答语义；stage_smoke 新增 swap-* 5 断言（LP/手牌坞/相机/事件映射
  连续）；theater_smoke 新增 replay-swap-* 4 断言；single/practice/
  replay/spec/widgets/deck/menu/lobby/netplay 冒烟回归全绿；
  `go test ./cmd/wails/` 通过。

## 卡组码导入导出 + wDeckManage + announce_card 实时过滤（2026-11，对照 deck_con.cpp / client_field.cpp）

- [x] **卡组码导入导出（deck_con.cpp:404-424 BUTTON_IMPORT/EXPORT_DECK_CODE）**：
  导出 = 当前卡组序列化为原版剪贴板 ydk 文本（`#created by`/`#main`/`#extra`/
  `!side`，卡号十进制逐行，`serializeDeckYdk`），优先 `navigator.clipboard`
  写剪贴板，Wails/受限环境失败时弹只读文本框手动复制；导入 = 粘贴 ydk
  文本解析三区（`parseDeckYdk`，与 Go `LoadDeck` 同语义：分段头/`#` 注释/
  空行跳过、非法卡号行丢弃），确认后加载进编辑器（标脏、命名「导入卡组」、
  沿用现有未存保护确认，不自动落盘）。入口在 DeckBuilder 工具栏
  （#deck-export-code-btn / #deck-import-code-btn）。
- [x] **卡组/分类管理窗口（原版 wDeckManage，game.cpp:660-683 +
  deck_con.cpp:423-676）**：新组件 `DeckManageModal.tsx`——左分类列表
  （未分类 + ./deck/ 子目录）右该分类卡组列表；分类新建/重命名/删除，
  卡组新建/重命名/删除/复制到分类/移动到分类（目标分类下拉 =
  cbDMCategory，复制保持同名另存）。Go 侧新增 `cmd/wails/deck_manage.go`
  绑定：CreateDeckCategory/RenameDeckCategory/DeleteDeckCategory/
  RenameDeck（新名不带 '/' 沿用原分类）/CopyDeck/MoveDeck，目录语义对照
  deck_manager.cpp CreateCategory 等，重名/穿越写法一律拒绝
  （safeCategoryName + safeDeckPath 复用）。操作后编辑器清单同步刷新，
  已载入卡组被改名/移动/删除时脱离关联（内容保留为未保存状态）。
- [x] **announce_card 输入实时过滤（client_field.cpp:1535-1569
  UpdateDeclarableList）**：`AnnounceCardModal` 顶部新增
  #announce-card-filter 搜索框，150ms 防抖经 `searchCards`（`$卡名`
  语法）查卡库——纯数字/片段按卡号包含本地匹配候选，卡名精确匹配置顶
  （原版 insertItem(0)）；有候选清单（decodable）时与清单求交，无候选时
  直接展示检索结果供点选（不再只有裸输卡号一条路，该路径保留）。
  与原版差距：is_declarable 的 opcode 判定在 Go 侧解码 candidates 时已
  完成，自由检索结果未过 is_declarable（服务端仍会校验宣言合法性）。
- 验证：`go test ./cmd/wails/`（新增 deck_manage_test.go：分类三操作 +
  改名/复制/移动含覆盖拒绝与非法名用例）全绿；deck_smoke 新增 16 断言
  （导出剪贴板格式/失败回退框、导入三区加载/标脏/垃圾文本拒绝、管理
  窗口分类与卡组全操作 + 清单同步）；prompts_smoke 新增 7 断言（过滤框
  出现、卡名/卡号过滤、清空恢复、点选应答、无候选自由检索点选）；
  typecheck / build / build:smoke 全绿。

## 连接怪互连高亮（DrawLinkedZones/CheckMutual）+ 装饰设置键定格（2026-09，对照 drawing.cpp:243-359）

- [x] **连接怪互连箭头/区域高亮（drawing.cpp:252-254 触发 + 278-359
  DrawLinkedZones/CheckMutual）**：数据链路走 Go 权威源——引擎 mzone
  刷新 flag 本就含 QUERY_LINK（服务端 0x881fff/RefreshSingle 0xf81fff），
  `decodeQueryBody` 已解出 `link/linkMarker` 键；本次把单人谜题刷新 flag
  由 0x681fff 修正为原版 SinglePlayRefresh 的默认 0xf81fff
  （single_mode.h:24），单人路径同享。前端 `BoardCard` 新增
  `type/link/linkMarker`（update_data/update_card 合并语义：query flag
  被缓存剔除时沿用旧值），DuelManager.syncLinkData 在
  update_data/update_card 后把它们写进 mzone mesh 的 userData。
  field3d 新增 computeLinkedZones（DrawLinkedZones 逐行移植，引擎座标
  运算，共享额外怪区 5/6↔6/5 的互连回指查空替换也照原版）+
  updateLinkedZones/clearLinkedZones：悬停 type&TYPE_LINK 的场怪时把
  箭头指向的格画半透明色块，单向 = 原版 0xff0261a2 蓝、互连（目标格
  连接怪带指回来的标记，CheckMutual）= 0xff009900 绿；悬停切换/移开/
  清盘/update_* 场面变化时清除或重算。视角交换无需换算（高亮在世界
  座标，setViewSwapped 只搬相机），观战/回放经同一 manager 生效。
  原版 duel_rule>=4 的分支恒取（本前端场地固定 MR2020 布局）。
- 验证：stage_smoke 新增 10 断言（数据到达 mesh/store、邻格互连绿、
  单向蓝、EMZ ↓↑ 跨边指向对方 mzone、非连接怪清除、移开清除、视角
  交换后重算稳定），54/54 全绿；prompts 131、practice 27、single 19、
  replay 12 回归全绿；typecheck / build / build:smoke 全绿；
  `go test ./core/duel/ ./cmd/wails/` 全绿。

## 刻意不做（装饰设置键）

下列 system.conf 键在 React 固定布局与本仓库资源约束下语义不成立，
保留键位（settings.ts 默认值与 Go config 读写同步）但不做行为实现：

- **separate_clear_button**：原版把「清除」从卡组编辑器按钮行拆成独立
  按钮的纯布局开关；React 工具行固定排布，无对应两种形态。
- **resize_select_window / resize_popup_menu**：原版按条目数动态改变
  wCardSelect/弹窗高度的窗口管理行为；Web 侧弹窗是 CSS 自适应的固定
  布局，resize 语义弱化到不可辨。
- **prefer_expansion_script**：依赖 expansions/ 卡包目录（先行卡脚本包），
  本仓库没有该目录结构，无可择优对象。
- **ignore_deck_changes**：原版在卡组与上次提交不一致时弹提醒的提示类
  功能；联机准备流程已有服务端 LoadDeck 校验兜底，提醒屬纯装饰。
