package stats

// Windows 虚拟键码常量。
//
// 这里刻意不用 golang.org/x/sys/windows：stats 包必须是平台无关的纯计算包，
// 才能在任何机器上跑单元测试。Windows 专有的钩子代码负责把系统键码转成这些值。
const (
	VKBack    uint16 = 0x08 // 退格
	VKTab     uint16 = 0x09
	VKEnter   uint16 = 0x0D // 回车
	VKShift   uint16 = 0x10
	VKControl uint16 = 0x11
	VKMenu    uint16 = 0x12 // Alt
	VKPause   uint16 = 0x13
	VKCaps    uint16 = 0x14
	VKEscape  uint16 = 0x1B
	VKSpace   uint16 = 0x20
	VKPageUp  uint16 = 0x21
	VKPageDn  uint16 = 0x22
	VKEnd     uint16 = 0x23
	VKHome    uint16 = 0x24
	VKLeft    uint16 = 0x25
	VKUp      uint16 = 0x26
	VKRight   uint16 = 0x27
	VKDown    uint16 = 0x28

	VKPrint   uint16 = 0x2C
	VKInsert  uint16 = 0x2D
	VKDelete  uint16 = 0x2E
	VKLWin    uint16 = 0x5B
	VKRWin    uint16 = 0x5C
	VKApps    uint16 = 0x5D
	VKNumLock uint16 = 0x90

	VKLShift   uint16 = 0xA0
	VKRShift   uint16 = 0xA1
	VKLControl uint16 = 0xA2
	VKRControl uint16 = 0xA3
	VKLMenu    uint16 = 0xA4
	VKRMenu    uint16 = 0xA5

	VK1 uint16 = 0x31
	VKA uint16 = 0x41
	VKB uint16 = 0x42
	VKC uint16 = 0x43
	VKD uint16 = 0x44
	VKR uint16 = 0x52
	VKT uint16 = 0x54
	VKV uint16 = 0x56
	VKZ uint16 = 0x5A

	VKF4 uint16 = 0x73

	// 主键盘 OEM 标点
	VKOEM1     uint16 = 0xBA // ;
	VKOEMPlus  uint16 = 0xBB // =
	VKOEMComma uint16 = 0xBC // ,
	VKOEMMinus uint16 = 0xBD // -
	VKOEMDot   uint16 = 0xBE // .
	VKOEM2     uint16 = 0xBF // /
	VKOEM3     uint16 = 0xC0 // `
	VKOEM4     uint16 = 0xDB // [
	VKOEM5     uint16 = 0xDC // backslash
	VKOEM6     uint16 = 0xDD // ]
	VKOEM7     uint16 = 0xDE // '
	VKOEM102   uint16 = 0xE2 // backslash (ISO)
)
