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

## 🔔 2026-11-03 · 托盘常驻（v0.4.0）

**🎯 做了什么**

- 新增 `internal/tray`：只放能测的纯逻辑——菜单项、ID→动作映射、悬停提示、气泡文案。
  真正调 `Shell_NotifyIconW` 的代码在 `internal/ui/tray_windows.go`。
- 关窗语义从「退出」改成「收进托盘」，于是**托盘成了唯一的入口**：显示、暂停、退出都在右键菜单里。
- 开机自启从「最小化显示」改成「不显示，直接待在托盘」，以免开机弹窗抢注意力。
- 退出时摘图标；托盘没放上时回退成「关窗即退出」，避免既没窗口也没图标。

**🕳️ 踩的坑**

1. **`WM_CLOSE` 不能再用作「退出」信号**。改数据目录的流程靠 `postMessage(WM_CLOSE)` 结束进程，
   语义一变它就变成「藏起来」，迁移永远等不到。现在统一走 `a.quit()`（直接 `DestroyWindow`）。
   教训：改一个系统消息的语义之前，先全仓搜它被谁用。
2. **模态弹框会绑架自动化**。首启欢迎框是 `MessageBox`，自带模态循环：外部投递
   `WM_COMMAND(IDOK)` 关不掉它（试过），`BM_CLICK` 点按钮也没成功。最后改成
   **直接往数据库写 `meta.onboarded=1`** 来跳过，并把欢迎框降级为手工验证项。
   顺带发现：只要弹框没被关掉，`MarkOnboarded` 就不会执行——这是正确行为，不是 bug。
3. **跨进程 `Shell_NotifyIconW(NIM_MODIFY)` 可以当探针**：对已注册的图标返回 TRUE，
   对不存在的窗口返回 FALSE。加一个「不存在的窗口」做对照组，结论才有鉴别力
   （只测真窗口会把「总是返回 TRUE」误判成注册成功）。
4. **ctypes 传 HWND 必须显式 `argtypes`**。不声明时句柄按 32 位 C int 传，
   高位被截断，`IsWindowVisible` 查到了另一个窗口，于是出现「窗口明明可见却报 False」的假象。
   这一条让我多排查了十几分钟。
5. **别在测试里用陈旧的可执行文件**。有一轮 `hide` 反复失败，最后发现跑的是上一次构建的 exe；
   用探针（往 `%TEMP%` 写一行）确认 `hideToTray` 到底跑没跑，才定位到是产物过期。

**✅ 测试状态**

- 单测：`internal/tray` 6 个用例（菜单文案/顺序、ID 唯一与稳定、状态切换打勾、动作映射、两种气泡文案）。
- 运行时（脚本 `bin/_testdata/tray_check.py`，连跑两轮全过）：

| 场景 | 结果 |
|---|---|
| `Shell_NotifyIconW` 注册 | ✅ 探针 TRUE，对照窗口 FALSE |
| 发 `WM_CLOSE` | ✅ 进程存活、窗口不可见（证明托盘图标当时确实在） |
| 发托盘左键回调 `msgTray/wmLButtonUp` | ✅ 窗口重新出现 |
| 隐藏状态下启动第二个实例 | ✅ 退出码 0、进程数 1、窗口被唤醒 |
| `-autostart` 启动 | ✅ 无窗口、无弹框、图标已注册 |

**📌 无法自动化的部分（写进 manual-test 9.7）**：右键菜单的实际点击、
气泡提示是否真的弹出、托盘图标在溢出区里是否可见——都需要人眼确认。

**🔁 与升级的冲突（这一条最要紧）**

托盘常驻之后，`WM_CLOSE` 不再是退出信号。而安装器的 Restart Manager 正是靠
`WM_CLOSE` 让目标程序退场的 —— 于是出现「程序收到关闭请求却只是收进托盘，
安装器干等三十秒再把进程强杀」：实测升级耗时 **32 秒**，而且强杀会丢掉内存里
还没落库的计数（最多 30 秒的量）。

解决办法是加一条**双方约定的注册消息**（`RegisterWindowMessage`）：

- 程序启动时 `RegisterWindowMessageW("KeyboardStatistics.QuitForUpgrade")`，收到即 `quit()`，
  走正常退出流程（`svc.Close()` 会把缓存刷进数据库）；
- 安装器在 `PrepareToInstall` 校验通过之后发这条消息，最多等 5 秒，然后交给 RM 兜底。

实测：握手 **0.10 秒**退出；带托盘的完整升级 **4.9 秒**（原 32 秒）：
`exe 7,889,408 → 7,885,824`、便携标记与数据库都在、默认目录没被创建、卸载键仍 1 个。

> 两侧字符串必须逐字一致（`internal/tray.QuitMessageName` ↔ `installer.iss` 的 `QuitForUpgradeMsg`），
> 所以 `internal/tray` 里有个测试专门盯这个常量，改了会红。

**➡️ 下一步**：托盘菜单加「打开数据目录」；托盘图标叠加未读/暂停角标；安装包里可选不创建托盘图标。

---

## 🔒 2026-11-03 · 单实例、升级只能原地、升级时自动关进程（v0.3.0）

**🎯 做了什么**（三条需求，两条要改代码）：

