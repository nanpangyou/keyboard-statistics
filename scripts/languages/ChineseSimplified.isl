; ============================================================================
; keyboard-statistics —— Inno Setup 6 简体中文语言文件
;
; 为什么自己写一份：Inno Setup 6.7.3 官方包不含简体中文（Languages\ 里只有
; 官方随包那批），而网上流传的中文 .isl 来源不明、版本对不上，按项目约定不引入。
; 这里按本机 Default.isl（Inno 6.5.0+ 消息集）逐条翻译，**消息一条不少**：
; 少任何一条，ISCC 编译时会直接报 "missing message"，不会静默变英文。
;
; 编码：**UTF-8 带 BOM**（仓库里唯一的例外）。Inno 只在文件以 UTF-8 BOM 开头时
; 才按 UTF-8 解析 .isl；没有 BOM 会按 LanguageCodePage 当 ANSI 读，中文就成乱码。
; 所以 LanguageCodePage 写 0（= Unicode 文件），字体显式指定雅黑。
;
; 占位符不能动：%1 %2 %n [name] [name/ver] [gb] [mb]。
; 按钮/选项里的 & 是助记符（Alt+字母），每条只留一个。
; ============================================================================

[LangOptions]
LanguageName=简体中文
LanguageID=$0804
LanguageCodePage=0
DialogFontName=Microsoft YaHei UI
DialogFontSize=9
WelcomeFontName=Microsoft YaHei UI
WelcomeFontSize=14

[Messages]

; *** 程序标题
SetupAppTitle=安装程序
SetupWindowTitle=安装程序 - %1
UninstallAppTitle=卸载
UninstallAppFullTitle=%1 卸载

; *** 通用
InformationTitle=提示
ConfirmTitle=确认
ErrorTitle=错误

; *** 安装引导程序
SetupLdrStartupMessage=即将安装 %1。要继续吗？
LdrCannotCreateTemp=无法创建临时文件，安装已中止
LdrCannotExecTemp=无法执行临时目录中的文件，安装已中止
HelpTextNote=

; *** 启动阶段错误
LastErrorMessage=%1。%n%n错误 %2：%3
SetupFileMissing=安装目录中缺少文件 %1。请修正该问题，或重新获取一份安装程序。
SetupFileCorrupt=安装文件已损坏，请重新获取一份安装程序。
SetupFileCorruptOrWrongVer=安装文件已损坏，或与本版本安装程序不兼容。请修正该问题，或重新获取一份安装程序。
InvalidParameter=命令行上有无效参数：%n%n%1
SetupAlreadyRunning=安装程序已经在运行。
WindowsVersionNotSupported=本程序不支持当前版本的 Windows。
WindowsServicePackRequired=本程序需要 %1 Service Pack %2 或更高版本。
NotOnThisPlatform=本程序无法在 %1 上运行。
OnlyOnThisPlatform=本程序必须在 %1 上运行。
OnlyOnTheseArchitectures=本程序只能安装在面向以下处理器架构的 Windows 上：%n%n%1
WinVersionTooLowError=本程序需要 %1 版本 %2 或更高。
WinVersionTooHighError=本程序无法安装在 %1 版本 %2 或更高版本上。
AdminPrivilegesRequired=安装本程序需要以管理员身份登录。
PowerUserPrivilegesRequired=安装本程序需要以管理员或 Power Users 组成员的身份登录。
SetupAppRunningError=安装程序检测到 %1 正在运行。%n%n请先关闭它的所有实例，然后点「确定」继续；点「取消」退出。
UninstallAppRunningError=卸载程序检测到 %1 正在运行。%n%n请先关闭它的所有实例，然后点「确定」继续；点「取消」退出。

