# 🧪 手动验证清单

自动化测试覆盖不到的 Windows 行为，都记在这里。**改动相关代码后必须照着跑一遍。**

---

## 1️⃣ 键盘钩子链路

```powershell
go run ./cmd/hookcheck
```

程序会自己注入按键（`keybd_event`），输出应当类似：

```
=== 单键 ===
  A        2
  C        1
  Ctrl     1
  Shift    1
  ←        1
=== 组合键 ===
  Ctrl+C           1
  Shift+←          1
合计 6 次按键、2 种组合
✅ 钩子链路正常
```

**要看的三点**：

1. `A` 必须是 **2** 而不是 6 —— 程序最后长按了 A 并连发 5 次 KEYDOWN，去重后只该多计 1 次
2. `Ctrl+C` 必须出现 —— 组合键判定真的生效
3. `Shift+←` 必须出现 —— 只有 Shift 时，非字符键仍算组合

手动版：另开一个记事本，随便打字，看程序窗口里的数字有没有涨（同时确认打字本身不卡）。

---

## 2️⃣ 管理员窗口收不到按键（预期行为）

1. 以管理员身份打开「任务管理器」
2. 在任务管理器里按几下键
3. 程序统计数**不应**增加

这是 Windows 的权限隔离，不是 bug，不要试图绕过。

---

## 3️⃣ 系统保留组合

按 `Ctrl+Alt+Delete` 或 `Win+L`。这类组合由系统在更底层截获，
任何 `WH_KEYBOARD_LL` 钩子都拿不到，统计里不该出现。

---

## 4️⃣ 开机自启

1. 设置页勾选「开机自启」
2. 打开 `regedit`，检查
   `HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`
   应存在名为 `KeyboardStatistics` 的字符串值，内容形如
   `"C:\路径\keyboard-statistics.exe" --autostart`
3. 注销后重新登录（或重启）
4. 程序应自动启动，且**窗口是最小化的**，任务栏可见
5. 回到设置页取消勾选，再看注册表，该值应**已被删除**

---

## 5️⃣ 首次启动引导

删掉数据库 `%LOCALAPPDATA%\keyboard-statistics\stats.db` 后重新启动：

1. 应弹出一次性说明窗口
2. 同时收到一条系统通知气泡（若系统未关闭通知）
3. 关掉说明窗口后重新启动，**不应**再弹

---

## 6️⃣ 采集开关

1. 暂停采集，随便敲键盘，统计数**不变**
2. 继续采集，敲键盘，统计数**恢复增长**

---

## 7️⃣ 清空数据

点「清空数据」后：概览页所有数字归零，但「采集开关」状态与「首次引导标记」保留
（不该清完就重新弹一次引导）。

---

## 8️⃣ 原生 Win32 界面（GDI 自绘）

界面没有用任何 GUI 框架，窗口、绘制、命中测试全是自己算的，以下几条只能手动看。

### 8.1 窗口与退出

```powershell
go build -o bin\app.exe ./cmd/app
Start-Process .\bin\app.exe -PassThru
```

1. 窗口标题必须是「键盘使用统计」，客户区 900×680（普通 96 DPI 下 916×719 含边框），屏幕上居中。
2. 三个标签页「概览 / 热力图 / 设置」都能点开，顶部「今日 / 近 7 天 / 近 30 天 / 全部」能切换，
   切了之后状态栏的时间范围跟着变。
3. 拖动窗口缩到最小：**不应**缩到比 800×600（客户区）更小。
4. 点右上角关闭：进程必须**立刻退出**，`Get-Process app` 查不到残留（没有托盘、没有后台常驻）。
5. 最小化/还原、被别的窗口遮挡再露出来，界面都应正常重绘，**不闪白、不残影**
   （双缓冲的一个已知坑：`BitBlt` 必须在位图仍被选中时做，先还原旧位图再拷只会拷到 1×1 占位图，整窗空白）。

### 8.2 设置页四个动作

