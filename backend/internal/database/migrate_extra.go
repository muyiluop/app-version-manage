package database

import (
	"errors"
	"log/slog"

	"gorm.io/gorm"

	"app_version_manage/internal/model"
)

// migrationEnsureAdminRole 保证系统中至少存在一个管理员账号。
//
// 历史 users 表没有角色列，新增列后所有老账号都会得到默认值 viewer；
// 这里把最早创建的账号提升为管理员，避免出现无人能登录后台的情况。
func migrationEnsureAdminRole(db *gorm.DB, log *slog.Logger) error {
	var admins int64
	if err := db.Model(&model.User{}).Where("role = ?", model.RoleAdmin).Count(&admins).Error; err != nil {
		return err
	}
	if admins > 0 {
		return nil
	}

	var user model.User
	err := db.Order("id ASC").First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 尚无任何用户，交由 Seed 创建默认管理员。
		return nil
	}
	if err != nil {
		return err
	}

	if err := db.Model(&model.User{}).Where("id = ?", user.ID).
		Update("role", model.RoleAdmin).Error; err != nil {
		return err
	}
	log.Warn("未发现管理员账号，已将最早创建的用户提升为管理员", "username", user.Username)
	return nil
}
