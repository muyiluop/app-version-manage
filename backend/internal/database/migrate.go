package database

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"app_version_manage/internal/config"
	"app_version_manage/internal/model"
	"app_version_manage/internal/pkg/semver"
)

// schemaMigration 迁移记录表。
type schemaMigration struct {
	Version   string    `gorm:"primaryKey;size:32"`
	Name      string    `gorm:"size:128"`
	AppliedAt time.Time `gorm:"not null"`
}

// TableName 指定表名。
func (schemaMigration) TableName() string { return "schema_migrations" }

// Migration 单个迁移步骤。
type Migration struct {
	Version string
	Name    string
	Up      func(tx *gorm.DB, log *slog.Logger) error
}

// migrations 迁移列表，按 Version 升序执行，且只增不改。
var migrations = []Migration{
	{Version: "0001", Name: "dedupe_and_schema", Up: migrationDedupeAndSchema},
	{Version: "0002", Name: "backfill_v2_fields", Up: migrationBackfill},
	{Version: "0003", Name: "ensure_default_channels", Up: migrationDefaultChannels},
	{Version: "0004", Name: "ensure_admin_role", Up: migrationEnsureAdminRole},
	{Version: "0005", Name: "rebuild_version_unique_index", Up: migrationRebuildVersionIndex},
}

// Migrate 执行所有未应用的迁移。
func Migrate(db *gorm.DB, log *slog.Logger) error {
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}

	applied := map[string]bool{}
	var records []schemaMigration
	if err := db.Find(&records).Error; err != nil {
		return fmt.Errorf("读取迁移记录失败: %w", err)
	}
	for _, r := range records {
		applied[r.Version] = true
	}

	// 每次启动都先把「表 + 列」与当前模型对齐（只增不改），
	// 这样后续迁移引用的新列一定存在；索引交由迁移在数据清理之后创建。
	if err := ensureSchemaColumns(db); err != nil {
		return fmt.Errorf("同步表结构失败: %w", err)
	}

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		log.Info("执行数据库迁移", "version", m.Version, "name", m.Name)
		if err := m.Up(db, log); err != nil {
			return fmt.Errorf("迁移 %s(%s) 失败: %w", m.Version, m.Name, err)
		}
		record := schemaMigration{Version: m.Version, Name: m.Name, AppliedAt: time.Now()}
		if err := db.Create(&record).Error; err != nil {
			return fmt.Errorf("记录迁移 %s 失败: %w", m.Version, err)
		}
	}
	return nil
}

// migrationDedupeAndSchema 清理重复版本并建立 v2 表结构（含唯一约束）。
//
// 重复数据（同一 app/platform/version 多条）会阻塞 (app_id, platform, channel, version)
// 唯一索引的创建，因此先按 id 保留最新一条、删除更早的记录。
// 物理文件不会被删除，仅清理重复的数据库记录。
func migrationDedupeAndSchema(db *gorm.DB, log *slog.Logger) error {
	if db.Migrator().HasTable("versions") {
		if err := dedupeVersions(db, log); err != nil {
			return err
		}
	}
	if err := ensureSchema(db); err != nil {
		return fmt.Errorf("迁移表结构失败: %w", err)
	}
	return nil
}

// dedupeVersions 删除 (app_id, platform, version) 维度上的重复记录，保留 id 最大者。
func dedupeVersions(db *gorm.DB, log *slog.Logger) error {
	type groupKey struct {
		AppID    uint
		Platform string
		Version  string
	}
	var groups []groupKey
	if err := db.Raw(
		"SELECT app_id, platform, version FROM versions GROUP BY app_id, platform, version HAVING COUNT(*) > 1",
	).Scan(&groups).Error; err != nil {
		return fmt.Errorf("统计重复版本失败: %w", err)
	}
	if len(groups) == 0 {
		return nil
	}

	log.Warn("检测到重复版本记录，将保留最新一条并删除更早的记录（物理文件不受影响）", "groups", len(groups))

	for _, g := range groups {
		var ids []uint
		if err := db.Raw(
			"SELECT id FROM versions WHERE app_id = ? AND platform = ? AND version = ? ORDER BY id DESC",
			g.AppID, g.Platform, g.Version,
		).Scan(&ids).Error; err != nil {
			return fmt.Errorf("查询重复版本失败: %w", err)
		}
		if len(ids) <= 1 {
			continue
		}
		toDelete := ids[1:]
		if err := db.Exec("DELETE FROM versions WHERE id IN ?", toDelete).Error; err != nil {
			return fmt.Errorf("删除重复版本失败: %w", err)
		}
		log.Warn("已合并重复版本",
			"appId", g.AppID, "platform", g.Platform, "version", g.Version,
			"keptId", ids[0], "deletedIds", toDelete)
	}
	return nil
}

// legacyVersionActive 仅用于读取旧表中的 is_active 列。
type legacyVersionActive struct {
	ID       uint `gorm:"primaryKey"`
	IsActive bool `gorm:"column:is_active"`
}

// TableName 指定表名。
func (legacyVersionActive) TableName() string { return "versions" }

// migrationBackfill 回填 v2 新增字段，确保历史数据语义正确。
func migrationBackfill(db *gorm.DB, log *slog.Logger) error {
	// 1) status 由旧的 is_active 推导（新列默认值为 published，需修正已下架记录）
	if db.Migrator().HasColumn(&legacyVersionActive{}, "is_active") {
		var rows []legacyVersionActive
		if err := db.Find(&rows).Error; err != nil {
			return fmt.Errorf("读取旧版本状态失败: %w", err)
		}
		var inactiveIDs []uint
		for _, r := range rows {
			if !r.IsActive {
				inactiveIDs = append(inactiveIDs, r.ID)
			}
		}
		if len(inactiveIDs) > 0 {
			if err := db.Model(&model.Version{}).
				Where("id IN ?", inactiveIDs).
				Update("status", model.VersionArchived).Error; err != nil {
				return fmt.Errorf("回填版本状态失败: %w", err)
			}
			log.Info("已按 is_active 回填版本状态为 archived", "count", len(inactiveIDs))
		}
	}

	// 2) published_at 缺省使用 created_at
	if err := db.Exec("UPDATE versions SET published_at = created_at WHERE published_at IS NULL").Error; err != nil {
		return fmt.Errorf("回填发布时间失败: %w", err)
	}

	// 3) 解析版本号，回填 semver 数值列（排序依赖）
	var versions []model.Version
	if err := db.Select("id", "version").Find(&versions).Error; err != nil {
		return fmt.Errorf("读取版本列表失败: %w", err)
	}
	var parsed, failed int
	for _, v := range versions {
		sv, err := semver.Parse(v.Version)
		if err != nil {
			failed++
			log.Warn("版本号无法解析，排序时将按 0 处理", "versionId", v.ID, "version", v.Version, "error", err)
			continue
		}
		if err := db.Model(&model.Version{}).Where("id = ?", v.ID).Updates(map[string]any{
			"version_major": sv.Major,
			"version_minor": sv.Minor,
			"version_patch": sv.Patch,
			"version_build": sv.Build,
			"prerelease":    sv.Prerelease,
		}).Error; err != nil {
			return fmt.Errorf("回填版本号失败: %w", err)
		}
		parsed++
	}
	log.Info("已回填版本号数值列", "parsed", parsed, "failed", failed)

	// 4) 历史用户密码明文 -> bcrypt
	if err := hashLegacyUserPasswords(db, log); err != nil {
		return err
	}

	// 5) 标记历史分享密码为 legacy-aes 算法
	if err := db.Model(&model.Share{}).
		Where("password <> '' AND password_hash = ''").
		Update("password_algo", "legacy-aes").Error; err != nil {
		return fmt.Errorf("标记历史分享密码失败: %w", err)
	}

	return nil
}

// hashLegacyUserPasswords 将明文密码升级为 bcrypt，原密码保持可用。
func hashLegacyUserPasswords(db *gorm.DB, log *slog.Logger) error {
	var users []model.User
	if err := db.Find(&users).Error; err != nil {
		return fmt.Errorf("读取用户失败: %w", err)
	}
	upgraded := 0
	for _, u := range users {
		if strings.HasPrefix(u.Password, "$2") {
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("哈希用户密码失败: %w", err)
		}
		if err := db.Model(&model.User{}).Where("id = ?", u.ID).Updates(map[string]any{
			"password":             string(hash),
			"must_change_password": true,
		}).Error; err != nil {
			return fmt.Errorf("更新用户密码失败: %w", err)
		}
		upgraded++
	}
	if upgraded > 0 {
		log.Warn("已将历史明文密码升级为 bcrypt，请提示相关用户修改密码", "count", upgraded)
	}
	return nil
}

// migrationDefaultChannels 为每个应用补齐默认通道，并修正应用默认通道字段。
func migrationDefaultChannels(db *gorm.DB, _ *slog.Logger) error {
	var apps []model.Application
	if err := db.Select("id", "default_channel").Find(&apps).Error; err != nil {
		return fmt.Errorf("读取应用失败: %w", err)
	}
	for _, app := range apps {
		for _, ch := range model.DefaultChannels {
			var count int64
			if err := db.Model(&model.Channel{}).
				Where("app_id = ? AND key = ?", app.ID, ch.Key).
				Count(&count).Error; err != nil {
				return fmt.Errorf("查询通道失败: %w", err)
			}
			if count > 0 {
				continue
			}
			channel := model.Channel{
				AppID:     app.ID,
				Key:       ch.Key,
				Name:      ch.Name,
				IsDefault: ch.IsDefault,
				Sort:      ch.Sort,
			}
			if err := db.Create(&channel).Error; err != nil {
				return fmt.Errorf("创建默认通道失败: %w", err)
			}
		}
		if app.DefaultChannel == "" {
			if err := db.Model(&model.Application{}).Where("id = ?", app.ID).
				Update("default_channel", model.DefaultChannelKey).Error; err != nil {
				return fmt.Errorf("修正应用默认通道失败: %w", err)
			}
		}
	}
	return nil
}

// Seed 首次启动时创建管理员账号。
func Seed(db *gorm.DB, cfg *config.Config, log *slog.Logger) error {
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("统计用户失败: %w", err)
	}
	if count > 0 {
		return nil
	}

	password := cfg.Security.AdminInitialPassword
	generated := false
	if strings.TrimSpace(password) == "" {
		pwd, err := randomPassword(16)
		if err != nil {
			return err
		}
		password = pwd
		generated = true
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("生成管理员密码失败: %w", err)
	}

	admin := model.User{
		Username:           cfg.Security.AdminUsername,
		Password:           string(hash),
		DisplayName:        "系统管理员",
		Role:               model.RoleAdmin,
		IsActive:           true,
		TokenVersion:       1,
		MustChangePassword: true,
	}
	if err := db.Create(&admin).Error; err != nil {
		return fmt.Errorf("创建管理员失败: %w", err)
	}

	if generated {
		log.Warn("已创建初始管理员账号，请登录后立即修改密码",
			"username", admin.Username, "initialPassword", password)
	} else {
		log.Info("已创建初始管理员账号", "username", admin.Username)
	}
	return nil
}

func randomPassword(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成随机密码失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)[:n], nil
}