1. **单实例**：新包 `internal/single`，用 `CreateMutexW` 抢命名互斥体
   （`KeyboardStatistics_SingleInstance`）。第二个实例把已有窗口拉到前台后安静退出；
   `-autostart` 拉起时只退出、不抢焦点。
2. **升级只允许原地**：`scripts/installer.iss` 里检测卸载注册表键拿原目录，
   升级时用 `ShouldSkipPage` 隐藏「选择安装目录」页，再用 `PrepareToInstall` 兜底：
   命令行显式 `/DIR` 指向别处就**直接拒绝安装**。
3. **升级前自动关掉正在运行的实例**：`CloseApplications=force` + Restart Manager。

**🕳️ 踩的坑（这一轮的坑比功能本身值钱）**：

1. **`AppId` 少写一个花括号 = 变成另一个软件**。我为了在 `[Code]` 里拼卸载键名，
   把写死的 `AppId={{A1C3...}` 改成 `AppId={{{#MyAppId}`——ISPP 展开后变成
   `{A1C3E5F7-...-0C1D2E3F4A5B`（**缺右花括号**），于是 0.3.0 的 AppId 与 0.2.0 不同，
   `UsePreviousAppDir` 自然认不出已安装 → 装到默认目录、留下两个卸载键。
   症状极易误判成「`[Code]` 破坏了升级」。正确写法是 `AppId={{{#MyAppId}}`
   （`{{` + 展开后的 GUID + `}`）。**验证手段**：装完后看卸载键名是不是
   `{GUID}_is1`，少了 `}` 立刻能发现。
2. **静默模式下向导页根本不存在**：第一版守卫用 `WizardDirValue` 比较目录，
   结果一次正常升级被冤枉成「换目录」，`exit 7` 卡死。改用 `{param:DIR|}` 只看命令行。
   教训：`PrepareToInstall` 里读任何 `WizardForm*` 都是不可靠的。
3. **声明 `AppMutex` 会挡住静默升级**：装上之后安装器启动就弹
   「Setup has detected that ... is currently running」，静默模式默认按 Cancel → `exit 1`，
   `/CLOSEAPPLICATIONS` 也救不了（那个开关只管 Restart Manager）。
   我们要的是「安装器替用户关掉」，所以**不声明 AppMutex**，改用 `CloseApplications=force`。
4. **`GetLongName` 在 Inno Pascal Script 里不存在**（编译报 `Unknown identifier`）。
   长短路径归一只能自己调 `GetLongPathNameW@kernel32.dll`。
5. **测试夹具自己坑自己**：我往 `data\stats.db` 写了个 11 字节的纯文本假库，
   程序启动后正确地拒绝打开并弹窗退出，被误读成「程序启动就崩」。测数据保留要用真库。
6. 又一次从**工作区内**直接运行安装包 → 沙箱 Low IL 让它挂住 10 分钟。
   规矩：**任何 exe/setup 先拷到 `%TEMP%` 再跑**，已写进 manual-test 9.7。

**✅ 实测矩阵**（0.3.0 装好 → 启动 → 0.3.1 静默升级，安装包都在工作区外）：

| 场景 | 结果 |
|---|---|
| 第二次启动 | 第二个进程退出码 0，存活实例数保持 1，日志 `已有实例在运行，本次启动退出` |
| 静默升级，不给 `/DIR` | ✅ 装回原目录，默认目录没被创建 |
| 静默升级，`/DIR` = 同一目录（长名形式） | ✅ 放行（`GetLongPathNameW` 归一后比较） |
| 静默升级，`/DIR` = 别处 | ✅ `exit 7` 拒绝，新目录未被创建，卸载键仍只有 1 个 |
| **程序运行中**静默升级 0.3.0→0.3.1 | ✅ RM 查到 `Keyboard usage statistics` 并自动关闭，exe 7,876,608→7,873,024，无重启提示 |
| 升级后数据 | ✅ `portable.txt` 与 `data\stats.db` 保留 |

**📌 已知边界**：交互式升级会走 `ShouldSkipPage` 隐藏目录页（无法自动化验证，需人工点一次）；
静默升级不会自动重启程序（`[Run]` 带 `skipifsilent`），交互式最后一页有「立即启动」可勾。

**➡️ 下一步**：托盘常驻；安装向导中文化；多实例互斥已经解决，但同一份库被两台不同用户共享的情况还没考虑。

---

## ✅ 2026-11-03 · 覆盖安装 / 运行中升级实测（含一次误判的复盘）

**🎯 做了什么**：把「用户已经装了旧版、又拿到新版安装包」这条路彻底测了一遍，覆盖七种情形。

**实测结果**：

