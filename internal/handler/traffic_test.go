package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// trafficTestHAR 是一份小的合成 HAR：1 条静态资源 + 3 条接口调用。
// 用于固定验证「上传 → 过滤 → 归并 → 外壳/分页识别 → 落库 → 读取 → 删除」整条链。
const trafficTestHAR = `{
  "log": {
    "version": "1.2",
    "creator": {"name": "test", "version": "1.0"},
    "entries": [
      {
        "startedDateTime": "2026-09-18T10:00:00.000Z",
        "time": 12.5,
        "request": {"method": "GET", "url": "https://demo.local/static/app.js", "headers": [], "queryString": []},
        "response": {"status": 200, "headers": [], "content": {"size": 5120, "mimeType": "application/javascript", "text": "console.log(1)"}}
      },
      {
        "startedDateTime": "2026-09-18T10:00:01.000Z",
        "time": 88.1,
        "request": {
          "method": "POST",
          "url": "https://demo.local/tsgz/api/v1/alarms/queryAdvPage?pageNo=1&pageSize=20",
          "headers": [{"name": "x-bff-mode", "value": "true"}, {"name": "Content-Type", "value": "application/json"}],
          "queryString": [{"name": "pageNo", "value": "1"}, {"name": "pageSize", "value": "20"}],
          "postData": {"mimeType": "application/json", "text": "{\"alarmLevel\":\"1\"}"}
        },
        "response": {
          "status": 200,
          "headers": [],
          "content": {"size": 240, "mimeType": "application/json", "text": "{\"code\":200,\"data\":{\"list\":[{\"id\":\"a1\",\"name\":\"越权访问\",\"level\":1}],\"total\":3},\"msg\":\"ok\"}"}
        }
      },
      {
        "startedDateTime": "2026-09-18T10:00:02.000Z",
        "time": 40.0,
        "request": {
          "method": "GET",
          "url": "https://demo.local/tsgz/api/v1/alarms/4c4561aabbccddeeff00112233445566",
          "headers": [{"name": "x-bff-mode", "value": "true"}],
          "queryString": []
        },
        "response": {
          "status": 200,
          "headers": [],
          "content": {"size": 120, "mimeType": "application/json", "text": "{\"code\":200,\"data\":{\"id\":\"a1\",\"name\":\"越权访问\",\"level\":1},\"msg\":\"ok\"}"}
        }
      },
      {
        "startedDateTime": "2026-09-18T10:00:03.000Z",
        "time": 33.0,
        "request": {
          "method": "GET",
          "url": "https://demo.local/tsgz/api/v1/sysMenu/routes",
          "headers": [{"name": "Authorization", "value": "Bearer abc"}],
          "queryString": []
        },
        "response": {
          "status": 200,
          "headers": [],
          "content": {"size": 90, "mimeType": "application/json", "text": "{\"code\":200,\"data\":[{\"path\":\"/alarm\",\"name\":\"告警\"}],\"msg\":\"ok\"}"}
        }
      }
    ]
  }
}`

