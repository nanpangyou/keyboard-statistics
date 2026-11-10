# AGENTS.md

给 AI 编码代理（以及新加入的人）的项目说明书。**动手前先读完**；与本文冲突的临时口头要求以本文为准，
除非用户明确要求覆盖。

## 项目定位

`keyboard-statistics`：统计本机键盘使用情况的 Windows 桌面工具。按键频次、按键热力图、
常用快捷键、按天/周趋势、导出。**单文件 exe 分发，免安装，双击即用**；
另提供可选安装包（自选安装目录，可勾选便携模式把数据放进程序目录）。

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
- 界面：原生 Win32（GDI 自绘，纯 Go 封装如 `github.com/lxn/walk`），**不依赖 OpenGL**。
  云桌面 / 远程桌面没有可用 OpenGL，这条路已实测堵死（见 [docs/devlog.md](docs/devlog.md)，
  落地方案见 [docs/ui-plan.md](docs/ui-plan.md)）
- 键盘采集：`golang.org/x/sys/windows` 直接调 `SetWindowsHookExW`（WH_KEYBOARD_LL），
  **不用 cgo 的第三方钩子库**，也不引 gohook/mhook
- 存储：`modernc.org/sqlite`（纯 Go，无 cgo）
- 日志：`log/slog` 写文件，按大小轮转；不引 zap/logrus
- 打包：`go build` 出单 exe；版本信息与图标用 `go-winres`

