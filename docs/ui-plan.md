# 🖼️ 界面方案（UI Plan）：原生 Win32 / GDI

> 状态：**进行中**。业务层（钩子、采集、存储、聚合、导出、自启、服务门面）已完成并通过测试；
> 界面层正在从 Fyne 重写为原生 Win32（GDI）。本文是这次重写的唯一方案说明，
> 界面代码与本文冲突时以本文为准，改方案先改本文并补一条 `docs/devlog.md`。
>
> 相关文档：[requirements.md](requirements.md)（界面规格）、[metrics.md](metrics.md)（统计口径，
> 界面只做展示、不改口径）、[devlog.md](devlog.md)（2026-11-02 那条记录了改道原因与实测报错）。

---

## 1. 结论先行

| 项 | 决定 |
|---|---|
| 界面框架 | **原生 Win32 窗口 + GDI 自绘**，不引入任何 GUI 框架 |
| 图形 API | **零**：不用 OpenGL / DirectX / Vulkan，不要求显卡加速 |
| 调用方式 | `syscall` LazyDLL 调 `user32.dll` / `gdi32.dll` / `kernel32.dll` / `shell32.dll`，**不需要 cgo** |
| 页面结构 | 顶部工具栏（时间范围 + 采集状态）+ 三页（概览 / 热力图 / 设置）+ 底部状态栏 |
| 刷新节奏 | `SetTimer` **每 2000ms** 触发一次重绘，频次固定、不随页面变化 |
| 旧依赖 | `internal/ui` 里现有 Fyne 实现全部作废，界面迁完后单独提交摘掉 `fyne.io/fyne/v2` |

一句话：**界面只允许依赖 Windows 自带 DLL，不允许依赖任何 GPU 图形栈。**

---

## 2. 为什么放弃 Fyne

### 2.1 直接原因：本机（云桌面）创建窗口就失败

实测报错原文：

```
Fyne error: window creation error
Cause: APIUnavailable: WGL: The driver does not appear to support OpenGL
```

关键点：这不是「画面花屏 / 掉帧」，而是**窗口根本创建不出来**。GLFW 在
`wglCreateContext`（WGL = Windows 的 OpenGL 入口）这一步就失败，进程拿不到可用窗口，
整个程序等于没有界面。云桌面、远程桌面、部分虚拟机、只有基础显示驱动的服务器环境
经常没有可用的 WGL 实现。

### 2.2 Fyne 的架构性依赖

- Fyne v2 的渲染后端是 **OpenGL 2.1+**（`github.com/go-gl/gl` + `go-gl/glfw`），
  且通过 cgo 调用，必须带 C 编译器构建。
- 这不是「升级 Fyne 或者换个驱动配置」能绕开的：**只要目标机器没有 GPU 加速的 OpenGL，
  程序就起不来**。而我们的分发形态是「给谁都能双击运行」的本地小工具。

### 2.3 软渲染（Mesa llvmpipe）为什么也放弃

试过的方案：把 MSYS2 的 Mesa `opengl32.dll`（llvmpipe 软件光栅化）放到 exe 同目录。

- ✅ 能起窗口，功能可用；
- ❌ 完整依赖链还要 `libLLVM-23.dll`（**约 170MB**）加若干 mingw 运行时 DLL；
- ❌ 结果是「exe + 一堆 DLL」，用户只拷走 exe 就报「找不到 DLL」，
  **「单文件 exe、双击即用」这条第一分发约束被彻底破坏**；
- ❌ 每次 Mesa / LLVM 升级都要重新验证依赖，维护成本远高于收益。

结论：**放弃，不要再试**。

---

## 3. 为什么选原生 Win32 / GDI

1. **无图形 API 依赖**：GDI 是指令级/CPU 侧的绘制（`FillRect` / `TextOutW` / `BitBlt`），
   全部由系统 DLL 实现，不需要任何图形驱动能力，也不需要创建 GL/D3D 上下文。
2. **RDP / 云桌面天然可用**：远程桌面默认显示通道就是 GDI 重定向（RemoteFX / 图形管线是对它的加速，
   不是前置条件），只要 RDP 能画出记事本，就能画出本程序。