| 动作 | 预期 |
| --- | --- |
| 暂停/继续采集 | 按钮文案与顶部状态点（● 采集中 / ● 已暂停）立刻变；**重启程序后状态保持** |
| 开机自启 | 勾选/取消写注册表；拿不到注册表时复选框是灰的「当前环境不可用」，点了不崩 |
| 导出 CSV | 弹选目录框；确认后弹窗列出两个 csv 的完整路径，Excel 打开中文不乱码 |
| 清空全部数据 | 弹二次确认，**默认按钮在「否」**；选否什么都不发生，选是之后概览归零 |

「打开数据目录」应打开**真正的数据目录**（默认 `%LOCALAPPDATA%\keyboard-statistics`，
便携模式下是 `<程序目录>\data`），不存在时先建出来。

### 8.3 自启最小化

```powershell
Start-Process .\bin\app.exe -ArgumentList '--autostart'
```

窗口应**最小化出现在任务栏**（`IsIconic` 为真），不抢当前窗口焦点；点任务栏还原后界面正常。

### 8.4 高分屏

把显示缩放调到 125% / 150% 后再启动：

1. 文字不糊（已 `SetProcessDPIAware`），布局整体等比放大。
2. 设置页「关于」里的四行说明**不能被裁掉**，按钮文字不出框。
3. 若窗口跑到另一块不同 DPI 的显示器上，下次启动以该显示器为准（本轮未做运行时迁移重排）。

## 9️⃣ 数据目录（便携模式 / 自定义位置）

数据目录有三级来源：**用户指定 > 便携模式 > `%LOCALAPPDATA%`**，每一级都要单独验。

### 9.1 便携模式生效

```powershell
$dir = "$env:TEMP\ks-portable"
Remove-Item $dir -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Copy-Item .\bin\app.exe "$dir\app.exe"
Set-Content -Path "$dir\portable.txt" -Value 'portable' -Encoding ASCII
Start-Process "$dir\app.exe"
```

1. 数据文件应出现在 `$dir\data\`（`stats.db`、`app.log`），**`%LOCALAPPDATA%\keyboard-statistics` 不新增文件**。
2. 设置页「关于」里「数据目录」显示 `$dir\data`，「存放方式」显示「便携模式（程序目录）」。
3. 「打开数据目录」打开的是 `$dir\data`。

### 9.2 便携标记存在但目录不可写 → 回退

把程序装到 `C:\Program Files\...`（或任何普通用户不可写的目录）并放上 `portable.txt`：

1. 程序**必须能正常启动**，数据回退到 `%LOCALAPPDATA%\keyboard-statistics`。
2. 设置页「存放方式」显示「用户数据目录」，不是「便携模式」。

### 9.3 数据目录不可写 → 必须弹窗

把 `LOCALAPPDATA` 指到一个非法路径再启动：

```powershell
$env:LOCALAPPDATA = 'C:\Windows\System32\config\sub'
Start-Process .\bin\app.exe
```

必须弹出「数据目录无法写入」的错误框并说明路径；**不能是双击之后毫无反应**
（`-H windowsgui` 没有控制台，静默 `os.Exit(1)` 等于没有任何反馈）。

### 9.4 更改数据目录 + 自动迁移

1. 设置页点「更改数据目录…」→ 选一个**空目录** → 确认。
2. 程序应提示即将退出并真的退出。
3. 重新打开后：新目录里出现 `stats.db`，旧目录里的 `stats.db` 已被搬走（不是复制）；
   概览页数据与迁移前一致。
4. 再点「更改数据目录…」并选中一个**已经存在 `stats.db`** 的目录：必须提示「已经有统计数据」
   并拒绝，不能覆盖。

### 9.5 安装包

```powershell
.\dist\keyboard-statistics-<版本>-setup.exe
```

1. 安装向导能自选安装目录（装到 `D:\...` 这种用户可写目录）。
2. 勾选「便携模式」→ 装完 `{app}\portable.txt` 存在，首次启动数据落在 `{app}\data`。
3. **不勾选**「便携模式」→ 没有 `portable.txt`，数据仍在 `%LOCALAPPDATA%`。
4. 卸载：程序文件被删；若 `{app}\data` 里有统计数据，卸载器要**先问**再删，默认保留。
5. 装到 `C:\Program Files\` 且**不勾选**便携模式：程序照常运行，数据在 `%LOCALAPPDATA%`
   （这条验证「默认不写程序目录」的约定没被安装包破坏）。

