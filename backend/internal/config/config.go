// Package config 负责加载、合并与校验运行配置。
//
// 加载优先级：代码默认值 < YAML 配置文件 < 环境变量（APPV_ 前缀）。
// 生产模式（server.mode=release）下缺失关键密钥会直接拒绝启动；
// 开发模式下缺失的密钥会临时随机生成并在启动日志中高亮告警。
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// EnvPrefix 所有环境变量的统一前缀。
const EnvPrefix = "APPV_"

// 支持的取值。
const (
	ModeDebug   = "debug"
	ModeRelease = "release"

	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
	DriverMySQL    = "mysql"

	StorageLocal = "local"
	StorageS3    = "s3"
)

// Config 是应用的完整配置。
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Security SecurityConfig `yaml:"security"`
	Storage  StorageConfig  `yaml:"storage"`
	Log      LogConfig      `yaml:"log"`
	Upload   UploadConfig   `yaml:"upload"`
	CORS     CORSConfig     `yaml:"cors"`

	// generatedSecrets 记录开发模式下临时生成的密钥名，供启动日志告警。
	generatedSecrets []string
}

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	Port    int    `yaml:"port"`
	Mode    string `yaml:"mode"`
	BaseURL string `yaml:"baseUrl"`
	// TrustedProxies 列出可信反向代理地址。只有来自这些地址的
	// X-Forwarded-For 才会被用于识别客户端 IP，避免限流被伪造头绕过。
	// 置空表示不信任任何代理（ClientIP 直接取 RemoteAddr）。
	TrustedProxies []string `yaml:"trustedProxies"`
}

// IsRelease 是否生产模式。
func (s ServerConfig) IsRelease() bool { return s.Mode == ModeRelease }

// DatabaseConfig 数据库配置，支持 sqlite / postgres / mysql。
type DatabaseConfig struct {
	Driver       string `yaml:"driver"`
	Path         string `yaml:"path"` // 仅 sqlite
	DSN          string `yaml:"dsn"`  // postgres / mysql
	MaxOpenConns int    `yaml:"maxOpenConns"`
	MaxIdleConns int    `yaml:"maxIdleConns"`
	// AutoMigrate 用指针：需要区分“未配置”（默认启用）与“显式关闭”。
	AutoMigrate *bool `yaml:"autoMigrate"`
	LogSQL      bool  `yaml:"logSql"`
}

// AutoMigrateEnabled 是否在启动时自动补齐表结构，默认启用。
func (d DatabaseConfig) AutoMigrateEnabled() bool {
	return d.AutoMigrate == nil || *d.AutoMigrate
}

// JWTConfig 令牌配置。
type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expireHours"`
	RefreshDays int    `yaml:"refreshDays"`
	Issuer      string `yaml:"issuer"`
}

// SecurityConfig 安全相关配置。
type SecurityConfig struct {
	DownloadTokenKey     string `yaml:"downloadTokenKey"`
	SharePasswordPepper  string `yaml:"sharePasswordPepper"`
	AdminUsername        string `yaml:"adminUsername"`
	AdminInitialPassword string `yaml:"adminInitialPassword"`

	RateLimitLoginPerMin int `yaml:"rateLimitLoginPerMin"`
	RateLimitSharePerMin int `yaml:"rateLimitSharePerMin"`
	RateLimitOpenPerMin  int `yaml:"rateLimitOpenPerMin"`
}

// UseSSLEnabled 是否使用 HTTPS 访问对象存储，默认 false。
func (s S3Storage) UseSSLEnabled() bool { return s.UseSSL != nil && *s.UseSSL }

// ForcePathStyleEnabled 是否使用 path-style 访问，默认 true（自建 MinIO 常用；
// 连接 AWS S3 或带虚主机名的网关时应显式设为 false）。
func (s S3Storage) ForcePathStyleEnabled() bool {
	return s.ForcePathStyle == nil || *s.ForcePathStyle
}

// StorageConfig 存储配置，支持 local / s3（MinIO、AWS S3 及兼容实现）。
type StorageConfig struct {
	Driver string `yaml:"driver"`
	// Path 为兼容旧配置 storage.path 而保留，等价于 local.root。
	Path                string       `yaml:"path"`
	Local               LocalStorage `yaml:"local"`
	S3                  S3Storage    `yaml:"s3"`
	SignedURLTTLMinutes int          `yaml:"signedUrlTtlMinutes"`
}

// LocalStorage 本地磁盘存储配置。
type LocalStorage struct {
	Root string `yaml:"root"`
}

