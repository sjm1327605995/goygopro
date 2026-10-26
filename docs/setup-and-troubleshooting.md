# 资源准备与异机运行排查

[项目展示](../README.md) · [前后端架构](architecture.md) · [开发与验证](development.md)

同一个 Git 提交只保证代码相同，不能保证启动方式、工作目录、本机资源、原生库依赖、网络和用户配置相同。本页用于排查“办公室能运行，家里单机或卡组编辑为空”。尚未拿到家里机器的报告前，不能确定它缺的是哪一项。

## 本次反馈已确认的启动问题

家里的启动方式是 **`npm run dev` 后在浏览器打开**。这会进入 Mock Bridge，而不是启动 Go / Wails 后端：

- Mock 的 `StartSingle()` 只返回成功，不加载 Lua，也不产生真实引擎事件。因此点进单人对局后可能只有空场地。
- Mock 卡组是前端固定样例，卡片检索也只查询少量英文 Mock 数据，不读取磁盘上的完整 `cards.cdb`。中文卡名可能搜不到；可用卡号 `89631139` 对照检查。
- 即使本机 `script/`、`cards.cdb` 和卡图齐全，纯浏览器也不会自动调用这些 Go 文件接口。

本次在纯 Vite 页面复现：`isWails=false`，有 2 个样例残局、3 个 Mock 卡组和 12 条 Mock 卡片数据；“青眼白龙”搜索结果为 0，而 `89631139` 搜索结果为 1。调用单机开始虽然返回成功，但没有发出 `duel:start` 引擎事件。这与磁盘真实卡库的 14,981 张卡是不同的数据来源。

要运行真实单人和本地卡组功能，请从仓库根目录执行 `npm --prefix frontend run build`，再运行 `go run ./cmd/wails`，操作弹出的桌面窗口。需要热更新时使用 `wails3 dev` 的桌面窗口，而不是只开 Vite 网页。完整步骤见下文。

这能解释真实单人功能不工作的主要原因。如果连 Mock 自带的两个残局名称、示例卡组也完全不出现，仍应检查 JavaScript 错误、前端版本、缓存及筛选条件；不能把这种情况一并断言为 Lua 缺失。

## 先区分你看到的“空”

| 现象 | 优先检查 | 对应数据来源 |
| --- | --- | --- |
| 单人模式没有任何残局名称 | 是否启动桌面程序、实际资源根目录、`single/` 是否存在、文件是否直接位于该目录且扩展名为小写 `.lua` | `App.ListSingles()` 只扫描目录，不读取卡图，也不启动引擎 |
| 有残局名称，开始后没有真实对局或不能操作 | 是否误用纯浏览器 Mock、引擎是否初始化、共享 Lua 与残局脚本是否齐全且匹配 | `StartSingle()` → `InitServerData()` → 原生引擎加载脚本 |
| 卡组下拉列表为空，中央卡组区也为空 | `deck/` 是否包含 `.ydk` 文件，是否复制了自己的卡组 | 卡组来自 `deck/`，不是数据库自动生成的牌组 |
| 卡组编辑右侧显示“搜索结果：0” | 清空所有筛选，输入 `89631139` 后按 Enter 或点击搜索；默认未开启自动搜索 | `auto_search_limit=-1` 时输入或打开页面不会自动列出全部卡片 |
| 手动搜索仍无结果，名称、效果也取不到 | 资源根目录的 `cards.cdb` 是否完整，是否有 `datas/texts` 及必要字段 | 桌面启动时打开数据库，失败后查询可能以空结果表现 |
| 卡名和效果正常，但图片空白或显示占位卡背 | 本地卡图未带过去、CDN 被阻断、代理或图片缺失 | 卡图读取与数据库、Lua 是三条独立链路 |
| 正常对局，但没有场地魔法大图 | `pics/field/<卡号>.png` 或 `.jpg`、环境开关 | 场地图是独立资源，缺失时保留基础场地 |

**缺卡图不会导致单人残局列表为空；缺 Lua 也不会直接导致数据库卡牌搜索为空。** 不应把所有空界面都归因于同一种资源。

## 1. 确认启动的是哪一种程序

| 启动方式 | 能做什么 | 需要注意 |
| --- | --- | --- |
| `npm run dev` 后直接打开 Vite 地址 | 展示页面、Mock 卡组和示例对局数据、前端交互开发 | 没有真实 Wails 文件接口和原生单机引擎；Mock 的 `StartSingle` 不会运行 Lua |
| 构建前端后运行 `go run ./cmd/wails` 或桌面 exe | 本地文件、真实卡库、原生残局、真实联机 | 需要完整资源目录、WebView2 与可加载的匹配引擎 |
| `wails3 dev` 启动桌面窗口 | 桌面后端与 Vite 开发服务器协作 | 操作桌面窗口；单独打开开发服务器地址仍是浏览器 Mock |
| `arena_smoke.html`、`hands_smoke.html` 等页面 | 确定性数据和本地固定图片的测试场景 | 页面用于验证表现与交互，不代表当前机器拥有完整卡图或真实引擎环境 |