> 界面已定为**原生 Win32（GDI）**，存储仍是 **SQLite**（`modernc.org/sqlite`）。
> 要换界面栈（别的 GUI 框架）或换存储（如 bbolt/JSON）**必须先问**，不要自己改。

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
internal/ui/             # Win32 页面与组件（GDI 自绘）
internal/config/         # 配置读写、路径解析
assets/                  # 图标、manifest、winres 配置
scripts/                 # build.ps1 等
README.md                # 使用手册（每个发布版本都必须重写）
CHANGELOG.md             # 版本变更（用户可见）
docs/requirements.md     # 需求说明
docs/metrics.md          # 统计口径（唯一标准）
docs/devlog.md           # 开发日志（面向后续 agent 的流水账）
docs/manual-test.md      # 钩子等无法自动化的手动验证步骤
docs/releases/v*.md      # 每个正式版本的发布说明（给用户看，可直接贴到发布页）
```

- 依赖方向单向：`cmd → ui → stats/store → hook`。`stats` 只做纯计算，不 import UI 和 sqlite。
- `internal/` 下不放可对外复用的东西；真要复用再开 `pkg/`。
- 所有平台相关代码必须有 build tag 和对应的 stub，保证 `go test ./...` 在无 GUI 环境也能过。

## 测试要求

- `internal/stats` 的聚合、分桶、导出：表驱动测试，边界至少覆盖空数据、单键、跨天、跨周、
  时间边界（午夜/周一零点）、超大计数。
- `internal/store`：用 `t.TempDir()` 建真实 sqlite 临时库，测建表、批量 upsert 累加、按天查询；
  **不要 mock 数据库**。
- `internal/hook` 与 UI 不做自动化测试，用接口隔离后测上层逻辑；但 GDI 自绘里的**纯函数**
  （布局计算、热力着色、命中测试等）**必须写表驱动测试**。钩子手动验证，
  验证步骤写进 `docs/manual-test.md`。
- 测试不读真实数据库、不碰真实采集数据，用固定 fixture。
- 测试不依赖执行顺序、不用 `time.Sleep` 等时序、不访问网络；时间相关逻辑注入可控的 clock。

## 编码规范

- **一律 UTF-8（无 BOM）**：源码、文档、配置全部 UTF-8，换行 LF（`*.ps1/*.bat/*.cmd` 用 CRLF），
  由 `.editorconfig` 约束。禁止 ANSI / GBK / 带 BOM 的文件，也禁止用 shell 重定向写源码
  （见工作守则第 9 条），否则中文注释会变成非法 UTF-8 把文件毁掉。
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
- 数据目录解析顺序：**用户指定（HKCU）> 便携模式（exe 旁有 `portable.txt` 且实测可写）>
  `%LOCALAPPDATA%\keyboard-statistics\`**。默认仍不写程序目录（Program Files 无写权限）；
  便携模式是用户在安装时显式勾选的例外，必须实测可写，写不进去就回退并说明。
- 数据目录不可写时必须**弹窗说明**（`ui.ShowFatal`），不能静默 `os.Exit(1)`——
  `-H windowsgui` 下没有控制台，用户只会看到「双击没反应」。
- 换数据目录时**搬文件只在启动早期做**（数据库尚未打开）：运行中 SQLite 持有文件句柄，
  Windows 不允许移动；先关库再搬会让界面停在「服务已关闭」的半死状态。
- **托盘常驻**：程序在通知区域有常驻图标（`internal/ui/tray_windows.go`），
  回调消息是 `msgTray`（`wmApp + 2`），菜单逻辑在 `internal/tray`（纯逻辑，有测试）。
  图标是 `assets/app.ico`（9 档尺寸，`scripts/make-icon.py` 从设计稿生成），
  经 `winres.json` 的 `RT_GROUP_ICON` 嵌进 exe；托盘用 `LoadImageW(module, 1, ...)` 取它，
  取不到才退回 `IDI_APPLICATION`——**别删 `winres.json` 里那段，删了托盘就变系统默认图标**。
- **小尺寸图标要现画，不能缩大图**：`make-icon.py` 里 16/20/24 走 `draw_small()`
  （浅灰面板 + 深蓝描边 + 绿色折线），32px 以上才用设计稿缩放。把设计稿直接缩到 16px
  会糊成一团灰点——那正是 v0.5.2 及以前托盘里的样子。改母题就改 `draw_small`，别去调缩放参数。
- **窗口图标必须显式 `WM_SETICON`**（`loadWindowIcon` + `setWindowIcon`）：
  只在 `registerClass` 里设是不够的（当时压根没设过），标题栏和 Alt+Tab 会一直是系统白板图标。
  ICON_SMALL 取 `px(16, scale)`、ICON_BIG 取 `px(32, scale)`，尺寸跟着 DPI 走。
  **验图标要先让 exe 带上资源**：`cmd/app/rsrc/*.syso` 是 `build.ps1` 生成、用完即删的，
  手搓 `go build` 出来的 exe 没有图标资源，`LoadImageW` 一律返回 0（踩过，白查一轮）。
- **消息循环必须锁 OS 线程**（`runtime.LockOSThread`，见 `ui.(*App).Run`）：
  窗口的消息队列属于**创建它的那个线程**，而 `GetMessage` 只从**调用它的那个线程**的队列取消息。
  Go 的调度器会把没锁定的协程挪到别的 OS 线程上；一旦在建窗口之后挪走，
  `GetMessage` 就等在了新线程的空队列上——窗口再也收不到任何队列消息（定时器不响、
  不再重绘、托盘右键无反应），发给窗口的 `SendMessage` 全部超时，
  从外面看就是「未响应 + CPU 0」。v0.5.1 及以前正是这么挂的（实测启动后几十秒必现）。
  钩子协程本来就锁着，**别只锁钩子不锁界面**。
- **判定「界面是否还活着」不能用 `PrintWindow` 截图**：它读的是 DWM 的重定向表面，
  界面线程死了照样能抓到一张好像没问题的图。可靠判据是「外部 `WM_NULL` 探针能过」
  +「底部状态栏的时刻像素在变」（`scripts/verify/soak_ui.py` 就是干这个的）。
- **界面刷新节奏是产品参数，不是实现细节**：`refreshInterval = 500ms`（v0.5.3 起，
  之前是 2 秒）。`internal/ui/refresh_cadence_test.go` 把「≤500ms」和
  「看门狗探针间隔 ≥1 秒且不短于刷新间隔」钉住了，想改先想想用户会不会觉得「统计有延迟」。
  刷得勤不等于堆积：`requestRefresh` 有 `fetching` 闸门，上一次取数没回来就跳过本次。
  想量实际延迟用 `scripts/verify/refresh_latency.py`（改 2 秒时代实测 ~1.1s，现在 ~0.6s）。
- **界面线程上不许做会阻塞的事**：刷新路径里每 2 秒读 3 次注册表这种「顺手」也要删掉
  （`applyPendingSnapshot` 里踩过）。已知仍在界面线程上的可疑调用：`svc.SetCollecting`
  的 meta 写库、`ClearData`、`ExportCSV`、托盘那几个 `Shell_NotifyIcon`（Explorer 忙时是
  同步跨进程调用）。要动它们时优先挪到协程，别让界面线程去等。
- **界面卡死要有现场**：`internal/ui/watchdog_windows.go` 每 2 秒用 `SendMessageTimeoutW`
  发一条 `WM_NULL` 当心跳，超时就先 flush 计数、再写全部 goroutine 栈到日志（走
  `Options.OnUIStall`）。节流逻辑在 `stall.go`（纯类型，有测试）。
  **进程从外面看不出卡死**：线程全是 `WaitReason=UserRequest`、CPU 0，和「安静等消息」一样，
  `Process.Responding` 还会骗你，只有 `IsHungAppWindow` 说真话——所以别删这个看门狗。
- **收进托盘前必须 `SendMessage(WM_CANCELMODE)`**：托盘菜单是模态的，窗口藏起来之后
  菜单会留在屏幕上既点不到也关不掉（实测过）。同理，别在窗口隐藏时弹任何模态框。
- **界面读的数据要把「还没落库」的部分算进来**：`collect.Collector.Peek()` 给一份不清空的
  增量拷贝，`service.Snapshot` 按 `day BETWEEN from AND to` 并进去。落库是 30 秒一次的
  批处理，只读数据库的话数字要等半分钟才动。并的时候**必须过滤时间范围**，
  否则切到近 7 天/过去某天时会把今天的计数多算一遍。
- **TOP 榜这类列表靠留白和列对齐撑层次**：`overviewRowCells` 里排名与键名之间的
  `baseOvRankGap` 不是装饰——贴在一起时「1空格」「2E」会被读成一个词。改布局先改这个纯函数
  和它的测试，再用 `PrintWindow` 截图确认。
- **`WM_CLOSE` 的语义由用户设置决定**（`config.CloseAction`，存 HKCU 的 `CloseAction`）：
  `tray` = 收进托盘、`exit` = 直接退出、**没选过** = 弹一次选项框再记住。
  所以任何「保存后退出程序」的流程都不许再 `postMessage(wmClose)`，必须走 `a.quit()`，
  否则那次退出会变成隐藏，后续动作永远等不到（改数据目录踩过这个坑）。
- 托盘图标必须在 `cleanup()` 里 `trayRemove()`，否则进程结束后图标会残留到鼠标划过。
- 托盘放不上时要**回退成关窗即退出**，不能把窗口藏起来又没有图标入口——
  那样用户只能去任务管理器。判断依据就是 `a.trayAdded`。
- 「首次收进托盘」的气泡标记（`config.FlagTrayHintShown`）**要落盘**：只记内存等于每次启动都弹。
  开机自启分支不许弹任何气泡或对话框（用户明确要求静默）。
- **升级握手**：安装器替换文件前会给程序发一条注册消息 `KeyboardStatistics.QuitForUpgrade`
  （`RegisterWindowMessage`，两个进程注册同一字符串即得到同一消息号），程序收到后走正常退出。
  因为 `WM_CLOSE` 已经不可用于「请退出」——Restart Manager 发的也是它，程序只会收进托盘，
  安装器干等 30 秒再强杀，还会丢掉没落库的计数（实测耗时 32s → 握手后 5s）。
  字符串两侧必须逐字一致：`internal/tray.QuitMessageName` ↔ `scripts/installer.iss` 的
  `QuitForUpgradeMsg`，`internal/tray` 有测试盯着这个常量。
- **安装器在程序运行时先问一句**（`InitializeSetup` + `AppIsRunning`）：交互模式弹确认框，
  选「否」用 `Result := False` 干净退出；静默模式（`WizardSilent`）不问，直接走握手消息。
  判断运行与否只能按**窗口标题**（`FindWindowW`）：托盘常驻时窗口可能被藏起来，
  但句柄还在——这正好把「收在托盘里也算在运行」覆盖住。

## 自动化验证脚本（`scripts/verify/`）

界面/安装相关的行为没法写 Go 单测，用这批只用 `ctypes` 的 Python 脚本验（不引第三方包）：

| 脚本 | 验什么 |
|---|---|
| `close_choice.py` | 首次关窗选项框的两个分支 + 选择与气泡标记有没有落盘 |
| `close_behavior.py` | 已设好的三种关窗模式各自的反应 |
| `tray_check.py` / `tray_probe.py` | 关窗隐藏/托盘唤醒、托盘图标是否真的注册（带对照窗口） |
| `tray_menu_hang.py` | 托盘右键之后界面线程还响不响应（有没有被模态菜单卡死） |
| `watch_hang.py` | 盯住运行中的窗口，记录「未响应」的时间段与那段 CPU |
| `soak_ui.py` | 浸泡验证界面是不是真的在处理消息（外部探针 + 底部时钟像素变化） |
| `refresh_latency.py` | 量「库里数字变了 → 界面上看见」的延迟（验刷新节奏，只对测试数据目录用） |
| `wizard_shots.py` | 逐页截图安装向导，验界面语言/文案（走正常流程，最后点取消，不会真装上） |
| `installer_prompt.py` | 升级/重装的确认框（连向导一起点完），是否与是否两条路 |
| `autostart_silent.py` | 自启静默无提示 + 欢迎说明推迟到打开窗口 |
| `handshake_timeline.py` | 升级握手到进程退出的时间线 |

写这类脚本的两个坑：Inno 的确认框属于 `%TEMP%` 里的 `*-setup.tmp` **子进程**（按安装包 pid 找不到），
向导窗口类名是 `TWizardForm` 而不是 `#32770`，按钮文案带 `&` 助记符。

## 打包与发布

- 产出只有一个 exe，用户双击即用，**不写注册表、不装服务**。
- **产物不得依赖 OpenGL 或任何额外运行库**：单 exe 拷到别的机器就能跑；发布前必须在
  **云桌面 / 远程桌面**（无 GPU 加速）实测能正常出窗口。
- 安装包用 Inno Setup，脚本 `scripts/installer.iss`：`PrivilegesRequired=lowest`（按用户安装，
  允许自选目录），「便携模式」任务在 `{app}` 下创建 `portable.txt`。安装包不改变主程序行为。
- **安装界面是简体中文，语言文件自己维护**：`scripts/languages/ChineseSimplified.isl`
  按 Inno 6 的完整消息集逐条翻译（少一条 ISCC 直接报 missing message），
  **必须 UTF-8 带 BOM**——Inno 只在有 BOM 时按 UTF-8 解析 `.isl`，否则按
  `LanguageCodePage` 当 ANSI 读，中文全乱码。这是仓库里唯一允许 BOM 的文件。
  `[Languages]` **只挂这一种语言**：挂两种会先弹「选择安装语言」，而 Inno 按系统的
  「首选界面语言」自动选，实测这台机器上自动选中的是英文，向导整页英文。
  `[Code]` 里的对话框文案一律走 `ExpandConstant('{cm:...}')`，别在那里写死中文或英文。
- **升级必须原地**：`AppId` 固定不变（改了就等于另一个软件，升级会装成两套）；
  升级时隐藏目录选择页（`ShouldSkipPage`），并用 `PrepareToInstall` + `{param:DIR|}` 拒绝指向别处的 `/DIR`。
  想换目录必须先卸载。
- **`AppId` 的花括号要数清楚**：写成 `AppId={{{#MyAppId}}`（`{{` + 展开后的 GUID + `}`）。
  少一个 `}` 会悄悄变成另一个 AppId，症状是「升级认不出已安装、装到默认目录」。
  装完检查卸载键名必须是 `{GUID}_is1`。
- **不要声明 `AppMutex`**：一旦声明，安装器会要求用户先手动关掉程序，静默安装直接 `exit 1`。
  要「安装器替用户关掉」就用 `CloseApplications=force`（配合 Restart Manager，实测可自动关闭并替换文件）。
- **单实例**：主程序启动时用 `internal/single` 抢命名互斥体，第二个实例激活已有窗口后退出。
  互斥体名只在一处定义（`single.DefaultName`），**不要**把它接到安装包的 `AppMutex` 上。
- 版本号注入 `main.version`，`--version` 能打印；发布产物命名 `keyboard-statistics-v1.2.0.exe`。
- 每次发布更新 `CHANGELOG.md`，写清统计口径有没有变。

### 发版清单（v1.0.0 定型，照这个顺序走）

版本号散落在好几处，**漏一处就会出现「安装包写旧号、程序里是新号」**，发布前先全捞出来核对：

```powershell
Select-String -Path README.md,scripts\installer.iss,CHANGELOG.md -Pattern '\d+\.\d+\.\d+'
```

1. **改版本号**：`scripts/installer.iss` 两处（`MyAppVersion` + 那句写死的 `#error` 断言，
   ISPP 的 `#error` 不吃字符串拼接）、`README.md` 三处（手册版本 + 两个产物文件名）。
2. **写文档**：`CHANGELOG.md` 新版本节（分类固定：新增/变更/修复/移除/安全）、
   `docs/releases/vX.Y.Z.md`（发布说明：下载对照表 + SHA256、能做什么、相比上一版的变化、
   安装升级步骤、已知限制、做过的验证）、`docs/devlog.md` 最上方一条。
3. **门禁**：`gofmt -l cmd internal` 无输出、`go vet ./...`、`go test ./...` 全绿。
4. **出产物**：`.\scripts\build.ps1 -Version X.Y.Z` → `ISCC.exe scripts\installer.iss`
   （ISCC 只吃 `dist\keyboard-statistics-vX.Y.Z.exe` 已经存在，缺了会 `#error`）。
   核对两个产物的 `VersionInfo.FileVersion` 都是 `X.Y.Z.0`，再算 SHA256 填进发布说明。
5. **打包**：`Compress-Archive` 只列**文件**（传目录会多套一层），zip 里放
   exe + 安装包 + 发布说明；旧版本产物移进 `dist\archive\` 留回滚底子。
6. **实机升级验证**（有安装版时必做）：先给窗口发 `KeyboardStatistics.QuitForUpgrade`
   **让它自己正常退出**（`Stop-Process -Force` 会丢掉最多 30 秒没落库的计数，
   实测优雅退出前后总按键 39531 → 39531 一个不丢）→ 静默装新版 →
   核对版本资源、`portable.txt`、数据总量、卸载登记项 `DisplayName` 里的版本号 → 起程序浸泡一次。
7. **提交 + 打标签**：`git commit`（`📝 docs:` 或 `🔧 chore:`）→
   `git tag -a vX.Y.Z -m "..."`。仓库目前**没有远端**，所以不推送；
   要发到托管平台时，把 zip 和发布说明正文贴过去即可。

- 本机 Inno Setup 是**按用户安装**的，`ISCC.exe` 在 `%LOCALAPPDATA%\Programs\Inno Setup 6\`，
  不在 `Program Files`。
- **在工作区内直接运行 exe / 安装包会被 DSH 沙箱以 Low Integrity 跑**（写不了 `HKCU` 与 `%LOCALAPPDATA%`，
  安装包甚至会在启动阶段挂住）。测安装与升级一律先把产物拷到 `%TEMP%` 之类的**工作区外**目录再跑。

## Git 约定

- 分支：`feat/xxx`、`fix/xxx`、`chore/xxx`。
- 提交信息：**emoji + 类型 + 冒号 + 中文描述**，例如
  `✨ feat: 热力图支持按周聚合`、`🐛 fix: 长按被重复计数`。
- emoji 与类型一一对应，不要自创：

  | emoji | 类型 | 用在 |
  |---|---|---|
  | ✨ | `feat` | 新功能 |
  | 🐛 | `fix` | 修 bug |
  | ✅ | `test` | 只加/改测试 |
  | ♻️ | `refactor` | 重构，行为不变 |
  | 📝 | `docs` | 文档、日志、手册 |
  | 🎨 | `style` | 格式、命名，不改逻辑 |
  | ⚡ | `perf` | 性能优化 |
  | 🔧 | `chore` | 构建、依赖、配置 |
  | 🔥 | `remove` | 删除代码或文件 |

- 一次提交一件事；TDD 的一个红绿循环可以单独提交；重构与功能分开提交；
  构建产物（`bin/`、`dist/`）不进仓库。

## 文档与日志

**每次开发都要留下日志，否则视为未完成。** 三个文件分工不同，不要混：

| 文件 | 读者 | 什么时候写 |
|---|---|---|
| `docs/devlog.md` | 后续的 AI 代理、新加入的人 | **每完成一个可提交的单元就在最上方追加一条** |
| `CHANGELOG.md` | 用户 | 每个发布版本新增一节；有用户可见变化就记 |
| `README.md` | 用户 | **每个发布版本按当时功能重写一遍**，开头标注手册版本号 |

`docs/devlog.md` 每条要写清：**做了什么 / 为什么这么做 / 踩了什么坑 / 测试状态 / 下一步**。
目的很直接——让下一个人不用翻 commit diff 就能接上活。

`CHANGELOG.md` 分类固定用：新增 / 变更 / 修复 / 移除 / 安全。涉及统计口径变化时
必须显式标注「口径变更」并说明对历史数据的影响。

`README.md` 是使用手册，必须与 `CHANGELOG.md` 的版本号对应。**功能改了但手册没改，等于没做完。**

## 代理工作守则

1. **测试先行**：先写失败的测试，再写实现；跳过 TDD 必须在回复里说明理由并等你确认。
2. **先读后改**：改文件前先读；不确定结构就先列目录。
3. **最小改动**：不顺手重排、不重命名无关符号、不升级无关依赖、不跑 `go mod tidy` 顺手升版本。
4. **不擅自加依赖**：新增第三方包先说明理由并等确认。
5. **不擅自改系统行为**：开机自启、注册表、权限提升、签名、原生模块，一律先问。
6. **不碰用户数据**：不为测试去读真实统计库或本机采集结果。
7. **改完自检**：跑 `go vet`、`gofmt -l .`、`go test ./...`，回复里写明跑了什么、结果如何。
8. **不确定就停下问**：宁可少做，不要猜着改。
9. **不要用 shell 重定向改源码**：Windows 上 `Set-Content` / `>` 默认用 ANSI 编码，
   会把 UTF-8 中文注释写成乱码并毁掉文件；写文件一律用编辑工具。
   同理，`gofmt -w` 之外的文本处理不要读写往返。

## 完成定义（DoD）

- [ ] 功能按描述可用，边界处理过（首次启动无数据、采集关闭、数据库损坏）
- [ ] 新增/修改的逻辑都有对应测试，且是**先写测试后写实现**
- [ ] `go vet ./... && go test ./...` 通过，`gofmt -l .` 无输出
- [ ] 涉及统计口径变化时更新了 `docs/metrics.md` 与 `CHANGELOG.md`
- [ ] 在 `docs/devlog.md` 追加了本次开发记录（做了什么/为什么/坑/下一步）
- [ ] 发布版本已重写 `README.md` 并标注手册版本号，与 `CHANGELOG.md` 对应
- [ ] 新增/改动 Windows API 调用在 `docs/manual-test.md` 补了验证步骤
- [ ] 没有新增未说明的依赖、没有提交密钥或构建产物