| 场景 | 结果 |
|---|---|
| 不给 `/DIR` 覆盖安装 | ✅ 装回原目录，不再弹默认目录、`%LOCALAPPDATA%\Programs\` 下没多出一套 |
| 二进制是否真被替换 | ✅ 把已装 exe 换成 24 字节占位文件，升级后恢复成 7,871,488 字节 |
| 上次勾了便携模式 | ✅ `portable.txt` 保留（`UsePreviousTasks`），`data\` 不动 |
| 上次**没**勾便携模式 | ✅ 升级后依然**没有** `portable.txt`，也没凭空建 `data\`（没把用户偏好改掉） |
| 卸载项 | ✅ 只有一个，`DisplayName` 版本号随之更新 |
| **程序正在运行时升级** | ✅ Restart Manager 认出 `Keyboard usage statistics` 并关闭它，文件替换成功，`Need to restart Windows? No` |
| 数据在升级过程中的保全 | ✅ 便携模式 `data\` 完整保留 |

**🕳️ 踩的坑（这条最值得记）**：

第一次测「运行中升级」时结论是「升级成功但二进制没换」，差点当成产品 bug 去改脚本。看日志才发现：

```
Directory for uninstall files: ...\ksu\appB
```

我先后把 0.2.0 装到了 `appA` 和 `appB` —— **同一个 AppId 装两处，注册表里只保留最后一个**，
所以那次升级正确地装到了 `appB`，而我启动的是 `appA` 的程序。**是我的测试环境互相污染了，
不是安装包的问题。** 教训：测升级前必须先清干净（卸载项 + 两个目录 + 快捷方式），
否则「上一次安装位置」根本不是你以为的那个。

另外两个真结论：

1. **同版本覆盖安装看不出问题**：Inno 发现源文件与目标文件一致（版本号 + 时间戳 + 大小）会**跳过复制**，
   退出码照样是 0。所以"升级测试"必须用**真正不同的版本**，或者先把目标文件改坏当探针——
   只看 exit code 会得到假阳性。
2. **静默升级不会自动重启程序**：`[Run]` 那条带 `skipifsilent`，静默模式下装完程序是关着的。
   交互式安装有「立即启动」勾选项。已在 README 里写明。

**❓ 另一个已知边界**：已经装了一套、**不卸载**又装到别的目录时，注册表只认最后一套，
先前那套的文件成为孤儿目录。这是 Inno 的固有行为，文档里不承诺「多套并存」。

**✅ 测试状态**：以上七种场景全部用 `/VERYSILENT /SUPPRESSMSGBOXES /LOG` 实跑；
版本对比用 0.2.0 → 临时构建的 0.2.1（测完删除，`scripts\_upgrade_probe.iss` 也已清理）。
`go test ./...` 全绿，工作区无残留。

**➡️ 下一步**：多实例互斥（同一份库被两个进程打开）、安装向导中文化、托盘常驻。

---

## 🔧 2026-11-03 · 安装包（Inno Setup）：自选安装目录 + 便携模式，v0.2.0 发版

**🎯 做了什么**：

1. `scripts/installer.iss`：Inno Setup 6 脚本，`PrivilegesRequired=lowest`（按用户安装、不弹 UAC），
   `DisableDirPage=no`（强制显示目录选择页）、`AllowRootDirectory=yes`（允许装到 `D:\` 根）。
2. `scripts/portable.txt`：便携标记模板，随「便携模式」任务装进 `{app}`。
3. 卸载逻辑：`{app}\data` 存在才动，**静默卸载一律保留数据**，交互时 `MB_DEFBUTTON2` 默认「否」。
4. 产物：`dist\keyboard-statistics-v0.2.0.exe`（7.51 MB）+
   `dist\keyboard-statistics-0.2.0-setup.exe`（4.49 MB）。

**💡 为什么**：

- 主程序是 `asInvoker`，装进 `C:\Program Files` 就写不了数据。所以安装包**按用户安装**，
  目录默认 `{localappdata}\Programs\KeyboardStatistics`，但允许用户改成 D 盘——用户可写，数据就放得下。
- 主程序路径故意**不用通配**：`dist\` 里同时有历史和当前产物，通配还会把 `*-setup.exe` 自己打包进去。
  改成 `#define` + 编译期 `FileExists` 断言，缺产物直接编译失败。
- 不勾便携模式时**不去删**已有的 `portable.txt`：静默删标记等于把用户数据源悄悄切走
  （数据还在 `data\` 里，程序却不读了），比留一个多余文件危险得多。

**🕳️ 踩的坑**：

1. **DSH 沙箱把工作区内的可执行文件以 Low Integrity Level 运行**（实测：工作区内 IL=4096，
   `%TEMP%` 下 IL=12288）。后果是：直接在 `dist\` 里跑安装包会在写 HKCU 卸载项时
   `exit 4 / 拒绝访问`，看起来像脚本 bug，其实是沙箱。**要测就先把 exe/setup 拷到工作区外**。
   同理，仓库内的 exe 手动验「能不能写 `%LOCALAPPDATA%`」也是失真的。
2. winget 装的 Inno Setup **不在 `Program Files`**，而在 `%LOCALAPPDATA%\Programs\Inno Setup 6\`——
   按用户级安装。找 `ISCC.exe` 要按这个路径找。
3. Inno Setup 6.7.3 官方**不带** `ChineseSimplified.isl`，安装向导是英文。
   没有自行下载第三方翻译文件（来源不可控），在脚本里留了「放好 .isl 后加一行即可」的注释。
4. ISPP 的 `#error` **只吃字面量、不做字符串拼接**，所以断言语句里的版本号得手写，
   改 `MyAppVersion` 时必须顺手改那一行——已在注释里标注。

**✅ 测试状态**：

