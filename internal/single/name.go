// Package single 保证同一时刻只有一个程序实例在跑。
//
// 为什么需要：统计库是同一个 SQLite 文件，两个实例同时往里写会互相打架；
// 升级安装时也要能确定「还有哪个实例挡着文件」。互斥体随进程句柄一起消失，
// 所以进程被杀掉不会留下死锁——下次启动照样能拿到。
package single

// DefaultName 是主程序使用的互斥体名。
//
// 注意：scripts\installer.iss 的 AppMutex 必须与本值逐字符一致，
// 否则安装器认不出「程序正在运行」，升级时就不会先关掉它再替换文件。
// 改这里请同步改 .iss（internal/single 的测试会盯着这件事）。
const DefaultName = "KeyboardStatistics_SingleInstance"
