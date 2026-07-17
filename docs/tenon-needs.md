# goygopro 对 tenon 的需求

记录 goygopro 在 tenon 上做 GUI 时撞到的库级问题：哪些已经解决、哪些卡着需要 tenon 改。
面向 tenon 的维护者，按「问题 → 证据 → 根因 → 方案」写，不是许愿单。

对应版本：tenon `44bb4fd`（gio 后端）。goygopro 侧 go.mod 仍 `replace` 到本地 tenon ——
`SrcImage`/`OnSubmit`/`PlaneImage` 三个提交尚未推到 origin，仓库也还没有 tag。

**状态**：1、2、3 已由 tenon 实现并接入。4 是新发现的库 bug（goygopro 侧已绕开，库里仍待修）。5 是文档问题，6 是备忘。

---

## 1. ✅ 已解决：Scene3D 里放不下一张「地板贴图」

tenon 实现了方案 A（`PlaneImage`，见 `pkg/ui/plane_image.go`）。goygopro 侧已接入
（`client/scenes/duel_field.go` 的 `fieldTexture`），原版场地图恢复使用，格线朝灭点收敛，
卡牌与格子严丝合缝（回归测试 `TestCardSitsOnItsZone`）。以下保留问题记录。

### 结论（当时）

决斗盘要铺原版那张整幅场地图（`textures/field2.png`，1024×640）。它是场上最大的元素，
被 Scene3D 的仿射近似画成**斜切的平行四边形**：格线不朝灭点收敛。而卡牌（72×80 的小元素）
的投影是准的，于是卡和格子明显对不上 —— 一眼可见的错。

当时只能把它关掉，用一堆卡片大小的小方块拼出分区示意。能玩，但丢了原版美术。

### 证据

当时用 `ui.Screenshot` 无头渲染决斗盘复现（`SHOT=1 SHOT_OUT=x.png go test ./client/scenes -run TestShot`）：
场地网格整体朝一侧斜切、远端不收窄，而卡牌各自落位正确，于是卡牌与格线互相错开。

场景参数：平面 799×499、`Perspective(1092)`、`RotateX(34)`。

### 根因（不是 bug，是已知近似的边界）

`pkg/ui/gio_3d.go` 文件头自己写清楚了：形状裁到四角真实投影出的四边形，**内容**则用三角仿射重放，
「偏差随元素尺寸与倾角增长（80×110 的卡 45° 下约 21px；150×180 在 60° 强透视下约 68px）」。

对卡牌，这个取舍完全正确 —— 21px 的内部斜切肉眼几乎看不出，因为卡面是张图、没有参照系。
但地板不同，它有两条性质让同样的偏差变得不可接受：

1. **它是参照系**。别的元素（卡）都要落在它的格子里。地板自己歪 5px 都看得出来，
   因为旁边就摆着按精确投影定位的卡。
2. **它大**。799×499 比卡大一个数量级，按文档给的趋势，偏差是几百像素级别，不是几十。

细分网格不是出路 —— `gio_3d.go` 里记着已经试过：「每格各用一个仿射，内容跨格对不上会撕裂，
且格内内容填不满自己的裁剪格、缺口露出底色 —— 密度越高越像铺了层铁丝网」。我信这条记录，
没有再试一遍。

根子在 gio 的 `f32.Affine2D` 只有 6 个自由度，表达不了投影变换（需要 8 个）。这是后端能力的边界，
在绘制阶段绕不过去。

### 方案

关键观察：**地板是静态的**。一局决斗里它的图和投影参数都不变。所以不必在每帧的绘制路径里
解决投影问题 —— 可以离线把这张图按精确投影**预先变形**一次，得到的结果就是一张普通的
2D 位图，正着贴上去即可，不再需要任何 3D 变换。CPU 上做一次逆映射采样，1024×640 是毫秒级，
之后每帧都是零成本。

难点只在于「预变形用的投影」必须和 Scene3D 给卡牌用的**完全一致**，否则卡还是对不上格子。
所以这件事必须由 tenon 提供，或至少由 tenon 导出同一份数学。

三个选项，推荐 A：

#### A（推荐）：tenon 内置平面贴图 `PlaneImage`