浏览器开发者工具中可查看：

```javascript
Boolean(window._wails)
```

这只是 Wails 容器的辅助线索，不能单独证明桥接成功。真正的判断点是 [`wails_bridge.ts`](../frontend/src/wails_bridge.ts) 是否成功加载 `/wails/runtime.js` 并获得 `Call.ByName`。在纯 Vite 页面中，该路径 404 并回退到 Mock 是预期行为；在桌面窗口中应检查同一加载失败和 `unknown bound method` 等调用错误。不要仅因为 UI 能打开就认为后端也已接通。

## 2. 当前仓库到底带了什么

2026-10-26 对当前提交的检查结果如下。它是核对基线，不是对家里机器的检测结果：

| 内容 | 当前状态 | 是否仅靠 Git 就有 |
| --- | --- | --- |
| `cards.cdb` | 8,050,688 字节；`datas`、`texts` 各 14,981 行，关联卡片 14,981 张 | 当前已跟踪；虽然 `.gitignore` 写有该文件，但已跟踪文件仍会克隆 |
| `script/` | 13,540 个已跟踪文件，含共享脚本和卡片规则 | 是；不可只复制几个 `c<卡号>.lua` |
| `single/` | `青眼一击.lua`、`魔导师的初阵.lua` | 是；完整工作树、正确路径下应能列出这两项 |
| `deck/` | `e2e_test.ydk` 示例 | 是；自己的其他卡组不会自动出现 |
| `ocgcore.dll`、`libocgcore.so` | 已跟踪的原生引擎文件 | 当前提交包含；与主机架构、系统依赖及 Lua API 是否兼容仍需验证 |
| `strings.conf` | 已跟踪的系统字符串资源 | 是 |
| `pics/` | 被忽略，0 个已跟踪文件 | **否**；这里可能只有办公室机器的 CDN 缓存 |
| `frontend/dist/`、`node_modules/` | 构建产物和依赖目录，已忽略 | 否；新机器需要安装依赖并构建，或随分发包一起提供 `dist` |
| `system.conf`、运行时录像、自定义外观 | 本机运行数据 | 否；外观存在该浏览器 / WebView 的本地存储中 |

在两台机器上分别运行并比较提交：

```powershell
git rev-parse HEAD
git status --short
git -c core.quotepath=false ls-files -- single deck
```

若使用的是 ZIP 或发布包，没有 `.git` 属于正常情况，应改为核对打包目录与资源报告。使用稀疏检出、只复制 exe、漏拷目录、混用旧版前端产物，均不能视为“相同运行环境”。

## 3. 源码方式启动

Windows 桌面建议准备 Go 1.25+、Node.js 20+ 和 WebView2。先进入包含 `cards.cdb` 与 `script/` 的仓库根目录：

```powershell
Set-Location E:\workspace\goygopro
npm --prefix frontend ci
npm --prefix frontend run build
go run ./cmd/wails
```

上面的路径应替换为家里的实际路径。启动后按顺序验证：

1. 打开单人模式，确认至少能看到仓库自带的两个残局。
2. 打开卡组编辑，确认能看到 `e2e_test`，或自行创建、保存新的卡组。
3. 重置筛选，以卡号 `89631139` 搜索，确认名称和效果可读；再检查图片。
4. 启动一个残局，确认不是只有静止棋盘，而是能收到阶段和操作询问。

前 1、2、3 项可分别验证目录、卡组、卡库；第 4 项才涉及完整引擎与 Lua 运行。浏览器 Mock 不能替代这个验收过程。

## 4. 只复制程序时，必须带上哪些文件

当前入口用 `os.DirFS` 读取前端产物，其他资源也从磁盘读取，并没有把整套运行数据嵌进 exe。建议以完整目录分发：

```text
goygopro/
├── goygopro.exe
├── ocgcore.dll                # Windows 示例，需匹配架构及其依赖
├── cards.cdb
├── strings.conf
├── script/                   # 全套共享及卡片 Lua，和引擎/卡库配套
│   ├── constant.lua
│   ├── utility.lua
│   ├── procedure.lua
│   └── c<卡号>.lua
├── single/                   # 至少包含要运行的残局 .lua
├── deck/                     # 自己需要的 .ydk，允许分类子目录
├── pics/                     # 希望离线显示的卡图
│   ├── 89631139.jpg
│   └── field/                # 可选的场地魔法图
└── frontend/
    └── dist/                 # npm run build 生成的完整目录
        ├── index.html
        ├── assets/
        └── textures/
```

可以在资源根目录启动 exe，或让 exe 放在资源根目录 / 其下的 `bin/` 中。不要把 `frontend/src/` 当成已经构建的 `dist/`；当前入口会回退查找源码 `index.html`，但静态文件服务不会代替 Vite 编译 TSX。

原生动态库不能跨操作系统混用，位数需与 Go 程序匹配。即使 DLL 文件存在，“找不到指定模块”也可能来自该库的系统依赖缺失。库能加载也不等于 Lua API 匹配。应复制同一套可用引擎、数据库与脚本，不应随意混合不同客户端的资源版本。