- ISCC 编译 exit 0；安装包版本资源 0.2.0.0
- 静默安装 5 种组合（带/不带便携任务、带/不带桌面图标、不给 `/DIR` 用默认值）全部 exit 0
- **完整用户故事实测**（把 setup 拷到 `%TEMP%` 外跑）：静默安装勾便携 → `{app}\portable.txt` 存在 →
  启动后数据写在 `{app}\data\`（`stats.db` + `app.log`）→ 假 `%LOCALAPPDATA%` 目录 **0 个文件** →
  关窗零残留；静默卸载后 exe 与 `portable.txt` 删除、`data\` 保留
- `go test ./... -count=1` 全绿；`go vet` 干净；`gofmt -l .` 无输出

**➡️ 下一步**：安装向导中文化（等可信的 `.isl`）；覆盖安装/升级路径（`UsePreviousAppDir/Tasks`）还没专门验；
多实例互斥；托盘常驻。

---

## ✨ 2026-11-03 · 数据目录可配置：便携模式、自定义位置与启动期迁移

**🎯 做了什么**：

1. **数据目录三级解析**（`config.ResolveDataDir`）：用户指定（`HKCU\Software\keyboard-statistics\DataDir`）
   > 便携模式（exe 旁有 `portable.txt` 且**实测可写**）> `%LOCALAPPDATA%\keyboard-statistics`。
2. **`EnsureWritable`**：不只 `MkdirAll`，真去建一个临时文件再删——「目录存在但没有写权限」是最常见的坑。
3. **迁移**（`MigrateData` / `MigrateIfNeeded`）：`stats.db` / `-wal` / `-shm` / `app.log` 一起搬；
   目标已有数据库时拒绝，绝不覆盖。
4. **`main` 启动早期迁移**：此时数据库尚未打开，没有文件句柄占用。
5. **设置页**：显示真实数据目录 + 存放方式（便携模式 / 用户目录 / 自定义）；
   新增「更改数据目录…」，只写设置并退出，真正的搬迁留给下次启动。
6. **顺手修了个真 bug**：数据目录不可写时原本是 `os.Exit(1)` 静默退出，`-H windowsgui` 下没有控制台，
   用户看到的就是「双击没反应」。现在改为 `ui.ShowFatal` 弹窗说明路径与原因。

**💡 为什么**：

- 安装包要能自选安装目录，用户又希望数据跟着程序走；但程序是 `asInvoker`，
  装进 `C:\Program Files` 后普通用户对该目录**没有写权限**，带 manifest 的程序也不会触发 UAC 虚拟化，
  就是硬生生的 Access Denied。所以便携模式必须「探测 + 回退」，不能默认信任那个标记文件。
- 迁移放在**启动早期**而不是运行中：SQLite 持有文件句柄时 Windows 不允许移动文件，
  先 `svc.Close()` 再搬会让界面停在「服务已关闭」的半死状态——刷新定时器还在跑，只能一直报错。
- 「更改目录」只写设置不搬文件，是因为用户随时可能在采集过程中点它；
  把有副作用的动作推迟到下次启动，是最省事也最不容易出错的顺序。

**🕳️ 踩的坑**：

1. `registry.CreateKey` 返回 **三个** 值 `(key, existed, err)`，不是两个——编译期就撞上了。
2. `MB_DEFBUTTON1` 的值是 `0x0000`，项目里之前只定义过 `mbDefButton2`（0x0100）；
   写「默认按钮放在否」时才发现「默认放在是」也需要一个常量。
3. **PowerShell 每次调用都是新进程**，上一次 `Add-Type` 定义的类型不会保留；
   验证窗口的脚本必须自带 `Add-Type`，否则「找不到类型 [WV]」会让你误以为窗口没出现。
4. 便携模式验证**必须把 `LOCALAPPDATA` 指向假目录**，否则程序回退写入用户目录你也看不出来，
   会误判成「便携模式生效了」。
5. 早先某次验证里 `meta` 表只有 `schema_version`，我据此判断「说明框没弹」——结论错了。
   模态框弹着时 UI 线程被阻塞，但**钩子在独立线程上照样采集**，
   所以「库里有按键数据」根本不能证明界面正常。

**✅ 测试状态**：

- `internal/config` 新增 15 个用例（优先级、空白设置、读设置失败回退、可写性、迁移、防覆盖、
  `SetDataDir` 往返）
- `go test ./... -count=1` 全绿；`go vet` 干净；`gofmt -l .` 无输出
- 真机实测：便携模式数据落在 `<exe>\data`，假 `LOCALAPPDATA` 目录 **0 个文件**；关窗进程归零
- 真机实测：`LOCALAPPDATA` 指向非法路径时弹出「数据目录无法写入」错误框，关闭后进程退出、无残留

**➡️ 下一步**：安装包（Inno Setup，并行子代理）；多实例互斥锁（同一份库被两个进程打开会打架）；
托盘常驻；链式快捷键仍不做。

---

## ✨ 2026-11-02 · 概览页 / 热力图页落地 + 首次启动说明 + 端到端验证

**🎯 做了什么**：

1. **概览页**：四张指标卡（总按键 / 活跃时长 / 日均 / 峰值）、按键与快捷键 TOP 10（两套占比分母独立）、
   24 小时时段柱状图、空状态与「有按键无组合」的差异化提示。布局全部走纯函数，22 个表驱动测试。
2. **热力图页**：5 行 QWERTY 键位（`unit = min(宽/15, 高/5)` 等比缩放居中）、
   `t = sqrt(count/max)` 着色、悬停 2px 描边 + 图下行「键名 · 次数 · 占比」、空状态全灰。13 个纯函数测试。
3. **采集入口归一化左右修饰键**：`collect.KeyDown/KeyUp` 一进来就 `stats.NormalizeVK`，4 个新用例。
4. **首次启动说明框**：`MessageBoxW` 弹一次，含「已经在采、采什么、怎么停」三句，确认后写 `meta.onboarded`；
   文案本身有测试盯着（少一句就红）。
5. **端到端验证**：真实钩子注入 `A` / `Ctrl+C` / `←`，库里落成 `A=1 C=1 Ctrl=1 ←=1`、`Ctrl+C=1`、
   `day_activity` 记到首末按键时间；关窗后进程归零。

**💡 为什么**：

- 界面用「页面 = 纯绘制函数 `paintXxx(cv canvas, content Rect)`」，把布局/着色/命中抽成无 build tag 的纯函数，
  才能在无 GUI 环境下跑单元测试——AGENTS.md 要求「UI 不做自动化测试，但 GDI 纯函数必须表驱动」。
- 归一化放在**采集入口**而不是展示层：否则按键计数、TOP 榜、热力图、组合判定会各算一套，
  出现「榜单两行 Shift、热力图 Shift 恒为 0」这种自相矛盾（这次真的踩到了）。
- 说明框用 `MessageBoxW` 而不是托盘气泡：不为一次性提示引入托盘依赖。

**🕳️ 踩的坑**：

1. **归一化写了但没接线**：`NormalizeVK` 有实现有测试，却没人调用。症状是热力图 Shift 永远是 0、
   TOP 榜出现两行 Shift。教训：新增的纯函数一定要在实现里搜一遍调用点，测试全绿不代表它被用上了。
2. **我的测试把口径写反了**：断言「Shift+A 是组合键」，但 `docs/metrics.md` 明确 Shift + 字符键不算组合
   （那是打大写字母）。错的是测试不是实现——先改测试再说，**规格文档是唯一标准**。
3. **`MessageBoxW` 是模态的**：说明框弹着时主窗口收不到 `WM_CLOSE`，自动化脚本里会看到「进程残留」。
   必须先 `WM_CLOSE` 关掉说明框，再关主窗口；`WM_COMMAND(IDOK)` 在这个场景下不生效。
4. **`FindWindowW` 找不到窗口不说明没窗口**：用 `EnumWindows` 按 PID 枚举才发现说明框 0.9 秒就在了。
   排查窗口问题一律先枚举该进程的所有顶层窗口，别只信一次 `FindWindow`。
5. **PS 5.1 的 `Get-Content` 会把 UTF-8 显示成乱码**：看着像文件被写坏，实际用 UTF-8 校验器一过是 0 个坏文件。
   判断编码只能靠校验器，不能靠控制台输出。

**✅ 测试状态**：

- `go test ./... -count=1` 全绿（collect / config / service / stats / store / ui）
- `go vet ./...` 干净，`gofmt -l .` 无输出
- 全仓库 61 个文件 UTF-8 校验通过（0 个坏文件）
- 真机像素采样：卡片白底、选中态 `#2F6FEB`、热力图渐变 `A→S→D→Q` 逐级变浅、悬停白边→移出复原
- 端到端：`Ctrl+C=1` 组合键落库成功，说明框确认后 `meta.onboarded=1`

