// Package model 定义领域实体与数据库映射。
//
// 兼容性原则：为兼容既有 SQLite 数据库，部分字段保留旧列名
// （如 versions.file_path、files.path、files.hash、shares.password），
// 通过 gorm 的 column 标签映射，Go 侧使用更准确的命名。
package model

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Platform 支持的客户端平台。
type Platform string

const (
	PlatformAndroid Platform = "android"
	PlatformIOS     Platform = "ios"
	PlatformWindows Platform = "windows"
	PlatformMacOS   Platform = "macos"
	PlatformLinux   Platform = "linux"
	PlatformHarmony Platform = "harmony"
)

// AllPlatforms 全部受支持平台，顺序与前端展示一致。
var AllPlatforms = []Platform{
	PlatformAndroid, PlatformIOS, PlatformWindows, PlatformMacOS, PlatformLinux, PlatformHarmony,
}

// Valid 判断平台取值是否合法。
func (p Platform) Valid() bool {
	for _, item := range AllPlatforms {
		if item == p {
			return true
		}
	}
	return false
}

// PlatformList 平台列表，以 JSON 文本形式落库，保证三种数据库行为一致。
type PlatformList []Platform

// Strings 返回字符串切片形式。
func (l PlatformList) Strings() []string {
	out := make([]string, 0, len(l))
	for _, p := range l {
		out = append(out, string(p))
	}
	return out
}

// Contains 判断是否包含指定平台。
func (l PlatformList) Contains(p Platform) bool {
	for _, item := range l {
		if item == p {
			return true
		}
	}
	return false
}

// ParsePlatforms 将字符串切片解析为平台列表，忽略非法值。
func ParsePlatforms(values []string) PlatformList {
	out := make(PlatformList, 0, len(values))
	for _, v := range values {
		p := Platform(strings.ToLower(strings.TrimSpace(v)))
		if p.Valid() {
			out = append(out, p)
		}
	}
	return out
}

// Role 用户角色。
type Role string

const (
	// RoleAdmin 管理员：全部权限，含用户管理。
	RoleAdmin Role = "admin"
	// RoleReleaser 发布员：应用、版本、文件、分享、模板的读写。
	RoleReleaser Role = "releaser"
	// RoleViewer 只读。
	RoleViewer Role = "viewer"
)

// AllRoles 全部角色。
var AllRoles = []Role{RoleAdmin, RoleReleaser, RoleViewer}

// Valid 判断角色是否合法。
func (r Role) Valid() bool {
	for _, item := range AllRoles {
		if item == r {
			return true
		}
	}
	return false
}

// CanWrite 是否具备写权限。
func (r Role) CanWrite() bool { return r == RoleAdmin || r == RoleReleaser }

// CanManageUsers 是否可管理用户。
func (r Role) CanManageUsers() bool { return r == RoleAdmin }

// VersionStatus 版本状态。
type VersionStatus string

const (
	// VersionDraft 草稿：仅后台可见。
	VersionDraft VersionStatus = "draft"
	// VersionPublished 已发布：客户端可见、可下载。
	VersionPublished VersionStatus = "published"
	// VersionArchived 已下架：客户端不可见，数据保留。
	VersionArchived VersionStatus = "archived"
)

// Valid 判断状态是否合法。
func (s VersionStatus) Valid() bool {
	switch s {
	case VersionDraft, VersionPublished, VersionArchived:
		return true
	}
	return false
}

// DefaultChannelKey 默认发布通道。
const DefaultChannelKey = "stable"

// DefaultChannels 新建应用时自动创建的通道。
var DefaultChannels = []struct {
	Key       string
	Name      string
	IsDefault bool
	Sort      int
}{
	{Key: "stable", Name: "稳定版", IsDefault: true, Sort: 1},
	{Key: "beta", Name: "测试版", IsDefault: false, Sort: 2},
}