```go
// PlaneImage 把一张图当作 Scene3D 的地板铺满整个场景平面：tenon 用与子元素一致的精确投影
// 把它 CPU 预变形一次并按 (key + 场景参数) 缓存，之后每帧只是贴一张普通位图。
// 仅在 Scene3D 的直接子元素上有意义。
func PlaneImage(key string, img image.Image) *Node
```

用法：

```go
ui.Box([]ui.StyleOpt{ui.Scene3D, ui.Perspective(1092), ui.RotateX(34), /* ... */},
    ui.Img(ui.PlaneImage("field:1", fieldTexture)),   // 地板：精确投影
    card(...), card(...),                              // 卡牌：照旧
)
```

- 好处：投影一致性由 tenon 内部保证，用户不可能写错；「棋盘/桌面/地板」是卡牌与桌游类
  UI 的通用需求，不是 goygopro 独有。
- 代价：tenon 里多一段 CPU 变形 + 缓存（约百行）。缓存键要含场景参数（perspective/角度/尺寸），
  参数变了要重算；窗口缩放会触发重算，可考虑降采样或异步。
- 边界：只解决「铺满平面的静态图」。倾斜的视频、动图不适用 —— 那种情况本就该另说。

#### B（次选）：只导出投影数学，变形由 goygopro 自己做

```go
// Plane 描述一个 Scene3D 平面。
type Plane struct {
    W, H             float32 // 场景平面的未投影尺寸
    Perspective      float32
    RotateX, RotateY float32
}

// Project 把平面上的点映射到场景盒内的屏幕坐标；Unproject 是它的逆。
func (p Plane) Project(x, y float32) (sx, sy float32)
func (p Plane) Unproject(sx, sy float32) (x, y float32)
```

goygopro 拿 `Unproject` 做逆映射采样，自己产出预变形的位图，再用 `SrcImage` 贴。

- 好处：tenon 只多两个纯函数（内部 `project3D` 已经有了，导出即可），无缓存无策略。
- 代价：每个需要地板的应用都要自己写一遍变形循环；漏了 `uiScale`、场景原点这类细节就会
  悄悄错位。
- 备注：如果选 B，我可以在 goygopro 这边实现，只需要 tenon 导出这两个函数。

#### C：维持现状，不铺整幅贴图

分区继续用小方块拼。代价是丢掉原版美术，且「桌面」缺少实体感。
如果 A/B 都排不上期，这是可接受的临时状态。

### 验收（已通过）

- 场地网格朝灭点收敛（远端窄、近端宽），不是平行四边形 ✅
- 每张卡落在它对应的格子里 ✅（`TestCardSitsOnItsZone` 钉住 `GetCardLocation` 与
  `ZoneCenter` 一致；无头截图 `SHOT=1 go test ./client/scenes -run TestShot` 目视复核）
- 静态图只变形一次，每帧零成本 ✅（tenon 侧按 key + 场景参数缓存）

---

## 2. ✅ 已解决：`Input` 收不到回车（没有 `OnSubmit`）

tenon 加了 `OnSubmit`（`pkg/ui/node.go`），harness 也改走真实按键路径。goygopro 侧已接入：
决斗聊天（`duel_side.go` 的 `chatBar`）、联机界面的主机/端口（`lan_window.go`）回车即提交。
以下保留问题记录。

### 结论（当时）

聊天框、搜索框、IP/端口这类输入，回车提交是肌肉记忆。tenon 的 `Input` **完全无法响应回车**，
决斗聊天只能点「发送」按钮。

### 根因

回车在两条路上都被挡掉了，不是漏了某个分支，是确实没有这个概念：

- `pkg/ui/run.go:634`：`if rn.multiline && input.keyJustPressed(keyEnter)` —— 单行输入框不插换行（正确），
  但也没有别的动作；
- `pkg/ui/run.go:476` `activateFocused()`：`if rn != nil && rn.kind != rnInput && rn.onClick != nil` ——
  显式跳过输入框，所以给 Input 挂 `OnClick` 也不会被回车触发（挂了反而更糟：点击聚焦时就误触发）。

### 方案

加一个属性即可，语义对齐 HTML 表单的 submit：

```go
// OnSubmit 在单行 Input 上按下回车时触发。多行 Input（Multiline）不触发 —— 那里回车是换行。
func OnSubmit(fn func(value string)) *Node
```

实现落点就在 `run.go:634` 那个 `if rn.multiline` 的 else 分支：非多行且 `onSubmit != nil` 时
调用 `rn.onSubmit(rn.value)`。harness 侧建议同步加 `Query.Submit()`，否则这条路径没法无头测试。