**➡️ 下一步**：出 v0.1.0 正式包并在云桌面/远程桌面复跑一遍；托盘常驻（v0.2）、
链式快捷键与按应用统计暂不做；键盘可达性（Tab 聚焦）需要先定自绘状态机。

---

## ✨ 2026-11-02 · 原生 Win32 界面落地：裸 syscall + GDI 自绘，Fyne 彻底移除

**🎯 做了什么**：

1. **先证明窗口能出来，再动界面代码**。`github.com/lxn/walk` 实测**建不出任何窗口**：
   `walk.NewMainWindow()` 在 `ToolTip.addTool` → `TTM_ADDTOOL` 处失败，空窗口、只有一个 Label、
   只有一个 PushButton 三种最小 demo 全部同样报错——每个 walk 控件在 init 时都会注册 tooltip，
   所以这不是用法问题，是这条路走不通。`Get-Process | Select-Object MainWindowTitle` 对
   notepad 能读出「无标题 - 记事本」，证明测量方法本身没问题，失败是 walk 特有的。
   结论：**不引任何 GUI 框架**，`syscall` 直接调 `user32` / `gdi32`，GDI 自绘。
   （原任务书里「walk 首选、windigo 兜底」两句随之作废，见 `docs/ui-plan.md` §1。）
2. `internal/ui` 整层重写，旧 Fyne 文件全删；`go.mod` 里 `fyne.io/fyne/v2` 及约 40 个间接依赖
   被 `go mod tidy` 清干净，现在只剩 `modernc.org/sqlite` + `golang.org/x/sys` 两系，
   `CGO_ENABLED=0` 直接构建通过。
