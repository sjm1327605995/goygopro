# 前后端架构与实现思路

[项目展示](../README.md) · [资源准备与故障排查](setup-and-troubleshooting.md) · [开发与验证](development.md)

## 总体结构

```mermaid
flowchart TB
  subgraph Frontend["前端：React / TypeScript / Three.js"]
    UI["页面、HUD、PromptHost"]
    Store["duelStore + 纯 reducer"]
    Manager["DuelManager：指令与表现编排"]
    Scene["DuelField3D：卡牌、场地、拾取、动画"]
    Bridge["WailsBridge + eventBus + PresentationQueue"]
    UI -->|"用户操作"| Bridge
    Bridge -->|"有序事件"| Store
    Store -->|"订阅快照"| UI
    Store -->|"视图同步"| Manager
    Bridge -->|"引擎事件"| Manager
    Manager --> Scene
    Scene -->|"悬停与点选"| Manager
    Manager -->|"语义化应答"| Bridge
  end
  subgraph Desktop["Go 桌面适配：cmd/wails"]
    App["App 服务：配置、卡图、卡组、录像、残局"]
    Client["WailsDuelClient：TCP 与 STOC 解码"]
    Bindings["engine_bindings / event_dto / responses"]
    App --> Client
    Client --> Bindings
  end
  subgraph Server["Go 服务器与规则层"]
    Network["gnet + packet 路由与校验"]
    Rooms["RoomManager / 房间 / 决斗模式"]
    Engine["ocgcore：purego 动态库绑定"]
    Lua["原生规则引擎 + Lua 卡片脚本"]
    Network --> Rooms --> Engine --> Lua
  end
  Bridge <-->|"Wails 调用与事件"| App
  Bindings -->|"事件 DTO"| Bridge
  Client <-->|"CTOS / STOC"| Network
  App -->|"本地残局"| Engine
```

### Go 后端的职责边界

| 模块 | 职责 |
| --- | --- |
| `cmd/wails/` | 桌面服务、TCP 客户端、事件 DTO、前端语义应答、资源与文件操作；把引擎数据转换成前端可消费的事件 |
| `cmd/server/` | 独立服务器入口，解析端口、数据库和脚本目录等启动参数 |
| `core/packet/`、`core/codec/` | 网络分帧、路由、中间件与二进制编解码；`zz_codec_gen.go` 为生成文件 |
| `core/duel/` | 房间生命周期、玩家与观战者、对局模式、计时、广播、引擎消息分析 |
| `core/data/` | 卡片数据库、卡组数据与校验 |
| `core/enginemsg/`、`protocol/` | 引擎消息解析及网络协议结构、常量 |
| `core/replay/`、`ocgcore/` | 录像格式与执行、原生引擎句柄和回调绑定 |

规则判定、合法操作和隐藏信息由后端及引擎控制。前端只展示收到的状态、允许的指令和询问结果；卡套、背景、镜头及动画不会改变规则和网络包内容。

### React 与 Three.js 如何协作

- `App.tsx` 负责页面切换与全局设置；`DuelStage.tsx` 创建、连接并销毁 renderer 与 manager。
- `domain/reducer.ts` 从事件计算前端状态，`duel/store.ts` 接入事件总线，组件使用 `useSyncExternalStore` 订阅。
- `DuelManager` 负责引擎指令、选位、选卡、连锁偏好及 3D 状态同步，不把规则判定移到浏览器。
- `field3d.ts` 负责相机、纹理、拾取与卡片实例；`arena_architecture.ts` 生成场景，`arena_hand.ts` 计算手牌构图，`field3d_anim.ts` 驱动动画。
- `field3d_geometry.ts` 与 `original_geometry.ts` 维护区域坐标。场景装饰不改变引擎的座位、区域和序号映射。
- 传统 `classic` 布局保留用于原版兼容验证与回放场景；实时决斗使用 `arena` 布局，回放的 `compact` 模式缩减 HUD。

## 对局事件与应答流程

```mermaid
sequenceDiagram
  participant E as 原生规则引擎
  participant G as Go 服务器
  participant W as Wails 客户端
  participant Q as PresentationQueue
  participant S as duelStore / React
  participant M as DuelManager / Three.js
  E->>G: 可用指令或选择询问
  G->>W: STOC 游戏消息
  W->>Q: 解码后的 duel 事件与 DTO
  Q->>S: 按表现时序分发事件
  Q->>M: 同一事件驱动场景表现
  S->>S: 显示合法操作与询问界面
  S->>W: 用户选择：RespondIdleCmd 等语义调用
  W->>G: CTOS 应答
  G->>E: 校验并提交引擎响应
  E->>G: 后续状态变化
```

`PresentationQueue` 保留抽牌、阶段、LP 变化和胜负等事件的表现间隔，避免连续消息把动画与询问瞬间覆盖。录像 seek 使用 `__instant` 路径即时重建。相同卡号可能同时存在于手牌中，因此点选与应答必须同时保留手牌序号或引擎指令下标，不能只按卡号匹配。

