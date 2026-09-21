package router

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"app_version_manage/internal/service"
)

type healthHandler struct {
	svc *service.Services
	log *slog.Logger
}

func newHealthHandler(svc *service.Services, log *slog.Logger) *healthHandler {
	return &healthHandler{svc: svc, log: log}
}

// Live 存活探针。
func (h *healthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready 就绪探针，探测数据库连通性。
func (h *healthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := h.svc.Store().Ping(ctx); err != nil {
		h.log.Error("就绪检查失败", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "error": "数据库不可用"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