// Application 应用。
type Application struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	Name           string         `json:"name" gorm:"size:100;not null"`
	Identifier     string         `json:"identifier" gorm:"size:100;not null;uniqueIndex:idx_apps_identifier"`
	Logo           string         `json:"logo" gorm:"size:512"`
	Description    string         `json:"description" gorm:"type:text"`
	Platforms      PlatformList   `json:"platforms" gorm:"type:text;serializer:json"`
	DefaultChannel string         `json:"defaultChannel" gorm:"size:32;not null;default:stable"`
	CreatedBy      uint           `json:"createdBy"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

// Channel 应用下的发布通道（stable / beta / hotfix ...）。
type Channel struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	AppID     uint      `json:"appId" gorm:"not null;index:idx_channels_app;uniqueIndex:idx_channels_app_key,priority:1"`
	Key       string    `json:"key" gorm:"size:32;not null;uniqueIndex:idx_channels_app_key,priority:2"`
	Name      string    `json:"name" gorm:"size:64"`
	IsDefault bool      `json:"isDefault"`
	Sort      int       `json:"sort"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Version 应用版本。
//
// 唯一性由 (app_id, platform, channel, version, deleted_at) 联合索引保证：
// SQLite / PostgreSQL / MySQL 中 NULL 互不相等，因此同一版本软删后可重新发布。
type Version struct {
	ID       uint     `json:"id" gorm:"primaryKey"`
	AppID    uint     `json:"appId" gorm:"not null;uniqueIndex:idx_versions_unique,priority:1;index:idx_versions_lookup,priority:1"`
	Platform Platform `json:"platform" gorm:"size:20;not null;uniqueIndex:idx_versions_unique,priority:2;index:idx_versions_lookup,priority:2"`
	Channel  string   `json:"channel" gorm:"size:32;not null;default:stable;uniqueIndex:idx_versions_unique,priority:3;index:idx_versions_lookup,priority:3"`
	Version  string   `json:"version" gorm:"size:64;not null;uniqueIndex:idx_versions_unique,priority:4"`

	// 语义化版本的数值分解，用于排序（避免按字符串或创建时间排序）。
	VersionMajor int    `json:"-" gorm:"not null;default:0"`
	VersionMinor int    `json:"-" gorm:"not null;default:0"`
	VersionPatch int    `json:"-" gorm:"not null;default:0"`
	VersionBuild int    `json:"-" gorm:"not null;default:0"`
	Prerelease   string `json:"prerelease" gorm:"size:64"`

	// FileKey 为存储层对象键，列名保持 file_path 以兼容既有数据。
	FileKey     string `json:"fileKey" gorm:"column:file_path;size:512;not null"`
	FileName    string `json:"fileName" gorm:"size:255;not null"`
	FileSize    int64  `json:"fileSize" gorm:"not null;default:0"`
	FileSHA256  string `json:"fileSha256" gorm:"size:64;index"`
	ContentType string `json:"contentType" gorm:"size:128"`

	Changelog           string        `json:"changelog" gorm:"type:text"`
	Ext                 string        `json:"ext" gorm:"type:text"`
	ForceUpdate         bool          `json:"forceUpdate" gorm:"not null;default:false"`
	Status              VersionStatus `json:"status" gorm:"size:16;not null;default:published;index:idx_versions_lookup,priority:4"`
	MinSupportedVersion string        `json:"minSupportedVersion" gorm:"size:64"`

	PublishedAt   *time.Time `json:"publishedAt" gorm:"index"`
	DownloadCount int64      `json:"downloadCount" gorm:"not null;default:0"`

	CreatedBy uint      `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// 软删除参与唯一索引：活跃行 deleted_at 为 NULL，从而保证活跃版本唯一。
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:idx_versions_unique,priority:5"`
}

// IsPublished 是否处于已发布状态。
func (v Version) IsPublished() bool { return v.Status == VersionPublished }