; *** 启动阶段询问
PrivilegesRequiredOverrideTitle=选择安装方式
PrivilegesRequiredOverrideInstruction=选择安装方式
PrivilegesRequiredOverrideText1=%1 可以为所有用户安装（需要管理员权限），也可以只为当前用户安装。
PrivilegesRequiredOverrideText2=%1 可以只为当前用户安装，也可以为所有用户安装（需要管理员权限）。
PrivilegesRequiredOverrideAllUsers=为所有用户安装(&A)
PrivilegesRequiredOverrideAllUsersRecommended=为所有用户安装(&A)（推荐）
PrivilegesRequiredOverrideCurrentUser=仅为当前用户安装(&M)
PrivilegesRequiredOverrideCurrentUserRecommended=仅为当前用户安装(&M)（推荐）

; *** 其他错误
ErrorCreatingDir=安装程序无法创建目录「%1」
ErrorTooManyFilesInDir=无法在目录「%1」中创建文件：该目录中的文件太多

; *** 安装程序通用
ExitSetupTitle=退出安装
ExitSetupMessage=安装尚未完成，现在退出将不会安装本程序。%n%n你可以稍后再次运行安装程序完成安装。%n%n要退出安装吗？
AboutSetupMenuItem=关于安装程序(&A)...
AboutSetupTitle=关于安装程序
AboutSetupMessage=%1 版本 %2%n%3%n%n%1 主页：%n%4
AboutSetupNote=
TranslatorNote=

; *** 按钮
ButtonBack=< 上一步(&B)
ButtonNext=下一步(&N) >
ButtonInstall=安装(&I)
ButtonOK=确定
ButtonCancel=取消
ButtonYes=是(&Y)
ButtonYesToAll=全部是(&A)
ButtonNo=否(&N)
ButtonNoToAll=全部否(&O)
ButtonFinish=完成(&F)
ButtonBrowse=浏览(&B)...
ButtonWizardBrowse=浏览(&R)...
ButtonNewFolder=新建文件夹(&M)

; *** 「选择语言」
SelectLanguageTitle=选择安装语言
SelectLanguageLabel=选择安装过程中使用的语言。

; *** 向导通用文字
ClickNext=点「下一步」继续，或点「取消」退出安装。
BeveledLabel=
BrowseDialogTitle=浏览文件夹
BrowseDialogLabel=在下面的列表中选择一个文件夹，然后点「确定」。
NewFolderName=新建文件夹

; *** 「欢迎」
WelcomeLabel1=欢迎使用 [name] 安装向导
WelcomeLabel2=本向导将在你的电脑上安装 [name/ver]。%n%n建议在继续之前关闭其他应用程序。

; *** 「密码」
WizardPassword=密码
PasswordLabel1=本次安装需要密码。
PasswordLabel3=请输入密码，然后点「下一步」继续。密码区分大小写。
PasswordEditLabel=密码(&P)：
IncorrectPassword=密码不正确，请重新输入。

; *** 「许可协议」
WizardLicense=许可协议
LicenseLabel=继续之前，请阅读以下重要信息。
LicenseLabel3=请阅读以下许可协议。必须接受本协议才能继续安装。
LicenseAccepted=我接受本协议(&A)
LicenseNotAccepted=我不接受本协议(&D)

; *** 「信息」页
WizardInfoBefore=信息
InfoBeforeLabel=继续之前，请阅读以下重要信息。
InfoBeforeClickLabel=准备好继续安装时，点「下一步」。
WizardInfoAfter=信息
InfoAfterLabel=继续之前，请阅读以下重要信息。
InfoAfterClickLabel=准备好继续安装时，点「下一步」。

; *** 「用户信息」
WizardUserInfo=用户信息
UserInfoDesc=请输入你的信息。
UserInfoName=用户名(&U)：
UserInfoOrg=单位(&O)：
UserInfoSerial=序列号(&S)：
UserInfoNameRequired=必须输入用户名。

