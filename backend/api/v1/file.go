package v1

import (
	"app_version_manage/config"
	"app_version_manage/model"
	"app_version_manage/repository"
	"app_version_manage/utils"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// UploadFile 上传文件
func UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件"})
		return
	}

	// 计算文件哈希
	hash, err := utils.CalculateFileHash(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "计算文件哈希失败"})
		return
	}

	// 检查文件是否已存在
	var existingFile model.File
	if err := repository.DB.Where("hash = ?", hash).First(&existingFile).Error; err == nil {
		c.JSON(http.StatusOK, existingFile)
		return
	}

	// 构造文件保存路径
	fileName := fmt.Sprintf("%s_%s", hash, file.Filename)
	filePath := filepath.Join(config.GlobalConfig.Storage.Path, fileName)

	// 保存文件
	if err := utils.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	// 获取文件大小
	fileSize, err := utils.GetFileSize(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件大小失败"})
		return
	}

	// 保存文件信息到数据库
	fileInfo := model.File{
		Name: file.Filename,
		Path: fileName,
		Size: fileSize,
		Type: file.Header.Get("Content-Type"),
		Hash: hash,
	}

	if err := repository.DB.Create(&fileInfo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件信息失败"})
		return
	}

	c.JSON(http.StatusOK, fileInfo)
}

// ListFiles 获取文件列表
func ListFiles(c *gin.Context) {
	var pagination model.PaginationQuery
	if err := c.ShouldBindQuery(&pagination); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的分页参数"})
		return
	}

	query := repository.DB.Model(&model.File{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件总数失败"})
		return
	}

	var files []model.File
	if err := query.Offset((pagination.Page - 1) * pagination.PageSize).
		Limit(pagination.PageSize).
		Order("created_at DESC").
		Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件列表失败"})
		return
	}

	c.JSON(http.StatusOK, model.NewPaginationResponse(files, total, pagination.Page, pagination.PageSize))
}

// CleanUnusedFiles 清理未使用的文件
func CleanUnusedFiles(c *gin.Context) {
	var usedFiles []string
	if err := repository.DB.Model(&model.Version{}).Distinct().Pluck("file_path", &usedFiles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取使用中的文件失败"})
		return
	}
	var usedFiles2 []string
	if err := repository.DB.Model(&model.Application{}).Distinct().Pluck("logo", &usedFiles2).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取使用中的文件失败"})
		return
	}
	usedFiles = append(usedFiles, usedFiles2...)
	var unusedFiles []model.File
	if err := repository.DB.Where("path NOT IN ?", usedFiles).Find(&unusedFiles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取未使用的文件失败"})
		return
	}
	// 删除文件
	for _, file := range unusedFiles {
		filePath := filepath.Join(config.GlobalConfig.Storage.Path, file.Path)
		if err := repository.DB.Delete(&file).Error; err != nil {
			continue
		}
		// 忽略文件删除错误，因为文件可能已经不存在
		_ = os.Remove(filePath)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已清理 %d 个未使用的文件", len(unusedFiles)),
	})
}

// DownloadFile 下载文件
func DownloadFile(c *gin.Context) {
	filePath := c.Param("path")

	// 构造完整的文件路径
	fullPath := filepath.Join(config.GlobalConfig.Storage.Path, filePath)

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	// 读取文件并发送
	c.File(fullPath)
}

// SecureDownload 安全的文件下载
func SecureDownload(c *gin.Context) {
	token := c.Param("token")

	// 验证token并获取真实文件路径
	filePath, err := utils.ValidateSecureToken(token)
	if err != nil {
		log.Printf("[Error] SecureDownload - 无效的下载token: token=%s, error=%v", token, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效或已过期的下载链接"})
		return
	}

	// 检查文件是否存在
	fullPath := filepath.Join(config.GlobalConfig.Storage.Path, filePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		log.Printf("[Error] SecureDownload - 文件不存在: path=%s", fullPath)
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	// 获取文件名 - 从数据库中获取原始文件名
	var fileInfo model.File
	if err := repository.DB.Where("path = ?", filePath).First(&fileInfo).Error; err != nil {
		// 如果找不到记录，使用路径中的文件名
		fileInfo.Name = filepath.Base(filePath)
	}

	// 设置下载响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileInfo.Name))
	c.File(fullPath)
}
