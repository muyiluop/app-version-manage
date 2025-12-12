package repository

import (
	"app_version_manage/config"
	"app_version_manage/model"
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() error {
	// 确保数据库目录存在
	dbDir := filepath.Dir(config.GlobalConfig.Database.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return err
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(config.GlobalConfig.Database.Path), &gorm.Config{})
	if err != nil {
		return err
	}

	// 自动迁移
	if err := DB.AutoMigrate(
		&model.Application{},
		&model.Version{},
		&model.Template{},
		&model.File{},
		&model.User{},
		&model.Share{},
	); err != nil {
		return err
	}

	// 确保文件上传目录存在
	if err := os.MkdirAll(config.GlobalConfig.Storage.Path, 0755); err != nil {
		return err
	}

	// 创建默认管理员用户
	var count int64
	DB.Model(&model.User{}).Count(&count)
	if count == 0 {
		admin := model.User{
			Username: "admin",
			Password: "123456", // 注意：实际应用中应该使用加密的密码
		}
		if err := DB.Create(&admin).Error; err != nil {
			log.Printf("Failed to create admin user: %v", err)
		}
	}

	return nil
}