// S3Storage 对象存储配置。
type S3Storage struct {
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"accessKey"`
	SecretKey string `yaml:"secretKey"`
	// UseSSL / ForcePathStyle 用指针：需要区分“未配置”（用默认值）与“显式 false”。
	// 尤其 ForcePathStyle：自建 MinIO 通常要 true，而 AWS S3 必须 false。
	UseSSL         *bool  `yaml:"useSsl"`
	ForcePathStyle *bool  `yaml:"forcePathStyle"`
	Prefix         string `yaml:"prefix"`
}

// LogConfig 日志配置。
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// UploadConfig 上传限制。
type UploadConfig struct {
	MaxSizeMB         int      `yaml:"maxSizeMB"`
	AllowedExtensions []string `yaml:"allowedExtensions"`
}

// CORSConfig 跨域配置。
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowedOrigins"`
}

// Default 返回内置默认配置。
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 9080,
			Mode: ModeDebug,
			// 默认部署是同机 nginx 反向代理
			TrustedProxies: []string{"127.0.0.1", "::1"},
		},
		Database: DatabaseConfig{
			Driver:       DriverSQLite,
			Path:         "./data/app_version.db",
			MaxOpenConns: 20,
			MaxIdleConns: 5,
			AutoMigrate:  boolPtr(true),
		},
		JWT: JWTConfig{ExpireHours: 24, RefreshDays: 7, Issuer: "app-version-manage"},
		Security: SecurityConfig{
			AdminUsername:        "admin",
			RateLimitLoginPerMin: 10,
			RateLimitSharePerMin: 20,
			RateLimitOpenPerMin:  600,
		},
		Storage: StorageConfig{
			Driver:              StorageLocal,
			Local:               LocalStorage{Root: "./static/uploads"},
			SignedURLTTLMinutes: 30,
			S3: S3Storage{
				// 默认留空：由 minio-go 探测 bucket 所在 region。
				// 写死 us-east-1 会让配置了非默认 region 的自建 MinIO 拒绝签名
				// （报错：the region is wrong; expecting 'xxx'）。
				ForcePathStyle: boolPtr(true), // 默认按自建 MinIO 的常见配置
				Prefix:         "appv",
			},
		},
		Log:    LogConfig{Level: "info", Format: "text"},
		Upload: UploadConfig{MaxSizeMB: 4096},
	}
}

