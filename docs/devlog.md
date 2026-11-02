# 开发日志

面向**后续接手的人和 AI 代理**的开发流水账。用户可见的版本变更看 [CHANGELOG.md](../CHANGELOG.md)，
统计口径看 [metrics.md](metrics.md)，需求看 [requirements.md](requirements.md)。

**写日志的规矩**：每完成一个可提交的单元（一次红绿循环、一次环境调整、一次决策）就在**最上方**追加一条。
条目要写清楚「做了什么 / 为什么这么做 / 踩了什么坑 / 测试状态 / 下一步」，让下一个人不用翻 commit diff 也能接上。

---

## 2026-11-02 · 工程规范：UTF-8、提交身份、日志体系

**做了什么**：

1. Git 提交身份统一为 `Afar的大肥鱼 <yigehd@foxmail.com>`，并用
   `git rebase --root --exec "git commit --amend --no-edit --reset-author"` 重写了前 4 个提交的作者与提交者。
2. 新增 `.editorconfig`，全项目强制 UTF-8（无 BOM）+ LF，`*.ps1/*.bat/*.cmd` 用 CRLF。
3. 新增三份文档：`README.md`（使用手册）、`CHANGELOG.md`（版本变更）、`docs/devlog.md`（本文件）。
4. `AGENTS.md` 增加「编码规范 · UTF-8」与「文档与日志」两节，DoD 增加日志与手册检查项。

**为什么**：中文注释被编码问题毁过一次（见下一条日志），必须从工具链层面堵住；
另外这个项目会由多个 agent 接着做，没有开发日志就只能靠翻 diff，成本太高。

**踩的坑**：`--reset-author` 会把提交时间改成当前时刻。对这个仓库无所谓（所有提交都是同一天），
但如果是移植老仓库，应改用 `git filter-branch --env-filter` 只改身份、保留时间。

**测试状态**：文档改动，未触及代码；`go test ./...` 仍全绿。

**下一步**：继续 M1 剩余纯函数。

---

## 2026-11-02 · M1 组合键判定（TDD 第一次红绿）

**做了什么**：建 `internal/stats` 包，实现 `ComboName(pressed, key)` 组合键判定与规范化命名，
外加 `KeyName` 键名映射、平台无关 VK 常量表。22 个子测试覆盖 `Ctrl+V`、`Alt+A`、`Shift+A`、
单按修饰键、左右修饰键合并等全部规则。

**为什么**：组合键判定是纯逻辑、规则最密、最容易写错，适合用来把 TDD 节奏跑顺；
放在 stats 包还能跨平台测试，不依赖 Windows。

**踩的坑**：

1. **PowerShell 编码毁文件**。用 `Get-Content -Raw | -replace | Set-Content` 批量替换时，
   PowerShell 默认按 ANSI 写回，把 `keyname.go` / `vk.go` 的中文注释写成非法 UTF-8，
   编译器报 `illegal UTF-8 encoding`，两个文件直接报废。已改为用编辑工具写文件，
   并把这条写进 AGENTS.md 工作守则第 9 条。
2. **自己两份文档打架**。`metrics.md` 规定 `VK_RETURN` 显示名是 `回车`，但测试里手滑写了 `Enter`。
   处理原则：口径文档是唯一标准，改测试而不是改文档。

**测试状态**：`go test ./internal/stats` 全绿（22 子测试），`gofmt -l .` 无输出，`go vet ./...` 干净。

**提交**：`feat: 组合键判定与规范化命名`

**下一步**：M1 剩余纯函数 —— TOP N 稳定排序（次数相同按 vk 升序）、占比、24 小时分布、
日均、峰值，每个各走一次红绿。

---

## 2026-11-02 · 环境搭建（Go + GCC）

**做了什么**：装 Go 1.27.3（winget）与 GCC 16.2.0（MSYS2 mingw-w64 UCRT）。
验证链路：`go run` → cgo 调 C 函数 → 真正编译并启动一个 Fyne v2.8.2 窗口。

**为什么**：Fyne 需要 cgo，没有 gcc 寸步难行。

**踩的坑**：

1. `winget install GoLang.Go` 直接装会报 `0x8a15005e 服务器证书不匹配`，
   必须加 `--source winget` 并忽略 msstore 源。
2. **WinLibs 那条路走不通**：`github.com` 与 `release-assets.githubusercontent.com` 当时不可达，
   `gh-proxy.com` 只有 11–140 KB/s，276MB 要下几个小时；换 `gh-proxy` 的兄弟站点全部超时。
   最后改用**腾讯云 MSYS2 镜像**（实测 ~1MB/s）拉 41MB 基础包，再 `pacman -S mingw-w64-x86_64-gcc`。
3. 首次 `pacman` 报「无法锁定数据库 File exists」，是初始化脚本留下的 `db.lck`，删掉重跑即可。
4. Windows 自带的 `tar.exe` 不支持 xz（缺少外部 `xz` 程序），改用 Python 的 `tarfile` 解包。

**持久化配置**：

- 系统 PATH 追加 `C:\Program Files\Go\bin`、`C:\msys64\mingw64\bin`
- `go env -w GOPROXY=https://goproxy.cn,direct GOSUMDB=off CGO_ENABLED=1`
  （`GOSUMDB=off` 是因为 `sum.golang.org` 不可达；介意供应链校验可改回 `sum.golang.google.cn`）

**下一步**：确认环境在别的终端也生效；继续 M1。

---

## 2026-11-02 · 立项与文档定稿

**做了什么**：确定产品形态（Windows 单 exe、Go + Fyne + SQLite、纯本地）、
写下 `AGENTS.md`（含 TDD 与红线）、`docs/requirements.md`、`docs/metrics.md`，
初始化 git 仓库与 `.gitignore` / `.gitattributes`。

**关键决策**：

- 技术栈从 Electron/React/TS 换成 Go（用户要求「能打包成 exe 方便别人安装」）
- 界面库 Fyne、存储 SQLite（`modernc.org/sqlite`，纯 Go 无 cgo）
- 组合键只统计「同时按下」，不做链式/顺序快捷键（那属于按键序列，碰红线）
- 首次启动默认开始采集，但必须弹说明 + 发通知，保证「采集可见」
- 活跃时长在采集时累加秒数，库里不存任何按键时间点

**下一步**：M1 纯函数。
