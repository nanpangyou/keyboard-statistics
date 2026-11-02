package stats

// NormalizeVK 把左右修饰键合并成通用键码。
//
// Ctrl / Shift / Alt 的左右两键在统计上没有任何区别，合并后才能让
// 热力图和 TOP 榜单上只出现一个「Shift」，而不是两行各占一半次数。
// Win 键不合并：左右 Win 键位不同，热力图上要分开显示。
func NormalizeVK(vk uint16) uint16 {
	switch vk {
	case VKLControl, VKRControl:
		return VKControl
	case VKLShift, VKRShift:
		return VKShift
	case VKLMenu, VKRMenu:
		return VKMenu
	}
	return vk
}
