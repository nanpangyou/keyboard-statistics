# AGENTS.md

给 AI 编码代理（以及新加入的人）的项目说明书。**动手前先读完**；与本文冲突的临时口头要求以本文为准，
除非用户明确要求覆盖。

## 项目定位

`keyboard-statistics`：统计本机键盘使用情况的 Windows 桌面工具。按键频次、按键热力图、
常用快捷键、按天/周趋势、导出。**单文件 exe 分发，免安装，双击即用。**

这是本地工具，不是云服务。所有统计只留在本机。

## 红线（违反即回滚）

1. **不记录按键内容**：只统计键码、次数、时间，以及**同时按下**的修饰键组合（如 `Ctrl+C`）。
   禁止记录按键**顺序/序列**、输入文本、剪贴板、窗口标题。组合键只记「哪几个键同时按下 + 次数」，
   不含任何内容与先后信息；链式快捷键（`Ctrl+K Ctrl+C`）属于序列，不做。
2. **不联网上传**：默认零网络请求；确需联网必须显式开关且默认关闭。
3. **不隐藏采集**：采集范围写在设置里，用户可一键清空统计、关闭采集。
4. 不把密钥、token 写进仓库。本地配置走用户目录，不进 repo。
5. 不做静默开机自启。自启必须是用户主动勾选，且可在设置里取消。

## 技术栈

- 语言：Go 1.22+（`go.mod` 里的版本就是最低支持版本，不要随意抬）
- 界面：Fyne v2（`fyne.io/fyne/v2`），单窗口 + 设置页
- 键盘采集：`golang.org/x/sys/windows` 直接调 `SetWindowsHookExW`（WH_KEYBOARD_LL），
  **不用 cgo 的第三方钩子库**，也不引 gohook/mhook
- 存储：`modernc.org/sqlite`（纯 Go，无 cgo）
- 日志：`log/slog` 写文件，按大小轮转；不引 zap/logrus
- 打包：`go build` 出单 exe；版本信息与图标用 `go-winres`

> 界面与存储已定为 **Fyne + SQLite**。要换（例如换成 `lxn/walk` 以摆脱 cgo，或换成 bbolt/JSON
> 以降低依赖）**必须先问**，不要自己改。

## 开发方式：TDD 驱动

**本项目一律先写测试、再写实现。没有失败的测试，就不写生产代码。**

循环（红 → 绿 → 重构）：

1. **红**：先写一个描述期望行为的测试，跑一遍，确认它**确实失败**（编译不过也算红）。
2. **绿**：写刚好让它通过的最小实现，不做测试没要求的设计。
3. **重构**：在全绿前提下清理重复、改命名、调结构；重构期间测试断言不许放宽。

硬性规则：

- 修 bug 一律**先写复现测试**，看到它红，再改实现；不允许"先改代码再补测试"。
- 写不出测试的新函数/新接口，说明职责或边界没想清楚——先改设计，别先写代码。
- 每完成一个小步就提交一次（红绿一个循环一次提交），提交信息写清这一步做了什么。
- 提交前必须全绿：`go test ./...`。提交前不得留下被 `t.Skip` 掉的失败测试。
- 禁止为了让测试通过而放宽断言、删测试、或把实现逻辑搬进测试。

## 常用命令

```bash
go run ./cmd/app                 # 本地运行
go build -o bin/app.exe ./cmd/app
go test ./...                    # 单元测试（提交前必须全绿）
go test ./internal/stats -v      # 单包调试
go test ./internal/stats -run TestAggregate -v   # 单个用例
go test ./... -cover             # 看覆盖率
go test -race ./...              # 涉及并发（采集缓存 flush）必跑
go vet ./...
gofmt -l .                       # 必须无输出
golangci-lint run                # 提交前
```

发布构建（Windows 本机）：

```powershell
.\scripts\build.ps1 -Version 1.2.0
# 等价于：
go build -trimpath -ldflags "-s -w -H windowsgui -X main.version=1.2.0" `
  -o dist/keyboard-statistics.exe ./cmd/app
```

`-H windowsgui` 必须带，否则双击会弹黑框控制台。

## 目录结构

```
cmd/app/main.go          # 唯一入口：解析参数、初始化、启动 UI
internal/hook/           # Windows 键盘钩子，带 //go:build windows
  hook_windows.go
  hook_stub.go           # 非 Windows 平台的空实现，保证 go test 能跑
