# 开发与验证

[项目展示](../README.md) · [资源准备与故障排查](setup-and-troubleshooting.md) · [前后端架构](architecture.md)

下面的命令均从仓库根目录执行。启动方式、分发目录和原生依赖见资源准备文档。浏览器演示、浏览器测试与真实桌面对局分别验证不同环节。

## 验证方式

### 类型检查与构建

```powershell
npm --prefix frontend run typecheck
npm --prefix frontend run build
npm --prefix frontend run build:smoke
go test ./...
go vet ./...
```

原生对局测试需要可加载的引擎与相应数据。`purego` 用于引擎绑定，桌面数据库使用纯 Go SQLite；独立服务器还导入 `go-sqlite3`，其运行与构建环境需相应检查，不能把整个项目理解为完全不涉及 CGO。

历史验证记录（2026-10-23）：`go test ./...` 中的 `core/replay.TestReplayAllRealFiles` 对部分历史 `.yrp` 文件报告重放失同步，包括 `008`、`013`、`052`、`065`、`075`、`084`、`089`、`093`、`094`。这些测试及其引擎、卡库和脚本未在当次界面改动中修改；该失败尚未修复，具体原因仍需核对录像与规则资源版本。不能把其他检查通过视为该项已通过，也不应仅跳过断言掩盖失同步。

### 浏览器冒烟测试

冒烟页使用确定性数据验证实际 React / Three.js 组件。新的竞技场、手牌页使用 `frontend/smoke/assets/` 中的固定图片，不依赖本机 `pics/` 缓存。浏览器测试用于验证界面和事件应答，不代替真实双客户端联机测试。

先构建并在一个终端启动固定端口：

```powershell
npm --prefix frontend run build:smoke
npm --prefix frontend run preview -- --outDir smoke-dist --host 127.0.0.1 --port 19500 --strictPort
```

在另一个终端，从仓库根目录运行。脚本默认使用 Windows 已安装的 Google Chrome：

```powershell
npm --prefix frontend install --no-save --package-lock=false playwright
$env:PLAYWRIGHT_MODULE_PATH = (Resolve-Path frontend/node_modules/playwright).Path
# Chrome 不在默认路径或使用其他平台时，另设 CHROME_PATH 为浏览器可执行文件。
node tools/ui_smoke_suite.cjs menu lobby netplay_lobby single deck settings widgets prompts arena hands appearance stage practice theater replay ui_parity
```

结果写入 `tmp_dbg/ui-smoke-results.json`。重点用例：`hands` 覆盖 1 / 5 / 12 张牌、逐张拾取、魔陷区避让、悬停稳定性、视角交换与重复卡牌精确应答；`appearance` 覆盖图片卡套、本机保存、环境叠加、缺图与异步竞态回退。
