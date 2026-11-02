# 🛠️ 开发日志

面向**后续接手的人和 AI 代理**的开发流水账。用户可见的版本变更看 [CHANGELOG.md](../CHANGELOG.md)，
统计口径看 [metrics.md](metrics.md)，需求看 [requirements.md](requirements.md)。

**写日志的规矩**：每完成一个可提交的单元（一次红绿循环、一次环境调整、一次决策）就在**最上方**追加一条。

每条写清楚五件事，让下一个人不用翻 commit diff 也能接上活：

- 🎯 **做了什么**
- 💡 **为什么这么做**
- 🕳️ **踩了什么坑**
- ✅ **测试状态**
- ➡️ **下一步**

标题用 emoji 标类型：`✨` 功能　`✅` 测试　`🐛` 修复　`⚙️` 环境　`🛠️` 规范　`📝` 文档　`♻️` 重构

---

## 🛠️ 2026-11-02 · 界面选型改道：Fyne 撞上 OpenGL 死路，改原生 Win32（GDI）

**🎯 做了什么**：

1. **业务层收口完成**，全部有单元测试：Windows 键盘钩子（`WH_KEYBOARD_LL` + `runtime.LockOSThread`
   + 消息循环）、采集状态机（长按去重 / 组合键判定 / 活跃时长）、SQLite 存储（WAL + 累加 upsert）、
   统计聚合（TOP10 / 24 小时时段分布 / 日均 / 峰值）、CSV 导出（UTF-8 BOM）、
   开机自启（`HKCU\...\Run`）、服务层门面 `internal/service`（UI 只依赖它）。
2. **界面层撞墙**：Fyne v2 在本机（云桌面，无 GPU 加速）创建窗口直接失败，报错原文见下方「踩的坑」。
3. 试过用 MSYS2 的 Mesa 软渲染替换 `opengl32.dll`，能起窗口但依赖链要带 `libLLVM-23.dll`（约 170MB），
   放弃——那会把「单文件 exe」变成「exe + 一堆 DLL」。
4. 用户拍板：**界面改用原生 Win32（GDI）**，不碰 OpenGL。界面层开始重写，顺序是
   骨架（窗口 + 消息循环 + 计时器刷新）→ 设置页 → 概览页 → 热力图页。
5. 新增 `docs/ui-plan.md`：选型理由、三页结构、GDI 自绘思路、「新依赖准入检查」全部写进文档。

**💡 为什么**：

- 「单文件 exe、双击即用」是本项目的第一分发约束。任何要求用户额外装运行库、驱动、
  甚至只是多带几个 DLL 的方案都不合格——Mesa 能跑，但 170MB 的 LLVM 依赖直接违背这条约束。
- Fyne 的渲染后端是 **OpenGL 2.1+**。云桌面 / 远程桌面 / 部分虚拟机与服务器核显环境经常没有可用的
  WGL 实现。这不是「换个版本就好」的问题，而是架构性依赖；目标用户里这类环境并不罕见。
- 原生 Win32 + GDI 只依赖 `user32.dll` / `gdi32.dll` 这些**系统自带** DLL，用
  `golang.org/x/sys/windows` 纯 syscall 调用，**不需要 cgo**、不要求任何图形加速、体积增量几乎为零，
  RDP 下走 GDI 重定向天然可用（这是 RDP 的默认显示通道）。
- 三页界面本身不复杂，GDI 的「矩形 + 文字」足够画完热力图与柱状图，没必要为控件体系扛着一个重型 GUI 框架。

**🕳️ 踩的坑**（后来人直接抄结论，别再重踩）：

1. **Fyne 在无 GPU 环境起不来窗口**，实测报错原文：

   ```
   Fyne error: window creation error
   Cause: APIUnavailable: WGL: The driver does not appear to support OpenGL
   ```

   注意它不是「渲染花屏」而是**窗口根本创建不出来**：GLFW 在 `wglCreateContext` 那一步就失败，
   程序没有任何可用界面，等于完全不可用。
2. **Mesa 软渲染这条路不值得，已放弃，不要再试**。把 MSYS2 的 Mesa `opengl32.dll`（llvmpipe）
   拷到 exe 旁边确实能起窗口，但完整依赖链还要 `libLLVM-23.dll`（约 170MB）以及若干 mingw 运行时 DLL；
   打包出来不再是单文件，用户只拷走 exe 就会报「找不到 DLL」。