3. 分层：**能脱离 Win32 的逻辑全部拆成纯函数**，独占单测。
   - `layout.go`：`Rect`（字段顺序与 Win32 `RECT` 一致）、`ComputeLayout`（工具栏/标签栏/状态栏/
     时间范围按钮）、`ComputeSettingsLayout`（设置页每个可点矩形）、`HitScope`/`HitTab`/`Hit` 命中测试、
     `px()` 按 DPI 缩放。绘制和点击**共用同一份布局**，不会出现「看着在这、点了没反应」。
   - `state.go`：`autostartAction`（`updating` 标志 + 勾选态 → 启用/停用/不动）、`collectButtonText`、
     `statusText`、`staleWarningText`、`rangeSummary`、`aboutLines`。
   - `win32.go`：约 50 个 `NewLazyDLL` + `NewProc`，结构体 `wndClassEx`/`msg`/`paintStruct`/
     `minMaxInfo`/`browseInfoW` 等，以及双缓冲绘制、DPI 查询、选目录框、`ShellExecuteW`。
   - `app_windows.go`：窗口生命周期 + 消息循环 + 状态机；`paint.go` + `page_*.go`：自绘。
   - `app_stub.go`（`!windows`）：保留同样的 `Options` / `App` / `New` / `Run`，让 `go test ./...` 在任何平台都能过。
4. 对外 API 没变：`ui.New(ui.Options{Version, ExePath, Autostart, StartMinimized}, svc)` + `app.Run()`，
   `cmd/app/main.go` 一行没改，`Run()` 关窗返回后 main 自己 `svc.Close()`。
5. 窗口：标题「键盘使用统计」，客户区 900×680、最小 800×600（`WM_GETMINMAXINFO`），启动居中，
   2 秒定时器（`WM_TIMER`）触发刷新；取数在后台 goroutine 里跑 `svc.Snapshot(scope)`，
   完成后 `PostMessage(msgSnapshotReady)` 回窗口线程——**UI 线程绝不等 SQLite**。
   切换时间范围时，回来的旧数据按 `scope` 比对后丢弃重取，不会画错。
6. 设置页本轮做完整（四个动作 + 关于）：
   采集开关（`svc.Collecting/SetCollecting`）、开机自启（`config.NewAutostart`，nil 时复选框禁用；
   `Stale()` 时显示「⚠ 自启路径已失效，请取消后重新勾选」）、导出 CSV（选目录 → `svc.ExportCSV(dir)` →
   弹窗列出两个文件全路径）、清空全部数据（二次确认，默认按钮在「否」）、打开数据目录、
   关于区（版本 / 数据目录 / 隐私声明 + 设置页写明 UAC 提权窗口收不到按键）。
   概览页与热力图页仍是占位文案，留给下一轮。

**💡 为什么**：

- 云桌面 / 远程桌面没有可用 OpenGL，Fyne 的老路已堵死（见下一条日志）；GDI 只依赖 `gdi32.dll`，
  单 exe、无运行库、无 GPU 加速也能画，是唯一能保证「拷到别的机器双击就出窗口」的路子。
- 布局做成纯函数不只是为了测试：GDI 自绘**没有控件帮忙记热点区域**，「画在哪」和「点到哪」
  必须算自洽，抽出来之后两边共用一份结果，改一处不会漏另一处。
- 自绘按钮/复选框就没有「程序化勾选会触发回调」的问题，但 `updating` 标志仍然保留：
  将来若换真控件，`syncSettingsState` 期间不会反过来写注册表。
- 2 秒刷新 + 后台取数：`WM_TIMER` 里直接查库会让窗口在大范围聚合时卡住；
  再加一个 `fetching` 闩，上一次没回来就跳过本次 tick，慢查询不会堆积。

**🕳️ 踩的坑**（后来人直接抄结论）：

1. **双缓冲顺序坑（这次真踩了，整窗空白）**：`BitBlt` 必须在**位图仍被 `SelectObject` 选中**时做。
   先 `SelectObject(memDC, old)` 还原旧位图、再 `BitBlt`，拷到的就是那块 1×1 占位位图，
   窗口整片纯白，但 `WM_PAINT` 明明在跑、客户区尺寸也对，非常像「根本没重绘」。
   正确顺序：画 → `BitBlt` → `SelectObject(old)` → `DeleteObject(bmp)` → `DeleteDC`。
2. **断言「界面画出来了」不能只看截图**：本机 `CopyFromScreen` 与 `PrintWindow(PW_RENDERFULLCONTENT)`
   对未实现 `WM_PRINTCLIENT` 的自绘窗口都返回整片纯白，看起来像「什么都没画」。
   可靠办法是 `GetDC(hwnd)` + `GetPixel` 直接读窗口 DC：能读到 `0x00F5F5F5` 内容区、
   `0x00EEEEEE` 状态栏、选中项 `0x002F6FEB` 才算真的画上了。另外注意 ClearType 会让文字边缘
   出现彩色（采样时避开文字，取按钮离字 20px 以上的位置）。
