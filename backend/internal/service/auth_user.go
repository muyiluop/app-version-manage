package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"app_version_manage/internal/apierr"
	"app_version_manage/internal/model"
)

// TokenType 令牌类型。
type TokenType string

const (
	// TokenAccess 访问令牌。
	TokenAccess TokenType = "access"
	// TokenRefresh 刷新令牌。
	TokenRefresh TokenType = "refresh"
)

// Claims JWT 载荷。
type Claims struct {
	UserID       uint      `json:"uid"`
	Username     string    `json:"usr"`
	Role         string    `json:"rol"`
	TokenVersion int       `json:"ver"`
	Type         TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// TokenPair 访问令牌与刷新令牌。
type TokenPair struct {
	AccessToken  string     `json:"accessToken"`
	RefreshToken string     `json:"refreshToken"`
	ExpiresAt    *time.Time `json:"expiresAt"`
}

// LoginResult 登录结果。
type LoginResult struct {
	TokenPair
	User *UserProfile `json:"user"`
}

// UserProfile 对外暴露的用户信息。
type UserProfile struct {
	ID                 uint       `json:"id"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"displayName"`
	Role               model.Role `json:"role"`
	MustChangePassword bool       `json:"mustChangePassword"`
	LastLoginAt        *time.Time `json:"lastLoginAt"`
}

// Profile 由用户实体构造对外信息。
func Profile(u *model.User) *UserProfile {
	return &UserProfile{
		ID:                 u.ID,
		Username:           u.Username,
		DisplayName:        u.DisplayName,
		Role:               u.Role,
		MustChangePassword: u.MustChangePassword,
		LastLoginAt:        u.LastLoginAt,
	}
}

// AuthService 认证与令牌。
type AuthService struct{ base }

// Login 校验账号密码并签发令牌。
func (s *AuthService) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	user, err := s.store.GetUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 统一错误信息，避免账号枚举。
			return nil, apierr.Unauthorized("用户名或密码错误")
		}
		return nil, internal("查询用户失败", err)
	}
	if !user.IsActive {
		return nil, apierr.Forbidden("账号已被禁用")
	}
	if !verifyUserPassword(user.Password, password) {
		return nil, apierr.Unauthorized("用户名或密码错误")
	}

	pair, err := s.issueTokens(user)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := s.store.UpdateUserFields(ctx, user.ID, map[string]any{"last_login_at": now}); err != nil {
		s.log.Warn("更新最后登录时间失败", "userId", user.ID, "error", err)
	}
	user.LastLoginAt = &now

	// 登录接口本身无需鉴权，这里补全发起者信息，保证审计可追溯。
	actor := ActorFrom(ctx)
	actor.UserID = user.ID
	actor.Username = user.Username
	actor.Role = user.Role
	s.audit(WithActor(ctx, actor), "auth.login", "user", uid(user.ID), "登录成功", nil, true)

	return &LoginResult{TokenPair: *pair, User: Profile(user)}, nil
}

// Refresh 使用刷新令牌换取新的令牌对。
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	claims, err := s.parseToken(refreshToken, TokenRefresh)
	if err != nil {
		return nil, err
	}
	user, err := s.Authenticate(ctx, claims)
	if err != nil {
		return nil, err
	}
	pair, err := s.issueTokens(user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{TokenPair: *pair, User: Profile(user)}, nil
}

// ParseAccessToken 解析访问令牌（供中间件使用）。
func (s *AuthService) ParseAccessToken(raw string) (*Claims, error) {
	return s.parseToken(raw, TokenAccess)
}

// Authenticate 依据令牌载荷加载并校验用户状态。
func (s *AuthService) Authenticate(ctx context.Context, claims *Claims) (*model.User, error) {
	user, err := s.store.GetUserByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierr.Unauthorized("账号不存在或已被删除")
		}
		return nil, internal("查询用户失败", err)
	}
	if !user.IsActive {
		return nil, apierr.Forbidden("账号已被禁用")
	}
	if user.TokenVersion != claims.TokenVersion {
		return nil, apierr.Unauthorized("登录状态已失效，请重新登录")
	}
	return user, nil
}

