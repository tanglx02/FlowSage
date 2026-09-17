// Package trafficparse 把不同来源的抓包数据归一化成统一的流量记录。
//
// 四种通道（HAR 导入 / 浏览器扩展回传 / 托管浏览器 CDP / CDP 连接现有浏览器）
// 最终都产出同一份 Record，分析引擎只认这一种格式，通道之间不各写一套逻辑。
package trafficparse

import (
	"net/url"
	"path"
	"strings"
	"time"
)

// Source 标识流量来源通道。
type Source string

const (
	SourceHAR       Source = "har"       // 人工从 DevTools 导出的 .har 文件
	SourceExtension Source = "extension" // 浏览器扩展回传
	SourceCDP       Source = "cdp"       // CDP 实时捕获（托管浏览器或连接现有浏览器）
)

// Record 是一条归一化后的流量记录。
type Record struct {
	ID         string            `json:"id"`
	ProjectID  string            `json:"projectId"`
	Source     Source            `json:"source"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Host       string            `json:"host"`
	Path       string            `json:"path"`
	Query      map[string]string `json:"query,omitempty"`
	ReqHeaders map[string]string `json:"reqHeaders,omitempty"`
	ReqBody    string            `json:"reqBody,omitempty"`
	ReqMIME    string            `json:"reqMime,omitempty"`
	Status     int               `json:"status"`
	RespHeaders map[string]string `json:"respHeaders,omitempty"`
	RespBody   string            `json:"respBody,omitempty"`
	RespMIME   string            `json:"respMime,omitempty"`
	StartedAt  time.Time         `json:"startedAt"`
	DurationMS int               `json:"durationMs"`
	Size       int               `json:"size"`
}

// IsAPI 判断该记录是否可能是接口调用。
//
// 抓包里绝大部分是静态资源，先过滤再分析——实测某站点 286 条流量中只有 49 条是接口。
// 命中任一条件即认为是接口候选。
func (r *Record) IsAPI() bool {
	// 写操作基本都是接口
	switch r.Method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}

	if isStaticAsset(r.Path) {
		return false
	}
	if isStaticMIME(r.RespMIME) || isStaticMIME(r.ReqMIME) {
		return false
	}

	// 响应是 JSON/XML 的基本都是接口
	if isJSONMIME(r.RespMIME) || isXMLMIME(r.RespMIME) {
		return true
	}

	// 路径里带接口特征词
	lower := strings.ToLower(r.Path)
	for _, hint := range []string{"/api", "/v1/", "/v2/", "/v3/", "/rest", "/graphql", "/rpc", "/oauth"} {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// staticExts 静态资源扩展名。
var staticExts = map[string]bool{
	".js": true, ".mjs": true, ".css": true, ".map": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".webp": true, ".ico": true, ".bmp": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".mp4": true, ".mp3": true, ".webm": true, ".avi": true,
	".pdf": true, ".zip": true, ".gz": true, ".tar": true,
	".html": true, ".htm": true,
}

func isStaticAsset(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	return staticExts[ext]
}

func isStaticMIME(m string) bool {
	m = strings.ToLower(strings.TrimSpace(strings.Split(m, ";")[0]))
	if m == "" {
		return false
	}
	return strings.HasPrefix(m, "text/css") ||
		strings.HasPrefix(m, "image/") ||
		strings.HasPrefix(m, "font/") ||
		strings.HasPrefix(m, "audio/") ||
		strings.HasPrefix(m, "video/")
}

func isJSONMIME(m string) bool {
	m = strings.ToLower(m)
	return strings.Contains(m, "json")
}

func isXMLMIME(m string) bool {
	m = strings.ToLower(m)
	return strings.Contains(m, "xml")
}

// ParseURL 从完整 URL 中拆出 host / path / query。
func ParseURL(raw string) (host, p string, query map[string]string) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", raw, nil
	}
	host = u.Host
	p = u.Path
	if len(u.Query()) > 0 {
		query = make(map[string]string, len(u.Query()))
		for k, v := range u.Query() {
			if len(v) > 0 {
				query[k] = v[0]
			}
		}
	}
	return host, p, query
}

// NormalizeHeaders 把 header 列表折叠为小写键的字典，并去掉噪声头。
//
// 噪声头（Cookie 值、时序、trace 等）保留会造成每条记录都不同，干扰"固定头"识别。
func NormalizeHeaders(pairs [][2]string) map[string]string {
	if len(pairs) == 0 {
		return nil
	}
	out := make(map[string]string, len(pairs))
	for _, kv := range pairs {
		k := strings.ToLower(strings.TrimSpace(kv[0]))
		if k == "" || noiseHeaders[k] {
			continue
		}
		out[k] = kv[1]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// noiseHeaders 每次请求都不同、对理解接口无帮助的头。
var noiseHeaders = map[string]bool{
	"accept-encoding":  true,
	"connection":       true,
	"content-length":   true,
	"host":             true,
	"if-none-match":    true,
	"if-modified-since": true,
	"referer":          true,
	"sec-ch-ua":        true,
	"sec-ch-ua-mobile": true,
	"sec-ch-ua-platform": true,
	"sec-fetch-dest":   true,
	"sec-fetch-mode":   true,
	"sec-fetch-site":   true,
	"sec-fetch-user":   true,
	"upgrade-insecure-requests": true,
	"user-agent":       true,
	"accept-language":  true,
	"traceparent":      true,
	"x-request-id":     true,
	"priority":         true,
}