// Load 读取配置文件（可为空路径/不存在）并应用环境变量覆盖，最后执行校验。
func Load(configPath string) (*Config, error) {
	cfg := Default()

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		switch {
		case err == nil:
			var fileCfg Config
			if err := yaml.Unmarshal(data, &fileCfg); err != nil {
				return nil, fmt.Errorf("解析配置文件 %s 失败: %w", configPath, err)
			}
			merge(cfg, &fileCfg)
		case os.IsNotExist(err):
			// 配置文件缺失时使用默认值，交由调用方提示。
		default:
			return nil, fmt.Errorf("读取配置文件 %s 失败: %w", configPath, err)
		}
	}

	applyEnv(cfg)
	normalize(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// merge 将文件配置中“有值”的字段覆盖到基础配置上。
func merge(base, file *Config) {
	if file.Server.Port != 0 {
		base.Server.Port = file.Server.Port
	}
	if file.Server.Mode != "" {
		base.Server.Mode = file.Server.Mode
	}
	if file.Server.BaseURL != "" {
		base.Server.BaseURL = file.Server.BaseURL
	}
	if len(file.Server.TrustedProxies) > 0 {
		base.Server.TrustedProxies = file.Server.TrustedProxies
	}

	if file.Database.Driver != "" {
		base.Database.Driver = file.Database.Driver
	}
	if file.Database.Path != "" {
		base.Database.Path = file.Database.Path
	}
	if file.Database.DSN != "" {
		base.Database.DSN = file.Database.DSN
	}
	if file.Database.MaxOpenConns != 0 {
		base.Database.MaxOpenConns = file.Database.MaxOpenConns
	}
	if file.Database.MaxIdleConns != 0 {
		base.Database.MaxIdleConns = file.Database.MaxIdleConns
	}
	if file.Database.AutoMigrate != nil {
		base.Database.AutoMigrate = file.Database.AutoMigrate
	}
	if file.Database.LogSQL {
		base.Database.LogSQL = true
	}

	if file.JWT.Secret != "" {
		base.JWT.Secret = file.JWT.Secret
	}
	if file.JWT.ExpireHours != 0 {
		base.JWT.ExpireHours = file.JWT.ExpireHours
	}
	if file.JWT.RefreshDays != 0 {
		base.JWT.RefreshDays = file.JWT.RefreshDays
	}
	if file.JWT.Issuer != "" {
		base.JWT.Issuer = file.JWT.Issuer
	}

	if file.Security.DownloadTokenKey != "" {
		base.Security.DownloadTokenKey = file.Security.DownloadTokenKey
	}
	if file.Security.SharePasswordPepper != "" {
		base.Security.SharePasswordPepper = file.Security.SharePasswordPepper
	}
	if file.Security.AdminUsername != "" {
		base.Security.AdminUsername = file.Security.AdminUsername
	}
	if file.Security.AdminInitialPassword != "" {
		base.Security.AdminInitialPassword = file.Security.AdminInitialPassword
	}
	if file.Security.RateLimitLoginPerMin != 0 {
		base.Security.RateLimitLoginPerMin = file.Security.RateLimitLoginPerMin
	}
	if file.Security.RateLimitSharePerMin != 0 {
		base.Security.RateLimitSharePerMin = file.Security.RateLimitSharePerMin
	}
	if file.Security.RateLimitOpenPerMin != 0 {
		base.Security.RateLimitOpenPerMin = file.Security.RateLimitOpenPerMin
	}

	if file.Storage.Driver != "" {
		base.Storage.Driver = file.Storage.Driver
	}
	if file.Storage.Path != "" {
		base.Storage.Path = file.Storage.Path
	}
	if file.Storage.Local.Root != "" {
		base.Storage.Local.Root = file.Storage.Local.Root
	}
	if file.Storage.S3.Endpoint != "" {
		base.Storage.S3.Endpoint = file.Storage.S3.Endpoint
	}
	if file.Storage.S3.Region != "" {
		base.Storage.S3.Region = file.Storage.S3.Region
	}
	if file.Storage.S3.Bucket != "" {
		base.Storage.S3.Bucket = file.Storage.S3.Bucket
	}
	if file.Storage.S3.AccessKey != "" {
		base.Storage.S3.AccessKey = file.Storage.S3.AccessKey
	}
	if file.Storage.S3.SecretKey != "" {
		base.Storage.S3.SecretKey = file.Storage.S3.SecretKey
	}
	if file.Storage.S3.UseSSL != nil {
		base.Storage.S3.UseSSL = file.Storage.S3.UseSSL
	}
	if file.Storage.S3.ForcePathStyle != nil {
		base.Storage.S3.ForcePathStyle = file.Storage.S3.ForcePathStyle
	}
	if file.Storage.S3.Prefix != "" {
		base.Storage.S3.Prefix = file.Storage.S3.Prefix
	}
	if file.Storage.SignedURLTTLMinutes != 0 {
		base.Storage.SignedURLTTLMinutes = file.Storage.SignedURLTTLMinutes
	}

	if file.Log.Level != "" {
		base.Log.Level = file.Log.Level
	}
	if file.Log.Format != "" {
		base.Log.Format = file.Log.Format
	}
	if file.Upload.MaxSizeMB != 0 {
		base.Upload.MaxSizeMB = file.Upload.MaxSizeMB
	}
	if len(file.Upload.AllowedExtensions) > 0 {
		base.Upload.AllowedExtensions = file.Upload.AllowedExtensions
	}
	if len(file.CORS.AllowedOrigins) > 0 {
		base.CORS.AllowedOrigins = file.CORS.AllowedOrigins
	}
}

// applyEnv 应用环境变量覆盖。
func applyEnv(cfg *Config) {
	cfg.Server.Port = envInt(cfg.Server.Port, "SERVER_PORT")
	cfg.Server.Mode = envString(cfg.Server.Mode, "SERVER_MODE")
	cfg.Server.BaseURL = envString(cfg.Server.BaseURL, "SERVER_BASE_URL")

	// none/空 表示不信任任何代理；* 表示信任全部（仅限外层已有可信网关时使用）
	if v, ok := os.LookupEnv(EnvPrefix + "SERVER_TRUSTED_PROXIES"); ok {
		switch strings.TrimSpace(v) {
		case "", "none":
			cfg.Server.TrustedProxies = nil
		case "*":
			cfg.Server.TrustedProxies = []string{"0.0.0.0/0", "::/0"}
		default:
			cfg.Server.TrustedProxies = splitCSV(v)
		}
	}

	cfg.Database.Driver = envString(cfg.Database.Driver, "DATABASE_DRIVER")
	cfg.Database.Path = envString(cfg.Database.Path, "DATABASE_PATH")
	cfg.Database.DSN = envString(cfg.Database.DSN, "DATABASE_DSN")
	cfg.Database.MaxOpenConns = envInt(cfg.Database.MaxOpenConns, "DATABASE_MAX_OPEN_CONNS")
	cfg.Database.MaxIdleConns = envInt(cfg.Database.MaxIdleConns, "DATABASE_MAX_IDLE_CONNS")
	if v := envBoolPtr("DATABASE_AUTO_MIGRATE"); v != nil {
		cfg.Database.AutoMigrate = v
	}
	cfg.Database.LogSQL = envBool(cfg.Database.LogSQL, "DATABASE_LOG_SQL")

	cfg.JWT.Secret = envString(cfg.JWT.Secret, "JWT_SECRET")
	cfg.JWT.ExpireHours = envInt(cfg.JWT.ExpireHours, "JWT_EXPIRE_HOURS")
	cfg.JWT.RefreshDays = envInt(cfg.JWT.RefreshDays, "JWT_REFRESH_DAYS")

	cfg.Security.DownloadTokenKey = envString(cfg.Security.DownloadTokenKey, "DOWNLOAD_TOKEN_KEY")
	cfg.Security.SharePasswordPepper = envString(cfg.Security.SharePasswordPepper, "SHARE_PASSWORD_PEPPER")
	cfg.Security.AdminUsername = envString(cfg.Security.AdminUsername, "ADMIN_USERNAME")
	cfg.Security.AdminInitialPassword = envString(cfg.Security.AdminInitialPassword, "ADMIN_INITIAL_PASSWORD")

	cfg.Storage.Driver = envString(cfg.Storage.Driver, "STORAGE_DRIVER")
	cfg.Storage.Local.Root = envString(cfg.Storage.Local.Root, "STORAGE_LOCAL_ROOT")
	cfg.Storage.S3.Endpoint = envString(cfg.Storage.S3.Endpoint, "STORAGE_S3_ENDPOINT")
	cfg.Storage.S3.Region = envString(cfg.Storage.S3.Region, "STORAGE_S3_REGION")
	cfg.Storage.S3.Bucket = envString(cfg.Storage.S3.Bucket, "STORAGE_S3_BUCKET")
	cfg.Storage.S3.AccessKey = envString(cfg.Storage.S3.AccessKey, "STORAGE_S3_ACCESS_KEY")
	cfg.Storage.S3.SecretKey = envString(cfg.Storage.S3.SecretKey, "STORAGE_S3_SECRET_KEY")
	if v := envBoolPtr("STORAGE_S3_USE_SSL"); v != nil {
		cfg.Storage.S3.UseSSL = v
	}
	if v := envBoolPtr("STORAGE_S3_FORCE_PATH_STYLE"); v != nil {
		cfg.Storage.S3.ForcePathStyle = v
	}
	cfg.Storage.S3.Prefix = envString(cfg.Storage.S3.Prefix, "STORAGE_S3_PREFIX")
	cfg.Storage.SignedURLTTLMinutes = envInt(cfg.Storage.SignedURLTTLMinutes, "STORAGE_SIGNED_URL_TTL_MINUTES")

	cfg.Log.Level = envString(cfg.Log.Level, "LOG_LEVEL")
	cfg.Log.Format = envString(cfg.Log.Format, "LOG_FORMAT")
	cfg.Upload.MaxSizeMB = envInt(cfg.Upload.MaxSizeMB, "UPLOAD_MAX_SIZE_MB")

	if v := envString("", "CORS_ALLOWED_ORIGINS"); v != "" {
		cfg.CORS.AllowedOrigins = splitCSV(v)
	}
}

// normalize 统一别名与取值大小写。
func normalize(cfg *Config) {
	cfg.Server.Mode = strings.ToLower(strings.TrimSpace(cfg.Server.Mode))
	cfg.Database.Driver = strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	cfg.Storage.Driver = strings.ToLower(strings.TrimSpace(cfg.Storage.Driver))
	cfg.Log.Level = strings.ToLower(strings.TrimSpace(cfg.Log.Level))
	cfg.Log.Format = strings.ToLower(strings.TrimSpace(cfg.Log.Format))

	// 兼容旧配置：storage.path 等价于 local.root
	if cfg.Storage.Local.Root == "" && cfg.Storage.Path != "" {
		cfg.Storage.Local.Root = cfg.Storage.Path
	}
	if cfg.Storage.Path == "" {
		cfg.Storage.Path = cfg.Storage.Local.Root
	}
}

// validate 校验配置的完整性。
func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port 非法: %d", cfg.Server.Port)
	}
	switch cfg.Server.Mode {
	case ModeDebug, ModeRelease:
	default:
		return fmt.Errorf("server.mode 必须为 debug 或 release，当前为 %q", cfg.Server.Mode)
	}

	switch cfg.Database.Driver {
	case DriverSQLite:
		if cfg.Database.Path == "" {
			return fmt.Errorf("database.driver=sqlite 时必须配置 database.path")
		}
	case DriverPostgres, DriverMySQL:
		if cfg.Database.DSN == "" {
			return fmt.Errorf("database.driver=%s 时必须配置 database.dsn", cfg.Database.Driver)
		}
	default:
		return fmt.Errorf("不支持的 database.driver: %q（可选 sqlite/postgres/mysql）", cfg.Database.Driver)
	}

	switch cfg.Storage.Driver {
	case StorageLocal:
		if cfg.Storage.Local.Root == "" {
			return fmt.Errorf("storage.driver=local 时必须配置 storage.local.root")
		}
	case StorageS3:
		s3 := cfg.Storage.S3
		if s3.Bucket == "" || s3.AccessKey == "" || s3.SecretKey == "" {
			return fmt.Errorf("storage.driver=s3 时必须配置 storage.s3.bucket/accessKey/secretKey")
		}
	default:
		return fmt.Errorf("不支持的 storage.driver: %q（可选 local/s3）", cfg.Storage.Driver)
	}

	if cfg.Upload.MaxSizeMB <= 0 {
		return fmt.Errorf("upload.maxSizeMB 必须大于 0")
	}
	if cfg.JWT.ExpireHours <= 0 || cfg.JWT.RefreshDays <= 0 {
		return fmt.Errorf("jwt.expireHours 与 jwt.refreshDays 必须大于 0")
	}

	return cfg.fillSecrets()
}

