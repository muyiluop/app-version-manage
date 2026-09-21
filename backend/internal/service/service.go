// Package service 承载业务逻辑，是 HTTP 层与数据访问层之间的唯一通道。
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"app_version_manage/internal/apierr"
	"app_version_manage/internal/config"
	"app_version_manage/internal/model"
	"app_version_manage/internal/pkg/token"
	"app_version_manage/internal/repository"
	"app_version_manage/internal/storage"
)

// Deps 服务依赖。
type Deps struct {
	Store   *repository.Store
	Storage storage.Storage
	Signer  *token.Signer
	Config  *config.Config
	Log     *slog.Logger
}

// Services 服务集合。
type Services struct {
	Auth     *AuthService
	User     *UserService
	App      *AppService
	Version  *VersionService
	File     *FileService
	Share    *ShareService
	Template *TemplateService
	Audit    *AuditService
}

// New 构造全部服务。
func New(deps Deps) *Services {
	b := base{
		store:   deps.Store,
		storage: deps.Storage,
		signer:  deps.Signer,
		cfg:     deps.Config,
		log:     deps.Log,
	}
	return &Services{
		Auth:     &AuthService{base: b},
		User:     &UserService{base: b},
		App:      &AppService{base: b},
		Version:  &VersionService{base: b},
		File:     &FileService{base: b},
		Share:    &ShareService{base: b},
		Template: &TemplateService{base: b},
		Audit:    &AuditService{base: b},
	}
}

// base 所有服务共享的依赖。
type base struct {
	store   *repository.Store
	storage storage.Storage
	signer  *token.Signer
	cfg     *config.Config
	log     *slog.Logger
}

// Actor 请求发起者，用于审计。
type Actor struct {
	UserID    uint
	Username  string
	Role      model.Role
	IP        string
	UserAgent string
}

type actorCtxKey struct{}

// WithActor 将发起者写入上下文。
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorCtxKey{}, actor)
}

// ActorFrom 从上下文读取发起者。
func ActorFrom(ctx context.Context) Actor {
	if v, ok := ctx.Value(actorCtxKey{}).(Actor); ok {
		return v
	}
	return Actor{}
}

// audit 写入审计日志；审计失败不影响主流程，仅记录告警。
func (b base) audit(ctx context.Context, action, targetType, targetID, summary string, detail any, success bool) {
	actor := ActorFrom(ctx)
	entry := &model.AuditLog{
		ActorID:    actor.UserID,
		ActorName:  actor.Username,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Summary:    summary,
		IP:         actor.IP,
		UserAgent:  truncate(actor.UserAgent, 256),
		Success:    success,
	}
	if detail != nil {
		if raw, err := json.Marshal(detail); err == nil {
			entry.Detail = truncate(string(raw), 4000)
		}
	}
	if err := b.store.CreateAuditLog(ctx, entry); err != nil {
		b.log.Warn("写入审计日志失败", "action", action, "error", err)
	}
}

// notFound 将 GORM 未找到错误转换为业务错误，其余按内部错误处理。
func notFoundOr(err error, message string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apierr.NotFound(message)
	}
	return apierr.Internal("数据库操作失败").WithCause(err)
}

// internal 包装内部错误。
func internal(message string, err error) error {
	return apierr.Internal(message).WithCause(err)
}

// hashSecret 使用 bcrypt 对密码（含 pepper）做单向哈希。
func (b base) hashSecret(plain string) (string, error) {
	peppered := plain + b.cfg.Security.SharePasswordPepper
	hash, err := bcrypt.GenerateFromPassword([]byte(peppered), bcrypt.DefaultCost)
	if err != nil {
		return "", internal("密码加密失败", err)
	}
	return string(hash), nil
}

// verifySecret 校验密码（含 pepper）。
func (b base) verifySecret(hash, plain string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain+b.cfg.Security.SharePasswordPepper))
	return err == nil
}

// hashUserPassword 用户口令哈希（不加 pepper，与密钥轮换解耦）。
func hashUserPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", apierr.Internal("密码加密失败").WithCause(err)
	}
	return string(hash), nil
}

// verifyUserPassword 校验用户口令。
func verifyUserPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// randomToken 生成 URL 安全的随机令牌。
func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", apierr.Internal("生成令牌失败").WithCause(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// truncate 按字节上限截断字符串。
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
