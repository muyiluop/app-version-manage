package service

import (
	"context"
)

// enrichFromFile 用文件记录中的元信息补全发布入参。
//
// 调用方通常只需提供文件对象键，内容校验值、大小与类型由服务端补齐，
// 避免客户端伪造或漏传这些字段。
func (s *VersionService) enrichFromFile(ctx context.Context, fileKey string, in PublishInput) PublishInput {
	file, err := s.store.GetFileByKey(ctx, fileKey)
	if err != nil {
		return in
	}
	if in.FileSHA256 == "" {
		in.FileSHA256 = file.SHA256
	}
	if in.FileSize == 0 {
		in.FileSize = file.Size
	}
	if in.ContentType == "" {
		in.ContentType = file.ContentType
	}
	if in.FileName == "" {
		in.FileName = file.Name
	}
	return in
}
