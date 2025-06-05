package model

// LatestVersionResponse 获取最新版本响应
type LatestVersionResponse struct {
	AppName    string   `json:"appName" example:"示例应用"`
	AppID      uint     `json:"appId" example:"1"`
	Identifier string   `json:"identifier" example:"com.example.app"`
	Version    string   `json:"version" example:"1.0.0"`
	Platform   Platform `json:"platform" example:"android"`
	Changelog  string   `json:"changelog" example:"1. 修复已知问题\n2. 性能优化"`
	IsForce    bool     `json:"isForce" example:"false"`
	FileName   string   `json:"fileName" example:"app-v1.0.0.apk"`
	FilePath   string   `json:"filePath" example:"downloads/app-v1.0.0.apk"`
	FileSize   int64    `json:"fileSize" example:"10485760"`
	CreatedAt  string   `json:"createdAt" example:"2025-06-03T10:00:00Z"`
}

// ChangelogResponse 获取更新日志响应
type ChangelogResponse struct {
	CreatedAt string   `json:"createdAt" example:"2025-06-03T10:00:00Z"`
	Version   string   `json:"version" example:"1.0.0"`
	Platform  Platform `json:"platform" example:"android"`
	Changelog string   `json:"changelog" example:"1. 修复已知问题\n2. 性能优化"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error" example:"应用不存在"`
}
