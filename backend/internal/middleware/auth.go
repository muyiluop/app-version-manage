package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"app_version_manage/internal/apierr"
	"app_version_manage/internal/model"
	"app_version_manage/internal/service"
)

// ContextUserKey 上下文中当前用户的键。
const ContextUserKey = "currentUser"

// JWTAuth 校验访问令牌并将用户写入上下文，同时记录审计所需的发起者信息。
func JWTAuth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearerToken(c)
		if raw == "" {
			apierr.FailRaw(c, apierr.CodeUnauthorized, http.StatusUnauthorized, "未提供认证信息")
			return
		}

		claims, err := auth.ParseAccessToken(raw)
		if err != nil {
			apierr.Fail(c, err)
			return
		}
		user, err := auth.Authenticate(c.Request.Context(), claims)
		if err != nil {
			apierr.Fail(c, err)
			return
		}

		c.Set(ContextUserKey, user)
		ctx := service.WithActor(c.Request.Context(), service.Actor{
			UserID:    user.ID,
			Username:  user.Username,
			Role:      user.Role,
			IP:        c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// CurrentUser 从上下文读取当前用户。
func CurrentUser(c *gin.Context) *model.User {
	if v, ok := c.Get(ContextUserKey); ok {
		if user, ok := v.(*model.User); ok {
			return user
		}
	}
	return nil
}

// RequireRole 限定访问角色；admin 始终放行。
func RequireRole(roles ...model.Role) gin.HandlerFunc {
	allowed := map[model.Role]bool{model.RoleAdmin: true}
	for _, r := range roles {
		allowed[r] = true
	}

	return func(c *gin.Context) {
		user := CurrentUser(c)
		if user == nil {
			apierr.FailRaw(c, apierr.CodeUnauthorized, http.StatusUnauthorized, "未提供认证信息")
			return
		}
		if !allowed[user.Role] {
			apierr.FailRaw(c, apierr.CodeForbidden, http.StatusForbidden, "没有权限执行此操作")
			return
		}
		c.Next()
	}
}

// RequireWrite 写操作角色：管理员与发布员。
func RequireWrite() gin.HandlerFunc {
	return RequireRole(model.RoleReleaser)
}

// bearerToken 从 Authorization 头解析 Bearer 令牌。
func bearerToken(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
