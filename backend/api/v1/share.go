package v1

import (
	"app_version_manage/model"
	"app_version_manage/repository"
	"app_version_manage/utils"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessShareRequest 访问分享请求
type AccessShareRequest struct {
	Password string `json:"password"` // 可选，如分享设置了密码则需提供
}

// GenerateShareLinkRequest 生成分享链接请求
type GenerateShareLinkRequest struct {
	Password  string `json:"password"`  // 可选，为空表示无密码
	ExpiresIn int    `json:"expiresIn"` // 有效期（天数），0表示永久有效
}

// UpdateShareRequest 编辑分享请求
type UpdateShareRequest struct {
	Password  *string `json:"password"`  // 可选，nil不修改；空字符串清除密码；非空则更新为新密码
	ExpiresIn *int    `json:"expiresIn"` // 可选，nil不修改；0表示永久有效；>0 表示按天设置
}

// GenerateShareLinkResponse 生成分享链接响应
type GenerateShareLinkResponse struct {
	ID          uint       `json:"id"`
	Token       string     `json:"token"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	HasPassword bool       `json:"hasPassword"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// GenerateShareLink 生成应用分享链接（需要认证）
func GenerateShareLink(c *gin.Context) {
	appId := c.Param("id")
	appIDUint, err := strconv.ParseUint(appId, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "应用ID无效"})
		return
	}

	// 检查应用是否存在
	var app model.Application
	if err := repository.DB.First(&app, uint(appIDUint)).Error; err != nil {
		log.Printf("[Error] GenerateShareLink - 应用不存在: appId=%s, error=%v", appId, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	var req GenerateShareLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 生成分享token
	token, err := utils.GenerateSecureToken(appId, 365*24*time.Hour) // Token本身有效期较长
	if err != nil {
		log.Printf("[Error] GenerateShareLink - 生成token失败: appId=%s, error=%v", appId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成分享链接失败"})
		return
	}

	share := model.Share{
		AppID:    uint(appIDUint),
		Token:    token,
		IsActive: true,
	}

	// 处理密码
	if req.Password != "" {
		hashedPassword, err := utils.HashPassword(req.Password)
		if err != nil {
			log.Printf("[Error] GenerateShareLink - 密码加密失败: error=%v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存分享链接失败"})
			return
		}
		share.Password = hashedPassword
	}

	// 处理过期时间
	if req.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(req.ExpiresIn) * 24 * time.Hour)
		share.ExpiresAt = &expiresAt
	}

	// 保存到数据库
	if err := repository.DB.Create(&share).Error; err != nil {
		log.Printf("[Error] GenerateShareLink - 保存分享信息失败: error=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存分享链接失败"})
		return
	}

	resp := GenerateShareLinkResponse{
		ID:          share.ID,
		Token:       share.Token,
		ExpiresAt:   share.ExpiresAt,
		HasPassword: share.Password != "",
		CreatedAt:   share.CreatedAt,
	}

	c.JSON(http.StatusOK, resp)
}

