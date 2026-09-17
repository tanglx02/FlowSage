package trafficparse

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// harFile 是 HAR 1.2 的最小结构映射，只保留归一化需要的字段。
type harFile struct {
	Log struct {
		Version string `json:"version"`
		Creator struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"creator"`
		Entries []harEntry `json:"entries"`
	} `json:"log"`
}

type harEntry struct {
	StartedDateTime string `json:"startedDateTime"`
	Time            float64 `json:"time"`
	Request         struct {
		Method      string       `json:"method"`
		URL         string       `json:"url"`
		Headers     []harNameVal `json:"headers"`
		QueryString []harNameVal `json:"queryString"`
		PostData    struct {
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"postData"`
	} `json:"request"`
	Response struct {
		Status  int          `json:"status"`
		Headers []harNameVal `json:"headers"`
		Content struct {
			Size     int    `json:"size"`
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"content"`
	} `json:"response"`
}

type harNameVal struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ParseHAR 解析 HAR 1.2 内容为归一化流量记录。
//
// projectID 用于把记录归属到某个项目；HAR 里没有项目概念，由调用方指定。
// 不做过多的字段校验——真实导出的 HAR 常有缺字段，缺什么就留空，由分析阶段处理。
func ParseHAR(r io.Reader, projectID string) ([]*Record, error) {
	var hf harFile
	dec := json.NewDecoder(r)
	if err := dec.Decode(&hf); err != nil {
		return nil, fmt.Errorf("解析 HAR 失败（不是合法的 JSON 或结构不符）: %w", err)
	}
	if hf.Log.Version == "" && len(hf.Log.Entries) == 0 {
		return nil, fmt.Errorf("解析 HAR 失败：缺少 log.entries 字段，请确认导出的是 HAR 格式")
	}

	records := make([]*Record, 0, len(hf.Log.Entries))
	for i, e := range hf.Log.Entries {
		rec := &Record{
			ID:        fmt.Sprintf("%s-%d", projectID, i+1),
			ProjectID: projectID,
			Source:    SourceHAR,
			Method:    strings.ToUpper(e.Request.Method),
			URL:       e.Request.URL,
			Status:    e.Response.Status,
			DurationMS: int(e.Time),
			Size:      e.Response.Content.Size,
		}

		rec.Host, rec.Path, rec.Query = ParseURL(e.Request.URL)
		rec.ReqHeaders = NormalizeHeaders(toPairs(e.Request.Headers))
		rec.RespHeaders = NormalizeHeaders(toPairs(e.Response.Headers))
		rec.ReqMIME = e.Request.PostData.MimeType
		rec.ReqBody = e.Request.PostData.Text
		rec.RespMIME = e.Response.Content.MimeType
		rec.RespBody = e.Response.Content.Text

		if ts, err := time.Parse(time.RFC3339Nano, e.StartedDateTime); err == nil {
			rec.StartedAt = ts
		} else if ts, err := time.Parse(time.RFC3339, e.StartedDateTime); err == nil {
			rec.StartedAt = ts
		}

		records = append(records, rec)
	}
	return records, nil
}

// ParseHARFile 从文件解析 HAR。
func ParseHARFile(path, projectID string) ([]*Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开 HAR 文件失败: %w", err)
	}
	defer f.Close()
	return ParseHAR(f, projectID)
}

func toPairs(in []harNameVal) [][2]string {
	if len(in) == 0 {
		return nil
	}
	out := make([][2]string, 0, len(in))
	for _, kv := range in {
		out = append(out, [2]string{kv.Name, kv.Value})
	}
	return out
}

// FilterAPIs 从记录中筛出接口候选。
func FilterAPIs(records []*Record) []*Record {
	out := make([]*Record, 0, len(records))
	for _, r := range records {
		if r.IsAPI() {
			out = append(out, r)
		}
	}
	return out
}