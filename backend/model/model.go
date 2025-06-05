package model

import (
	"time"
)

type Platform string

const (
	Android Platform = "android"
	IOS     Platform = "ios"
	Windows Platform = "windows"
	Linux   Platform = "linux"
	MacOS   Platform = "macos"
	Harmony Platform = "harmony"
)

// Application 应用信息
type Application struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Name        string     `json:"name" gorm:"size:100;not null"`
	Identifier  string     `json:"identifier" gorm:"size:100;unique;not null"`
	Logo        string     `json:"logo" gorm:"size:255"`
	Description string     `json:"description" gorm:"type:text"`
	Platforms   []Platform `json:"platforms" gorm:"serializer:json"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// Version 版本信息
type Version struct {
	ID          uint        `json:"id" gorm:"primaryKey"`
	AppID       uint        `json:"appId" gorm:"not null"`
	Platform    Platform    `json:"platform" gorm:"size:20;not null"`
	Version     string      `json:"version" gorm:"size:50;not null"`
	FilePath    string      `json:"filePath" gorm:"size:255;not null"`
	FileSize    int64       `json:"fileSize" gorm:"not null"`
	FileName    string      `json:"fileName" gorm:"size:255;not null"`
	Changelog   string      `json:"changelog" gorm:"type:text"`
	ForceUpdate bool        `json:"forceUpdate" gorm:"default:false"`
	IsActive    bool        `json:"isActive" gorm:"default:true"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	Application Application `json:"-" gorm:"foreignKey:AppID"`
}

// Template 输出模板
type Template struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	AppID     uint      `json:"appId" gorm:"not null"`
	Name      string    `json:"name" gorm:"size:100;not null"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// File 文件信息
type File struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:255;not null"`
	Path      string    `json:"path" gorm:"size:255;not null"`
	Size      int64     `json:"size" gorm:"not null"`
	Type      string    `json:"type" gorm:"size:100"`
	Hash      string    `json:"hash" gorm:"size:64"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// User 用户信息
type User struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Username  string    `json:"username" gorm:"size:50;unique;not null"`
	Password  string    `json:"-" gorm:"size:100;not null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PaginationQuery 分页查询参数
type PaginationQuery struct {
	Page     int `form:"page" binding:"required,min=1"`
	PageSize int `form:"pageSize" binding:"required,min=1,max=100"`
}

// PaginationResponse 分页响应
type PaginationResponse struct {
	Total    int64       `json:"total"`
	List     interface{} `json:"list"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

// NewPaginationResponse 创建分页响应
func NewPaginationResponse(list interface{}, total int64, page, pageSize int) *PaginationResponse {
	return &PaginationResponse{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}