3. **体积几乎为零**：`user32` / `gdi32` 是系统 DLL，不随 exe 打包；单文件 exe 不会再膨胀几十 MB。
4. **不需要 cgo**：`golang.org/x/sys/windows` + `syscall.NewLazyDLL` 就能调，
   `CGO_ENABLED=0` 也能构建（迁移完成后要实测一次）。
5. **控件量小，自绘反而更可控**：三页界面只有文字、数字、矩形（柱状图 / 键位图）。
   用 GDI 画比套控件体系简单，也不受第三方控件风格、DPI 缩放、字体回退的坑。
6. **文件名 / 中文 / DPI**：Win32 用宽字符（`...W`）API + `assets/app.manifest` 声明
   Per-Monitor DPI 感知，高分屏不糊；字体统一用「微软雅黑 UI」（`Microsoft YaHei UI`）。

代价（接受）：所有控件都要自己画、自己处理命中测试（hover / click）、自己管重绘。
因此本文第 6 节把绘制思路定死，避免每个人各写一套。

---

## 4. 窗口与页面结构

### 4.1 窗口

- 标题：`键盘使用统计`（**固定不变**，自启最小化时靠它定位/调试）
- 初始尺寸 **900×680**，最小 **800×600**（`WM_GETMINMAXINFO` 里限制）
- 标题栏用系统默认（不折腾自定义 chrome）
- 关闭按钮 = **退出程序并停止采集**（`WM_CLOSE` → flush → `DestroyWindow` → 退出消息循环），
  不做「关窗后台常驻」（托盘是 v0.2 候选）
- 开机自启启动（`--autostart`）：`CreateWindowExW` 时 `wShowWindow = SW_SHOWMINNOACTIVE`，
  **最小化但不隐藏**，任务栏可见

### 4.2 三段式布局

```
┌──────────────────────────────────────────────────────────────┐
│ 键盘使用统计   [ 今日 | 近 7 天 | 近 30 天 | 全部 ]   ● 采集中 │ ← 顶部工具栏 (56px)
├──────────────────────────────────────────────────────────────┤
│                                                              │
│                       页面内容区（自绘）                       │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│ 2026-11-02 ~ 2026-11-02 · 共 7 天 · 最后刷新 22:41:02        │ ← 底部状态栏 (24px)
└──────────────────────────────────────────────────────────────┘
```

- **顶部工具栏**
  - 左：标题
  - 中：时间范围四选一（`今日` / `近 7 天` / `近 30 天` / `全部`，默认 `今日`，
    与 `stats.Scope` / `metrics.md` 的口径一一对应）——自绘分段按钮，选中项用强调色底 + 白字
  - 右：采集状态 `● 采集中`（绿点）/ `● 已暂停`（灰点）/ `● 采集中断`（红点，钩子安装失败）
- **底部状态栏**：当前数据范围、天数、最后刷新时刻；出错时在这里显示一行红字
  （如「读取数据失败」），不弹模态框打断用户
- **空状态**：无数据时页面中央显示「当前范围还没有数据」，不画 0 值图表

### 4.3 三页

页面切换用顶部工具栏下方的自绘标签条（`概览` / `热力图` / `设置`），
**不用 `comctl32` 的 Tab 控件**：自绘三个矩形按钮 + 一个 `currentPage` 枚举，命中测试足够，
也避免引入 comctl32 的初始化（`InitCommonControlsEx`）与主题差异。

| 页 | 内容 |
|---|---|
| 概览 | 四张指标卡（总按键数 / 活跃时长 / 日均按键 / 峰值）+ 按键 TOP 10 + 快捷键 TOP 10 + 24 小时柱状图 |
| 热力图 | QWERTY 全键盘键位图，按次数着色；鼠标悬停高亮 + 显示键名 / 次数 / 占比 |
| 设置 | 采集开关、清空数据（二次确认）、导出 CSV、开机自启开关、关于（版本 / 数据目录 / 打开目录） |

数据全部来自 `service.Snapshot(scope)`（总按键数、活跃时长、日均、峰值、TOP 列表、
24 小时桶、`KeyCounts` 按 vk 的次数、`ComboCounts`），界面**只做展示，不重新计算口径**。