3. **Fyne 窗口创建失败后进程不退出，变成没有窗口的僵尸进程，并且占住调用方的 stdout 管道**。
   `ShowAndRun()` 失败后进程还活着：没有窗口、也没有退出，而且它继承了调用方（终端 / 脚本 / agent）
   的 stdout 句柄，调用方一直等管道 EOF，命令就永远不返回——表象是「编译或运行卡死」，
   实际是进程还在跑。
   **排查顺序**：先 `Get-Process` 看有没有残留的 `keyboard-statistics` / `go` 进程和它占的句柄，
   该杀就 `Stop-Process -Force`，再去看真实报错；不要直接反复重跑同一条命令，残留进程只会越堆越多。
4. Fyne v2 没有暴露「最小化窗口」的 API，之前自启最小化是靠 `FindWindowW` + `ShowWindow` 打的补丁
   （见 `internal/ui/window_windows.go`）。换成原生 Win32 后这段补丁可以整体删掉，
   `CreateWindowExW` 的 `STARTUPINFO.wShowWindow` 直接给 `SW_SHOWMINNOACTIVE` 即可。

**✅ 测试状态**：`go test ./...` 全绿 —— 实测 50 个 `Test` 函数、含子测试共 **115 个用例**通过
（`internal/stats` / `internal/store` / `internal/collect` / `internal/config` / `internal/service`；
`hook` 与 `ui` 无测试文件）。按 AGENTS.md 的约定，**钩子与 UI 不做自动化测试**，界面靠手动验证，
验证步骤待补进 `docs/manual-test.md`。
注意：`internal/ui` 当前仍是**旧的 Fyne 实现**，重写期间它会短暂处于「能编译但方向已废弃」的状态，
新代码按 `docs/ui-plan.md` 落地；也正因为 Fyne 走 cgo，跑 `go test ./...` 时 PATH 里必须有
`C:\msys64\mingw64\bin`（否则 `cmd/app` 与 `internal/ui` 报 `cgo: C compiler "gcc" not found`
而 `[build failed]`，这不是代码坏了）。Fyne 摘掉后这条 PATH 依赖会一起消失。

**➡️ 下一步**：

1. 按 `docs/ui-plan.md` 写 Win32 骨架：`CreateWindowExW` + `WndProc` + `SetTimer(2000ms)` 驱动刷新。
2. 设置页（采集开关 / 清空 / 导出 / 自启 / 关于）→ 概览页（指标 + 24 小时柱状图）→ 热力图页（键位图）。
3. 界面全部迁完后**单独一次 `🔧 chore` 提交**摘掉 `fyne.io/fyne/v2` 与 `go-gl/glfw`，并复跑
   `CGO_ENABLED=0 go build` 确认单文件 exe 仍能构建、体积没膨胀。
4. 把「云桌面 / RDP 下能起窗口」写进 `docs/manual-test.md`，列为发布前必测项。
5. `AGENTS.md` 的「技术栈」一节仍写 Fyne，与本次决策冲突，需确认后再改；本次不擅自改工程说明书。

---

## 🛠️ 2026-11-02 · 工程规范：UTF-8、提交身份、日志体系

**🎯 做了什么**：

1. Git 提交身份统一为 `Afar的大肥鱼 <yigehd@foxmail.com>`，并用
   `git rebase --root --exec "git commit --amend --no-edit --reset-author"` 重写了前 4 个提交的作者与提交者。
2. 新增 `.editorconfig`，全项目强制 UTF-8（无 BOM）+ LF，`*.ps1/*.bat/*.cmd` 用 CRLF。
3. 新增三份文档：`README.md`（使用手册）、`CHANGELOG.md`（版本变更）、`docs/devlog.md`（本文件）。
4. `AGENTS.md` 增加「编码规范 · UTF-8」与「文档与日志」两节，DoD 增加日志与手册检查项。

**💡 为什么**：中文注释被编码问题毁过一次（见下一条日志），必须从工具链层面堵住；
另外这个项目会由多个 agent 接着做，没有开发日志就只能靠翻 diff，成本太高。

**🕳️ 踩的坑**：`--reset-author` 会把提交时间改成当前时刻。对这个仓库无所谓（所有提交都是同一天），
但如果是移植老仓库，应改用 `git filter-branch --env-filter` 只改身份、保留时间。

**✅ 测试状态**：文档改动，未触及代码；`go test ./...` 仍全绿。

**➡️ 下一步**：继续 M1 剩余纯函数。

