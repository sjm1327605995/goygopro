# C++ 原版对照差距 TODO Plan

## 前后端交互逻辑 CodeReview 修复（2026-10-19）

对 Go 桥接层（事件生产侧）与前端事件消费侧做专项审计，修复真实行为缺陷
与维护性炸弹（验证：Go test / tsc / 双构建 / 14 页冒烟全绿）：

### Go 侧（cmd/wails）

- [x] **STOC Unpack 吞错发伪造事件**（stoc_handlers.go）：截断/错位包以前
  按零值结构体 emit（如 lp0=0 的 duel:start）；改为记日志 + 丢弃整包，与
  handleGameMessage 错误路径一致。
- [x] **Connect 僵尸状态**（duel_client.go）：sendPlayerInfo 失败时走
  abortConnect（代际守卫）收尾到断开态并广播 stoc:disconnected，不再留下
  isConnected=true 的不可用连接挡住 StartSingle 守卫。
- [x] **重连串事件**（readLoop）：代际检查下沉到分发点——读包期间连接被
  替换时丢弃剩余旧会话包，不混入新事件流。
- [x] **持锁 emit**（Disconnect/teardown）：改为解锁后 emit，消除 emit 链路
  反向取锁的自死锁隐患。
- [x] **StopSingle double-close 竞态**：select-default-close 换 stopOnce
  （sync.Once），Wails 绑定方法不保证跨调用串行。
- [x] **event_dto.go 死代码**：删除 26 个零引用 DTO（含形状已漂移的
  selectPlaceDTO/zoneRefDTO 误导性类型）；updateCardDTO 的字节拼接
  MarshalJSON 改匿名内嵌 *queryCardDTO（json 字段提升，形状不变）。
- [x] **decodeQueryBody 越界 panic 构造性脆弱**：u32/i32p/count 闭包自带
  钳制（越界读不到 → failed），漏写 ensure 的新 flag 分支不再 panic。
- [x] **PlayReplay 吞解析错误**（app.go）：desync 时记日志（录像照常回放）。
- [x] SendHandResult/SendTPResult 的 Pack 错误不再吞；UTF-16 定长编码三处
  重复抽 writeFixedUTF16；replayDir 跨层直改私有字段改 SetReplayDir（加锁）；
  events_parity_test 扫描名单补 single_mode.go；STOC_ERROR_MSG 分发前重复
  解码移除。
- 未改（记录）：StartLocalServer 重复调用泄漏旧广播/无停服通道（需存 server
  句柄的产品级改动）；MSG_CHAIN_NEGATED/DISABLED 合并事件语义保持（原版
  同 case）。

### 前端侧（frontend/src）

- [x] **EventBus.emit 异常隔离 + 快照遍历**：单监听器抛错只 console.error
  不再中止其余监听者/冒泡回 Wails 回调；回调里 off（dispose 链）不跳过后续。
- [x] **PresentationQueue.drain 保护**：deliver 抛错记日志后按原定延迟继续
  调度剩余事件，队列不再因单事件卡死到下一次 enqueue。
- [x] **retry 重放走表现队列**（store.ts）：与原始询问同通道，不再插队到
  积压动画之前；清理时机扩展到 duel:end/single:ended/disconnected。
- [x] **duel:hint 过期校验**：_hintSeq 序号——await resolveDesc 期间有新
  hint 到达时旧续体放弃写入提示条。
- [x] **dispose() 清理补全**：hintZoneTimer、field3D 六个回调全清（解开
  manager↔field 互相引用）、placeSelect/idleCmd/battleCmd 复位、_disposed
  标志；draw/召唤三事件/move/chaining 的 await 续体统一早退守卫。
- [x] **fireAndForget 统一入口**：minimise/stopSingle/toObserver/toDuelist/
  kickPlayer/leaveGame/surrender/sendTimeConfirm/setReady/startDuel/sendChat/
  sendHandResult/sendTPResult 的 Go 调用失败只记日志，消除 unhandledrejection。
- [x] **getCard 失败不缓存**（与 getCardImage 的 miss 缓存语义区分）；
  _picCache 恒假防御代码移除；chaining 双订阅合并。
- 未改（记录）：事件载荷 any 类型的 discriminated union 化（大工程，需逐步）；
  presentation_queue frames() 小数毫秒仅调度处 ceil 的小瑕疵。

## 战斗 UI neos-ts 风格重皮肤（2026-10-19）