func newTrafficTestHandler(t *testing.T) *TrafficHandler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := database.NewDB(filepath.Join(t.TempDir(), "traffic-handler.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewTrafficHandler(db, zap.NewNop())
}

func trafficMultipartBody(t *testing.T, filename, content string, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func TestTrafficHandlerUploadListGetDelete(t *testing.T) {
	h := newTrafficTestHandler(t)
	body, contentType := trafficMultipartBody(t, "demo.har", trafficTestHAR, map[string]string{"project_id": "proj-1"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/traffic/har", body)
	c.Request.Header.Set("Content-Type", contentType)
	c.Set(security.ContextSessionKey, security.Session{UserID: "user-1", Scope: database.RBACScopeAll})
	h.UploadHAR(c)
	if w.Code != http.StatusOK {
		t.Fatalf("上传失败: %d %s", w.Code, w.Body.String())
	}

	var uploaded struct {
		ID            string `json:"id"`
		RecordCount   int    `json:"recordCount"`
		EndpointCount int    `json:"endpointCount"`
		Host          string `json:"host"`
		Analysis      struct {
			Envelope struct {
				Detected     bool   `json:"detected"`
				SuccessCheck string `json:"successCheck"`
			} `json:"envelope"`
			Endpoints []struct {
				Method      string `json:"method"`
				PathPattern string `json:"pathPattern"`
				Pagination  *struct {
					Detected bool `json:"detected"`
				} `json:"pagination"`
			} `json:"endpoints"`
		} `json:"analysis"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.RecordCount != 4 {
		t.Fatalf("记录数 = %d, 期望 4", uploaded.RecordCount)
	}
	// 4 条记录里 1 条是静态资源，剩 3 条接口；alarms 的两条归并为一个泛化路径
	if uploaded.EndpointCount != 3 {
		t.Fatalf("接口数 = %d, 期望 3", uploaded.EndpointCount)
	}
	if uploaded.Host != "demo.local" {
		t.Fatalf("主机 = %q, 期望 demo.local", uploaded.Host)
	}
	if !uploaded.Analysis.Envelope.Detected || uploaded.Analysis.Envelope.SuccessCheck != "code == 200" {
		t.Fatalf("外壳识别异常: %+v", uploaded.Analysis.Envelope)
	}
	patterns := map[string]bool{}
	pagedPattern := ""
	for _, ep := range uploaded.Analysis.Endpoints {
		patterns[ep.Method+" "+ep.PathPattern] = true
		if ep.Pagination != nil && ep.Pagination.Detected {
			pagedPattern = ep.PathPattern
		}
	}
	if !patterns["GET /tsgz/api/v1/alarms/{id}"] {
		t.Fatalf("路径参数未泛化: %v", patterns)
	}
	if pagedPattern != "/tsgz/api/v1/alarms/queryAdvPage" {
		t.Fatalf("分页识别异常: %q", pagedPattern)
	}

	// 列表：只返回摘要，不带 payload
	lw := httptest.NewRecorder()
	lc, _ := gin.CreateTestContext(lw)
	lc.Request = httptest.NewRequest(http.MethodGet, "/api/traffic/inventories?project_id=proj-1", nil)
	lc.Set(security.ContextSessionKey, security.Session{UserID: "user-1", Scope: database.RBACScopeAll})
	h.ListInventories(lc)
	if lw.Code != http.StatusOK {
		t.Fatalf("列表失败: %d %s", lw.Code, lw.Body.String())
	}
	if !strings.Contains(lw.Body.String(), `"total":1`) || strings.Contains(lw.Body.String(), `"payload"`) {
		t.Fatalf("列表响应异常: %s", lw.Body.String())
	}

	// 详情：返回完整分析结果
	gw := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(gw)
	gc.Request = httptest.NewRequest(http.MethodGet, "/api/traffic/inventories/"+uploaded.ID, nil)
	gc.Params = gin.Params{{Key: "id", Value: uploaded.ID}}
	gc.Set(security.ContextSessionKey, security.Session{UserID: "user-1", Scope: database.RBACScopeAll})
	h.GetInventory(gc)
	if gw.Code != http.StatusOK || !strings.Contains(gw.Body.String(), `"pathPattern":"/tsgz/api/v1/alarms/queryAdvPage"`) {
		t.Fatalf("详情失败: %d %s", gw.Code, gw.Body.String())
	}

	// 非所有者且非全局范围时拒绝访问
	fw := httptest.NewRecorder()
	fc, _ := gin.CreateTestContext(fw)
	fc.Request = httptest.NewRequest(http.MethodGet, "/api/traffic/inventories/"+uploaded.ID, nil)
	fc.Params = gin.Params{{Key: "id", Value: uploaded.ID}}
	fc.Set(security.ContextSessionKey, security.Session{UserID: "user-2", Scope: database.RBACScopeOwn})
	h.GetInventory(fc)
	if fw.Code != http.StatusForbidden {
		t.Fatalf("越权访问未被拒绝: %d %s", fw.Code, fw.Body.String())
	}

	// 删除
	dw := httptest.NewRecorder()
	dc, _ := gin.CreateTestContext(dw)
	dc.Request = httptest.NewRequest(http.MethodDelete, "/api/traffic/inventories/"+uploaded.ID, nil)
	dc.Params = gin.Params{{Key: "id", Value: uploaded.ID}}
	dc.Set(security.ContextSessionKey, security.Session{UserID: "user-1", Scope: database.RBACScopeAll})
	h.DeleteInventory(dc)
	if dw.Code != http.StatusOK {
		t.Fatalf("删除失败: %d %s", dw.Code, dw.Body.String())
	}

	aw := httptest.NewRecorder()
	ac, _ := gin.CreateTestContext(aw)
	ac.Request = httptest.NewRequest(http.MethodGet, "/api/traffic/inventories/"+uploaded.ID, nil)
	ac.Params = gin.Params{{Key: "id", Value: uploaded.ID}}
	ac.Set(security.ContextSessionKey, security.Session{UserID: "user-1", Scope: database.RBACScopeAll})
	h.GetInventory(ac)
	if aw.Code != http.StatusNotFound {
		t.Fatalf("删除后仍可读取: %d", aw.Code)
	}
}

func TestTrafficHandlerRejectsNonHAR(t *testing.T) {
	h := newTrafficTestHandler(t)
	body, contentType := trafficMultipartBody(t, "broken.har", "{not json", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/traffic/har", body)
	c.Request.Header.Set("Content-Type", contentType)
	c.Set(security.ContextSessionKey, security.Session{UserID: "user-1", Scope: database.RBACScopeAll})
	h.UploadHAR(c)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("非法 HAR 未被拒绝: %d %s", w.Code, w.Body.String())
	}
}