### 4.4 刷新节奏：2 秒

- 用 `SetTimer(hwnd, idRefresh, 2000, nil)` 每 2 秒触发一次 `WM_TIMER`；关窗时 `KillTimer`。
- 为什么是 2 秒：采集数字不是秒表，肉眼可感知即可；太快只是白白触发 GDI 重绘和 SQLite 查询。
- 为什么用 `SetTimer` 而不是自己起 goroutine + `time.Ticker`：Win32 控件只能在创建它的线程上操作，
  `SetTimer` 天然把回调送进同一条消息循环，省掉跨线程同步。
- **注意**：`WM_TIMER` 里**不要直接查库**。正确做法是「`WM_TIMER` 只投递一次取数请求 → 后台 goroutine
  取快照 → 结果用 `PostMessage(hwnd, WM_APP_SNAPSHOT, 0, 0)` 交回窗口线程 → 窗口线程只拿现成数据重绘」，
  否则一次慢查询（大范围 `全部` 聚合）会卡住消息循环，窗口变「未响应」。上一次取数还没回来时，
  本次 tick 直接跳过，不排队堆积。
- 用户改时间范围、切页面、点按钮时立即重绘一次，不等下一个 tick。

---

## 5. 交互

| 交互 | 处理 |
|---|---|
| 点时间范围 | 更新 `scope` → 立即重新取快照 → `InvalidateRect` 重绘 |
| 切页面 | 更新 `currentPage` → `InvalidateRect` 重绘 |
| 悬停热力图某键 | `WM_MOUSEMOVE` 命中测试出 `vk` → 该键描边高亮 + 底部信息条显示「`A` · 1,234 次 · 12%」 |
| 离开热力图 | 清空 hover 状态，恢复中性描边 |
| 点「清空数据」 | `MessageBoxW` 二次确认（默认焦点在「取消」）→ 确认后调 `service.ClearData()` |
| 点「导出 CSV」 | `SHBrowseForFolderW` / 保存对话框选目录 → `service.ExportCSV(dir)` → 弹结果提示 |
| 点「打开数据目录」 | `ShellExecuteW(..., "open", dir, ...)` |
| 采集开关 | `service.SetCollecting(bool)` → 刷新状态栏圆点 |

悬停信息**优先画在底部信息条**（一行文本，零额外窗口）；后续如果需要跟随鼠标的浮动
tooltip，再加一个 `WS_POPUP` 小窗口 + `TrackMouseEvent`，属于增强项，不影响任何口径。

---

## 6. GDI 自绘思路

通用规则（三页共用）：

- **双缓冲**：`WM_PAINT` 里先 `CreateCompatibleDC` + `CreateCompatibleBitmap` 画到内存位图，
  最后一次性 `BitBlt` 到窗口 DC。否则每个矩形都直接往窗口上画会明显闪烁。
- **文字**：`SetBkMode(hdc, TRANSPARENT)` + `DrawTextW`（带 `DT_CENTER | DT_VCENTER | DT_SINGLELINE`
  居中，`DT_RIGHT` 右对齐数字）。字体统一 `Microsoft YaHei UI`，字号按 DPI 缩放。
- **颜色**：全部用 `RGB()` 常量集中定义（背景、卡片底、边框、主文字、次文字、强调色、热力图渐变两端），
  不在绘制代码里散落魔法数。
- **文本度量**：`GetTextExtentPoint32W` 量宽，用于右对齐与截断，不靠猜字符宽度。
- 所有坐标计算集中在「布局函数」里，绘制函数只负责按算好的 `RECT` 填色填字，
  换分辨率 / 缩放只改布局。

### 6.1 热力图

1. **键位表是数据不是画**：定义一张键位表 `[]KeyBox{Label, VK, X, Y, W, H}`
   （单位是格子，不是像素），覆盖主键区、数字排、功能键排、修饰键、方向键、小键盘。
   每行按 US QWERTY 布局排，`Tab` / `CapsLock` / `Shift` / `Enter` 等宽键用多个格子宽度表示。