## 5. 资源根目录为什么会选错

[`findResourceDir()`](../cmd/wails/resource_paths.go) 的选择顺序是：当前工作目录，然后可执行文件所在目录及其最多三级父目录。候选只要求存在**非空 `cards.cdb`** 和 **`script/` 目录**，不检查数据库结构，也不要求 `single/` 和 `deck/` 齐全。

找到后会把工作目录切换到该位置。因此：

- 错误目录中的非空坏数据库加上一个空 `script/`，仍可能被选中。
- 正确数据放在别处、但不在上述候选路径内，不会被自动发现。
- 启动时只创建 `deck/` 和 `replay/` 空目录，不会自动下载卡库、补齐残局或生成卡组。
- `ListSingles()` 对缺失目录返回空列表；`ServiceStartup()` 对不存在的卡库不会自动修复，这些情况可能仅表现为界面为空。

源码依据：[`resource_paths.go`](../cmd/wails/resource_paths.go)、[`app.go`](../cmd/wails/app.go)、[`single_mode.go`](../cmd/wails/single_mode.go)、[`card_db.go`](../cmd/wails/card_db.go)。先核对实际被选中的资源目录，再补文件。

## 6. 卡图、场地图与 Lua 分开处理

### 普通卡图

桌面侧按资源根、exe 目录以及可选的 `YGOPRO_PICS_DIR` 搜索本地 `pics/`、`expansions/pics/`、`YGOPro/pics/`。支持 `.jpg` / `.png` / `.webp`，以及 `<卡号>-1`、`<卡号>-2` 等备用图名。找不到时，Go 才尝试从 `images.ygoprodeck.com` 获取并缓存到工作目录的 `pics/`。

若家里离线、被代理限制或图床不可达，不能依赖这条回退。可复制合法获得的所需卡图，保留原文件名。外置卡图配置示例：

```powershell
# 对应 D:\YGOResources\pics\89631139.jpg，变量指向包含 pics 的父目录。
$env:YGOPRO_PICS_DIR = 'D:\YGOResources'
go run ./cmd/wails
```

纯浏览器的图片回退由前端 fetch 发起，还受 CORS 限制；它与桌面 Go 代拉不是同一条网络路径。补齐图片或恢复网络后建议重新启动程序，清除当前前端会话里的缺图缓存。

### 场地魔法图

单独放在 `pics/field/<卡号>.png` 或 `.jpg`，也支持 `expansions/pics/field/`。当前桌面接口不会自动从普通卡片正面图生成场地大图。缺场地图只会退回基础场地，不应使残局列表或卡牌检索消失。

### Lua

`single/*.lua` 是“可选择的残局”；`script/constant.lua`、`utility.lua`、`procedure.lua` 和 `c<卡号>.lua` 是“运行残局和规则所需的依赖”。两者不可替代。`single/` 目前不递归扫描子目录，只有直接放在里面且扩展名为小写 `.lua` 的文件才进入列表。

`Error reading script file`、`attempt to call a nil value`、`initial_effect` 相关错误，应结合终端中请求的脚本路径与引擎/资源版本排查。不要只补一张图片，也不要把另一个客户端的整套 Lua 当成一定兼容的替代品。

## 7. 一条命令生成资源报告

工具只读检查运行资源，不启动原生引擎、不访问 CDN、不自动下载或修复文件。需要 Python 3.9+，仅使用标准库；没有 Python 也可以按前面的路径表手动排查。

```powershell
python tools/check_resources.py --root .
python tools/check_resources.py --root . --output home-resource-report.json
```

`--output` 只额外写出你指定的 UTF-8 JSON 报告。退出码：`0` 表示已检查的必需文件未发现错误，`1` 表示有必需项错误，`2` 表示参数、资源读取或报告保存失败。`WARN` 不会单独让命令失败，例如没有场地图是允许的。

检查内容包括数据库表结构、卡片关联数量、共享 Lua、卡片脚本、单机和卡组目录、构建后的前端、引擎文件头以及本地图片。报告同时给出关键文件 SHA-256，便于对比两台机器的文件内容；同样的文件名和大小不能证明版本一致。

限制：工具不验证 Wails 桥接、DLL 的完整导出 API、依赖库、Lua 实际运行和真实联网。数据库按静态文件只读检查，请先关闭数据库编辑器，不要以尚未合并 WAL 的编辑中快照作为结论。

两台机器均报告正常仍无法运行时，应对照收集以下信息，而不是继续猜测卡图问题：Git 提交、具体启动命令、资源根目录、该 JSON 报告、桌面程序启动及开始残局时的终端错误，以及卡组手动搜索的结果。不要用浏览器演示成功代替桌面验收。

## 8. 已知的历史录像限制

此前 `core/replay.TestReplayAllRealFiles` 对部分旧录像存在失同步失败，已记录在 [开发与验证](development.md)。历史录像与当前卡库、脚本和引擎的兼容性是单独问题，不能用于解释单人目录为空，也不应通过跳过断言掩盖问题。
