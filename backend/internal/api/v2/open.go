package v2

import (
	"strings"

	"github.com/gin-gonic/gin"

	"app_version_manage/internal/api/common"
	"app_version_manage/internal/apierr"
	"app_version_manage/internal/model"
)

// Check 检测更新（v2 语义化接口）。
//
// GET /api/v2/check?identifier=&platform=&channel=&currentVersion=
func (h *Handler) Check(c *gin.Context) {
	identifier := strings.TrimSpace(c.Query("identifier"))
	if identifier == "" {
		apierr.Fail(c, apierr.BadRequest("identifier 不能为空"))
		return
	}
	platform := model.Platform(strings.TrimSpace(c.Query("platform")))
	if platform == "" {
		platform = model.Platform(common.ClientPlatform(c))
	}

	result, err := h.svc.Version.Check(c.Request.Context(), identifier, platform, c.Query("channel"), c.Query("currentVersion"))
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, result)
}

// Download 通过签名令牌下载文件（公开接口）。
func (h *Handler) Download(c *gin.Context) {
	if err := common.ServeDownload(c, h.svc.File, h.svc.Signer(), c.Param("token")); err != nil {
		apierr.Fail(c, err)
	}
}