2. **绘制**：遍历键位表，把格子坐标换算成像素 `RECT`：
   - `FillRect` 填充底色（有数据：热力色；无数据：中性灰）；
   - `FrameRect` 或 `SelectObject(细笔)` 画 1px 边框；
   - `DrawTextW` 居中写键名（格子太小时只画色块，不写字）。
3. **着色：按「图内最大值」归一化**（**渲染用**，与 `metrics.md` 的占比不是一回事：
   占比显示用 `count / 总按键数`，着色用下面这套归一化，否则个别高频键会把整张图压成同一色）：

   ```
   ratio = count(vk) / max(count(*))        // 用本图最大值做归一化，避免个别键把整图压平
   t     = sqrt(ratio)                      // 开方让低中频段可分辨，纯线性会被最大值吃掉
   color = lerp(浅色, 深色, clamp(t, 0, 1)) // 逐通道线性插值 R/G/B
   ```

   无数据（`count == 0`）→ 中性灰，不参与渐变。悬停键额外画 2px 强调描边。
4. **组合键不参与着色**（口径要求：热力图只反映单键）。数据源只用 `Snapshot.KeyCounts`。
5. 悬停命中测试：鼠标坐标反查键位表（`PtInRect`），缓存当前 `hoverVK`，值没变就不重绘。

### 6.2 24 小时柱状图

1. 固定 24 个柱（`Snapshot.Hourly` 已经是长度 24、无数据补 0 的数组）。
2. 布局：横向均分内容区宽度，柱宽 = `可用宽 / 24 - gap`（`gap` 固定 4–6px），
   基线统一在图表底部，柱高 = `count / max * 图表可用高`；`max == 0` 时整图显示空状态，不画柱。
3. 每根柱：`FillRect` 填强调色（`count == 0` 用浅灰占位 1–2px 高，让「这小时没数据」可见）。
4. 刻度文字：横轴标注小时，空间够就每格标，不够就每 3 小时标一次（按 `GetTextExtentPoint32W`
   量出的宽度决定，不写死）；纵轴只在顶部标一次最大值，不画密网格线。
5. 顶部指标卡（总按键数 / 活跃时长 / 日均 / 峰值）用同一套 `DrawTextW` 右对齐排版，
   数字千分位格式在 `stats` 侧或界面侧统一一个 helper，别两处各写一遍。

---

## 7. 文件落点（`internal/ui`）

```
internal/ui/
  win32.go          // LazyDLL / 常量 / 结构体 / 通用包装（只放 Windows 侧）
  win32_stub.go     // //go:build !windows，保证 go test ./... 在别的平台也能编译
  app.go            // 窗口创建、WndProc、消息循环、SetTimer 刷新、页面分发
  layout.go         // 像素布局计算（纯函数，最容易用表驱动测试兜住）
  paint.go          // 双缓冲、字体、颜色、通用绘制 helper
  page_overview.go  // 概览：指标卡 + TOP 列表 + 24 小时柱状图
  page_heatmap.go   // 热力图：键位表、着色、命中测试
  page_settings.go  // 设置：开关、清空、导出、自启、关于
```

约束：

- 平台相关文件必须有 build tag 和 `!windows` stub，保证 `go test ./...` 在任何平台都能过（AGENTS.md 要求）。
- `layout.go` 的坐标计算、`page_heatmap.go` 的「次数 → 颜色」插值、命中测试等**纯函数**
  应该抽出来写表驱动测试；真正调 GDI 的绘制函数不测。
- UI 只依赖 `internal/service`，**不直接 import `internal/store` 或 `internal/sqlite`**（依赖方向单向）。
- 现有 `window_windows.go` 的 `FindWindowW` + `ShowWindow` 最小化补丁在原生窗口下**整体删除**，
  改用 `CreateWindowExW` 的 `wShowWindow`。

---

## 8. 无 OpenGL 这条硬约束 → 新依赖准入检查

**硬约束**：界面必须能在**没有 GPU 加速、没有可用 OpenGL / 显卡驱动**的机器上显示
（云桌面、RDP、虚拟机、只有基础显示驱动的服务器）。任何新依赖都必须先过下面这张表，
否则不引入：