; *** 「选择安装位置」
WizardSelectDir=选择安装位置
SelectDirDesc=[name] 要安装到哪里？
SelectDirLabel3=安装程序将把 [name] 安装到下面的文件夹。
SelectDirBrowseLabel=要继续，请点「下一步」。想换一个文件夹，请点「浏览」。
DiskSpaceGBLabel=至少需要 [gb] GB 可用磁盘空间。
DiskSpaceMBLabel=至少需要 [mb] MB 可用磁盘空间。
CannotInstallToNetworkDrive=安装程序无法安装到网络驱动器。
CannotInstallToUNCPath=安装程序无法安装到 UNC 路径。
InvalidPath=必须输入带盘符的完整路径，例如：%n%nC:\APP%n%n或者以下形式的 UNC 路径：%n%n\\server\share
InvalidDrive=所选驱动器或 UNC 共享不存在或无法访问，请另选一个。
DiskSpaceWarningTitle=磁盘空间不足
DiskSpaceWarning=安装至少需要 %1 KB 可用空间，但所选驱动器只剩 %2 KB。%n%n仍要继续吗？
DirNameTooLong=文件夹名或路径太长。
InvalidDirName=文件夹名无效。
BadDirName32=文件夹名不能包含下列字符：%n%n%1
DirExistsTitle=文件夹已存在
DirExists=文件夹：%n%n%1%n%n已经存在。仍要安装到这个文件夹吗？
DirDoesntExistTitle=文件夹不存在
DirDoesntExist=文件夹：%n%n%1%n%n不存在。要创建这个文件夹吗？

; *** 「选择组件」
WizardSelectComponents=选择组件
SelectComponentsDesc=要安装哪些组件？
SelectComponentsLabel2=勾选要安装的组件，取消勾选不需要的组件。准备好后点「下一步」继续。
FullInstallation=完整安装
CompactInstallation=紧凑安装
CustomInstallation=自定义安装
NoUninstallWarningTitle=组件已存在
NoUninstallWarning=安装程序检测到你的电脑上已安装下列组件：%n%n%1%n%n取消勾选这些组件不会卸载它们。%n%n仍要继续吗？
ComponentSize1=%1 KB
ComponentSize2=%1 MB
ComponentsDiskSpaceGBLabel=当前选择至少需要 [gb] GB 磁盘空间。
ComponentsDiskSpaceMBLabel=当前选择至少需要 [mb] MB 磁盘空间。

; *** 「选择附加任务」
WizardSelectTasks=选择附加任务
SelectTasksDesc=要执行哪些附加任务？
SelectTasksLabel2=选择安装 [name] 时要执行的附加任务，然后点「下一步」。

; *** 「选择开始菜单文件夹」
WizardSelectProgramGroup=选择开始菜单文件夹
SelectStartMenuFolderDesc=安装程序要把程序的快捷方式放在哪里？
SelectStartMenuFolderLabel3=安装程序将在下面的开始菜单文件夹里创建程序快捷方式。
SelectStartMenuFolderBrowseLabel=要继续，请点「下一步」。想换一个文件夹，请点「浏览」。
MustEnterGroupName=必须输入文件夹名。
GroupNameTooLong=文件夹名或路径太长。
InvalidGroupName=文件夹名无效。
BadGroupName=文件夹名不能包含下列字符：%n%n%1
NoProgramGroupCheck2=不创建开始菜单文件夹(&D)

; *** 「准备安装」
WizardReady=准备安装
ReadyLabel1=现在可以开始在电脑上安装 [name] 了。
ReadyLabel2a=点「安装」开始安装；想检查或修改设置，点「上一步」。
ReadyLabel2b=点「安装」开始安装。
ReadyMemoUserInfo=用户信息：
ReadyMemoDir=安装位置：
ReadyMemoType=安装类型：
ReadyMemoComponents=已选组件：
ReadyMemoGroup=开始菜单文件夹：
ReadyMemoTasks=附加任务：

; *** 下载页
DownloadingLabel2=正在下载文件...
ButtonStopDownload=停止下载(&S)
StopDownload=确定要停止下载吗？
ErrorDownloadAborted=下载已中止
ErrorDownloadFailed=下载失败：%1 %2
ErrorDownloadSizeFailed=获取大小失败：%1 %2
ErrorProgress=进度无效：%1 / %2
ErrorFileSize=文件大小无效：期望 %1，实际 %2