// Template 输出模板，用于自定义开放接口返回格式。
type Template struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	AppID       uint      `json:"appId" gorm:"not null;index"`
	Name        string    `json:"name" gorm:"size:100;not null"`
	Description string    `json:"description" gorm:"size:255"`
	Content     string    `json:"content" gorm:"type:text;not null"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// File 已存储文件（版本产物、Logo 等）。
type File struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"size:255;not null"`
	Key         string `json:"key" gorm:"column:path;size:512;not null;index"`
	Size        int64  `json:"size" gorm:"not null;default:0"`
	ContentType string `json:"contentType" gorm:"column:type;size:128"`
	SHA256      string `json:"sha256" gorm:"size:64;index"`
	// MD5 兼容旧列 hash。
	MD5       string    `json:"md5" gorm:"column:hash;size:32;index"`
	Storage   string    `json:"storage" gorm:"size:16;not null;default:local"`
	RefCount  int64     `json:"refCount" gorm:"not null;default:0"`
	CreatedBy uint      `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// User 后台用户。
type User struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	Username     string     `json:"username" gorm:"size:64;not null;uniqueIndex"`
	Password     string     `json:"-" gorm:"size:128;not null"`
	DisplayName  string     `json:"displayName" gorm:"size:64"`
	Role         Role       `json:"role" gorm:"size:16;not null;default:viewer"`
	IsActive     bool       `json:"isActive" gorm:"not null;default:true"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	// TokenVersion 自增即可吊销该用户已签发的全部令牌。
	TokenVersion       int       `json:"-" gorm:"not null;default:1"`
	MustChangePassword bool      `json:"mustChangePassword" gorm:"not null;default:false"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// Share 应用分享链接。
type Share struct {
	ID    uint   `json:"id" gorm:"primaryKey"`
	AppID uint   `json:"appId" gorm:"not null;index"`
	Token string `json:"token" gorm:"size:128;not null;uniqueIndex"`

	// PasswordHash 为 bcrypt 哈希；历史数据中的 AES 密文保留在 PasswordLegacy（列名 password）。
	PasswordHash   string `json:"-" gorm:"size:128"`
	PasswordAlgo   string `json:"passwordAlgo" gorm:"size:16"`
	PasswordLegacy string `json:"-" gorm:"column:password;size:255"`

	ExpiresAt   *time.Time `json:"expiresAt"`
	IsActive    bool       `json:"isActive" gorm:"not null;default:true"`
	AccessCount int64      `json:"accessCount" gorm:"not null;default:0"`
	CreatedBy   uint       `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// AuditLog 审计日志。
type AuditLog struct {
	ID         uint64    `json:"id" gorm:"primaryKey"`
	ActorID    uint      `json:"actorId" gorm:"index"`
	ActorName  string    `json:"actorName" gorm:"size:64"`
	Action     string    `json:"action" gorm:"size:64;not null;index"`
	TargetType string    `json:"targetType" gorm:"size:32"`
	TargetID   string    `json:"targetId" gorm:"size:64"`
	Summary    string    `json:"summary" gorm:"size:512"`
	Detail     string    `json:"detail" gorm:"type:text"`
	IP         string    `json:"ip" gorm:"size:64"`
	UserAgent  string    `json:"userAgent" gorm:"size:256"`
	Success    bool      `json:"success" gorm:"not null;default:true"`
	CreatedAt  time.Time `json:"createdAt" gorm:"index"`
}

// MigrateModels 返回需要自动迁移的全部实体，顺序即建表顺序。
func MigrateModels() []any {
	return []any{
		&Application{},
		&Channel{},
		&Version{},
		&Template{},
		&File{},
		&User{},
		&Share{},
		&AuditLog{},
	}
}

// PageQuery 通用分页参数。
type PageQuery struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"pageSize" json:"pageSize"`
}

// Normalize 归一化分页参数，缺省为第 1 页、每页 20 条，上限 200 条。
func (p *PageQuery) Normalize() {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.PageSize > 200 {
		p.PageSize = 200
	}
}

// Offset 返回 SQL offset。
func (p PageQuery) Offset() int { return (p.Page - 1) * p.PageSize }

// Paginated 分页结果包装。
type Paginated struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// NewPaginated 构造分页结果。
func NewPaginated(list any, total int64, page, pageSize int) *Paginated {
	return &Paginated{List: list, Total: total, Page: page, PageSize: pageSize}
}

// String 便于日志输出。
func (p Platform) String() string { return string(p) }

// GoString 便于调试。
func (p Platform) GoString() string { return fmt.Sprintf("Platform(%q)", string(p)) }