// ChangePassword 修改当前用户密码，并使已签发的令牌全部失效。
//
// 返回更新后的用户信息：前端改密成功后需要立刻拿到权威的 mustChangePassword=false，
// 否则仍会按本地旧状态把用户判定为"需要改密"。
func (s *AuthService) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) (*UserProfile, error) {
	if len(strings.TrimSpace(newPassword)) < 6 {
		return nil, apierr.BadRequest("新密码长度不能少于 6 位")
	}

	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, notFoundOr(err, "用户不存在")
	}
	if !verifyUserPassword(user.Password, oldPassword) {
		return nil, apierr.BadRequest("原密码错误")
	}

	hash, err := hashUserPassword(newPassword)
	if err != nil {
		return nil, err
	}
	if err := s.store.UpdateUserFields(ctx, user.ID, map[string]any{
		"password":             hash,
		"must_change_password": false,
	}); err != nil {
		return nil, internal("更新密码失败", err)
	}
	if err := s.store.BumpTokenVersion(ctx, user.ID); err != nil {
		return nil, internal("吊销令牌失败", err)
	}

	s.audit(ctx, "auth.change_password", "user", uid(user.ID), "修改密码", nil, true)

	updated, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, notFoundOr(err, "用户不存在")
	}
	return Profile(updated), nil
}

// issueTokens 签发新的令牌对。
func (s *AuthService) issueTokens(user *model.User) (*TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(time.Duration(s.cfg.JWT.ExpireHours) * time.Hour)
	refreshExp := now.AddDate(0, 0, s.cfg.JWT.RefreshDays)

	access, err := s.sign(user, TokenAccess, now, accessExp)
	if err != nil {
		return nil, err
	}
	refresh, err := s.sign(user, TokenRefresh, now, refreshExp)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: &accessExp}, nil
}

// sign 生成指定类型的 JWT。
func (s *AuthService) sign(user *model.User, typ TokenType, now, exp time.Time) (string, error) {
	claims := Claims{
		UserID:       user.ID,
		Username:     user.Username,
		Role:         string(user.Role),
		TokenVersion: user.TokenVersion,
		Type:         typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.JWT.Issuer,
			Subject:   uid(user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWT.Secret))
	if err != nil {
		return "", internal("生成令牌失败", err)
	}
	return signed, nil
}

// parseToken 解析并校验令牌类型。
func (s *AuthService) parseToken(raw string, want TokenType) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(strings.TrimSpace(raw), claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("非预期的签名算法")
		}
		return []byte(s.cfg.JWT.Secret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, apierr.Unauthorized("无效或已过期的令牌")
	}
	if claims.Type != want {
		return nil, apierr.Unauthorized("令牌类型不正确")
	}
	return claims, nil
}

// ---------- 用户管理 ----------

// UserService 用户管理。
type UserService struct{ base }

// CreateUserInput 创建用户入参。
type CreateUserInput struct {
	Username    string     `json:"username"`
	Password    string     `json:"password"`
	DisplayName string     `json:"displayName"`
	Role        model.Role `json:"role"`
}

// UpdateUserInput 更新用户入参。
type UpdateUserInput struct {
	DisplayName *string     `json:"displayName"`
	Role        *model.Role `json:"role"`
	IsActive    *bool       `json:"isActive"`
	Password    *string     `json:"password"`
}

// List 分页查询用户。
func (s *UserService) List(ctx context.Context, page model.PageQuery) ([]*UserProfile, int64, error) {
	page.Normalize()
	users, total, err := s.store.ListUsers(ctx, page)
	if err != nil {
		return nil, 0, internal("查询用户列表失败", err)
	}
	out := make([]*UserProfile, 0, len(users))
	for i := range users {
		out = append(out, Profile(&users[i]))
	}
	return out, total, nil
}