---

## ✅ 2026-11-02 · M1 组合键判定（TDD 第一次红绿）

**🎯 做了什么**：建 `internal/stats` 包，实现 `ComboName(pressed, key)` 组合键判定与规范化命名，
外加 `KeyName` 键名映射、平台无关 VK 常量表。22 个子测试覆盖 `Ctrl+V`、`Alt+A`、`Shift+A`、
单按修饰键、左右修饰键合并等全部规则。

**💡 为什么**：组合键判定是纯逻辑、规则最密、最容易写错，适合用来把 TDD 节奏跑顺；
放在 stats 包还能跨平台测试，不依赖 Windows。

**🕳️ 踩的坑**：

1. **PowerShell 编码毁文件**。用 `Get-Content -Raw | -replace | Set-Content` 批量替换时，
   PowerShell 默认按 ANSI 写回，把 `keyname.go` / `vk.go` 的中文注释写成非法 UTF-8，
   编译器报 `illegal UTF-8 encoding`，两个文件直接报废。已改为用编辑工具写文件，
   并把这条写进 AGENTS.md 工作守则第 9 条。
2. **自己两份文档打架**。`metrics.md` 规定 `VK_RETURN` 显示名是 `回车`，但测试里手滑写了 `Enter`。
   处理原则：口径文档是唯一标准，改测试而不是改文档。

**✅ 测试状态**：`go test ./internal/stats` 全绿（22 子测试），`gofmt -l .` 无输出，`go vet ./...` 干净。

**➡️ 下一步**：M1 剩余纯函数 —— TOP N 稳定排序、占比、24 小时分布、日均、峰值。

---

## ⚙️ 2026-11-02 · 环境搭建（Go + GCC）

**🎯 做了什么**：装 Go 1.27.3（winget）与 GCC 16.2.0（MSYS2 mingw-w64 UCRT）。
验证链路：`go run` → cgo 调 C 函数 → 真正编译并启动一个 Fyne v2.8.2 窗口。

**💡 为什么**：Fyne 需要 cgo，没有 gcc 寸步难行。

**🕳️ 踩的坑**：

1. `winget install GoLang.Go` 直接装会报 `0x8a15005e 服务器证书不匹配`，
   必须加 `--source winget` 并忽略 msstore 源。
2. **WinLibs 那条路走不通**：`github.com` 与 `release-assets.githubusercontent.com` 当时不可达，
   `gh-proxy.com` 只有 11–140 KB/s，276MB 要下几个小时；换 `gh-proxy` 的兄弟站点全部超时。
   最后改用**腾讯云 MSYS2 镜像**（实测 ~1MB/s）拉 41MB 基础包，再 `pacman -S mingw-w64-x86_64-gcc`。
3. 首次 `pacman` 报「无法锁定数据库 File exists」，是初始化脚本留下的 `db.lck`，删掉重跑即可。
4. Windows 自带的 `tar.exe` 不支持 xz（缺少外部 `xz` 程序），改用 Python 的 `tarfile` 解包。

**📌 持久化配置**：

- 系统 PATH 追加 `C:\Program Files\Go\bin`、`C:\msys64\mingw64\bin`
- `go env -w GOPROXY=https://goproxy.cn,direct GOSUMDB=off CGO_ENABLED=1`
  （`GOSUMDB=off` 是因为 `sum.golang.org` 不可达；介意供应链校验可改回 `sum.golang.google.cn`）

**➡️ 下一步**：确认环境在别的终端也生效；继续 M1。

---

## 🚀 2026-11-02 · 立项与文档定稿

**🎯 做了什么**：确定产品形态（Windows 单 exe、Go + Fyne + SQLite、纯本地）、
写下 `AGENTS.md`（含 TDD 与红线）、`docs/requirements.md`、`docs/metrics.md`，
初始化 git 仓库与 `.gitignore` / `.gitattributes`。

**💡 关键决策**：

- 技术栈从 Electron/React/TS 换成 Go（用户要求「能打包成 exe 方便别人安装」）
- 界面库 Fyne、存储 SQLite（`modernc.org/sqlite`，纯 Go 无 cgo）
- 组合键只统计「同时按下」，不做链式/顺序快捷键（那属于按键序列，碰红线）
- 首次启动默认开始采集，但必须弹说明 + 发通知，保证「采集可见」
- 活跃时长在采集时累加秒数，库里不存任何按键时间点

**➡️ 下一步**：M1 纯函数。
