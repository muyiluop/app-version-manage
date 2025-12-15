package config

import (
	"errors"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

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

// Override 为命令行传入的覆盖选项，使用哨兵值避免与有效值混淆。
type Override struct {
	Port      int
	DBPath    string
	Storage   string
	JWTSecret string
	JWTExpire int
}

var GlobalConfig Config

func defaultConfig() Config {
	return Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Type: "sqlite", Path: "/app/data/app_version.db"},
		JWT:      JWTConfig{Secret: "attach@2025", ExpireTime: 24},
		Storage:  StorageConfig{Path: "static/uploads"},
	}
}

// Load 从文件加载配置，文件不存在时保持默认值；随后应用覆盖参数。
func Load(configPath string, override Override) error {
	GlobalConfig = defaultConfig()

	if configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			var fileCfg Config
			if err := yaml.Unmarshal(data, &fileCfg); err != nil {
				return err
			}
			log.Println("Loaded configuration from", configPath)
			mergeConfig(&GlobalConfig, &fileCfg)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	applyOverride(&GlobalConfig, override)
	return nil
}

func mergeConfig(base *Config, override *Config) {
	if override.Server.Port != 0 {
		base.Server.Port = override.Server.Port
	}

	if override.Database.Type != "" {
		base.Database.Type = override.Database.Type
	}
	if override.Database.Path != "" {
		base.Database.Path = override.Database.Path
	}

	if override.JWT.Secret != "" {
		base.JWT.Secret = override.JWT.Secret
	}
	if override.JWT.ExpireTime != 0 {
		base.JWT.ExpireTime = override.JWT.ExpireTime
	}

	if override.Storage.Path != "" {
		base.Storage.Path = override.Storage.Path
	}
}

func applyOverride(cfg *Config, o Override) {
	if o.Port > 0 {
		cfg.Server.Port = o.Port
	}
	if o.DBPath != "" {
		cfg.Database.Path = o.DBPath
	}
	if o.Storage != "" {
		cfg.Storage.Path = o.Storage
	}
	if o.JWTSecret != "" {
		cfg.JWT.Secret = o.JWTSecret
	}
	if o.JWTExpire > 0 {
		cfg.JWT.ExpireTime = o.JWTExpire
	}
}