参考 [neos-ts](https://github.com/DarkNeos/neos-ts)（React+antd 的 YGOPro Web
复刻）的决斗界面风格，对战斗场景 2D HUD 做整体重皮肤：**布局几何全部保持**
（stage_smoke 的原版坐标断言不改语义），只换视觉皮。

- [x] 新增 `frontend/css/duel-neos.css` 覆盖层（在 duel-original.css 与
  cpp-parity.css 之后加载）：neos 设计变量（深海军蓝底 #0d1522、玻璃面板
  rgba(13,21,34,.86)、蓝色点缀 #4aa3ff、圆角 10-14px、柔阴影、backdrop blur）。
  所有选择器带 `:is(#app,#root)` 前缀 + 元素选择器抬特异性，压过
  cpp-parity 的 Irrlicht 灰斜面层（其 `.duel-stage button:not()` 规则
  (1,2,1) 需 `button.xx` (1,3,1) 或同特异性后加载胜出）。
- [x] **按钮总闸**：`.duel-stage button:not(.hand-card-item)` 统一回暗色
  胶囊；阶段钮/连锁键/投降键/页签/弹窗钮各自细化（当前阶段蓝底、可行动
  蓝描边、pressed 蓝高光）。
- [x] **玩家面板**：去 lpf.png 木框 → 暗色圆角轨道（fill 仍用 lp.png 彩条
  保留分层语义）；LP 数字黄字描边 → 大白字；回合高亮红描边 → 蓝色高光；
  时限条 10px 灰条 → 6px 圆条（y=34 几何不动）；计时/堆区计数改 neos
  文字层级（数值蓝字）。
- [x] **左预览面板**：浅灰原型 → 深色玻璃，页签暗色（激活蓝），卡名白字 /
  类型蓝字 / 描述灰字。
- [x] **提示/浮条**：stHintMsg 白底黑字 → 顶部暗色胶囊；stTip 白底 →
  neos 暗色卡提示（JSX 内联样式迁出为 class）；ACMessage 浅蓝白底 →
  暗色圆条；阶段/胜利横幅去硬描边改柔光；胜利原因行改暗色胶囊。
- [x] **弹窗/菜单**：决斗作用域 .modal-box/.wcs-window 深色玻璃圆角；
  金钮 → 蓝主钮；动作弹出菜单/堆区列表/投降确认/场形态选择条/聊天输入/
  手牌坞统一暗色皮。LP 浮动字保留红/绿/蓝语义色。
- [x] CardTooltip/SpecOverlay 的内联配色迁为 class（几何内联保留）。
- [x] 断言更新：stage_smoke 的 lpf 纹理/红描边/黄字/名字内缩/时限条高度
  5 断言改新设计值（几何断言不动）；spec_smoke 的 stTip 白底断言改暗色。
- [x] 接齐用户 5074e2a 的单人录像窗流程：ReplaySavePrompt 单人模式
  取消/保存/自动保存均经 finishSingle() 回主菜单（net 模式不变）。
- 验证：14 页冒烟全绿（stage 76、prompts 174、single 22、spec 36、
  widgets 71、ui_parity 75 等）；tsc + 双 vite 构建 + Go test 全绿。
  前后截图：tmp_dbg/duel-before.png / duel-final.png。

## 全仓残留差距批（2026-10-16，五路审计后实施）

对照审计（duelclient/event_handler/drawing/game/menu/deck_con/replay_mode +
netserver/single/tag/replay/deck_manager/data_manager 逐行复核，排除已完成项）
出的残留差距，本批修复：

### 批次A：Go 服务端（core/duel、cmd/wails、protocol、ocgcore）

- [x] **Tag UpdateDeck 空指针 panic**：`tag_duel.go` pDeck 是 `[4]*Deck` 初值
  nil（原版 tag_duel.cpp:281 是值成员），首传卡组即 nil deref，tag 房间无法
  开赛。补与 single 同款的兜底分配 + 重复上传清空（原版 LoadDeck 入口
  deck.clear()）。新增 `tag_duel_test.go::TestTagUpdateDeckAllocatesPDeck`。
- [x] **Tag GetResponse 未截断响应**：对齐 tag_duel.cpp:1519-1520 的
  SIZE_RETURN_VALUE 截断（single 侧已有），杜绝 >512 字节响应切片越界。
  新增 `TestTagGetResponseTruncatesOversize`。
- [x] **AcceptingConnections 置 false 后永不复位**：每场对局结束停服后只能
  办一场。StartLocalServer 成功启动新监听即复位（对齐原版 StopListen 只
  停收、StartServer 重建监听恢复接受）。
- [x] **Tag 刷新参数表系统性偏离**：Go tag 全套 0x81fff4/0x781fff/无缓存，
  而 C++ tag_duel.h:35-40 与 single_duel.h:35-40 默认参数完全相同
  （0x881fff/0x681fff/0x781fff/0x81fff/0xe81fff，use_cache=1）。统一为
  `defaultRefreshFlags()` 单表；连带修正 analyzeShuffleExtra 的
  RefreshExtra（两模式均默认 0xe81fff/缓存1，single_duel.cpp:863、
  tag_duel.cpp:834）与 tag refreshOnDuelStart（tag_duel.cpp:467-468）。
- [x] **Single MSG_NEW_PHASE 不刷新手牌**：恢复 single_duel.cpp:942-943 的
  RefreshHand(0)/(1)（旧注释"与原版一致"系误读）。
- [x] **skipCardQuery 边界**：统一为 clen <= LEN_HEADER（C++ 两模式一致，
  single_duel.cpp:1484/1505/1526），修正"原版两种实现都必须原样保留"的
  误读注释。
- [x] **waitForResponse 多重置 timeElapsed**：删除（C++ WaitforResponse 不
  触碰，single_duel.cpp:1443-1456；清零只在扣减后与 TimeConfirm<10s）。
  新增 `TestWaitForResponseKeepsTimeElapsed`。
- [x] **Replay 加固/头部**：OpenReplay 补 datasize > MAX_REPLAY_SIZE 拒绝
  （replay.cpp:117-118，防伪造头越界）；录像头 header_version 置 1
  （C++ 类内初始化 {1}，replay.h:39）。
- [x] **rule_code 全量推导**：CardDataC 新增 RuleCode 字段与 DuelCode()
  （get_duel_code 语义 data_manager.h:60-62）：LoadDB 按 data_manager.cpp:
  61-68 迁移非异画 alias（5405695 特例）、全表后继承 alias 目标 rule_code
  （:86-92）；CheckDeck 同名/禁限计数改用 DuelCode()。
- [x] **extra_setcode 补丁**：data_manager.cpp:24-27 的 8512558/55088578
  系列码覆盖此前从未填充（Go 声明了 map 但恒空）。
- [x] **空聊天消息**：CheckMsgSize 下限对齐原版 CreateChatPacket 只查 NUL
  结尾（netserver.cpp:374-386），2 字节空消息照发。
  新增 `data_manager_test.go`（DuelCode 优先级/rule_code 不变式/
  extra_setcode/CheckMsgSize 用例）。
- 维持现状不改：StateMiddleware 拒绝时回 STOC_ERROR_MSG（C++ 静默丢弃，
  自制前端的刻意差异）。

### 批次B：消息链路（MSG_RETRY / SWAP_GRAVE_DECK / SHUFFLE_DECK / MOVE 落位）

- [x] **MSG_RETRY**：Go 侧不再跳过，emit `duel:retry`（服务端 analyzeRetry
  只重发 RETRY 字节，不重发上一询问）；前端 `duel/store.ts` 记录最近询问
  + select_hint 快照，retry 到达按原版 duelclient.cpp:990-1043 弹错误文案
  （sys_strings 补 1421-1431）、恢复 select_hint、经总线重放询问重开弹窗。
  store 新增 `restoreSelectHint`。
- [x] **MSG_SWAP_GRAVE_DECK**：Go emit `duel:swap_grave_deck`；reducer 交换
  该座 deck/grave 计数并清空墓地列表，紧随的 RefreshGrave 经扩展后的
  update_data（GRAVE/BANISH/EXTRA 堆区整表替换，对齐原版 UpdateFieldCard
  无区域白名单）重建；update_card 同步支持堆区单卡合并。额外列表腐坏
  （shuffle_extra 后 RefreshExtra 被丢弃）同路修复。
- [x] **MSG_SHUFFLE_DECK 表现**：Go emit `duel:shuffle_deck`；sound_manager
  新增合成洗牌音（原版 SOUND_SHUFFLE），field3d 新增 `shakeDeckPile`
  （计数浮签正弦抖 5 轮，卡组无实体 mesh），shuffle_extra 同步播音。
- [x] **MSG_MOVE pl==0 落位**：从不可见区直入 mzone/szone 的卡此前不产生
  mesh（等 update_data 漂移重建），现 placeCard + `fadeInMesh` 20 帧淡入
  （原版 FadeCard，client_field.cpp:1107-1111）；placeCard 槽位复用前先
  静默清旧 mesh（修 replay seek 双份网格泄漏）。
- 验证：Go build/vet/test 全绿；tsc + 双 vite 构建全绿；ui_smoke_suite
  14 页全绿（prompts 165 项含 retry×5/swap_grave_deck×4 新断言）。

### 批次C：前端交互还原（event_handler.cpp / game.cpp / menu_handler.cpp / replay_mode.cpp）

- [x] **SELECT_POSITION 单选项自动应答**（duelclient.cpp:1916-1919）：只有一
  个可选表示形式时直接应答不弹窗。
- [x] **是/否询问右键 = 否**（event_handler.cpp:2564-2571 CancelOrFinish）：
  select_yesno/select_effectyn 弹窗期右键应答 0，与连锁右键取消同通道。
- [x] **宣言属性/种族选满自动应答**（event_handler.cpp:843-846/857-860）：
  BitmaskModal 勾选数达到 announce_count 立即应答并关窗；连点闭包用 ref
  同步避免同渲染批次旧值漏判。SortModal 排满自动应答同理（:784-791）。
- [x] **回放播完提示「录像结束」**：ReplayTheater advance 到末尾按原版
  replay_mode.cpp:223-251 EndReplay 弹 SysString(1501)；原版随后自动回录像
  窗，Web 形态刻意保留事件流以便回退步/重看（差异项，记此）。
- [x] **回放单步/撤销播放中先暂停**（原版 event_handler.cpp:68-71 两钮仅
  暂停后可见）：修播放中点单步与推进定时器并发双重推进。
- [x] **回放/观战可浏览堆区**（event_handler.cpp:1113-1162）：解除
  onPileRightClick 的 interactive 限制；CardListOverlay 在 compact（剧场）
  也挂载。
- [x] **点超量怪「查看素材」**（event_handler.cpp:1211-1216 COMMAND_LIST）：
  双方场上带素材的怪点击弹动作菜单含查看素材，CardListOverlay 支持
  ui:show_pile 带 seq 只看该槽素材。
- [x] **右键收起堆区列表**（event_handler.cpp:2535-2562）：新增
  ui:close_pile 事件，CancelOrFinish 路径发出。
- [x] **选择期间悬停照做**（event_handler.cpp:1507-1704）：field3d
  handleHover 在落点/选卡点选期间不再 early-return，悬停照常更新卡片
  详情与 stTip，仅光标体现可选态。
- [x] **大厅准备后锁定卡组下拉**（menu_handler.cpp:178-185）：卡组与分类
  下拉 disabled={isReady}，取消准备解锁。
- [x] **加入游戏连接防重入**（menu_handler.cpp:94-98）：joiningRef 守卫。
- [x] **建房禁限卡表默认值跟随配置**（game.cpp:236）：use_lflist=1 选
  default_lflist 索引，否则 N/A（同 DeckBuilder 语义）。
- [x] **设置面板补 5 个有消费方但无开关的键**：use_lflist+default_lflist
  （LFListRow 勾选+选表）、music_mode、auto_search_limit、
  search_multiple_keywords、ignore_deck_changes。
- 验证：tsc + 双构建全绿；prompts 168 项（含种族选满/排序排满/表示形式
  单选/右键=否 5 项新断言）、theater 53 项全绿。

### 批次D：表现层还原（drawing.cpp / duelclient.cpp 表现分支）

- [x] **LP 三事件事件级音效**：duel:damage/recover/pay_lpcost 播
  SOUND_DAMAGE/SOUND_RECOVER（新增 playRecover 合成音）合成等价；
  数字滚动本身仍不加 tick（todo 刻意项）。
- [x] **连锁应答即清 act 角标**（client_field.cpp:404-420 ClearChainSelect）：
  chaining 到达即 clearActivatable，不残留到 chain_end/下一阶段。
- [x] **draw_single_chain 设置消费**（drawing.cpp:541）：连锁序号角标仅
  在连锁数>1 或开启该设置时绘制。
- [x] **额外卡组双数字**（drawing.cpp:726-727,749-750）：store PileCounts
  增 extraP（reload_field 的 extraP / tag_swap 的 extraFaceUpCount 落库），
  额外堆浮签渲染「总数(表侧数)」。
- [x] **卡背按控制方区分**（drawing.cpp:425 tCover[controler]）：
  1 号位加载 cover2.jpg，createCardMesh 增加 player 参数。
- [x] **指示物增减浮条+音效**（duelclient.cpp:3390-3430）：sys_strings 补
  1617/1618，SpecOverlay 按位置解析卡名 + getCounterName 弹 stACMessage
  浮条；sound_manager 新增 playCounter。
- [x] **HINT_EVENT 进询问弹窗**（duelclient.cpp:1084-1086/1524/1813-1817）：
  store 增 hintEvent，select_effectyn/单候选连锁的文案以 event_string 开头。
- 验证：同批次C 全量验证。

### 本批未做（审计残留，后续可选）

- SELECT_CHAIN 空列表 + always_chain 仍弹询问、chkAutoChain 只代答强制连锁
  的原版语义（现有三键模型与原版有出入，改动会动 A/S/D 热键与冒烟契约）。
- MSG_CONFIRM_DECKTOP/EXTRATOP 逐张揭示动画（duelclient.cpp:2086-2160）。
- is_highlighting 青色选框（EFFECTYN/CHAINING/BECOME_TARGET/RANDOM_SELECTED，
  drawing.cpp:437-442）。
- MSG_BECOME_TARGET 对象卡三次闪烁、MSG_RANDOM_SELECTED 30 帧高亮。
- ANNOUNCE_NUMBER ≤12 时的原版 12 键数字盘（duelclient.cpp:3673-3709）。
- 灵摆刻度场上常显数字（drawing.cpp:689-723）、悬停空格/堆区呼吸高亮
  （drawing.cpp:242-276）、卡组/额外实体卡叠层（drawing.cpp:360-378，
  连带 deck_top/REVERSE_DECK 表侧承载）、conti_act 永续角标堆
  （drawing.cpp:523-540）、CHINT_DESC_ADD/REMOVE 悬停效果文本。
- 单人模式查看卡组列表（需 deck 内容落 store，数据面改动）。
- CLI 参数/文件关联（gframe.cpp:24-202）、录像信息面板 version 行
  （极轻）、聊天淡出 12s→20s 节奏。

## 实机对照后的修正（2026-10-15）

- [x] Wails 实时事件增加 WaitFrameSignal 表现队列，伤害/回复/支付等待 41 帧、
  LPUPDATE 等待 11 帧、阶段/回合/攻击等待 40 帧；胜利等前序表现结束后再播放。
  新局和退出清理待播消息，自动回放使用同一时长表，seek 保持瞬时重建。
- [x] LP 数字、血条和浮字共用 lpChange 与逐帧采样：前 30 帧保持，后 10 帧
  按整数 lpd 递减，最后补齐余数。伤害红色、回复绿色、支付蓝色，固定位置；
  去掉 700ms 二次插值、额外 tick 音效、1300ms 上移浮字与附加震屏。
- [x] 卡组/战斗预览使用 ResizeCardImgWin；宽屏图像不再横向拉伸。
  SourceWindow 复用 ResizeWin 语义，修正连接箭头、卡组管理、管理询问、
  是/否与投降窗口。横幅字号按 yScale 缩放，补齐开局/换回合文案。
- 验证：14 页 636 项、鼠标交互 29 项、真实 Wails 12 项通过；
  TypeScript、双前端构建、Go test/vet/build 通过。
  完整对照及前后证据见 [实机对比报告](cpp-ui-comparison-2026-10-15.md)。

## DrawSpec 帧绘制与录像窗口（2026-10-15）

- [x] 按 `drawing.cpp:851-937` 直接绘制召唤梯形、特殊召唤缩放、发动遮罩、
  无效化收缩与回合数字；单次播放 30 帧，使用原版 mask / negated / number
  贴图。删除水平 rotateY、装饰边框、阴影与额外的确认卡片浮层。
- [x] `ResizeCardMid` 按窗口中心定位、以较小缩放轴保持卡图比例；
  数字按 `value-1` 取 5 列精灵图，连锁无效按对应连锁序号取卡图。
- [x] 猜拳按 `showcarddif` 的逐帧整数累加公式运动，持续 60 帧；
  正确映射后攻方与交换视角，89×128 图像尺寸不随窗口拉伸。
- [x] 宣言、掷币、掷骰保留原版括号格式，ACMessage 持续 40 帧并支持换行；
  新消息不会被前一条的定时器提前关闭。新局与终局取消旧特效及异步图片结果。
- [x] 录像保存窗口按 `ResizeWin(510,200,820,320)` 缩放中心，保持 310×120；
  对齐标题「是否保存录像？」、标签「录像文件：」、输入框和两枚按钮。
- 验证：14 页浏览器冒烟 **624 项通过**（spec 34、widgets 66）；
  鼠标交互 E2E **25 项通过**；TypeScript、生产/冒烟构建、Go build/test/vet 通过。
  原生 Wails 的「青眼一击」实战确认对方 LP=0，胜利横幅约 2 秒后才出现保存窗口；
  1264×681 窗口下实测录像窗仍为 310×120，未捕获到 JavaScript 运行错误。
- 证据：`tmp_dbg/cpp-spec-native-win.png`、`tmp_dbg/cpp-spec-native-final.png`、
  `tmp_dbg/ui-smoke-results.json`、`tmp_dbg/ui-parity-results.json`。
  这些是源码公式、渲染像素和当前客户端验证，尚未运行 C++ 可执行程序进行整屏截图差分。

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

## UI 全量对照修复批（2026-11，依据 tmp_dbg/ui-parity-audit.md 实机核验 + gframe 逐行对照）

审计（2026-10-13）按优先顺序列出的 5 个操作问题与两组视觉/状态差异，
本轮全部对照原版源码修复；收尾阶段删除了手牌 3D 化后的旧手背行死代码。

- [x] **双击误删两张卡**：恢复 deck_con.cpp:1120 原版鼠标语义——左键拖动、
  右键删除（pop_main/extra/side）、双击开 BigCardPanel（ShowBigCard），
  单击不再删除；新增 `components/BigCardPanel.tsx`。
- [x] **card-inspector 透明外壳挡「退出编辑」**：302×640 外壳
  `pointer-events:auto` 覆盖退出按钮（原版两区域 x 不重叠，
  game.cpp:335/:949）→ 外壳收缩/穿透，冒烟 `exit-click-not-obstructed` 用
  elementFromPoint 断言。
- [x] **拖拽插入位置被父网格冒泡覆盖**：卡片 drop 阻止向网格冒泡 +
  同区移动索引修正（deck_con.cpp:1089 hovered_seq 语义），
  `drag-inserts-at-target` 断言。
- [x] **投降一次确认两次调用 + 立即退出画面**：RightControls 加
  `surrenderSent` 守卫只发一次 CTOS_SURRENDER、留在对局等服务端
  （event_handler.cpp:151），`surrender-sent-once/waits-for-server` 断言。
- [x] **墓地禁查 F1 绕过**：`hotkeys.ts` 与 `CardListOverlay` 补
  cantCheckGrave 守卫（event_handler.cpp:1787），三个入口同语义，
  `grave-hotkey-respects-lock`/`all-grave-entrypoints-respect-lock` 断言。
- [x] **卡组编辑器对齐 deck_con.cpp**：搜索排序下拉 = 原版
  deck_sort_lv/atk/def/name（`domain/deck_sort.ts` 逐行移植
  data_manager.cpp 比较器）；搜索结果全量返回（card_db.go `AllResults`
  去 LIMIT，原版扫全库后排序）；中键复制按容量回退；搜索结果可拖入
  指定区/拖出删除；打乱只洗主卡组（deck_con.cpp:187）；载入卡组时下拉
  选中态关联 loadedDeck；禁限/发行范围徽标改用原版贴图图集
  （lim.png/ot.png，`CardAvailabilityBadge` = DrawThumb 的 tLim/tOT 行
  选取，drawing.cpp:1176-1198）。
- [x] **战斗画面对齐 drawing.cpp/client_field.cpp/game.cpp**：
  - 手牌改 3D 直排（GetCardLocation LOCATION_HAND →
    `duel/original_geometry.ts handPose`：同旋转、≤6 张固定间距、更多
    压缩、悬停抬升），`field3d.setHands` 双座渲染（零码 = 卡背）；
    HandDock 退化为无障碍/键盘入口，手牌动作按**手牌序号**而非卡号
    匹配（`duplicate-hand-card-keeps-sequence`）。
  - 阶段条改原版 wPhase 形态：动态当前阶段钮（`domain/original_phases.ts`
    PHASE_ABBR，ＤＰ/ＳＰ/Ｍ１/ＢＰ/Ｍ２/ＥＰ）+ BP/M2、EP 按可行动
    状态出现（game.cpp:343-353）；横幅寿命 40 帧（WaitFrameSignal）。
  - 相机改原版固定不对称视锥（game.cpp:994-998
    BuildProjectionMatrix → `projectionMatrix.makePerspective(-1.8,0.9,
    0.84,-0.84,2,200)`，位置 (0.4,15.6,16) 即原版 (4.2,8,7.8) 的世界
    座标换算）。
  - 旧规则场地：duelRule<4 时换 field2/field-transparent2 贴图与
    旧区域坐标（original_geometry 的 modern 分支，materials.cpp），
    `setDuelRule` 随 MSG_START 切换；灯光回归原版全亮环境光
    （game.cpp:999，去方向光/接触阴影外的彩色光）。
- [x] **Go/桥接补口**：`App.Minimise`（gframe Esc 最小化）、
  `ListDeckCategories`（空分类也列出，对齐 RefreshCategory）、
  `GetFieldImage`（ImageManager::GetTextureField：expansions/pics/field
  → pics/field，缺失不回退卡图/网络）、`SearchCards.AllResults`。
- [x] **工具与验证基建**：`tools/ui_parity_e2e.cjs`（真 Wails/WebView2
  CDP 驱动真实卡库 + 真实引擎，21 项检查全过）；`tools/ui_smoke_suite.cjs`
  把 14 个冒烟页套件化（本轮补 settings→__smokeResult、
  netplay_lobby→__netplaySmoke 的键映射与就绪判定）。
- [x] **收尾清理（本轮）**：手牌 3D 化后删除旧的远侧手背行死代码——
  field3d 的 `opponentHandMeshes`/`setFarHandCount`/`layoutFarHandRow`
  空桩与 duel_manager 的 `refreshFarHandRow`（收编进 `syncHands`）；
  theater_smoke 的 `opp-hand-backs` 断言改查 `handMeshes`、
  single_smoke 的 reload_field 桩改捕获 `setHands` 补零码数组。
- 验证：`go build/vet/test ./...` 全绿；前端 `tsc --noEmit` + 双 vite
  构建全绿；ui_smoke_suite 14 页 552 项检查全绿（deck 32、lobby 49、
  menu 7、netplay_lobby 11、practice 27、prompts 133、replay 12、
  settings 33、single 19、spec 23、stage 71、theater 47、ui_parity 34、
  widgets 54）。
- 未做（审计列为视觉设计差异，非行为缺陷）：Irrlicht 控件皮肤与
  CSS/Mantine 的皮肤差异保持现状；相机投影尺寸逐像素一致未担保。

## 悬停关系图标 SetShowMark（2026-11，对照 drawing.cpp:446-457 / client_field.cpp SetShowMark / client_card.cpp 关系表）

- [x] **装备/对象/连锁对象图标替换自加的关系连线**：原版没有事件驱动的
  关系连线——`is_showequip/is_showtarget/is_showchaintarget` 是
  **悬停驱动**的（client_field.cpp:2444-2462 `SetShowMark`：悬停某卡时，
  它的装备/对象/连锁对象**关联卡**上画 `equip.png`/`target.png`/
  `chaintarget.png` 平铺图标，materials.cpp:19 vSymbol 0.7×0.7），同卡
  else-if 优先级 equip > target > chaintarget > negated。
  - field3d：删除 `relationLines`/`addRelationLine`/`removeRelationLine`
    连线机制，新增 `setRelationMarks/clearRelationMarks`（0.7×0.7 平铺
    图标、逐帧跟随卡 mesh、显示期间压住该卡 negated 徽章）与
    `onHoverChange` 回调（handleHover 触发，SetShowMark 的触发点）。
  - duel_manager：新增关系数据表——`equipPairs`（MSG_EQUIP 先解旧绑，
    duelclient.cpp:3249-3254）、`targetPairs`、`chainTargets`
    （MSG_CHAINING 登记链卡、MSG_BECOME_TARGET 追加到最后一条链、
    chain_end 清除）；`refreshRelationMarks` 按悬停卡双向查表布图标
    （Map 保序实现同卡优先级）；离场卡按 MSG_MOVE 清理关系
    （duelclient.cpp:2645-2653 ClientCard 析构同款），duel:swap/duel:start/
    整板重建（resync drift）全量清除。
  - chaintarget 此前无任何可视化（become_target 只落日志），本次补齐。
- 验证：prompts_smoke 原 equip/target 连线 4 断言改写为悬停图标 9 断言
  （无悬停无图标、双向悬停互换、unhover 清除、unequip/cancel_target/
  chain_end 解除、连锁对象反向悬停），139/139 全绿；ui_smoke_suite
  14 页 558 项检查全绿；`tsc --noEmit` + 双 vite 构建 + `go build/vet` 全绿。

## 对局交互优化批（参考 DarkNeos/neos-ts 2D 前端的交互盘点，落到 three.js 3D 场）

调研 neos-ts（React+antd 的 YGOPro Web 复刻）的可借鉴交互并对照 gframe
原版后实施；下列各项均以原版 event_handler.cpp 语义为准，neos-ts 作为
Web 化形态参考。

- [x] **场上卡「发动」入口**（此前只有攻击/表示形式变更，act.png 角标照画
  但点击无菜单）：`duel_manager.onFieldCardClick` 按格匹配
  idleCmd/battleCmd 的 activate 条目（c/l/s）加入 ActionPopup；同一卡
  多个可发动效果按 desc 解析效果文本各列一项（原版 ShowSelectOption
  的平铺，event_handler.cpp:375-405），应答走 idx 精确路径
  （`respondIdleCmd(idx,5)` / `respondBattleCmd(idx,0)`）。战阶询问中
  手牌二速效果（速攻/手坑）同步补入 getHandOptions。
- [x] **堆区可发动卡流程**（原版点墓地/除外/额外弹「发动/特殊召唤/查看」
  菜单，event_handler.cpp:1269-1302 的 list_command 形态）：reducer 新增
  `cmdActions`（select_idlecmd/select_battlecmd 发布、new_phase/new_turn/
  waiting 清除——原版 ClearCommandFlag 时机）；CardListOverlay 据此把
  墓地/除外/额外列表里的可操作卡高亮（`.card-display-actionable` 金框），
  点击弹 ActionPopup（发动含多效果 desc 文本、特殊召唤），应答带精确
  idx（`handlePlayerAction` 的 activate/spsummon 新增 idx 直答路径）。
  左键点堆区（含空堆与堆顶卡）= 打开列表查看（原版 MSG_WAITING 点堆区
  的 COMMAND_LIST），field3d 新增 `onPileLeftClick`。
- [x] **连锁询问期右键 = 取消**（原版 CancelOrFinish）：PromptHost 在
  非强制连锁弹窗期间挂 document contextmenu 监听，右键应答 -1；
  顺带修复多候选连锁弹窗「取消」按钮此前不发包导致对局干等的缺陷
  （现在取消同样应答 -1）。
- [x] **悬停 tooltip 多行信息**（原版 stTip，event_handler.cpp:1592-1631）：
  卡名 + 攻守（或 LINK-n）+ ★等级/☆阶级 + 种族/属性 + 灵摆刻度 + 魔陷
  种类行；攻守/星阶优先用场上实时值（field3d 悬停事件新增 live 载荷，
  syncLinkData 写入的 DrawStatus 同源数据），手牌/堆区回退静态卡数据。
- [x] **破坏音效接线**：按 MSG_MOVE 的 `reason & REASON_DESTROY(0x1)`
  播放破坏音效。2026-10-14 复查 `duelclient.cpp:2590` 后修正为原版
  SOUND_DESTROYED 路径，去掉额外冲击波、火花和弹跳，保留 moveCard
  的正常移动；除外音效优先于破坏音效。
- 验证：prompts_smoke 新增 9 断言（连锁右键单/多候选、多候选取消钮
  应答 -1、场上发动单/多效果、战阶手牌二速、堆区特召 idx）；widgets_smoke
  新增 3 断言（堆区高亮/点击弹动作/新阶段清除）；spec_smoke tooltip
  断言改写为多行内容 + live 覆盖。ui_smoke_suite 14 页 607 项检查全绿；
  `tsc --noEmit` + 双 vite 构建 + `go build/vet/test ./...` 全绿。
- 仍未做（与 neos-ts 差距的刻意保留项）：弹窗最小化、操作历史时间线
  （本项目日志在左侧预览面板 Log 页签）、回放按消息类型掩码步进、
  这些属于其他前端的扩展功能，不能作为 gframe 一致性的依据。

## 源码行为补齐（2026-10-14）

- [x] **阶段码**：按 `ocgcore/common.h:405-407` 修正 BATTLE=0x80、
  MAIN2=0x100、END=0x200；统一阶段标签和模拟器，避免模拟数据掩盖
  真实引擎中的主要阶段 2 / 结束阶段误识别。
- [x] **战斗数值**：MSG_BATTLE 实时修改攻击方与目标的攻守，立即刷新
  DrawStatus / 悬停提示；移动宿主保留其数值，状态快照保持不可变。
- [x] **超量素材**：QUERY_OVERLAY_CARD / MSG_MOVE 驱动 3D 叠放，位置
  来自 `client_field.cpp:1053`，最多绘制 6 层；支持真实引擎 0x84
  位置编码、按 pp 下标取除同名素材和随宿主移动。
- [x] **高级场上选择**：纯手牌/场上来源的 select_tribute、select_sum、
  select_counter 走 3D 黄色选框与直接点选；混合/堆区来源保留列表。
  祭品支持 1/双重贡献；合计支持双备选数值、完整 31 位参数及
  CheckSelectSum 的非冗余选择判定；指示物按精确数量自动应答。
- [x] **原版提示**：指示物名称读 strings.conf 的 !counter；卡片详情
  按原版类型位顺序、原始卡号/名称、星/阶/LINK 与灵摆刻度格式显示。
  stTip 在鼠标左上角显示 0xc0 白底，选择提示留在 stHintMsg；
  非手牌悬停不抬升，手牌洗切、移动使用独立的原版时序。
- [x] **查询与生命周期**：异步重建场地期间保留后续权威查询；新一局
  清空旧场面、素材与动作菜单，避免残留上一局的数据。

验证入口：`tools/ui_parity_e2e.cjs`（真实鼠标）、
`frontend/ui_parity_smoke.html`（C++ 常量/几何/应答规则）、
`tools/ui_smoke_suite.cjs`（全部冒烟页）。这些验证源码行为与当前客户端，
不代表已经运行原版 C++ 可执行程序进行逐像素截图差分。

## 选卡弹窗交互细节批（2026-11，neos-ts NeosModal/双击快选/悬停预览的 3D 化）

- [x] **选卡弹窗最小化**（neos-ts NeosModal 的等价物）：CardSelectModal
  标题栏新增「—/□」按钮，收起卡行后只剩标题条，方便参照场面做选择；
  选择态（本地 useCardSelection 或 store.cardSelect 外置集）在收起/展开
  间完整保留。覆盖 select_card/select_option/连锁多候选/StoreCardSelectModal
  全部走该组件的弹窗。
- [x] **双击快选**：单选询问（min=max=1，select_option/连锁单选等）双击
  卡图直接应答，省去「点选 + 确定」两步（neos-ts 同款）。
- [x] **悬停卡图联动左侧详情**（neos-ts 悬停预览语义）：选卡弹窗卡图
  hover 即 `duelStore.inspect`；select_option 的占位 code（选项序号而非
  卡密码）经 `hoverInspect={false}` 排除，避免详情面板加载假卡。
- [x] **并行批次汇合修复**：PromptHost 传入的 `tribute` 标记补进
  CardSelectModal 签名（tsc 兼容）；widgets_smoke 误用 `duel:swap {}`
  （MSG_SWAP 控制权交换）驱动视角交换的断言改走公开 UI 动作
  `duelStore.toggleViewSwap()`。
- 验证：prompts_smoke 新增 6 断言（悬停详情/最小化/还原保选择态/流程
  收场应答/option 假 code 不联动/双击快选）。ui_smoke_suite 14 页全绿
  （prompts 155、widgets 61、spec 26、ui_parity 53 等）；`tsc --noEmit`
  + 双 vite 构建 + `go build/vet/test ./...` 全绿。
- 说明：超量素材 3D 叠放（field3d syncOverlays/layoutOverlays）与
  select_tribute/select_sum/select_counter 场上点选（field_selection.ts）
  由同工作区并行批次完成，本批验证其 smoke 覆盖（ui_parity 的
  overlay-six-visible-layers 等）随套件全绿。BattleOverlay（MSG_BATTLE
  现代化对撞浮层）维持不挂载——stage_smoke 的
  battle-does-not-add-modern-overlay 断言即项目方向：不加原版没有的 UI。

## 动画速度 / 回放快进 / 鼠标模式连锁批（2026-11，neos-ts 交互盘点的第二批落地）

- [x] **动画速度全局设置**（neos-ts settingStore.animation 的等价物）：新增
  配置键 `animation_speed`（百分比 10-200，默认 100=原速；Go AppConfig
  反射注册 + 0 回退默认 + 越界夹取），SettingsPanel 辅助页滑块，
  field3d.makeTween 统一按 100/speed 缩放 tween 时长（与原有
  quick_animation 200ms 封顶叠加——先缩放再封顶）。
- [x] **回放快进：跳到下一关键事件**（neos-ts ReplayAdvanceFlag 按消息
  类型掩码步进的等价物）：ReplayTheater 扩展面板新增「快进 ⏩」——
  关键类型（new_turn/draw/三种召唤/set/chaining/attack/damage/recover/
  pay_lpcost/win）之前的过场事件瞬时应用，关键事件正常播放动画；
  之后无关键事件时一路瞬时到终局。
- [x] **鼠标模式按住连锁**（原版 control_mode==1，event_handler.cpp:
  1709-1730 EMIE_LMOUSE/RMOUSE_PRESSED_DOWN）：场地内（原版
  X>300*xScale 的等价物=#duel-canvas-container 内）按住左键=全部连锁、
  按住右键=忽略连锁，松开即清；仅改本地 chain_prefs 不发包。
  hotkeys.installHotkeys 增加 mousedown/mouseup 监听。
- [x] **A/S 连锁键映射修正（parity bug）**：此前 A/S 与原版相反
  （event_handler.cpp:1740-1765：A=always_chain、S=ignore_chain、
  D=chain_when_avail），已翻转并同步 hotkeys 注释与 widgets_smoke 断言。
- 验证：settings_smoke 新增 2 断言（滑块存在/写回 150%）；theater_smoke
  新增 5 断言（连续快进停 draw→draw→new_turn、过场瞬时跳到终局、画面
  与逐步播放一致）；widgets_smoke 新增 4 断言（A 翻转、S=ignore、鼠标
  左/右按住连锁、模式 0 不响应鼠标）；config_test 新增
  animation_speed 解析/夹取用例。ui_smoke_suite 14 页全绿（settings 35、
  theater 53、widgets 65）；`tsc --noEmit` + 双 vite 构建 +
  `go build/vet/test` 全绿。