// GetSharedApp 获取分享的应用信息（公开接口，可选密码保护）
func GetSharedApp(c *gin.Context) {
	token := c.Param("token")

	// 先查询分享是否存在
	var share model.Share
	if err := repository.DB.Where("token = ? AND is_active = ?", token, true).First(&share).Error; err != nil {
		log.Printf("[Error] GetSharedApp - 分享不存在: token=%s", token)
		c.JSON(http.StatusNotFound, gin.H{"error": "分享链接无效或已过期"})
		return
	}

	// 检查分享是否过期
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		log.Printf("[Error] GetSharedApp - 分享已过期: token=%s", token)
		c.JSON(http.StatusNotFound, gin.H{"error": "分享链接已过期"})
		return
	}

	// 如果需要密码，检查密码
	if share.Password != "" {
		var req AccessShareRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			req.Password = ""
		}

		// 验证密码
		if req.Password == "" || utils.VerifyPassword(share.Password, req.Password) != nil {
			// 需要密码，返回209表示需要密码验证
			c.JSON(209, gin.H{
				"error":           "需要密码验证",
				"requirePassword": true,
			})
			return
		}
	}

	// 获取应用信息
	var app model.Application
	if err := repository.DB.First(&app, share.AppID).Error; err != nil {
		log.Printf("[Error] GetSharedApp - 应用不存在: appId=%d, error=%v", share.AppID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 获取各平台最新版本
	// 先用子查询找出每个平台最新版本的ID，再获取这些版本
	var versions []model.Version
	subQuery := repository.DB.
		Select("MAX(id)").
		Table("versions").
		Where("app_id = ? AND is_active = ?", app.ID, true).
		Group("platform")

	if err := repository.DB.
		Where("id IN (?)", subQuery).
		Order("created_at DESC").
		Find(&versions).Error; err != nil {
		log.Printf("[Error] GetSharedApp - 获取版本信息失败: appId=%d, error=%v", app.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取版本信息失败"})
		return
	}

	// 检测当前平台
	platform := c.Query("platform")
	if platform == "" {
		platform = c.GetHeader("X-Platform")
		if platform == "" {
			userAgent := c.GetHeader("User-Agent")
			platform = detectPlatform(userAgent)
		}
	}

	// 将versions中的文件地址修改为加密地址
	for i := range versions {
		if versions[i].FilePath != "" {
			downloadToken, err := utils.GenerateSecureToken(versions[i].FilePath, 24*time.Hour)
			if err != nil {
				log.Printf("[Error] GetSharedApp - 生成下载token失败: versionId=%d, error=%v", versions[i].ID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件下载链接失败"})
				return
			}
			versions[i].FilePath = downloadToken
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"app": gin.H{
			"name":        app.Name,
			"identifier":  app.Identifier,
			"logo":        app.Logo,
			"description": app.Description,
			"platforms":   app.Platforms,
		},
		"versions":        versions,
		"currentPlatform": platform,
	})
}

// GetSharedAppVersions 获取分享应用的版本历史（公开接口，可选密码保护）
func GetSharedAppVersions(c *gin.Context) {
	token := c.Param("token")
	platform := c.Query("platform")

	// 先查询分享是否存在
	var share model.Share
	if err := repository.DB.Where("token = ? AND is_active = ?", token, true).First(&share).Error; err != nil {
		log.Printf("[Error] GetSharedAppVersions - 分享不存在: token=%s", token)
		c.JSON(http.StatusNotFound, gin.H{"error": "分享链接无效或已过期"})
		return
	}

	// 检查分享是否过期
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		log.Printf("[Error] GetSharedAppVersions - 分享已过期: token=%s", token)
		c.JSON(http.StatusNotFound, gin.H{"error": "分享链接已过期"})
		return
	}

	// 如果需要密码，检查密码
	if share.Password != "" {
		var req AccessShareRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			req.Password = ""
		}

		// 验证密码
		if req.Password == "" || utils.VerifyPassword(share.Password, req.Password) != nil {
			// 需要密码，返回209表示需要密码验证
			c.JSON(209, gin.H{
				"error":           "需要密码验证",
				"requirePassword": true,
			})
			return
		}
	}

	var pagination model.PaginationQuery
	if err := c.ShouldBindQuery(&pagination); err != nil {
		pagination = model.PaginationQuery{
			Page:     1,
			PageSize: 99,
		}
	}

	query := repository.DB.Model(&model.Version{})
	query = query.Where("app_id = ?", share.AppID)

	if platform != "" {
		query = query.Where("platform = ?", platform)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取版本总数失败"})
		return
	}

	var versions []model.Version
	if err := query.Offset((pagination.Page - 1) * pagination.PageSize).
		Limit(pagination.PageSize).
		Order("created_at DESC").
		Find(&versions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取版本列表失败"})
		return
	}

	// 将versions中的文件地址修改为加密地址
	for i := range versions {
		if versions[i].FilePath != "" {
			downloadToken, err := utils.GenerateSecureToken(versions[i].FilePath, 24*time.Hour)
			if err != nil {
				log.Printf("[Error] GetSharedAppVersions - 生成下载token失败: versionId=%d, error=%v", versions[i].ID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件下载链接失败"})
				return
			}
			versions[i].FilePath = downloadToken
		}
	}

	c.JSON(http.StatusOK, model.NewPaginationResponse(versions, total, pagination.Page, pagination.PageSize))
}

// ListShares 列出应用的所有分享链接（需要认证）
func ListShares(c *gin.Context) {
	appId := c.Param("id")
	appIDUint, err := strconv.ParseUint(appId, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "应用ID无效"})
		return
	}

	// 检查应用是否存在
	var app model.Application
	if err := repository.DB.First(&app, uint(appIDUint)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	var shares []model.Share
	if err := repository.DB.Where("app_id = ? AND is_active = ?", uint(appIDUint), true).
		Order("created_at DESC").
		Find(&shares).Error; err != nil {
		log.Printf("[Error] ListShares - 获取分享列表失败: appId=%s, error=%v", appId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取分享列表失败"})
		return
	}

	// 不返回密码的散列值
	var respShares []gin.H
	for _, share := range shares {
		respShares = append(respShares, gin.H{
			"id":          share.ID,
			"token":       share.Token,
			"hasPassword": share.Password != "",
			"expiresAt":   share.ExpiresAt,
			"isActive":    share.IsActive,
			"createdAt":   share.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"shares": respShares,
	})
}

// DeactivateShare 禁用分享链接（需要认证）
func DeactivateShare(c *gin.Context) {
	appId := c.Param("id")
	shareId := c.Param("shareId")

	appIDUint, err := strconv.ParseUint(appId, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "应用ID无效"})
		return
	}

	shareIDUint, err := strconv.ParseUint(shareId, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分享ID无效"})
		return
	}

	// 检查分享是否属于该应用
	var share model.Share
	if err := repository.DB.Where("id = ? AND app_id = ?", uint(shareIDUint), uint(appIDUint)).First(&share).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	// 禁用分享
	if err := repository.DB.Model(&share).Update("is_active", false).Error; err != nil {
		log.Printf("[Error] DeactivateShare - 禁用分享失败: shareId=%s, error=%v", shareId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "禁用分享失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "分享已禁用"})
}

// DeleteShare 删除分享链接（需要认证）
func DeleteShare(c *gin.Context) {
	appId := c.Param("id")
	shareId := c.Param("shareId")

	appIDUint, err := strconv.ParseUint(appId, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "应用ID无效"})
		return
	}

	shareIDUint, err := strconv.ParseUint(shareId, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分享ID无效"})
		return
	}

	// 检查分享是否属于该应用
	var share model.Share
	if err := repository.DB.Where("id = ? AND app_id = ?", uint(shareIDUint), uint(appIDUint)).First(&share).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	// 删除分享
	if err := repository.DB.Delete(&share).Error; err != nil {
		log.Printf("[Error] DeleteShare - 删除分享失败: shareId=%s, error=%v", shareId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除分享失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "分享已删除"})
}

// UpdateShare 编辑分享链接（需要认证）
func UpdateShare(c *gin.Context) {
	appId := c.Param("id")
	shareId := c.Param("shareId")

	appIDUint, err := strconv.ParseUint(appId, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "应用ID无效"})
		return
	}

	shareIDUint, err := strconv.ParseUint(shareId, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分享ID无效"})
		return
	}

	var share model.Share
	if err := repository.DB.Where("id = ? AND app_id = ?", uint(shareIDUint), uint(appIDUint)).First(&share).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
		return
	}

	var req UpdateShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	updates := map[string]interface{}{}

	// 处理密码更新
	if req.Password != nil {
		if *req.Password == "" {
			updates["password"] = ""
		} else {
			hashedPassword, err := utils.HashPassword(*req.Password)
			if err != nil {
				log.Printf("[Error] UpdateShare - 密码加密失败: error=%v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "更新分享失败"})
				return
			}
			updates["password"] = hashedPassword
		}
	}

	// 处理有效期更新
	if req.ExpiresIn != nil {
		if *req.ExpiresIn <= 0 {
			updates["expires_at"] = nil
		} else {
			expiresAt := time.Now().Add(time.Duration(*req.ExpiresIn) * 24 * time.Hour)
			updates["expires_at"] = expiresAt
		}
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未提供需要更新的字段"})
		return
	}

	if err := repository.DB.Model(&share).Updates(updates).Error; err != nil {
		log.Printf("[Error] UpdateShare - 更新分享失败: shareId=%s, error=%v", shareId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新分享失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "分享已更新"})
}

// validateShareToken 验证分享token和密码的通用函数
func validateShareToken(token string, _ string) (*model.Share, error) {
	// 查询分享记录
	var share model.Share
	if err := repository.DB.Where("token = ? AND is_active = ?", token, true).First(&share).Error; err != nil {
		return nil, &ShareError{"无效的分享链接"}
	}

	// 检查分享是否过期
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		return nil, &ShareError{"分享链接已过期"}
	}

	return &share, nil
}

// ShareError 自定义分享错误
type ShareError struct {
	Message string
}

func (e *ShareError) Error() string {
	return e.Message
}