3. **`go vet` 会拦 `uintptr → unsafe.Pointer`**（`possible misuse of unsafe.Pointer`）：
   - `WM_GETMINMAXINFO` 的 `lParam` 直接声明成 `unsafe.Pointer`（`syscall.NewCallback` 接受
     `unsafe.Pointer` 参数，尺寸等于指针即可），再 `(*minMaxInfo)(lParam)`，连转换都不用写。
   - 结构体字段里的字符串指针用 `utf16Str(s) *uint16`，不要 `(*uint16)(unsafe.Pointer(utf16Ptr(s)))`。
4. **测试环境里 `%TEMP%` 下建目录会被拒**：沙箱化启动的子进程 `MkdirAll(%TEMP%\xxx\keyboard-statistics)`
   报 `Access is denied`，程序在 `config.EnsureDir` 就 `fatal` 退出，且因为 `logger == nil` **一声不响**
   （exit code 1、无输出、无日志），很容易误判成「窗口没建出来」。本地验证时把 `LOCALAPPDATA`
   指到仓库内的 `bin\_testdata` 这类可写目录即可（`bin/` 已在 `.gitignore`）。
5. `CreateFontW` 的 `lfHeight` 要取负数，且 `-20` 这种常量直接传 `uintptr` 会报
   `constant -20 overflows uintptr`：先赋给 `int32` 变量再转。
6. 取 `GOPATH` 别用 `$env:GOPATH`（本机是空的、被 `D:\` 顶掉），用 `go env GOMODCACHE`。

**✅ 测试状态**：

- 纯函数层**先写测试后写实现**：`layout_test.go`（`Rect` 语义、五种尺寸下的布局关系、命中测试、
  设置页矩形不重叠且自上而下有序、2× 缩放正好翻倍）、`state_test.go`（按钮文案/状态文案/
  自启动作三态/失效提示/时间范围摘要/关于四行）。
  `go test ./internal/ui` 全绿。
- `gofmt -l .` 无输出；`go vet ./...`、`go test ./...` 全绿；`CGO_ENABLED=0 go build ./cmd/app` 通过，
  产物 7.4MB 单 exe。
- 手工实测（release 构建，`LOCALAPPDATA` 指到可写目录）：
  `Get-Process | Select-Object MainWindowTitle` → **`键盘使用统计`**，窗口 916×719（客户区 900×680）；
  `GetDC`+`GetPixel` 抽查工具栏白、选中时间范围 `#2F6FEB`、内容区 `#F5F5F5`、状态栏 `#EEEEEE`；
  点击「近 7 天」选中态跟着走，点「设置」标签下划线跟着走；点「暂停采集」按钮由白变主色，
  再用 `service.New` 读同一个库确认 `Collecting() = false`（界面 → 服务 → 落库整条链路通）；
  `WM_CLOSE` 后进程 5 秒内退出、`Get-Process` 无残留；`--autostart` 启动 `IsIconic = true`（最小化）。
- 本机沙箱拿不到注册表（`Access is denied`），正好走了「自启不可用 → 复选框禁用」的兜底分支，
  日志里是一条 WARN，不崩。

**➡️ 下一步**：

- 概览页（总按键数/活跃时长/日均/峰值/TOP 10/24 小时柱状图）与热力图页（QWERTY 键位 + 着色 + 悬停）实现。
- `docs/ui-plan.md` §10 里还欠的：首次启动引导对话框 + 系统通知气泡（§10.1，需产品确认）、
  文件对话框语义（§10.2）、应用清单的 per-monitor DPI（§10.3，`assets/app.manifest` 已声明，
  但运行时跨显示器迁移不重排）、字体回退链（§10.4）。
- 键盘可达性（Tab 聚焦、空格/回车触发）目前没做：自绘界面没有焦点概念，接入前先想清楚状态机。

---

## 🛠️ 2026-11-02 · 打包：应用清单 + 版本资源进 exe，单文件 7.4MB

**🎯 做了什么**：

1. `scripts/build.ps1`：`-Version <ver>`（默认 `0.1.0-dev`），产出 `dist/keyboard-statistics-v<ver>.exe`；
   构建前先删同名旧产物，可重复执行。核心命令保持一条：
   `go build -trimpath -ldflags "-s -w -H windowsgui -X main.version=<ver>" -o <产物> ./cmd/app`。
2. `assets/app.manifest`：`asInvoker`（不请求管理员）、DPI `PerMonitorV2` + `true/pm` 兼容写法、
   声明 Win10/11 兼容（`supportedOS` GUID）、Common-Controls v6。
3. `assets/winres.json`：把上面这份 manifest 和版本信息（FileVersion / ProductName / FileDescription，
   CompanyName 与 LegalCopyright 留空）编成 `cmd/app/rsrc_windows_amd64.syso`，由 `go build` 自动链进 exe；
   syso 是构建输入，脚本在 `finally` 里删掉，`.gitignore` 再补 `*.syso` 兜底。
4. 实测产物 `keyboard-statistics-v0.1.0-test.exe`：**7,759,872 字节（7.40 MB）**，
   PE 头 `Subsystem = 2 (WINDOWS_GUI)`，双击不弹黑框控制台。
5. 版本号与 manifest 做了**三重验证**（详见「测试状态」），其中前两重脚本每次构建都会自己跑。

**💡 为什么**：

- 「单文件 exe、免安装、双击即用」是第一分发约束：manifest 和版本资源必须**编进 exe**，
  不能靠外置 `.manifest` 文件，也不该依赖安装包去写注册表。
