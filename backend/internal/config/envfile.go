package config

import "strings"

// EnvFileCandidates 计算环境文件的候选路径，按优先级从高到低：
//
//  1. 命令行 -env-file 指定的路径
//  2. 环境变量 APPV_ENV_FILE
//  3. 进程当前目录下的 .env
//  4. 上一级目录的 .env（便于在 backend/ 下直接 go run，而把 .env 放在仓库根）
//
// 空字符串会被 LoadDotEnv 忽略，因此这里可以放心地把「未指定」也放进去。
func EnvFileCandidates(explicit string) []string {
	candidates := []string{strings.TrimSpace(explicit)}
	if v := strings.TrimSpace(envString("", "ENV_FILE")); v != "" {
		candidates = append(candidates, v)
	}
	candidates = append(candidates, ".env", "../.env")
	return candidates
}