; *** 解压页
ExtractingLabel=正在解压文件...
ButtonStopExtraction=停止解压(&S)
StopExtraction=确定要停止解压吗？
ErrorExtractionAborted=解压已中止
ErrorExtractionFailed=解压失败：%1

; *** 解压失败细节
ArchiveIncorrectPassword=密码不正确
ArchiveIsCorrupted=压缩包已损坏
ArchiveUnsupportedFormat=不支持这种压缩格式

; *** 「正在准备安装」
WizardPreparing=正在准备安装
PreparingDesc=安装程序正在准备在你的电脑上安装 [name]。
PreviousInstallNotCompleted=之前某个程序的安装/卸载尚未完成。需要重启电脑才能完成那次安装。%n%n重启后请再次运行安装程序，以完成 [name] 的安装。
CannotContinue=安装程序无法继续，请点「取消」退出。
ApplicationsFound=下列应用程序正在使用安装程序需要更新的文件，建议允许安装程序自动关闭它们。
ApplicationsFound2=下列应用程序正在使用安装程序需要更新的文件，建议允许安装程序自动关闭它们。安装完成后，安装程序会尝试重新启动这些程序。
CloseApplications=自动关闭这些应用程序(&A)
DontCloseApplications=不要关闭这些应用程序(&D)
ErrorCloseApplications=安装程序无法自动关闭所有应用程序。建议先手动关闭所有正在使用待更新文件的应用程序再继续。
PrepareToInstallNeedsRestart=安装程序必须重启你的电脑。重启后请再次运行安装程序，以完成 [name] 的安装。%n%n要现在重启吗？

; *** 「正在安装」
WizardInstalling=正在安装
InstallingLabel=正在把 [name] 安装到你的电脑上，请稍候。

; *** 「安装完成」
FinishedHeadingLabel=正在完成 [name] 安装向导
FinishedLabelNoIcons=安装程序已完成 [name] 的安装。
FinishedLabel=安装程序已完成 [name] 的安装。可以通过已安装的快捷方式启动本程序。
ClickFinish=点「完成」退出安装程序。
FinishedRestartLabel=要完成 [name] 的安装，必须重启电脑。要现在重启吗？
FinishedRestartMessage=要完成 [name] 的安装，必须重启电脑。%n%n要现在重启吗？
ShowReadmeCheck=是，我要查看自述文件
YesRadio=是，现在重启电脑(&Y)
NoRadio=否，我稍后自己重启(&N)
RunEntryExec=运行 %1
RunEntryShellExec=查看 %1

; *** 「需要下一张磁盘」
ChangeDiskTitle=安装程序需要下一张磁盘
SelectDiskLabel2=请插入第 %1 张磁盘并点「确定」。%n%n如果这张磁盘上的文件在下面显示的路径之外的文件夹里，请输入正确路径或点「浏览」。
PathLabel=路径(&P)：
FileNotInDir2=在「%2」中找不到文件「%1」。请插入正确的磁盘或选择其他文件夹。
SelectDirectoryLabel=请指定下一张磁盘的位置。

; *** 安装阶段消息
SetupAborted=安装未完成。%n%n请修正问题后重新运行安装程序。
AbortRetryIgnoreSelectAction=选择操作
AbortRetryIgnoreRetry=重试(&T)
AbortRetryIgnoreIgnore=忽略错误并继续(&I)
AbortRetryIgnoreCancel=取消安装
RetryCancelSelectAction=选择操作
RetryCancelRetry=重试(&T)
RetryCancelCancel=取消

