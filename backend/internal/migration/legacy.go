package migration

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Source 旧版（v1）SQLite 数据源。
//
// 这里刻意使用 SELECT * 直接扫描到「宽结构体」：旧库在不同时期的列可能有增删，
// GORM 按列名映射，缺失的列会保持零值，因此无需为每个历史版本维护列清单。
type Source struct {
	db *gorm.DB
}

// OpenSource 打开旧数据库。文件损坏或路径错误会立即报错。
func OpenSource(path string) (*Source, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("打开旧数据库 %s 失败: %w", path, err)
	}
	return &Source{db: db}, nil
}

// Close 关闭数据源。
func (s *Source) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// HasTable 判断旧库中是否存在某张表。
func (s *Source) HasTable(name string) bool { return s.db.Migrator().HasTable(name) }

// ---------- 旧表结构（只声明关心的列） ----------

type legacyApplication struct {
	ID          uint
	Name        string
	Identifier  string
	Logo        string
	Description string
	Platforms   string // JSON 文本，如 ["windows","android"]
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (legacyApplication) TableName() string { return "applications" }

type legacyVersion struct {
	ID          uint
	AppID       uint
	Platform    string
	Version     string
	FilePath    string
	FileName    string
	FileSize    int64
	Changelog   string
	Ext         string
	ForceUpdate bool
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (legacyVersion) TableName() string { return "versions" }

type legacyTemplate struct {
	ID        uint
	AppID     uint
	Name      string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (legacyTemplate) TableName() string { return "templates" }

type legacyFile struct {
	ID        uint
	Name      string
	Path      string
	Size      int64
	Type      string
	Hash      string // md5
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (legacyFile) TableName() string { return "files" }

type legacyUser struct {
	ID        uint
	Username  string
	Password  string // v1 为明文
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (legacyUser) TableName() string { return "users" }

type legacyShare struct {
	ID        uint
	AppID     uint
	Token     string
	Password  string // v1 为 AES 密文
	ExpiresAt *time.Time
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (legacyShare) TableName() string { return "shares" }

// ---------- 读取 ----------

func (s *Source) Applications() ([]legacyApplication, error) {
	if !s.HasTable("applications") {
		return nil, nil
	}
	var rows []legacyApplication
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取 applications 失败: %w", err)
	}
	return rows, nil
}

func (s *Source) Versions() ([]legacyVersion, error) {
	if !s.HasTable("versions") {
		return nil, nil
	}
	var rows []legacyVersion
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取 versions 失败: %w", err)
	}
	return rows, nil
}

func (s *Source) Templates() ([]legacyTemplate, error) {
	if !s.HasTable("templates") {
		return nil, nil
	}
	var rows []legacyTemplate
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取 templates 失败: %w", err)
	}
	return rows, nil
}

func (s *Source) Files() ([]legacyFile, error) {
	if !s.HasTable("files") {
		return nil, nil
	}
	var rows []legacyFile
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取 files 失败: %w", err)
	}
	return rows, nil
}

func (s *Source) Users() ([]legacyUser, error) {
	if !s.HasTable("users") {
		return nil, nil
	}
	var rows []legacyUser
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取 users 失败: %w", err)
	}
	return rows, nil
}

func (s *Source) Shares() ([]legacyShare, error) {
	if !s.HasTable("shares") {
		return nil, nil
	}
	var rows []legacyShare
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取 shares 失败: %w", err)
	}
	return rows, nil
}