| # | 检查项 | 判定标准 |
|---|---|---|
| 1 | GPU 图形栈 | 直接或**间接**依赖 OpenGL / Vulkan / Direct3D / GLFW / SDL / Skia / Cairo 的一律 ❌ |
| 2 | 间接依赖审查 | 必须 `go mod graph` 看过一眼，不能只看 `go.mod` 的 require 列表 |
| 3 | cgo | 优先选纯 Go；要求 cgo 的必须给出理由并等确认（默认 `CGO_ENABLED=0` 也要能构建） |
| 4 | 体积 | 新增依赖让单文件 exe 增量 > 5MB 的，必须单独论证 |
| 5 | 系统 DLL | 只允许 `user32` / `gdi32` / `kernel32` / `shell32` / `advapi32` / `comctl32` 这类系统自带 DLL |
| 6 | RDP 实测 | 在**无 GPU 的云桌面 / RDP 会话**里构建并起窗口，成功才算通过（截图或报错原文写进 devlog） |
| 7 | 同一进程不复活 GL | 已废弃的 Fyne / GLFW 依赖必须真正从 `go.mod` 移除，不能只「不再调用」 |

补充：

- 选型**已知安全**的方向：标准库、`golang.org/x/sys/windows`、`modernc.org/sqlite`（纯 Go）。
- 需要自绘控件时，**先写 GDI**，不要为了「控件丰富」重新引入 GUI 框架——那会把 OpenGL 依赖带回来。
- 每次引入新依赖，在 `docs/devlog.md` 记一条，写清版本、体积、上面 7 项的实测结论。

---

## 9. 分阶段落地与验证

| 阶段 | 内容 | 验证 |
|---|---|---|
| 1 | Win32 骨架：注册窗口类、`CreateWindowExW`、`WndProc`、消息循环、`WM_TIMER` 2000ms | 云桌面 / RDP 下能正常起窗口、拖动、缩放、关闭后进程退出 |
| 2 | 顶部工具栏 + 底部状态栏 + 页面切换（纯绘制，先放占位文本） | 点时间范围 / 切页不闪烁、不错位 |
| 3 | 设置页（采集开关、清空、导出、自启、关于） | 见 `docs/manual-test.md` 的设置页清单 |
| 4 | 概览页（指标卡 + TOP 10 + 24 小时柱状图） | 与 `metrics.md` 口径逐项对数（手算一组 fixture 对比） |
| 5 | 热力图页（键位表 + 着色 + 悬停） | 着色与按键 TOP 10 一致；无数据时中性色 |
| 6 | 摘掉 Fyne：删旧实现、`go mod tidy`、`CGO_ENABLED=0 go build` | 单 exe 能起、体积与依赖图确认无 GL |

每阶段都要：`go vet ./... && go test ./...` 全绿、`gofmt -l .` 无输出，
并在 `docs/devlog.md` 追加一条（做了什么 / 为什么 / 坑 / 测试状态 / 下一步）。

---

## 10. 风险与待确认项

1. **首次启动说明 + 系统通知**：Fyne 的 `App.SendNotification` 随 Fyne 一起消失。
   原生路线是 `MessageBoxW` 做说明对话框 + `Shell_NotifyIcon(NIM_ADD)` 气泡做通知，
   或者退化成「只有常驻状态条」。**该项需要确认后再实现**，不在本次重写中擅自定。
2. **文件对话框**：`SHBrowseForFolderW`（选目录）与 `GetSaveFileNameW`（选文件）都能用，
   但要确认导出语义到底是「选目录导两个文件」还是「选文件名导一个」，保持与 README 一致。
3. **DPI**：`assets/app.manifest` 必须声明 Per-Monitor DPI 感知，否则高分屏文字发虚、坐标算错。
4. **字体缺失**：`Microsoft YaHei UI` 在精简版 Windows 上可能没有，需要字体回退链
   （`Microsoft YaHei UI` → `Microsoft YaHei` → `SimSun` → 系统默认），否则中文变方块。
5. **`全部` 范围的聚合开销**：数据量大时 `Snapshot` 可能变慢，先靠「后台取快照 + `PostMessage`」
   兜住界面响应；真慢了再给聚合加索引或缓存，不在界面层硬扛。