internal/store/          # sqlite 打开、建表、批量 upsert
internal/stats/          # 纯函数：聚合、时间分桶、导出
internal/ui/             # Fyne 页面与组件
internal/config/         # 配置读写、路径解析
assets/                  # 图标、manifest、winres 配置
scripts/                 # build.ps1 等
docs/metrics.md          # 统计口径说明
docs/manual-test.md      # 钩子等无法自动化的手动验证步骤
```

- 依赖方向单向：`cmd → ui → stats/store → hook`。`stats` 只做纯计算，不 import UI 和 sqlite。
- `internal/` 下不放可对外复用的东西；真要复用再开 `pkg/`。
- 所有平台相关代码必须有 build tag 和对应的 stub，保证 `go test ./...` 在无 GUI 环境也能过。

## 测试要求

- `internal/stats` 的聚合、分桶、导出：表驱动测试，边界至少覆盖空数据、单键、跨天、跨周、
  时间边界（午夜/周一零点）、超大计数。
- `internal/store`：用 `t.TempDir()` 建真实 sqlite 临时库，测建表、批量 upsert 累加、按天查询；
  **不要 mock 数据库**。
- `internal/hook` 与 Fyne UI 不做自动化测试，用接口隔离后测上层逻辑；钩子手动验证，
  验证步骤写进 `docs/manual-test.md`。
- 测试不读真实数据库、不碰真实采集数据，用固定 fixture。
- 测试不依赖执行顺序、不用 `time.Sleep` 等时序、不访问网络；时间相关逻辑注入可控的 clock。

## 编码规范

- 错误必须 wrap：`fmt.Errorf("flush stats: %w", err)`，不要吞错、不要 `panic`。
- 导出的函数要有 godoc 首行注释；注释写"为什么"，不写"做了什么"。
- 并发用 `context.Context` 传取消；后台 goroutine 必须能被关闭，程序退出不残留。
- 接口在**使用方**定义，不要为了 mock 提前抽一堆 interface。
- 不用 `init()` 做有副作用的初始化。
- 不引依赖除非说明理由并等确认；标准库能做的用标准库。

## Windows 特有注意

- 键盘钩子必须跑在**锁定到 OS 线程**的 goroutine 里（`runtime.LockOSThread`），
  且该线程要有消息循环（`GetMessage`/`TranslateMessage`/`DispatchMessage`），否则钩子立刻失效。
- 钩子回调里**只做计数**：`vkCode` 直接丢进内存 map 累加，禁止在回调里做 IO、加锁重活、或碰 UI，
  否则系统输入延迟会肉眼可见。
- 统计落库走后台批处理：内存攒到 N 条或每 30s `flush` 一次，用 upsert 累加，不逐键写库。
- **UAC 提权窗口收不到按键**：本程序以普通权限运行，管理员权限窗口的输入统计不到。
  这是预期行为，必须在 README 和设置页说明，不要试图绕过。
- 全局键盘钩子会被部分杀软误报。不要混淆、不要加壳；发布时尽量代码签名，README 里如实说明行为。
- 加 `assets/app.manifest` 声明 DPI 感知，否则高分屏会糊。
- 配置文件/数据库放 `%LOCALAPPDATA%\keyboard-statistics\`，不写程序目录（Program Files 无写权限）。

## 打包与发布

- 产出只有一个 exe，用户双击即用，**不写注册表、不装服务**。
- 需要安装包时再另加 Inno Setup / NSIS 脚本，脚本放 `scripts/`，不改变主程序行为。
- 版本号注入 `main.version`，`--version` 能打印；发布产物命名 `keyboard-statistics-v1.2.0.exe`。
- 每次发布更新 `CHANGELOG.md`，写清统计口径有没有变。

## Git 约定

- 分支：`feat/xxx`、`fix/xxx`、`chore/xxx`。
- 提交信息：`feat: 热力图支持按周聚合`（Conventional Commits，描述用中文可以）。
- 一次提交一件事；TDD 的一个红绿循环可以单独提交；重构与功能分开提交；
  构建产物（`bin/`、`dist/`）不进仓库。

## 代理工作守则

1. **测试先行**：先写失败的测试，再写实现；跳过 TDD 必须在回复里说明理由并等你确认。
2. **先读后改**：改文件前先读；不确定结构就先列目录。
3. **最小改动**：不顺手重排、不重命名无关符号、不升级无关依赖、不跑 `go mod tidy` 顺手升版本。
4. **不擅自加依赖**：新增第三方包先说明理由并等确认。
5. **不擅自改系统行为**：开机自启、注册表、权限提升、签名、原生模块，一律先问。
6. **不碰用户数据**：不为测试去读真实统计库或本机采集结果。
7. **改完自检**：跑 `go vet`、`gofmt -l .`、`go test ./...`，回复里写明跑了什么、结果如何。
8. **不确定就停下问**：宁可少做，不要猜着改。

## 完成定义（DoD）

- [ ] 功能按描述可用，边界处理过（首次启动无数据、采集关闭、数据库损坏）
- [ ] 新增/修改的逻辑都有对应测试，且是**先写测试后写实现**
- [ ] `go vet ./... && go test ./...` 通过，`gofmt -l .` 无输出
- [ ] 涉及统计口径变化时更新 `docs/metrics.md` 与 `CHANGELOG.md`
- [ ] 新增/改动 Windows API 调用在 `docs/manual-test.md` 补了验证步骤
- [ ] 没有新增未说明的依赖、没有提交密钥或构建产物
