package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"cyberstrike-ai/internal/apianalyze"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"
	"cyberstrike-ai/internal/trafficparse"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// maxHARUploadBytes 限制单个 HAR 文件大小。真实站点抓包通常在几 MB 到几十 MB。
const maxHARUploadBytes = 128 << 20

// TrafficHandler 流量接入与接口清单处理器。
//
// 当前实现第一种通道（HAR 文件导入）：上传 → 解析 → 分析 → 落库。
// 其余三种通道（浏览器扩展 / 托管浏览器 / CDP 连接）产出同一份
// trafficparse.Record 后复用同一条分析链，待后续阶段接入。
type TrafficHandler struct {
	db     *database.DB
	logger *zap.Logger
}

// NewTrafficHandler 创建流量处理器。
func NewTrafficHandler(db *database.DB, logger *zap.Logger) *TrafficHandler {
	return &TrafficHandler{db: db, logger: logger}
}

// canAccessInventory 判断当前会话能否读写指定归属的清单。
func (h *TrafficHandler) canAccessInventory(c *gin.Context, ownerUserID string) bool {
	session, ok := security.CurrentSession(c)
	if !ok {
		return false
	}
	if session.Scope == database.RBACScopeAll {
		return true
	}
	return session.UserID != "" && session.UserID == ownerUserID
}

// UploadHAR POST /api/traffic/har
//
// multipart/form-data: file=<.har>，可选 name / project_id。
func (h *TrafficHandler) UploadHAR(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxHARUploadBytes+1)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "HAR 文件超过大小限制（最大 128 MB）"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传 .har 文件"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxHARUploadBytes+1))
	if err != nil || len(data) > maxHARUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "HAR 文件超过大小限制（最大 128 MB）"})
		return
	}

	projectID := strings.TrimSpace(c.PostForm("project_id"))
	records, err := trafficparse.ParseHAR(bytes.NewReader(data), projectID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "HAR 解析失败: " + err.Error()})
		return
	}
	if len(records) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "HAR 里没有任何请求记录"})
		return
	}

	inventory := apianalyze.Analyze(projectID, records)
	if len(inventory.Endpoints) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "没有识别出接口，请确认抓包中包含业务请求"})
		return
	}

	payload, err := json.Marshal(inventory)
	if err != nil {
		h.logger.Error("序列化接口清单失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "接口清单序列化失败"})
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		name = strings.TrimSpace(header.Filename)
	}
	if name == "" {
		name = "未命名抓包"
	}

	owner := ""
	if session, ok := security.CurrentSession(c); ok {
		owner = session.UserID
	}

	saved, err := h.db.CreateAPIInventory(&database.APIInventory{
		ProjectID:     projectID,
		Name:          name,
		Source:        string(trafficparse.SourceHAR),
		Host:          primaryHost(records),
		RecordCount:   len(records),
		APICount:      inventory.APICount,
		EndpointCount: len(inventory.Endpoints),
		Payload:       string(payload),
		OwnerUserID:   owner,
	})
	if err != nil {
		h.logger.Error("保存接口清单失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            saved.ID,
		"name":          saved.Name,
		"source":        saved.Source,
		"projectId":     saved.ProjectID,
		"host":          saved.Host,
		"recordCount":   saved.RecordCount,
		"apiCount":      saved.APICount,
		"endpointCount": saved.EndpointCount,
		"createdAt":     saved.CreatedAt,
		"analysis":      inventory,
	})
}

// ListInventories GET /api/traffic/inventories?project_id=&page=&page_size=
func (h *TrafficHandler) ListInventories(c *gin.Context) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", "50")))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	owner := h.scopeOwner(c)
	items, total, err := h.db.ListAPIInventories(
		strings.TrimSpace(c.Query("project_id")), owner, pageSize, (page-1)*pageSize)
	if err != nil {
		h.logger.Error("查询接口清单失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

// GetInventory GET /api/traffic/inventories/:id
func (h *TrafficHandler) GetInventory(c *gin.Context) {
	item, err := h.db.GetAPIInventory(c.Param("id"))
	if err != nil {
		h.logger.Error("读取接口清单失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "接口清单不存在"})
		return
	}
	if !h.canAccessInventory(c, item.OwnerUserID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该接口清单"})
		return
	}

	item.OwnerUserID = ""
	payload := json.RawMessage(item.Payload)
	item.Payload = ""
	c.JSON(http.StatusOK, gin.H{
		"id":            item.ID,
		"name":          item.Name,
		"source":        item.Source,
		"projectId":     item.ProjectID,
		"host":          item.Host,
		"recordCount":   item.RecordCount,
		"apiCount":      item.APICount,
		"endpointCount": item.EndpointCount,
		"createdAt":     item.CreatedAt,
		"analysis":      payload,
	})
}

// DeleteInventory DELETE /api/traffic/inventories/:id
func (h *TrafficHandler) DeleteInventory(c *gin.Context) {
	item, err := h.db.GetAPIInventory(c.Param("id"))
	if err != nil {
		h.logger.Error("读取接口清单失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "接口清单不存在"})
		return
	}
	if !h.canAccessInventory(c, item.OwnerUserID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该接口清单"})
		return
	}
	if err := h.db.DeleteAPIInventory(item.ID); err != nil {
		h.logger.Error("删除接口清单失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": item.ID})
}

// scopeOwner 返回列表过滤用的归属用户；全局范围返回空串表示不过滤。
func (h *TrafficHandler) scopeOwner(c *gin.Context) string {
	session, ok := security.CurrentSession(c)
	if !ok || session.Scope == database.RBACScopeAll {
		return ""
	}
	return session.UserID
}

// primaryHost 取样本里出现最多的主机名，作为清单的主机标识。
func primaryHost(records []*trafficparse.Record) string {
	counts := map[string]int{}
	for _, r := range records {
		if r.Host != "" {
			counts[r.Host]++
		}
	}
	best, bestCount := "", 0
	for host, n := range counts {
		if n > bestCount || (n == bestCount && host < best) {
			best, bestCount = host, n
		}
	}
	return best
}