// Package v2 提供 v2 版本的 HTTP 处理器，响应统一使用 apierr 信封。
package v2

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"

	"app_version_manage/internal/apierr"
	"app_version_manage/internal/config"
	"app_version_manage/internal/middleware"
	"app_version_manage/internal/model"
	"app_version_manage/internal/service"
)

// Handler v2 处理器集合。
type Handler struct {
	svc *service.Services
	cfg *config.Config
	log *slog.Logger
}

// New 构造 v2 处理器。
func New(svc *service.Services, cfg *config.Config, log *slog.Logger) *Handler {
	return &Handler{svc: svc, cfg: cfg, log: log}
}

// bindJSON 绑定并校验 JSON 请求体。
func (h *Handler) bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		apierr.Fail(c, apierr.BadRequest("请求参数不合法"))
		return false
	}
	return true
}

// pathUint 解析路径参数中的正整数 ID。
func (h *Handler) pathUint(c *gin.Context, name string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || v == 0 {
		apierr.Fail(c, apierr.BadRequest("参数 "+name+" 不合法"))
		return 0, false
	}
	return uint(v), true
}

// pageQuery 解析分页参数。
func (h *Handler) pageQuery(c *gin.Context) model.PageQuery {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	q := model.PageQuery{Page: page, PageSize: pageSize}
	q.Normalize()
	return q
}

// actorContext 返回携带发起者信息的请求上下文。
func actorContext(c *gin.Context) *gin.Context { return c }

// ---------- 认证 ----------

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 登录。
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if !h.bindJSON(c, &req) {
		return
	}
	ctx := service.WithActor(c.Request.Context(), service.Actor{
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})
	result, err := h.svc.Auth.Login(ctx, req.Username, req.Password)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, result)
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// Refresh 刷新令牌。
func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if !h.bindJSON(c, &req) {
		return
	}
	result, err := h.svc.Auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, result)
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

// ChangePassword 修改当前用户密码。
func (h *Handler) ChangePassword(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		apierr.Fail(c, apierr.Unauthorized("未提供认证信息"))
		return
	}
	var req changePasswordRequest
	if !h.bindJSON(c, &req) {
		return
	}
	profile, err := h.svc.Auth.ChangePassword(c.Request.Context(), user.ID, req.OldPassword, req.NewPassword)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	// 同时返回最新用户信息，前端据此同步 mustChangePassword，避免继续按旧状态引导改密。
	apierr.OK(c, gin.H{"message": "密码已更新，请重新登录", "user": profile})
}

// Profile 返回当前用户信息。
func (h *Handler) Profile(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		apierr.Fail(c, apierr.Unauthorized("未提供认证信息"))
		return
	}
	apierr.OK(c, service.Profile(user))
}

// ---------- 用户管理 ----------

// ListUsers 用户列表。
func (h *Handler) ListUsers(c *gin.Context) {
	users, total, err := h.svc.User.List(c.Request.Context(), h.pageQuery(c))
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	page := h.pageQuery(c)
	apierr.OK(c, model.NewPaginated(users, total, page.Page, page.PageSize))
}

// CreateUser 创建用户。
func (h *Handler) CreateUser(c *gin.Context) {
	var req service.CreateUserInput
	if !h.bindJSON(c, &req) {
		return
	}
	user, err := h.svc.User.Create(c.Request.Context(), req)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.Created(c, user)
}

// UpdateUser 更新用户。
func (h *Handler) UpdateUser(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	var req service.UpdateUserInput
	if !h.bindJSON(c, &req) {
		return
	}
	user, err := h.svc.User.Update(c.Request.Context(), id, req)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, user)
}

// DeleteUser 删除用户。
func (h *Handler) DeleteUser(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.svc.User.Delete(c.Request.Context(), id); err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{"message": "用户已删除"})
}

// ListAuditLogs 审计日志列表。
func (h *Handler) ListAuditLogs(c *gin.Context) {
	page := h.pageQuery(c)
	logs, total, err := h.svc.Audit.List(c.Request.Context(), page, c.Query("action"))
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, model.NewPaginated(logs, total, page.Page, page.PageSize))
}