// fillSecrets 校验密钥强度：生产模式缺失即失败，开发模式随机生成并告警。
func (cfg *Config) fillSecrets() error {
	required := []struct {
		name   string
		envKey string
		value  *string
	}{
		{"jwt.secret", "JWT_SECRET", &cfg.JWT.Secret},
		{"security.downloadTokenKey", "DOWNLOAD_TOKEN_KEY", &cfg.Security.DownloadTokenKey},
	}

	for _, s := range required {
		switch {
		case strings.TrimSpace(*s.value) == "":
			if cfg.Server.IsRelease() {
				return fmt.Errorf("生产模式必须配置 %s（环境变量 %s%s）", s.name, EnvPrefix, s.envKey)
			}
			generated, err := randomHex(32)
			if err != nil {
				return err
			}
			*s.value = generated
			cfg.generatedSecrets = append(cfg.generatedSecrets, s.name)
		case cfg.Server.IsRelease() && len(*s.value) < 16:
			return fmt.Errorf("%s 长度不足 16，生产环境请使用足够强度的密钥", s.name)
		}
	}
	return nil
}

// GeneratedSecrets 返回开发模式下临时生成的密钥名称列表。
func (cfg *Config) GeneratedSecrets() []string { return cfg.generatedSecrets }

// Redacted 返回可安全写入日志的配置摘要。
func (cfg *Config) Redacted() map[string]any {
	return map[string]any{
		"server":   map[string]any{"port": cfg.Server.Port, "mode": cfg.Server.Mode},
		"database": map[string]any{"driver": cfg.Database.Driver, "path": cfg.Database.Path, "dsn": redact(cfg.Database.DSN)},
		"storage":  map[string]any{"driver": cfg.Storage.Driver, "root": cfg.Storage.Local.Root, "bucket": cfg.Storage.S3.Bucket},
		"log":      map[string]any{"level": cfg.Log.Level, "format": cfg.Log.Format},
	}
}

func redact(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成随机密钥失败: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func envString(def, key string) string {
	if v, ok := os.LookupEnv(EnvPrefix + key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func envInt(def int, key string) int {
	v, ok := os.LookupEnv(EnvPrefix + key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

// boolPtr 返回布尔值指针，用于区分“未配置”与“显式 false”。
func boolPtr(v bool) *bool { return &v }

// envBoolPtr 环境变量存在且可解析时返回其值，否则返回 nil（表示未配置）。
func envBoolPtr(key string) *bool {
	v, ok := os.LookupEnv(EnvPrefix + key)
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return nil
	}
	return &b
}

func envBool(def bool, key string) bool {
	v, ok := os.LookupEnv(EnvPrefix + key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return b
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