// Create 创建用户。
func (s *UserService) Create(ctx context.Context, in CreateUserInput) (*UserProfile, error) {
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" {
		return nil, apierr.BadRequest("用户名不能为空")
	}
	if len(in.Password) < 6 {
		return nil, apierr.BadRequest("密码长度不能少于 6 位")
	}
	if !in.Role.Valid() {
		return nil, apierr.BadRequest("角色不合法")
	}

	if _, err := s.store.GetUserByUsername(ctx, in.Username); err == nil {
		return nil, apierr.Conflict("用户名已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, internal("查询用户失败", err)
	}

	hash, err := hashUserPassword(in.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:     in.Username,
		Password:     hash,
		DisplayName:  in.DisplayName,
		Role:         in.Role,
		IsActive:     true,
		TokenVersion: 1,
	}
	if err := s.store.CreateUser(ctx, user); err != nil {
		return nil, internal("创建用户失败", err)
	}

	s.audit(ctx, "user.create", "user", uid(user.ID), "创建用户 "+user.Username,
		map[string]any{"role": user.Role}, true)
	return Profile(user), nil
}

// Update 更新用户信息、角色或状态。
func (s *UserService) Update(ctx context.Context, id uint, in UpdateUserInput) (*UserProfile, error) {
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "用户不存在")
	}

	fields := map[string]any{}
	if in.DisplayName != nil {
		fields["display_name"] = *in.DisplayName
	}
	if in.Role != nil {
		if !in.Role.Valid() {
			return nil, apierr.BadRequest("角色不合法")
		}
		if user.Role == model.RoleAdmin && *in.Role != model.RoleAdmin {
			if err := s.ensureNotLastAdmin(ctx, "不能移除最后一个管理员的角色"); err != nil {
				return nil, err
			}
		}
		fields["role"] = *in.Role
	}
	if in.IsActive != nil {
		if !*in.IsActive {
			if user.ID == ActorFrom(ctx).UserID {
				return nil, apierr.BadRequest("不能禁用当前登录账号")
			}
			if user.Role == model.RoleAdmin {
				if err := s.ensureNotLastAdmin(ctx, "不能禁用最后一个管理员"); err != nil {
					return nil, err
				}
			}
			// 禁用账号同时吊销其令牌。
			fields["token_version"] = gorm.Expr("token_version + 1")
		}
		fields["is_active"] = *in.IsActive
	}
	if in.Password != nil {
		if len(*in.Password) < 6 {
			return nil, apierr.BadRequest("密码长度不能少于 6 位")
		}
		hash, err := hashUserPassword(*in.Password)
		if err != nil {
			return nil, err
		}
		fields["password"] = hash
		fields["must_change_password"] = true
		fields["token_version"] = gorm.Expr("token_version + 1")
	}

	if len(fields) == 0 {
		return nil, apierr.BadRequest("没有需要更新的字段")
	}
	if err := s.store.UpdateUserFields(ctx, id, fields); err != nil {
		return nil, internal("更新用户失败", err)
	}

	s.audit(ctx, "user.update", "user", uid(id), "更新用户 "+user.Username, fields, true)

	updated, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "用户不存在")
	}
	return Profile(updated), nil
}

// Delete 删除用户。
func (s *UserService) Delete(ctx context.Context, id uint) error {
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		return notFoundOr(err, "用户不存在")
	}
	if user.ID == ActorFrom(ctx).UserID {
		return apierr.BadRequest("不能删除当前登录账号")
	}
	if user.Role == model.RoleAdmin {
		if err := s.ensureNotLastAdmin(ctx, "不能删除最后一个管理员"); err != nil {
			return err
		}
	}
	if err := s.store.DeleteUser(ctx, id); err != nil {
		return internal("删除用户失败", err)
	}
	s.audit(ctx, "user.delete", "user", uid(id), "删除用户 "+user.Username, nil, true)
	return nil
}

// ensureNotLastAdmin 保证系统中至少保留一个启用状态的管理员。
func (s *UserService) ensureNotLastAdmin(ctx context.Context, message string) error {
	total, err := s.store.CountActiveAdmins(ctx)
	if err != nil {
		return internal("统计管理员失败", err)
	}
	if total <= 1 {
		return apierr.BadRequest(message)
	}
	return nil
}

// uid 将主键转为字符串形式，便于审计字段复用。
func uid(id uint) string {
	return itoa(id)
}
