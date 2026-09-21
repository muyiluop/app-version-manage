package service

import "strconv"

// itoa 无依赖的整数转字符串，避免在热点路径引入 fmt 开销。
func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }
