// Package migration 提供从 v1（旧 SQLite + 本地目录）迁移到 v2 的能力。
//
// 与启动时自动执行的「原地 schema 升级」不同，本包用于跨数据库、跨存储的搬迁：
//   - 数据源：任意版本的旧 SQLite 文件（只读取）
//   - 目标：任意受支持数据库（sqlite/postgres/mysql）+ 任意受支持存储（local/s3）
//
// 设计要点：
//   - 保留旧 ID：应用、版本、分享等主键沿用旧值，避免外部引用失效
//   - 保留分享令牌：旧分享链接迁移后仍可访问
//   - 文件重新分片：按 sha256 生成新对象键，同时保留映射以便回写引用
//   - 可重复执行：目标已存在的对象跳过，配合 -overwrite 可安全重跑
package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"app_version_manage/internal/model"
	"app_version_manage/internal/pkg/semver"
	"app_version_manage/internal/storage"
)

// Options 迁移参数。
type Options struct {
	SourceDB    string     // 旧 SQLite 文件（必填）
	SourceFiles string     // 旧上传目录（可选；为空则不搬迁文件）
	DryRun      bool       // 只输出计划，不写入任何数据
	Rekey       bool       // 是否把引用重写为新对象键
	Overwrite   bool       // 目标非空时是否先清空
	BatchSize   int        // 批量写入大小
	DefaultRole model.Role // 旧用户迁移后的角色
}

// Report 迁移结果统计。
type Report struct {
	Applications int
	Channels     int
	Versions     int
	Templates    int
	Users        int
	Shares       int
	Files        int

	FilesScanned int
	FilesCopied  int
	FilesSkipped int
	KeyRewrites  int

	MissingFiles []string
	Warnings     []string
}

func (r *Report) warn(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// Run 执行迁移。dst 为目标存储（用于写入文件），target 为目标数据库。
func Run(ctx context.Context, opts Options, target *gorm.DB, dst storage.Storage, log *slog.Logger) (*Report, error) {
	if strings.TrimSpace(opts.SourceDB) == "" {
		return nil, fmt.Errorf("必须指定旧数据库路径（-src-db）")
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 200
	}
	if opts.DefaultRole == "" {
		opts.DefaultRole = model.RoleAdmin
	}
	if !opts.DefaultRole.Valid() {
		return nil, fmt.Errorf("非法的默认角色: %s", opts.DefaultRole)
	}

	src, err := OpenSource(opts.SourceDB)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	report := &Report{}

	// 目标非空检查：非空必须显式 -overwrite，避免误把数据混进已有环境。
	var existing int64
	if err := target.WithContext(ctx).Model(&model.Application{}).Count(&existing).Error; err != nil {
		return nil, fmt.Errorf("统计目标库应用失败: %w", err)
	}
	// dry-run 只是「排练」，即使目标非空也要能把计划打出来，所以只在正式执行时拦截。
	if existing > 0 && !opts.Overwrite && !opts.DryRun {
		return nil, fmt.Errorf("目标库已存在 %d 个应用；确需覆盖请加 -overwrite（会先清空目标库业务数据）", existing)
	}
	if existing > 0 {
		report.warn("目标库已有 %d 个应用，正式执行会先清空这些数据（需加 -overwrite）", existing)
	}

	apps, err := src.Applications()
	if err != nil {
		return nil, err
	}
	versions, err := src.Versions()
	if err != nil {
		return nil, err
	}
	templates, err := src.Templates()
	if err != nil {
		return nil, err
	}
	users, err := src.Users()
	if err != nil {
		return nil, err
	}
	shares, err := src.Shares()
	if err != nil {
		return nil, err
	}
	legacyFiles, err := src.Files()
	if err != nil {
		return nil, err
	}

	// 1) 文件搬迁
	var mapping map[string]FileMapping
	if strings.TrimSpace(opts.SourceFiles) != "" {
		stats, err := copyFiles(ctx, opts.SourceFiles, dst, opts.DryRun, log)
		if err != nil {
			return nil, err
		}
		mapping = stats.Mapping
		report.FilesScanned = stats.Scanned
		report.FilesCopied = stats.Copied
		report.FilesSkipped = stats.Skipped
		if log != nil {
			log.Info("文件搬迁完成", "scanned", stats.Scanned, "copied", stats.Copied, "skipped", stats.Skipped)
		}
	}

	// 2) 旧对象键解析：返回写入目标库时使用的键
	missing := map[string]bool{}
	resolve := func(oldKey string) (string, string) {
		if strings.TrimSpace(oldKey) == "" {
			return "", ""
		}
		m, ok := mapping[oldKey]
		if !ok {
			missing[oldKey] = true
			return oldKey, ""
		}
		if !opts.Rekey {
			return oldKey, m.SHA256
		}
		return m.NewKey, m.SHA256
	}

	if opts.DryRun {
		report.Applications = len(apps)
		report.Channels = len(apps) * len(model.DefaultChannels)
		report.Versions = len(versions)
		report.Templates = len(templates)
		report.Users = len(users)
		report.Shares = len(shares)
		for _, a := range apps {
			resolve(a.Logo)
		}
		for _, v := range versions {
			resolve(v.FilePath)
		}
		for _, f := range legacyFiles {
			resolve(f.Path)
		}
		collectMissing(report, missing)
		return report, nil
	}

	// 3) 数据搬迁（单事务）
	dstDriver := dst.Driver()
	err = target.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing > 0 {
			if err := clearBusinessData(tx); err != nil {
				return err
			}
		}
		if err := insertApplications(tx, apps, resolve, report); err != nil {
			return err
		}
		if err := insertTemplates(tx, templates, report); err != nil {
			return err
		}
		if err := insertVersions(tx, versions, resolve, report); err != nil {
			return err
		}
		if err := insertUsers(tx, users, opts, report); err != nil {
			return err
		}
		if err := insertShares(tx, shares, report); err != nil {
			return err
		}
		if err := insertLegacyFiles(tx, legacyFiles, resolve, dstDriver, report); err != nil {
			return err
		}

		// 显式 ID 插入后修正序列，再插入自增 ID 的补充记录
		fixSequences(tx, report)
		if err := insertUnrecordedFiles(tx, mapping, opts, dstDriver, report); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return report, err
	}

	collectMissing(report, missing)
	return report, nil
}

// clearBusinessData 清空目标库业务数据（仅 -overwrite 时调用）。
func clearBusinessData(tx *gorm.DB) error {
	targets := []any{
		&model.Version{}, &model.Channel{}, &model.Application{},
		&model.Template{}, &model.File{}, &model.Share{}, &model.User{},
	}
	for _, m := range targets {
		if err := tx.Unscoped().Where("1 = 1").Delete(m).Error; err != nil {
			return fmt.Errorf("清空目标库失败: %w", err)
		}
	}
	return nil
}

func insertApplications(tx *gorm.DB, rows []legacyApplication, resolve func(string) (string, string), report *Report) error {
	for _, a := range rows {
		logo, _ := resolve(a.Logo)
		app := model.Application{
			ID:             a.ID,
			Name:           a.Name,
			Identifier:     a.Identifier,
			Logo:           logo,
			Description:    a.Description,
			Platforms:      parsePlatforms(a.Platforms),
			DefaultChannel: model.DefaultChannelKey,
			CreatedAt:      a.CreatedAt,
			UpdatedAt:      a.UpdatedAt,
		}
		if err := tx.Create(&app).Error; err != nil {
			return fmt.Errorf("写入应用 %s 失败: %w", a.Identifier, err)
		}
		report.Applications++

		for _, def := range model.DefaultChannels {
			channel := model.Channel{
				AppID:     app.ID,
				Key:       def.Key,
				Name:      def.Name,
				IsDefault: def.IsDefault,
				Sort:      def.Sort,
				CreatedAt: a.CreatedAt,
				UpdatedAt: a.UpdatedAt,
			}
			if err := tx.Create(&channel).Error; err != nil {
				return fmt.Errorf("为应用 %s 创建通道失败: %w", a.Identifier, err)
			}
			report.Channels++
		}
	}
	return nil
}

func insertTemplates(tx *gorm.DB, rows []legacyTemplate, report *Report) error {
	for _, t := range rows {
		tpl := model.Template{
			ID:        t.ID,
			AppID:     t.AppID,
			Name:      t.Name,
			Content:   t.Content,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		}
		if err := tx.Create(&tpl).Error; err != nil {
			return fmt.Errorf("写入模板 %s 失败: %w", t.Name, err)
		}
		report.Templates++
	}
	return nil
}

func insertVersions(tx *gorm.DB, rows []legacyVersion, resolve func(string) (string, string), report *Report) error {
	// v1 允许同一 (应用/平台/版本号) 存在多条记录；v2 在该维度上有唯一约束。
	// 与启动时的原地迁移保持一致：保留 id 最大的一条，其余跳过并记录。
	keepID := map[string]uint{}
	for _, v := range rows {
		key := versionKey(v.AppID, v.Platform, model.DefaultChannelKey, v.Version)
		if current, ok := keepID[key]; !ok || v.ID > current {
			keepID[key] = v.ID
		}
	}
	skipped := 0

	for _, v := range rows {
		if keepID[versionKey(v.AppID, v.Platform, model.DefaultChannelKey, v.Version)] != v.ID {
			skipped++
			continue
		}
		fileKey, sha := resolve(v.FilePath)

		sv, err := semver.Parse(v.Version)
		if err != nil {
			report.warn("版本号 %q 无法解析，排序值按 0 处理", v.Version)
		}

		status := model.VersionPublished
		if !v.IsActive {
			status = model.VersionArchived
		}
		publishedAt := v.CreatedAt

		version := model.Version{
			ID:           v.ID,
			AppID:        v.AppID,
			Platform:     model.Platform(v.Platform),
			Channel:      model.DefaultChannelKey,
			Version:      v.Version,
			VersionMajor: sv.Major,
			VersionMinor: sv.Minor,
			VersionPatch: sv.Patch,
			VersionBuild: sv.Build,
			Prerelease:   sv.Prerelease,
			FileKey:      fileKey,
			FileName:     v.FileName,
			FileSize:     v.FileSize,
			FileSHA256:   sha,
			ContentType:  contentTypeOf(v.FilePath),
			Changelog:    v.Changelog,
			Ext:          v.Ext,
			ForceUpdate:  v.ForceUpdate,
			Status:       status,
			PublishedAt:  &publishedAt,
			CreatedAt:    v.CreatedAt,
			UpdatedAt:    v.UpdatedAt,
		}
		if err := tx.Create(&version).Error; err != nil {
			return fmt.Errorf("写入版本 %s(%s/%s) 失败: %w", v.Version, v.Platform, v.FilePath, err)
		}
		report.Versions++
	}

	if skipped > 0 {
		report.warn("旧库存在 %d 条重复版本记录（同应用/平台/版本号），已保留 id 最大的一条，其余跳过；如需全部保留请先在旧库清理",
			skipped)
	}
	return nil
}

// versionKey 生成版本唯一性判断用的键。
func versionKey(appID uint, platform, channel, version string) string {
	return fmt.Sprintf("%d|%s|%s|%s", appID, platform, channel, version)
}

func insertUsers(tx *gorm.DB, rows []legacyUser, opts Options, report *Report) error {
	upgraded := 0
	for _, u := range rows {
		password := u.Password
		if !strings.HasPrefix(password, "$2") {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("加密用户 %s 的口令失败: %w", u.Username, err)
			}
			password = string(hash)
			upgraded++
		}
		user := model.User{
			ID:                 u.ID,
			Username:           u.Username,
			Password:           password,
			Role:               opts.DefaultRole,
			IsActive:           true,
			TokenVersion:       1,
			MustChangePassword: true,
			CreatedAt:          u.CreatedAt,
			UpdatedAt:          u.UpdatedAt,
		}
		if err := tx.Create(&user).Error; err != nil {
			return fmt.Errorf("写入用户 %s 失败: %w", u.Username, err)
		}
		report.Users++
	}
	if report.Users > 0 {
		report.warn("旧库用户已统一迁移为角色 %s（v1 无角色概念），并按需升级口令哈希；请登录后核对权限与口令", opts.DefaultRole)
	}
	if upgraded > 0 {
		report.warn("其中 %d 个用户的口令由明文升级为 bcrypt，登录后请立即修改", upgraded)
	}
	return nil
}