; *** 安装状态
StatusClosingApplications=正在关闭应用程序...
StatusCreateDirs=正在创建目录...
StatusExtractFiles=正在解压文件...
StatusDownloadFiles=正在下载文件...
StatusCreateIcons=正在创建快捷方式...
StatusCreateIniEntries=正在写入 INI 配置...
StatusCreateRegistryEntries=正在写入注册表...
StatusRegisterFiles=正在注册文件...
StatusSavingUninstall=正在保存卸载信息...
StatusRunProgram=正在完成安装...
StatusRestartingApplications=正在重新启动应用程序...
StatusRollback=正在回滚更改...

; *** 其他错误
ErrorInternal2=内部错误：%1
ErrorFunctionFailedNoCode=%1 失败
ErrorFunctionFailed=%1 失败；代码 %2
ErrorFunctionFailedWithMessage=%1 失败；代码 %2。%n%3
ErrorExecutingProgram=无法执行文件：%n%1

; *** 注册表错误
ErrorRegOpenKey=打开注册表键出错：%n%1\%2
ErrorRegCreateKey=创建注册表键出错：%n%1\%2
ErrorRegWriteKey=写入注册表键出错：%n%1\%2

; *** INI 错误
ErrorIniEntry=在文件「%1」中写入 INI 项出错。

; *** 复制文件错误
FileAbortRetryIgnoreSkipNotRecommended=跳过这个文件(&S)（不推荐）
FileAbortRetryIgnoreIgnoreNotRecommended=忽略错误并继续(&I)（不推荐）
SourceIsCorrupted=源文件已损坏
SourceDoesntExist=源文件「%1」不存在
SourceVerificationFailed=源文件校验失败：%1
VerificationSignatureDoesntExist=签名文件「%1」不存在
VerificationSignatureInvalid=签名文件「%1」无效
VerificationKeyNotFound=签名文件「%1」使用了未知密钥
VerificationFileNameIncorrect=文件名不正确
VerificationFileTagIncorrect=文件标记不正确
VerificationFileSizeIncorrect=文件大小不正确
VerificationFileHashIncorrect=文件哈希不正确
ExistingFileReadOnly2=现有文件被标记为只读，无法替换。
ExistingFileReadOnlyRetry=去掉只读属性后重试(&R)
ExistingFileReadOnlyKeepExisting=保留现有文件(&K)
ErrorReadingExistingDest=尝试读取现有文件时出错：
FileExistsSelectAction=选择操作
FileExists2=文件已经存在。
FileExistsOverwriteExisting=覆盖现有文件(&O)
FileExistsKeepExisting=保留现有文件(&K)
FileExistsOverwriteOrKeepAll=对后面的冲突都这样处理(&D)
ExistingFileNewerSelectAction=选择操作
ExistingFileNewer2=现有文件比安装程序要装入的文件更新。
ExistingFileNewerOverwriteExisting=覆盖现有文件(&O)
ExistingFileNewerKeepExisting=保留现有文件(&K)（推荐）
ExistingFileNewerOverwriteOrKeepAll=对后面的冲突都这样处理(&D)
ErrorChangingAttr=尝试修改现有文件属性时出错：
ErrorCreatingTemp=尝试在目标目录中创建文件时出错：
ErrorReadingSource=尝试读取源文件时出错：
ErrorCopying=尝试复制文件时出错：
ErrorDownloading=尝试下载文件时出错：
ErrorExtracting=尝试解压压缩包时出错：
ErrorReplacingExistingFile=尝试替换现有文件时出错：
ErrorRestartReplace=重启后替换文件失败：
ErrorRenamingTemp=尝试在目标目录中重命名文件时出错：
ErrorRegisterServer=无法注册 DLL/OCX：%1
ErrorRegSvr32Failed=RegSvr32 失败，退出代码 %1
ErrorRegisterTypeLib=无法注册类型库：%1

; *** 卸载显示名后缀
UninstallDisplayNameMark=%1（%2）
UninstallDisplayNameMarks=%1（%2，%3）
UninstallDisplayNameMark32Bit=32 位
UninstallDisplayNameMark64Bit=64 位
UninstallDisplayNameMarkAllUsers=所有用户
UninstallDisplayNameMarkCurrentUser=当前用户

