package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"app_version_manage/internal/model"
)

// ---------- 文件 ----------

// CreateFile 记录已存储文件。
func (s *Store) CreateFile(ctx context.Context, file *model.File) error {
	return s.db.WithContext(ctx).Create(file).Error
}

// GetFileByKey 按对象键查询文件记录。
func (s *Store) GetFileByKey(ctx context.Context, key string) (*model.File, error) {
	var file model.File
	if err := s.db.WithContext(ctx).Where("path = ?", key).First(&file).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

// GetFileBySHA256 按内容哈希查询，用于秒传与去重。
func (s *Store) GetFileBySHA256(ctx context.Context, sha256Hex string) (*model.File, error) {
	var file model.File
	if err := s.db.WithContext(ctx).Where("sha256 = ?", sha256Hex).First(&file).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

// GetFileByID 按主键查询文件。
func (s *Store) GetFileByID(ctx context.Context, id uint) (*model.File, error) {
	var file model.File
	if err := s.db.WithContext(ctx).First(&file, id).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

// ListFiles 分页查询文件，支持名称关键字。
func (s *Store) ListFiles(ctx context.Context, keyword string, query model.PageQuery) ([]model.File, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.File{})
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR path LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var files []model.File
	if err := q.Order("id DESC").Offset(query.Offset()).Limit(query.PageSize).Find(&files).Error; err != nil {
		return nil, 0, err
	}
	return files, total, nil
}

// AllFiles 返回全部文件记录（用于清理任务）。
func (s *Store) AllFiles(ctx context.Context) ([]model.File, error) {
	var files []model.File
	err := s.db.WithContext(ctx).Order("id ASC").Find(&files).Error
	return files, err
}

// DeleteFile 删除文件记录。
func (s *Store) DeleteFile(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.File{}, id).Error
}

// FileDeleteFields 清理时使用的批量删除条件。
type FileDeleteFields struct {
	IDs []uint
}

// DeleteFilesByIDs 批量删除文件记录。
func (s *Store) DeleteFilesByIDs(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Delete(&model.File{}, ids).Error
}

// ReferencedVersionKeys 返回仍被版本引用的对象键。
func (s *Store) ReferencedVersionKeys(ctx context.Context) ([]string, error) {
	var keys []string
	err := s.db.WithContext(ctx).Model(&model.Version{}).Distinct().Pluck("file_path", &keys).Error
	return keys, err
}

// ReferencedLogoKeys 返回仍被应用引用的 Logo 对象键。
func (s *Store) ReferencedLogoKeys(ctx context.Context) ([]string, error) {
	var keys []string
	err := s.db.WithContext(ctx).Model(&model.Application{}).
		Where("logo <> ''").Distinct().Pluck("logo", &keys).Error
	return keys, err
}

// ---------- 用户 ----------

// CountUsers 统计用户数。
func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	var total int64
	err := s.db.WithContext(ctx).Model(&model.User{}).Count(&total).Error
	return total, err
}

// CreateUser 创建用户。
func (s *Store) CreateUser(ctx context.Context, user *model.User) error {
	return s.db.WithContext(ctx).Create(user).Error
}

// SaveUser 保存用户。
func (s *Store) SaveUser(ctx context.Context, user *model.User) error {
	return s.db.WithContext(ctx).Save(user).Error
}

// UpdateUserFields 局部更新用户字段。
func (s *Store) UpdateUserFields(ctx context.Context, id uint, fields map[string]any) error {
	return s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(fields).Error
}

// GetUserByID 按主键查询用户。
func (s *Store) GetUserByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername 按用户名查询用户。
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers 分页查询用户。
func (s *Store) ListUsers(ctx context.Context, query model.PageQuery) ([]model.User, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.User{})

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.User
	if err := q.Order("id ASC").Offset(query.Offset()).Limit(query.PageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// CountActiveAdmins 统计启用状态的管理员数量，避免误删最后一个管理员。
func (s *Store) CountActiveAdmins(ctx context.Context) (int64, error) {
	var total int64
	err := s.db.WithContext(ctx).Model(&model.User{}).
		Where("role = ? AND is_active = ?", model.RoleAdmin, true).
		Count(&total).Error
	return total, err
}

// DeleteUser 删除用户。
func (s *Store) DeleteUser(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.User{}, id).Error
}

// BumpTokenVersion 递增令牌版本，吊销该用户已签发的所有令牌。
func (s *Store) BumpTokenVersion(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}

// ---------- 分享 ----------

// CreateShare 创建分享。
func (s *Store) CreateShare(ctx context.Context, share *model.Share) error {
	return s.db.WithContext(ctx).Create(share).Error
}

// GetShareByToken 按令牌查询启用中的分享。
func (s *Store) GetShareByToken(ctx context.Context, token string) (*model.Share, error) {
	var share model.Share
	if err := s.db.WithContext(ctx).Where("token = ? AND is_active = ?", token, true).First(&share).Error; err != nil {
		return nil, err
	}
	return &share, nil
}

// GetShare 按主键查询分享。
func (s *Store) GetShare(ctx context.Context, id uint) (*model.Share, error) {
	var share model.Share
	if err := s.db.WithContext(ctx).First(&share, id).Error; err != nil {
		return nil, err
	}
	return &share, nil
}

// GetAppShare 查询属于指定应用的分享。
func (s *Store) GetAppShare(ctx context.Context, appID, shareID uint) (*model.Share, error) {
	var share model.Share
	if err := s.db.WithContext(ctx).Where("id = ? AND app_id = ?", shareID, appID).First(&share).Error; err != nil {
		return nil, err
	}
	return &share, nil
}

// ListShares 查询应用下全部分享。
func (s *Store) ListShares(ctx context.Context, appID uint) ([]model.Share, error) {
	var shares []model.Share
	err := s.db.WithContext(ctx).Where("app_id = ?", appID).Order("id DESC").Find(&shares).Error
	return shares, err
}

// UpdateShareFields 局部更新分享字段。
func (s *Store) UpdateShareFields(ctx context.Context, id uint, fields map[string]any) error {
	return s.db.WithContext(ctx).Model(&model.Share{}).Where("id = ?", id).Updates(fields).Error
}

// DeleteShare 删除分享。
func (s *Store) DeleteShare(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.Share{}, id).Error
}

// IncrementShareAccess 记录分享访问次数。
func (s *Store) IncrementShareAccess(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Model(&model.Share{}).Where("id = ?", id).
		UpdateColumn("access_count", gorm.Expr("access_count + 1")).Error
}
