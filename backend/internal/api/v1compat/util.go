package v1compat

import "strconv"

// parseInt 解析十进制整数，失败返回错误。
func parseInt(v string) (int, error) { return strconv.Atoi(v) }
