; ============================================================================
; keyboard-statistics —— Inno Setup 6 安装脚本
;
; 编译（本机 Inno Setup 6 是按当前用户装的，所以不在 Program Files）：
;   "%LOCALAPPDATA%\Programs\Inno Setup 6\ISCC.exe" scripts\installer.iss
;   （若装在全局则是 "C:\Program Files (x86)\Inno Setup 6\ISCC.exe"）
;
; 产物：dist\keyboard-statistics-<version>-setup.exe
;
; 设计要点（每一条都对应一个明确需求，改之前先读）：
;
;   1) PrivilegesRequired=lowest：按当前用户安装，不弹 UAC。
;      主程序清单是 asInvoker，装到 Program Files 会写不了数据；按用户安装
;      才能让用户把目录选到 D:\... 这种自己可写的路径。
;   2) DisableDirPage=no：强制显示「选择安装位置」页。
;      默认目录 {localappdata}\Programs\KeyboardStatistics，允许改成任意路径
;      （含 D:\ 根目录，故显式打开 AllowRootDirectory）。
;   3) [Tasks] 里两个任务都默认勾选：便携模式（portable.txt）与桌面快捷方式。
;      程序约定：exe 同目录存在 portable.txt → 数据写 <exe 目录>\data\，
;      否则写 %LOCALAPPDATA%\keyboard-statistics。本脚本只负责创建/不创建
;      这个标记文件，判定逻辑在 Go 侧。
;   4) 不勾便携模式时**不会**去删已存在的 portable.txt：
;      静默删标记等于悄悄把用户的数据源切走（数据还在 data\ 里，但程序不再读它），
;      比留一个文件危险得多。这里只保证「不创建」。
;   5) 卸载：{app}\data 存在时用 MsgBox 问用户，默认按钮是「否」；
;      静默卸载（/SILENT、/VERYSILENT）一律保留，绝不静默删统计数据。
;   6) 语言：本机 Inno Setup 6 的 Languages\ 目录里没有 ChineseSimplified.isl
;      （只有官方随包的那批语言），按约定不自行下载翻译文件，界面用英文。
;      见 [Languages] 段注释。
;   7) 主程序来源：显式路径 ..\dist\keyboard-statistics-v<MyAppVersion>.exe，
;      不用通配。通配会同时匹配到多个历史产物，还有把 *-setup.exe 自己装进去
;      的风险；这里用 #define + 编译期 FileExists 断言，产物缺失直接报错。
;      **发新版时只改下面 MyAppVersion 一处。**
;
; 编码：UTF-8 无 BOM + LF（仓库 .gitattributes 要求）。界面文案全是 ASCII 英文，
;      中文只出现在注释里，即使被误判编码也不会影响编译产物。
; ============================================================================

#define MyAppName "Keyboard Statistics"
#define MyAppVersion "0.3.0"
#define MyAppPublisher "keyboard-statistics"
#define MyAppExeName "keyboard-statistics.exe"