## 场地、卡套与手牌的实现思路

### 石质竞技场

场地由程序生成的风化石砖纹理、带厚度的台座、金属镶边、独立八角区域、圆形牌堆槽和边缘装饰组成。重复静态几何通过实例化合并绘制，卡牌与动态效果继续独立更新。区域拾取沿用统一坐标，装饰不会加入规则区域。

相机按实际 canvas 尺寸自动取景，容器变化由 `ResizeObserver` 同步。切换视角时，相机、卡牌朝向和手牌显示使用同一个下侧座位映射。

### 场地背景与环境效果

```mermaid
flowchart BT
  Background["所选场地背景：基础盘面与外围"]
  Stone["石板、台座与区域底座"]
  Environment["场地魔法：图片 + 柔边遮罩 + 可调透明度"]
  Lines["独立区域轮廓与选位高亮"]
  Cards["场上卡牌、牌堆与状态标记"]
  Hands["面向相机的手牌扇形"]
  HUD["React HUD、提示与操作弹窗"]
  Background --> Stone --> Environment --> Lines --> Cards --> Hands --> HUD
```

图中表示显示层次；实际渲染同时使用高度、深度测试、透明材质与 `renderOrder`，不是给每层另建一套业务状态。

单一场地魔法以柔边图层融入盘面；双方存在不同环境时分别覆盖各自半场。区域线保持在环境之上，卡牌仍可辨认和点选。关闭环境、环境离场或找不到对应场地图时恢复所选基础场地；异步图片请求带版本检查，迟到结果不会覆盖当前环境。

### 图片卡套与本机外观

卡套只替换我方里侧材质和牌堆卡背，表侧卡图不变；归属按本地座位判断，交换视角不会把卡套归属交换。图片导入在本机解码、居中裁切并压缩：卡套为 `512×744`，背景为 `1600×900`。支持 PNG / JPG / WebP，提供文件类型、大小、分辨率和存储失败提示。

| 状态 | 保存位置 | 作用范围 |
| --- | --- | --- |
| 音量、辅助选项、大厅记忆、环境开关 | Go `system.conf`，经 `settingsStore` 与桥接读写 | 桌面应用配置；浏览器预览为 Mock |
| 卡套、背景、自定义图片、环境浓度 | 浏览器 / WebView 的 `localStorage`，键为 `goygopro.appearance.v1` | 当前设备与应用来源；不上传图片、不同步给对手、不写入规则协议 |
| LP、阶段、手牌、场上卡片、询问 | `duelStore` 的内存快照 | 当前对局，来自后端或测试事件 |

### 扇形手牌

`arena_hand.ts` 根据手牌数量和 canvas 尺寸计算屏幕位置、间距、扇形角度与悬停状态，再通过相机反投影转换回 3D 坐标。卡牌继续使用同一射线拾取与座位/序号数据。

我方手牌在底部较大展开，悬停时放大、抬起并摆正；对方在顶部紧凑显示卡背。多张手牌自动压缩间距，布局避开我方魔陷区中心。抽牌从牌堆飞入手牌位置，洗切后恢复扇形；零卡号始终显示卡背，视角交换不会产生未收到的卡片信息。

## 目录索引与开发约定

```text
cmd/wails/           Wails 桌面服务、网络客户端和本地资源适配
cmd/server/          独立对战服务器入口
core/               房间、对局、数据、协议路由、编解码与录像
ocgcore/            原生规则引擎动态库绑定
protocol/           网络协议常量与结构
frontend/src/       React 页面、领域状态、桥接、3D 场景与音频
frontend/css/       公共主题、传统布局和竞技场样式
frontend/smoke/     浏览器冒烟页的驱动与固定资源
frontend/textures/  内置卡背、背景、场地与状态标记
tools/              浏览器验证与调试脚本
resources/          外部规则脚本的来源与固定版本清单
docs/               重构记录、协议设计与原版对照说明
source/ygopro/      对照用的原版源码
```

修改协议结构时同步检查 Go 编解码、Wails DTO、转发事件名单和 reducer；生成文件应通过对应 `go:generate` 指令更新。修改场地与手牌时保留座位/序号映射，并验证视角交换、隐藏信息和实际点选。新增纹理与实例化几何需要覆盖场景重建及退出时的资源释放。

运行时 `script/` 通过外部 Git 获取并被本项目忽略，版本记录在 `resources/scripts.json`。程序仍从该目录加载规则；单机示例 `single/` 是本项目保留的入口脚本，不能用外部规则目录代替。

详细历史与差距记录见 [重构状态](refactor-status.md)、[协议路由设计](packet-router-design.md) 和 [原版对照 TODO](cpp-parity-todo.md)。
