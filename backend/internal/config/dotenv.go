package config

import (
	"fmt"
	"os"
	"strings"
)

// DotEnvResult 环境文件的加载结果。
type DotEnvResult struct {
	// Path 实际载入的文件；为空表示候选文件都不存在。
	Path string
	// Loaded 实际写入进程环境的变量个数。
	Loaded int
	// Skipped 因环境里已存在而保留原值、未被文件覆盖的个数。
	Skipped int
}

// LoadDotEnv 按顺序尝试候选文件，载入第一个存在的那个。
//
// 约定（与常见 dotenv 实现保持一致）：
//   - 支持 KEY=VALUE、export KEY=VALUE、# 注释与空行；
//   - 值可用单引号（原样）或双引号（识别 \n \t \r \" \\ 转义）包裹；
//   - **已存在的环境变量不会被覆盖**：真实环境变量优先级最高，
//     这样同一份 .env 既能用于本地开发，也不会污染 CI/容器里显式注入的值。
//
// 找不到任何候选文件时返回空结果且不报错（本地不写 .env 也应能启动）。
func LoadDotEnv(candidates []string) (DotEnvResult, error) {
	for _, path := range candidates {
		if strings.TrimSpace(path) == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return DotEnvResult{}, fmt.Errorf("读取环境文件 %s 失败: %w", path, err)
		}
		loaded, skipped, err := applyDotEnv(string(data))
		if err != nil {
			return DotEnvResult{}, fmt.Errorf("解析环境文件 %s 失败: %w", path, err)
		}
		return DotEnvResult{Path: path, Loaded: loaded, Skipped: skipped}, nil
	}
	return DotEnvResult{}, nil
}

// applyDotEnv 逐行解析并写入进程环境。
func applyDotEnv(content string) (loaded, skipped int, err error) {
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))

		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			// 非 KEY=VALUE 的行直接跳过：不因为一行写错就拒绝启动
			continue
		}

		key := strings.TrimSpace(line[:eq])
		if !validEnvKey(key) {
			return 0, 0, fmt.Errorf("第 %d 行的变量名不合法: %q", i+1, key)
		}

		value := unquoteEnvValue(strings.TrimSpace(line[eq+1:]))
		if _, exists := os.LookupEnv(key); exists {
			skipped++
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return 0, 0, fmt.Errorf("设置 %s 失败: %w", key, err)
		}
		loaded++
	}
	return loaded, skipped, nil
}

// validEnvKey 校验变量名，避免把 YAML 片段或说明文字当成变量写进环境。
func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r == '_':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// unquoteEnvValue 去掉包裹引号；未加引号时去掉行尾注释。
func unquoteEnvValue(v string) string {
	if len(v) >= 2 {
		if v[0] == '\'' && v[len(v)-1] == '\'' {
			return v[1 : len(v)-1] // 单引号：完全原样
		}
		if v[0] == '"' && v[len(v)-1] == '"' {
			return unescapeDoubleQuoted(v[1 : len(v)-1])
		}
	}
	// 去掉 " # 注释" 形式（只认前面有空格的 #，避免误伤密码里的 #）
	if idx := strings.Index(v, " #"); idx >= 0 {
		v = strings.TrimSpace(v[:idx])
	}
	return v
}

// unescapeDoubleQuoted 处理双引号内的常见转义。
func unescapeDoubleQuoted(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '"':
			b.WriteByte('"')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