- **CGO_ENABLED=0 探测**：脚本先试纯 Go 构建，失败才退回默认 cgo。界面换原生 Win32 后这条路已经通了：
  产物 7.4MB，依赖只剩 `modernc.org/sqlite` / `golang.org/x/sys` 一系，连 MSYS2 的 gcc 都不用装。
  但脚本**不写死**这个结论——每次运行实时探测，将来依赖变了也能自动判断。
- go-winres 用 **`make` 而不是 `simply`**：`simply` 只能选它内置的 `cli`/`gui`/`none` 清单，塞不了自定义
  `app.manifest`；只有 `make` + `winres.json` 能把外置 manifest 编进去
  （`winres.json` 里的资源路径相对于该 json 所在目录，所以写 `app.manifest` 就够了）。
- 不往 `go.mod` / `go.sum` 加依赖：go-winres 用 `go run github.com/tc-hib/go-winres@v0.3.3` 跑，用完即走，
  构建工具不该变成项目依赖。

**🕳️ 踩的坑**（后来人直接抄结论）：

1. **本机只有 Windows PowerShell 5.1，没有 `pwsh`**（`where pwsh` 找不到）。
   PS 5.1 对**无 BOM** 的 `.ps1` 按 ANSI/GBK 解码，第一版带中文注释的脚本直接被解出 **15 个语法报错**
   —— UTF-8 中文的尾字节会让解码错位，把后面的 ASCII 字符（如 `}`）一起吞掉。
   项目规定 `.ps1` 必须 UTF-8 无 BOM，唯一安全交集就是**纯 ASCII**：脚本注释与输出全用英文，
   保持 0 个非 ASCII 字节（PS 5.1 与 PS 7 都能跑）。**想放中文注释就必须加 BOM，那会违反
   `.editorconfig`，别开这个口子。**
2. **在本仓库路径下直接跑 `.ps1` 会被执行策略拦住**：报 "is not digitally signed"，
   但同样内容拷到 `%TEMP%` 却能跑，且 `Get-ExecutionPolicy` 显示 RemoteSigned、文件也没有 Zone.Identifier 流。
   本机统一用 `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build.ps1 -Version x.y.z`；
   用户机器上正常的 RemoteSigned 不受影响，README 里写 `pwsh -File ...` 即可。
3. **Fyne 时期 `CGO_ENABLED=0` 必然失败**，真实报错：
   `imports github.com/go-gl/gl/v2.1/gl: build constraints exclude all Go files in …/go-gl/gl@…/v2.1/gl`
   —— OpenGL 驱动是 cgo-only，跟「sqlite 是纯 Go」无关。换成原生 Win32 后这条约束才消失。
4. **`go-winres --file-version` 只吃纯数字**（`1.2.3` / `1.2.3.4`），`0.1.0-test` 直接传会失败；
   脚本先把 `0.1.0-test` 规整成 `0.1.0.0`（截掉 prerelease、补齐四段）再传。
5. GitHub 直连不通不影响：`go run 模块@版本` 走 `GOPROXY`，`goproxy.cn` 实测可用，版本钉在 v0.3.3。

**✅ 测试状态**：

- 脚本实跑三次（`-Version 0.1.0-test` 连跑两遍 + 一次不带参数走默认 `0.1.0-dev`），每次 exit 0；
  两次同版本构建的产物字节数完全一致（7,759,872），`cmd/app/*.syso` 无残留，可重复执行成立。
- ① **产物字节搜**：GUI 版没有控制台，`--version` 看不到输出，改为在 exe 字节里搜 `0.1.0-test`
  （`-s -w` 不会删数据段里的字符串字面量），同时搜 `level="asInvoker"` / `PerMonitorV2` / `true/pm` /
  Win10-11 的 `supportedOS` GUID，全部命中，说明 manifest 真的嵌进去了而不只是文件存在。
- ② **控制台变体实跑**：脚本第 4 步用同样的 ldflags 去掉 `-H windowsgui` 另编一个临时 exe，
  实跑 `--version` 得到 `keyboard-statistics 0.1.0-test`，退出码 0，带 10s 超时保护
  （防止将来 `--version` 被改坏把 GUI 拉起来卡住构建）。
- ③ **脚本外复核**：`go version -m` 显示 `CGO_ENABLED=0` + `-trimpath=true`；PE 头 `Subsystem=2`；
  `(Get-Item exe).VersionInfo` 读到 FileVersion `0.1.0.0`；`go-winres extract` 抽出的资源里
  `dpi-awareness = per monitor v2`、`use-common-controls-v6 = true`。
- 本次未改任何 Go 代码，未动 `go.mod` / `go.sum`（`go run 模块@版本` 不会写 go.mod）。

**➡️ 下一步**：

1. 等 `internal/ui` 的 Win32 界面写完、`fyne` 从 `go.mod` 摘干净后，重新出一版包。
2. 按 AGENTS 的发布要求，在**云桌面 / 远程桌面**（无 GPU 加速）双击实跑，确认能出窗口、无黑框、DPI 不糊，
   验证步骤补进 `docs/manual-test.md`。
3. 首次发版前同步 `CHANGELOG.md` 与 `README.md` 的版本号。

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