顺带一提，`Harness.Enter()` 现在的语义是「激活聚焦的可点击元素」，对 Input 是空操作 ——
加了 OnSubmit 后它应当也能触发提交，否则测试里的回车和真实回车行为不一致。

---

## 3. ✅ 已解决：`Img` 只能从路径加载

`Src(path)` 走后台 goroutine 读盘 + 解码，但 goygopro 的卡图由 `client/image_manager.go`
统一管着（缓存、unknown 占位、将来要从压缩包里取），手上已经是 `image.Image` 了，没有路径可给。

tenon 加了：

```go
// SrcImage 用一张已在内存里的图片作为来源，跳过 Src 的读盘与解码。
func SrcImage(key string, img image.Image) *Node
```

- 同步安装位图（无 IO 无解码，不必等一次 Post），首帧不闪空白；
- 与 `Src` 共用同一份按字节预算的 LRU，同 key 的多个节点共用一张位图；
- 测试见 `pkg/ui/srcimage_test.go`。

goygopro 侧卡图、卡背、背景贴图全部走它（`duel_card.go` 的 `cardFace`、`duel_field.go` 的
`texture`）。这个改动在无头截图里价值也很明显：`Screenshot` 不跑 Post 队列，所以 `Src` 的图
在黄金测试里永远是空的，`SrcImage` 的图正常出现。

**仍待 tenon 侧**：把 `SrcImage`/`OnSubmit`/`PlaneImage` 推到 origin 并打 tag —— 目前它们只在
本地，origin/main 还停在 `6b543c9`，仓库一个 tag 都没有，所以 goygopro 只能 `replace` 指本地，
换台机器就编不过。

---

## 4. 🐞 Bug：Scene3D 里带 `Rotate` 的子元素会被甩离原位、并丢失透视

**goygopro 已绕开**（见文末「临时状态」），但库里这个 bug 仍在，任何在 Scene3D 里用
`Rotate` 的人都会撞上。

### 现象

决斗盘上，**对手的每一张卡都画到了我方场地上**（原版里对手的卡一律 180°），
表侧守备的怪兽（90°）同样飞走，而且它们不再被前缩，看上去像立起来浮在空中。
用户一眼就看出来了：「这个垂直的是什么浮空卡牌」。

### 证据（像素实测，非目视）

回归测试 `client/scenes/rotate_bug_test.go`（`go test ./client/scenes -run TestRotatedCardStaysInItsZone`）
在空场上摆一张卡、把卡图换成纯品红、全屏扫描它的包围盒：

| 卡 | 期望屏幕位置 | 实测 | 实测尺寸 |
|---|---|---|---|
| 表侧攻击（`Rotate(0)`） | (324, 369) | (323, 370) ✅ | 62×54（宽>高，已前缩）✅ |
| 表侧守备（`Rotate(-90)`） | (324, 369) | **(588, 482)** ❌ 偏 (265,113) | **55×62**（高>宽，未前缩）❌ |

期望位置由 goygopro 独立实现的同一套投影算出，且与不旋转那张的实测位置吻合到 1px，
所以基准可信。场景参数：平面 799×499、`Perspective(1092)`、`RotateX(34)`、场景盒在窗口
(112.5, 44)。

### 根因

`pkg/ui/project3d.go` 末尾：

```go
orgX, orgY := t.origin() // 无相机=元素中心，有相机=场景中心
return pt{
    orgX + float32(x2*cosZ-y2*sinZ) + t.tx,
    orgY + float32(x2*sinZ+y2*cosZ) + t.ty,
}
```

2D 旋转（`cosZ/sinZ`）作用在 `(x2,y2)` 上，而它是**相对投影原点的偏移**。没有相机时原点就是
元素自身中心，转的是自己，正确；但在 Scene3D 里原点是**场景中心**，于是元素被绕着场景中心
「公转」出去。

手算可复现实测值：卡中心相对场景中心偏移 (-179.7, +87.4)，投影后 (x2,y2)=(-188.2, 75.9)，
代入 `rotate=-90°` 得 (75.9, 188.2) → 屏幕 (587.9, 481.8)，与实测 (588, 482) 吻合。
180° 同理：对手怪兽区 0 号位本该在 (684, 206)，被镜像到 (348, 363) —— 正好压在我方怪兽区上。

