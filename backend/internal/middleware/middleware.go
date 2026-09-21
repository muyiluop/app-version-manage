// Package middleware 提供 HTTP 中间件：请求 ID、恢复、访问日志、CORS、请求体限制。
package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"app_version_manage/internal/apierr"
)

// RequestIDKey 上下文中请求 ID 的键，与 apierr 保持一致。
const RequestIDKey = apierr.RequestIDKey

// RequestID 为每个请求生成唯一 ID，并写入响应头。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-Id"))
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(RequestIDKey, id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

// Recovery 捕获 panic，记录日志并返回统一错误响应。
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("请求处理发生 panic",
					"requestId", c.GetString(RequestIDKey),
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"panic", rec,
				)
				apierr.FailRaw(c, apierr.CodeInternal, http.StatusInternalServerError, "服务器内部错误")
			}
		}()
		c.Next()
	}
}

// AccessLog 记录访问日志。
func AccessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []any{
			"requestId", c.GetString(RequestIDKey),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"cost", time.Since(start).String(),
			"ip", c.ClientIP(),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		switch {
		case c.Writer.Status() >= 500:
			log.Error("请求处理失败", attrs...)
		case c.Writer.Status() >= 400:
			log.Warn("请求被拒绝", attrs...)
		default:
			log.Info("请求完成", attrs...)
		}
	}
}

// CORS 按白名单放行跨域请求；白名单为空时不输出任何 CORS 头。
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range allowedOrigins {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}

	return func(c *gin.Context) {
		if len(allowed) == 0 {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin != "" && (allowed[origin] || allowed["*"]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-Id,X-Platform")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// SecurityHeaders 输出基础安全响应头。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

// MaxBodySize 限制请求体大小（上传接口按配置放行）。
func MaxBodySize(limitBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limitBytes)
		}
		c.Next()
	}
}