; *** 安装后错误
ErrorOpeningReadme=尝试打开自述文件时出错。
ErrorRestartingComputer=安装程序无法重启电脑，请手动重启。

; *** 卸载程序
UninstallNotFound=文件「%1」不存在，无法卸载。
UninstallOpenError=无法打开文件「%1」，无法卸载
UninstallUnsupportedVer=卸载日志文件「%1」的格式不被此版本的卸载程序识别，无法卸载
UninstallUnknownEntry=卸载日志中有未知项（%1）
ConfirmUninstall=确定要完全移除 %1 及其所有组件吗？
UninstallOnlyOnWin64=本安装只能在 64 位 Windows 上卸载。
OnlyAdminCanUninstall=本安装只能由具有管理员权限的用户卸载。
UninstallStatusLabel=正在从电脑上移除 %1，请稍候。
UninstalledAll=已成功从电脑上移除 %1。
UninstalledMost=%1 卸载完成。%n%n有些项无法移除，可以手动删除。
UninstalledAndNeedsRestart=要完成 %1 的卸载，必须重启电脑。%n%n要现在重启吗？
UninstallDataCorrupted=文件「%1」已损坏，无法卸载

; *** 卸载阶段消息
ConfirmDeleteSharedFileTitle=要移除共享文件吗？
ConfirmDeleteSharedFile2=系统显示下列共享文件已不再被任何程序使用。要让卸载程序移除这个共享文件吗？%n%n如果还有程序在使用它，移除后那些程序可能无法正常工作。不确定就选「否」——把它留在系统里不会造成任何损害。
SharedFileNameLabel=文件名：
SharedFileLocationLabel=位置：
WizardUninstalling=卸载状态
StatusUninstalling=正在卸载 %1...

; *** 阻止关机的理由
ShutdownBlockReasonInstallingApp=正在安装 %1。
ShutdownBlockReasonUninstallingApp=正在卸载 %1。

; 以下自定义消息不是给安装程序本身用的，而是本项目的脚本在用。

[CustomMessages]

NameAndVersion=%1 版本 %2
AdditionalIcons=附加快捷方式：
CreateDesktopIcon=创建桌面快捷方式(&D)
CreateQuickLaunchIcon=创建快速启动快捷方式(&Q)
ProgramOnTheWeb=%1 网站
UninstallProgram=卸载 %1
LaunchProgram=启动 %1
AssocFileExtension=把 %1 关联到 %2 文件扩展名(&A)
AssocingFileExtension=正在把 %1 关联到 %2 文件扩展名...
AutoStartProgramGroupDescription=启动：
AutoStartProgram=自动启动 %1
AddonHostProgramNotFound=在所选文件夹中找不到 %1。%n%n仍要继续吗？

; ---- 本项目自己的文案（English.isl 里有对应英文）----
; 说明：这些会被 [Code] 里的 ExpandConstant('{cm:...}') 取出来拼字符串，
; 所以只放短句，不放 %1 之类的占位符——拼参数交给 Pascal 侧。
SetupDataLocationGroup=数据位置：
SetupPortableModeDesc=便携模式：把统计数据放在程序目录下的 data 文件夹（会创建 portable.txt）
SetupKeepDataHead=要删除程序目录里的统计数据吗？
SetupKeepDataTail=选「否」会保留你的数据。删掉之后无法恢复。
SetupAppRunningHead=键盘使用统计正在运行。
SetupAppRunningBody=继续安装需要先退出它（已经统计的按键会先保存好）。
SetupAppRunningAsk=现在退出程序并继续安装吗？
SetupUpgradeDirLockedHead=这是一次升级，程序必须留在原来的目录。
SetupUpgradeDirExisting=现有位置：
SetupUpgradeDirRequested=请求的位置：
SetupUpgradeDirLockedTail=确实想换目录，请先卸载旧版本。
