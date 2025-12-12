package config

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Storage  StorageConfig  `yaml:"storage"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type DatabaseConfig struct {
	Type string `yaml:"type"`
	Path string `yaml:"path"`
}

type JWTConfig struct {
	Secret     string `yaml:"secret"`
	ExpireTime int    `yaml:"expireTime"` // 过期时间（小时）
}

type StorageConfig struct {
	Path string `yaml:"path"` // 文件存储路径
}

var GlobalConfig Config

func init() {
	// 默认配置
	GlobalConfig = Config{
		Server: ServerConfig{
			Port: 9080,
		},
		Database: DatabaseConfig{
			Type: "sqlite",
			Path: "data/app_version.db",
		},
		JWT: JWTConfig{
			Secret:     "attach@2025",
			ExpireTime: 24,
		},
		Storage: StorageConfig{
			Path: "static/uploads",
		},
	}
}
