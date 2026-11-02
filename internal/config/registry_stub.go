//go:build !windows

package config

import "errors"

// OpenRunRegistry 在非 Windows 平台不可用。
func OpenRunRegistry() (Registry, error) {
	return nil, errors.New("config: 开机自启仅支持 Windows")
}