func insertShares(tx *gorm.DB, rows []legacyShare, report *Report) error {
	legacyLocked := 0
	for _, s := range rows {
		share := model.Share{
			ID:             s.ID,
			AppID:          s.AppID,
			Token:          s.Token,
			PasswordLegacy: s.Password,
			ExpiresAt:      s.ExpiresAt,
			IsActive:       s.IsActive,
			CreatedAt:      s.CreatedAt,
			UpdatedAt:      s.UpdatedAt,
		}
		if s.Password != "" {
			share.PasswordAlgo = "legacy-aes"
			legacyLocked++
		}
		if err := tx.Create(&share).Error; err != nil {
			return fmt.Errorf("写入分享 %s 失败: %w", s.Token, err)
		}
		report.Shares++
	}
	if legacyLocked > 0 {
		report.warn("%d 个分享使用了旧版可逆加密密码，无法自动转换；旧分享链接仍可打开，但需在后台重新设置访问密码", legacyLocked)
	}
	return nil
}

func insertLegacyFiles(tx *gorm.DB, rows []legacyFile, resolve func(string) (string, string), dstDriver string, report *Report) error {
	seen := map[string]bool{}
	for _, f := range rows {
		key, sha := resolve(f.Path)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true

		file := model.File{
			ID:          f.ID,
			Name:        f.Name,
			Key:         key,
			Size:        f.Size,
			ContentType: f.Type,
			SHA256:      sha,
			MD5:         f.Hash,
			Storage:     dstDriver,
			CreatedAt:   f.CreatedAt,
			UpdatedAt:   f.UpdatedAt,
		}
		if err := tx.Create(&file).Error; err != nil {
			return fmt.Errorf("写入文件记录 %s 失败: %w", f.Name, err)
		}
		report.Files++
	}
	return nil
}

// insertUnrecordedFiles 为「磁盘上存在但旧 files 表没有记录」的对象补记录，
// 否则它们既不会出现在文件管理里，也不会被清理任务处理。
func insertUnrecordedFiles(tx *gorm.DB, mapping map[string]FileMapping, opts Options, dstDriver string, report *Report) error {
	keys := make([]string, 0, len(mapping))
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	seen := map[string]bool{}
	for _, oldKey := range keys {
		m := mapping[oldKey]
		key := m.NewKey
		if !opts.Rekey {
			key = m.OldKey
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		var count int64
		if err := tx.Model(&model.File{}).Where("path = ?", key).Count(&count).Error; err != nil {
			return fmt.Errorf("查询文件记录失败: %w", err)
		}
		if count > 0 {
			continue
		}

		file := model.File{
			Name:        m.Name,
			Key:         key,
			Size:        m.Size,
			ContentType: contentTypeOf(m.OldKey),
			SHA256:      m.SHA256,
			Storage:     dstDriver,
		}
		if err := tx.Create(&file).Error; err != nil {
			return fmt.Errorf("补充文件记录 %s 失败: %w", key, err)
		}
		report.Files++
	}
	return nil
}

func collectMissing(report *Report, missing map[string]bool) {
	if len(missing) == 0 {
		return
	}
	keys := make([]string, 0, len(missing))
	for k := range missing {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	report.MissingFiles = keys
	report.warn("有 %d 个被引用的对象在源目录中不存在，已保留原对象键（对应下载会 404）：%s",
		len(keys), strings.Join(sample(keys, 5), ", "))
}

// parsePlatforms 兼容 JSON 数组与逗号分隔两种历史格式。
func parsePlatforms(raw string) model.PlatformList {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var list []string
	if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
		list = strings.Split(trimmed, ",")
	}
	return model.ParsePlatforms(list)
}

func sample(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}