; AppId 去掉外层花括号的形式，单独定义一份，供 [Code] 拼卸载注册表键名用。
; [Setup] 里写成 AppId={{{#MyAppId}} —— 数一下花括号：ISPP 把 {#MyAppId} 换成纯 GUID 后，
; 剩下的 {{ 和末尾的 } 正好是 Inno 要求的转义写法（{{ 表示一个字面量 {）。
; **少写一个 } 会变成另一个 AppId**，结果就是「升级认不出已安装」，实测会装到默认目录去。
; 一旦发布就不能改这个 GUID，否则升级会被装成两套。
#define MyAppId "A1C3E5F7-2B4D-4E6F-8A9B-0C1D2E3F4A5B"

; 源 exe：dist\keyboard-statistics-v<version>.exe（scripts\build.ps1 -Version <version> 的产物）
#define MyAppSourceExe "..\dist\keyboard-statistics-v" + MyAppVersion + ".exe"

; 安装包自带的版本资源要 x.x.x.x 四段，这里给三段版本补一段。
; 注意 MyAppVersion 必须是纯数字点号，不能带 -beta 这种后缀。
#define MyAppVersionQuad MyAppVersion + ".0"

; 编译期断言：源 exe 必须存在。ISPP 的 #error 只吃字面量，不做字符串拼接，
; 所以这句话里的版本号是写死的，改 MyAppVersion 时顺手改这里。
#if !FileExists(AddBackslash(SourcePath) + MyAppSourceExe)
  #error dist artifact not found: dist\keyboard-statistics-v0.3.0.exe is missing. Run scripts\build.ps1 -Version 0.3.0 first.
#endif

[Setup]
; AppId 一旦发布就不能改，否则升级会被装成两套。
AppId={{{#MyAppId}}
; 程序启动时会创建同名互斥体（internal/single 的 DefaultName），保证只有一个实例。
; 但这里**故意不写 AppMutex**：一旦声明，安装器会在启动阶段弹「程序正在运行，请先关掉」
; 并停下来等人操作——静默模式下直接失败（实测 exit 1，/CLOSEAPPLICATIONS 也救不了）。
; 我们要的是「安装器替用户关掉再装」，所以交给下面的 CloseApplications + Restart Manager。
;
; 升级时强制关闭占用文件的程序（含本程序自己），不弹窗问，直接关掉再替换：
; 程序没有未保存的数据，优雅关闭（WM_CLOSE）即可，关掉是安全的。
CloseApplications=force
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\Programs\KeyboardStatistics
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
; 必须显示目录选择页：默认值是 auto（升级时会跳过），这里强制显示。
DisableDirPage=no
; 关键：按当前用户安装，不请求提权，所以不弹 UAC。
PrivilegesRequired=lowest
; 不写 PrivilegesRequiredOverridesAllowed，安装向导就不会出现
; "为所有用户安装" 的选择项，也就不可能被提到管理员权限。
; 允许把目录改到 D:\ 之类的盘根（默认 no 会拒绝根目录）。
AllowRootDirectory=yes
UsePreviousAppDir=yes
OutputDir=..\dist
OutputBaseFilename=keyboard-statistics-{#MyAppVersion}-setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
UninstallDisplayName={#MyAppName} {#MyAppVersion}
UninstallDisplayIcon={app}\{#MyAppExeName}
VersionInfoVersion={#MyAppVersionQuad}
VersionInfoProductVersion={#MyAppVersionQuad}
VersionInfoProductName={#MyAppName}
VersionInfoDescription={#MyAppName} {#MyAppVersion} Setup
VersionInfoCompany={#MyAppPublisher}

[Languages]
; 本机 Languages\ 下没有 ChineseSimplified.isl（Inno Setup 6.7.3 官方包不含简体中文），
; 按任务约定不自行下载第三方翻译文件，因此只保留英文向导。
; 若以后把官方/自制的 ChineseSimplified.isl 放进 Languages\，在这里追加一行即可：
;   Name: "chinesesimplified"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
; 两个任务都不加 unchecked / checkedonce，即默认勾选。
; 升级安装时 Inno 会用上次选择（UsePreviousTasks 默认 yes），不会把用户的偏好改掉。
Name: "portablemode"; Description: "Portable mode: keep statistics in a ""data"" folder next to the program (creates portable.txt)"; GroupDescription: "Data location:"
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
; 主程序：改写名字后装成 {app}\keyboard-statistics.exe
Source: "{#MyAppSourceExe}"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion
; 便携标记：只有勾了便携模式才装，没勾就没有这个文件（卸载时随主程序一起删掉）
Source: "portable.txt"; DestDir: "{app}"; Tasks: portablemode; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
; lowest 权限下 {autodesktop} 指向当前用户的桌面
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent

[Code]
var
  { 检测到的「已安装目录」；空字符串表示这是一次全新安装。
    升级必须留在原目录：换目录会变成两套并存，注册表只认最后装的那套，
    先前那套的文件就成了没人管的孤儿目录——所以这里要拦住。 }
  GPreviousDir: String;

{ 上一套安装的卸载注册表键名。PrivilegesRequired=lowest 且非 64 位安装模式，
  所以键就在 HKCU\Software 下，不会被重定向到 Wow6432Node。 }
function UninstallKeyName: String;
begin
  Result := 'Software\Microsoft\Windows\CurrentVersion\Uninstall\{' + '{#MyAppId}' + '}_is1';
end;

function InitializeSetup(): Boolean;
var
  Dir: String;
begin
  GPreviousDir := '';
  if RegQueryStringValue(HKCU, UninstallKeyName, 'InstallLocation', Dir) then
    GPreviousDir := RemoveBackslashUnlessRoot(Dir);
  Result := True;
end;

{ 升级时跳过「选择安装目录」页：目录只能是原来那个，不给用户改的机会。
  全新安装仍然显示，这样第一次装的时候用户可以选到 D:\ 之类的盘。 }
function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := (PageID = wpSelectDir) and (GPreviousDir <> '');
end;

{ 路径比较：卸载注册表里存的可能是 8.3 短名（C:\Users\USER29~1\...），
  命令行给的是长名。Inno 脚本里没有 GetLongName，直接调 kernel32 归一成长名再比，
  免得把一次正常的原地升级误判成「换目录」而拒绝安装。
  转换失败（返回 0）时退回原样比较——宁可放过，也不要冤枉一次正常升级。 }
function GetLongPathNameW(ShortPath: String; LongPath: String; BufSize: DWORD): DWORD;
  external 'GetLongPathNameW@kernel32.dll stdcall';

function NormalizeDir(const Dir: String): String;
var
  Buf: String;
  Len: DWORD;
begin
  Result := RemoveBackslashUnlessRoot(Dir);
  SetLength(Buf, 1024);
  Len := GetLongPathNameW(Result, Buf, 1024);
  if (Len > 0) and (Len < 1024) then
  begin
    SetLength(Buf, Len);
    Result := RemoveBackslashUnlessRoot(Buf);
  end;
end;

function SameDir(const A, B: String): Boolean;
begin
  Result := CompareText(NormalizeDir(A), NormalizeDir(B)) = 0;
end;

{ 兜底拦一道：静默安装不显示向导页，命令行 /DIR 仍然可能把目录换掉，
  那就装出第二套了（实测：卸载注册表会改指向新目录，旧目录变成没人管的孤儿）。
  与其留下两套，不如直接失败——失败信息里写清原目录在哪。

  注意**不能**用 WizardDirValue 判断：静默模式下向导页压根没创建，
  它返回的是 DefaultDirName，会把一次正常的原地升级冤枉死（实测失败在 exit 7）。
  所以这里只看命令行究竟有没有显式给 /DIR。 }
function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Requested: String;
begin
  Result := '';
  Requested := ExpandConstant('{param:DIR|}');
  if (GPreviousDir <> '') and (Requested <> '') and not SameDir(Requested, GPreviousDir) then
    Result := 'This is an upgrade: the program must stay in its existing folder.' + #13#10#13#10 +
              'Existing:  ' + GPreviousDir + #13#10 +
              'Requested: ' + Requested + #13#10#13#10 +
              'Uninstall the old version first if you really want a different folder.';
end;

{ 便携模式的数据目录（有 portable.txt 时程序写这里）。
  卸载完成后才问，避免用户删完数据又把卸载取消掉。 }
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  DataDir: String;
begin
  if CurUninstallStep <> usPostUninstall then
    Exit;

  DataDir := ExpandConstant('{app}\data');
  if not DirExists(DataDir) then
    Exit;

  { 静默卸载一律保留：/SILENT、/VERYSILENT 下不能弹窗，默认动作必须是「不删」。 }
  if UninstallSilent then
  begin
    Log('Portable data kept (silent uninstall): ' + DataDir);
    Exit;
  end;

  if MsgBox('Delete the statistics data stored next to the program?' + #13#10 + #13#10 +
            DataDir + #13#10 + #13#10 +
            'Choose No to keep your data. Deleted data cannot be recovered.',
            mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES then
  begin
    DelTree(DataDir, True, True, True);
    Log('Portable data deleted: ' + DataDir);
  end
  else
    Log('Portable data kept: ' + DataDir);
end;