第二个问题是**顺序**：`rotate` 在透视除法**之后**才施加，等于在屏幕空间转已经投影好的四边形，
所以前缩被一起转掉（62×54 转 90° 变成 54×62）。

### 期望语义

`Rotate` 应当和 `RotateX/RotateY` 一致，作用在元素自身的 3D 坐标系里 —— 即绕平面法线在**平面内**
旋转，然后再投影。这样：

- 绕元素自己的中心转，不会离开自己的格子；
- 转 90° 的卡是「平躺在桌上、横过来」，仍被前缩 —— 正是游戏王里守备表示的样子；
- 转 180° 的卡（对手视角）位置不变，只是图案倒过来。

实现上，子元素的自身变换（rotate/scale/translate）应以**元素中心**为支点：先在平面内对
「角点相对元素中心的偏移」施加 rotateZ，再把结果连同元素中心一起走相机投影。

### 验收

修好后，在 Scene3D 的子元素上加 `Rotate(90)`：元素应当仍以自己的中心为支点（不移位），
且仍被相机前缩（宽 > 高，而不是把投影结果整个转 90°）。

### goygopro 侧的临时状态：已绕开，不用 `Rotate`

卡的朝向只有四种（0/±90/180，见 `GetCardLocation`），于是改成**预转位图**：
`ImageMgr.Rotated(key, img, quarter)` 缓存转好的图，节点自身只在 90° 时对调宽高
（`duel_card.go` 的 `quarter`）。这天然就是「平面内旋转」，且比 `Rotate` 更好用 ——
徽标和边框保持正立（可读），命中区也自动正确。回归测试 `TestRotatedCardStaysInItsZone`
钉住这条不变量。

所以这个 bug **不再阻塞 goygopro**，但库里还是该修：`Rotate` 在 Scene3D 里的行为与
文档/直觉都不符，而且下一个用它的人不会知道要绕开。

---

## 5. 文档与实现对不上（大部分已修）

`44bb4fd` 已把后端名从 Ebiten 刷成 Gio，`pkg/ui/README.md` 的「Notes & limits」也跟着改了。
剩一条还没改；另外我原先列的「`Div` vs `Box`」是我自己看错了 —— 两个 API 都存在，
只是签名不同（`Div(args ...*Node)` 收属性与子节点，`Box(opts []StyleOpt, kids ...*Node)`
把样式提到第一个参数），README 没有错，这条撤回。

| README 说 | 实际 | 状态 |
|---|---|---|
| 渲染后端是 Ebiten（`vector` + `text/v2`） | 是 gio（`gioui.org v0.10.1`） | ✅ 已修 |
| 组件kit 表格里写 `Checkbox(checked bool, onChange func(bool))`，且称「all in package `ui`」 | `ui` 里没有 `Checkbox`（编译期就报 undefined）；它在 `pkg/shadcn`，签名是 props 结构体 `shadcn.Checkbox(CheckboxProps{...})` | ❌ 待修 |

另有一处不算文档错误、但值得在 harness 一节点一句的坑：`Query.ByKind` 的种类名是 `"image"`，
而构造函数叫 `Img` —— 按构造函数名去查会得到空 Query，且因为空 Query 的读方法是安全 no-op，
不会报错，只会让断言默默失败。

---

## 6. 备忘：用到但未成为问题的边界

不需要改，写下来免得下次重新踩：

- **Scene3D 只对直接子元素生效**。卡牌必须直接挂在场景节点下；中间套容器，那层会被当整体投影。
  goygopro 因此把双方所有区域的卡摊平成场景的直接子元素（`cardNodes`）。
- **Scene3D 自身的 `Clip` 在 3D 下不生效**（裁剪矩形未投影）。没用上。
- **跨 goroutine 必须 `ui.Post`**。决斗状态由网络线程改写，goygopro 用 `client.Store` +
  `scenes.UseStore` 桥接，setState 一律经 Post 回渲染线程。
- **偏心视锥做不到**。ygopro 原版相机是偏心的（`game.cpp`:
  `BuildProjectionMatrix(-0.90, 0.45, -0.42, 0.42, 1, 100)`），Scene3D 只能绕场景中心对称透视。
  goygopro 改用「桌面缩小 + 居中」取景（`tableScale`/`tableTop`）绕开，效果可接受，不算需求。
