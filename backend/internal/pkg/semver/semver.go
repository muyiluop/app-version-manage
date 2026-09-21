// Package semver 提供宽松的语义化版本解析与比较。
//
// 兼容非严格 semver：允许 "v1.2.3"、"1.1.26245.26815"、"1.0.0-beta.1"。
// 主版本段最多取 4 段（major.minor.patch.build），缺失段按 0 处理。
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Version 解析后的版本号。
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Build      int
	Prerelease string
	Raw        string
}

// Parse 解析版本字符串。
func Parse(raw string) (Version, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Version{}, fmt.Errorf("版本号不能为空")
	}

	v := Version{Raw: trimmed}
	body := strings.TrimLeft(trimmed, "vV")

	// 去掉构建元数据
	if idx := strings.IndexByte(body, '+'); idx >= 0 {
		body = body[:idx]
	}
	// 提取预发布标识
	if idx := strings.IndexByte(body, '-'); idx >= 0 {
		v.Prerelease = body[idx+1:]
		body = body[:idx]
	}
	if body == "" {
		return Version{}, fmt.Errorf("版本号 %q 非法", raw)
	}

	parts := strings.Split(body, ".")
	if len(parts) > 4 {
		return Version{}, fmt.Errorf("版本号 %q 段数过多（最多 4 段）", raw)
	}

	nums := make([]int, 4)
	for i, part := range parts {
		if part == "" {
			return Version{}, fmt.Errorf("版本号 %q 存在空段", raw)
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, fmt.Errorf("版本号 %q 的第 %d 段 %q 不是数字", raw, i+1, part)
		}
		if n < 0 {
			return Version{}, fmt.Errorf("版本号 %q 的第 %d 段不能为负数", raw, i+1)
		}
		nums[i] = n
	}

	v.Major, v.Minor, v.Patch, v.Build = nums[0], nums[1], nums[2], nums[3]
	return v, nil
}

// MustParse 解析失败时 panic，仅用于常量场景。
func MustParse(raw string) Version {
	v, err := Parse(raw)
	if err != nil {
		panic(err)
	}
	return v
}

// Compare 比较两个版本：a<b 返回 -1，a==b 返回 0，a>b 返回 1。
// 预发布版本小于同号正式版本。
func Compare(a, b Version) int {
	if c := compareInt(a.Major, b.Major); c != 0 {
		return c
	}
	if c := compareInt(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := compareInt(a.Patch, b.Patch); c != 0 {
		return c
	}
	if c := compareInt(a.Build, b.Build); c != 0 {
		return c
	}
	// 正式版本（无预发布标识）优先级更高
	switch {
	case a.Prerelease == "" && b.Prerelease == "":
		return 0
	case a.Prerelease == "":
		return 1
	case b.Prerelease == "":
		return -1
	default:
		return strings.Compare(a.Prerelease, b.Prerelease)
	}
}

// GreaterThan 判断 a 是否高于 b。
func (a Version) GreaterThan(b Version) bool { return Compare(a, b) > 0 }

// String 返回原始字符串。
func (v Version) String() string { return v.Raw }

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
