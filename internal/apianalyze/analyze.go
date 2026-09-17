// Package apianalyze 把归一化流量归纳成接口清单。
//
// 输入是 trafficparse 产出的流量记录，输出是可直接用于代码生成的清单。
// 分析分九步（见 skills/traffic-to-api-inventory）：
// 过滤静态资源 → 端点归并 → 路径参数泛化 → 请求结构 → 响应结构 →
// 统一外壳 → 分页模式 → 鉴权方式 → 分组。
package apianalyze

import (
	"sort"
	"strings"
	"time"

	"cyberstrike-ai/internal/trafficparse"
)

// Inventory 是一次分析的产出。
type Inventory struct {
	ProjectID   string      `json:"projectId"`
	GeneratedAt time.Time   `json:"generatedAt"`
	SourceCount int         `json:"sourceCount"` // 输入记录总数
	APICount    int         `json:"apiCount"`    // 过滤后接口候选数
	Hosts       []string    `json:"hosts"`
	Envelope    *Envelope   `json:"envelope,omitempty"`
	Auth        *AuthInfo   `json:"auth,omitempty"`
	Groups      []Group     `json:"groups"`
	Endpoints   []*Endpoint `json:"endpoints"`
}

// Envelope 描述统一响应外壳。
type Envelope struct {
	Detected     bool     `json:"detected"`
	SuccessField string   `json:"successField,omitempty"`
	SuccessCheck string   `json:"successCheck,omitempty"`
	DataField    string   `json:"dataField,omitempty"`
	MessageField string   `json:"messageField,omitempty"`
	CommonKeys   []string `json:"commonKeys,omitempty"`
	Coverage     string   `json:"coverage,omitempty"` // 如 "18/19 个接口"
}

// Pagination 描述分页模式。
type Pagination struct {
	Detected     bool   `json:"detected"`
	Mode         string `json:"mode,omitempty"`
	PageParam    string `json:"pageParam,omitempty"`
	SizeParam    string `json:"sizeParam,omitempty"`
	ListField    string `json:"listField,omitempty"`
	TotalField   string `json:"totalField,omitempty"`
	ObservedSize string `json:"observedSize,omitempty"` // 样本里实际用过的每页条数（非站点默认值）
}

// AuthInfo 描述鉴权方式与业务必带头。
type AuthInfo struct {
	Modes        []string          `json:"modes"`
	FixedHeaders map[string]string `json:"fixedHeaders,omitempty"`
	TokenPath    string            `json:"tokenPath,omitempty"`
	LoginPaths   []string          `json:"loginPaths,omitempty"`
}

// Analyze 对流量记录做完整分析。
func Analyze(projectID string, records []*trafficparse.Record) *Inventory {
	apis := trafficparse.FilterAPIs(records)

	accs := mergeEndpoints(apis)

	endpoints := make([]*Endpoint, 0, len(accs))
	for _, acc := range accs {
		ep := acc.ep
		ep.Query = FieldsFromValues(acc.queryValues, ep.CallCount)
		ep.Request = acc.reqBuilder.build()
		if ep.Request != nil && acc.reqMIME != "" {
			ep.Request.MIME = acc.reqMIME
		}
		ep.Response = acc.respBuilder.build()
		if ep.Response != nil && acc.respMIME != "" {
			ep.Response.MIME = acc.respMIME
		}
		endpoints = append(endpoints, ep)
	}

	sortEndpoints(endpoints)

	// 先识别外壳，再用它下探业务数据层做分页判定（顺序不能颠倒）
	envelope := detectEnvelope(endpoints)
	dataField := ""
	if envelope != nil {
		dataField = envelope.DataField
	}
	for _, ep := range endpoints {
		ep.Pagination = detectPagination(ep, dataField)
	}

	inv := &Inventory{
		ProjectID:   projectID,
		GeneratedAt: time.Now(),
		SourceCount: len(records),
		APICount:    len(apis),
		Hosts:       collectHosts(apis),
		Envelope:    envelope,
		Auth:        detectAuth(apis),
		Groups:      buildGroups(endpoints),
		Endpoints:   endpoints,
	}
	return inv
}

// responseFieldNames 返回用于分页判定的响应字段集合。
//
// 若外壳存在 data 字段，则下探到 data 内部——列表与总数通常在那里，
// 只看顶层会漏掉全部分页接口。
func responseFieldNames(ep *Endpoint, dataField string) map[string]bool {
	names := map[string]bool{}
	if ep.Response == nil || ep.Response.Type != "object" {
		return names
	}
	fields := ep.Response.Fields
	if dataField != "" {
		for _, f := range fields {
			if f.Name == dataField && len(f.Fields) > 0 {
				fields = f.Fields
				break
			}
		}
	}
	for _, f := range fields {
		names[f.Name] = true
	}
	return names
}

func sortEndpoints(eps []*Endpoint) {
	sort.Slice(eps, func(i, j int) bool {
		if eps[i].Module != eps[j].Module {
			return eps[i].Module < eps[j].Module
		}
		if eps[i].CallCount != eps[j].CallCount {
			return eps[i].CallCount > eps[j].CallCount
		}
		if eps[i].PathPattern != eps[j].PathPattern {
			return eps[i].PathPattern < eps[j].PathPattern
		}
		return eps[i].Method < eps[j].Method
	})
}

func collectHosts(recs []*trafficparse.Record) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range recs {
		if r.Host == "" || seen[r.Host] {
			continue
		}
		seen[r.Host] = true
		out = append(out, r.Host)
	}
	sort.Strings(out)
	return out
}

// detectEnvelope 识别统一响应外壳。
//
// 判定依据：多数接口的响应顶层键高度一致，且包含 code/status/success 之一。
// 识别出来必须写进清单——否则生成物会在每个接口上重复解包逻辑。
func detectEnvelope(eps []*Endpoint) *Envelope {
	type kv struct {
		keys   []string
		values []string
	}
	var samples []kv
	for _, ep := range eps {
		if ep.Response == nil || ep.Response.Type != "object" {
			continue
		}
		keys := make([]string, 0, len(ep.Response.Fields))
		for _, f := range ep.Response.Fields {
			keys = append(keys, f.Name)
		}
		sort.Strings(keys)
		samples = append(samples, kv{keys: keys})
	}
	if len(samples) < 2 {
		return nil
	}

	// 统计每个顶层键出现在多少个接口里
	count := map[string]int{}
	for _, s := range samples {
		for _, k := range s.keys {
			count[k]++
		}
	}
	var common []string
	for k, n := range count {
		// 允许少量接口不带外壳（如某些上传接口），取 80% 覆盖率
		if n*5 >= len(samples)*4 {
			common = append(common, k)
		}
	}
	sort.Strings(common)

	sharedCount := 0
	for _, s := range samples {
		if containsAll(s.keys, common) {
			sharedCount++
		}
	}

	env := &Envelope{CommonKeys: common, Coverage: coverageText(sharedCount, len(samples))}

	for _, cand := range []string{"code", "status", "success", "errno", "retCode", "resultCode"} {
		if contains(common, cand) {
			env.SuccessField = cand
			break
		}
	}
	if env.SuccessField == "" {
		return env
	}
	env.Detected = true
	for _, cand := range []string{"data", "result", "body", "payload", "rows"} {
		if contains(common, cand) {
			env.DataField = cand
			break
		}
	}
	for _, cand := range []string{"msg", "message", "messageText", "errorMsg", "errmsg"} {
		if contains(common, cand) {
			env.MessageField = cand
			break
		}
	}
	env.SuccessCheck = successCheck(eps, env.SuccessField)
	return env
}

// successCheck 从"HTTP 成功"的响应里归纳业务成功码。
//
// 空值与 null 会被排除：它们表示"该字段未返回"，不是成功码。
// 否则会得出 `code ∈ {200, null}` 这种无法直接用于判定的结论。
func successCheck(eps []*Endpoint, field string) string {
	values := map[string]bool{}
	for _, ep := range eps {
		if !isHTTPSuccess(ep.StatusCodes) || ep.Response == nil {
			continue
		}
		for _, f := range ep.Response.Fields {
			if f.Name != field {
				continue
			}
			candidates := f.Enum
			if f.Sample != "" {
				candidates = append(append([]string(nil), candidates...), f.Sample)
			}
			for _, v := range candidates {
				if v == "" || v == "null" {
					continue
				}
				values[v] = true
			}
		}
	}
	if len(values) == 0 {
		return ""
	}
	list := make([]string, 0, len(values))
	for v := range values {
		list = append(list, v)
	}
	sort.Strings(list)
	if len(list) == 1 {
		return field + " == " + list[0]
	}
	return field + " ∈ {" + strings.Join(list, ", ") + "}"
}

func isHTTPSuccess(codes []int) bool {
	for _, c := range codes {
		if c >= 200 && c < 300 {
			return true
		}
	}
	return len(codes) == 0
}

// detectPagination 识别分页模式。
//
// dataField 是统一外壳里承载业务数据的字段名：列表与总数通常在外壳内部
// （如 {code,data:{list,total},msg}），只看顶层会全部漏掉。
func detectPagination(ep *Endpoint, dataField string) *Pagination {
	reqNames := map[string]bool{}
	if ep.Request != nil {
		for _, f := range ep.Request.Fields {
			reqNames[f.Name] = true
		}
	}
	for _, f := range ep.Query {
		reqNames[f.Name] = true
	}

	pageParam := firstMatch(reqNames, []string{"pageNo", "pageNum", "page", "current", "pageIndex", "pageNumber"})
	sizeParam := firstMatch(reqNames, []string{"pageSize", "size", "limit", "perPage", "rows"})
	cursorParam := firstMatch(reqNames, []string{"cursor", "nextToken", "offset"})

	respNames := responseFieldNames(ep, dataField)
	listField := firstMatch(respNames, []string{"list", "records", "rows", "items", "dataList", "content"})
	totalField := firstMatch(respNames, []string{"total", "totalCount", "count", "totalElements", "totalNum"})

	switch {
	case pageParam != "" && sizeParam != "" && listField != "":
		// 页码式分页必须同时满足：请求有页码与页大小、响应有列表字段。
		// 只看请求会误判——很多统计接口与列表接口共用同一套查询条件模板。
		p := &Pagination{
			Detected:   true,
			Mode:       "page",
			PageParam:  pageParam,
			SizeParam:  sizeParam,
			ListField:  listField,
			TotalField: totalField,
		}
		p.ObservedSize = observedValue(ep, sizeParam)
		return p
	case cursorParam != "":
		return &Pagination{Detected: true, Mode: "cursor", PageParam: cursorParam, ListField: listField}
	case listField != "" && totalField != "":
		// 有列表与总数但没有分页参数：可能是"全量返回"，标出但不算分页
		return &Pagination{ListField: listField, TotalField: totalField}
	}
	return nil
}

// observedValue 取参数在样本中的实际取值，用于推断默认页大小。
func observedValue(ep *Endpoint, name string) string {
	for _, f := range ep.Query {
		if f.Name == name {
			if f.Fixed || len(f.Enum) > 0 {
				return f.Sample
			}
		}
	}
	if ep.Request != nil {
		for _, f := range ep.Request.Fields {
			if f.Name == name {
				return f.Sample
			}
		}
	}
	return ""
}

func firstMatch(names map[string]bool, cands []string) string {
	for _, c := range cands {
		if names[c] {
			return c
		}
	}
	// 大小写不敏感的回退匹配
	for n := range names {
		for _, c := range cands {
			if strings.EqualFold(n, c) {
				return n
			}
		}
	}
	return ""
}

// detectAuth 识别鉴权方式与业务必带头。
func detectAuth(recs []*trafficparse.Record) *AuthInfo {
	info := &AuthInfo{Modes: []string{}}

	modeSet := map[string]bool{}
	headerValues := map[string][]string{}
	headerCount := map[string]int{}

	for _, r := range recs {
		for k, v := range r.ReqHeaders {
			headerValues[k] = append(headerValues[k], v)
			headerCount[k]++
		}
		// 登录类端点
		lower := strings.ToLower(r.Path)
		if strings.Contains(lower, "oauth2/token") || strings.HasSuffix(lower, "/token") {
			info.LoginPaths = appendUnique(info.LoginPaths, r.Path, 5)
			modeSet["oauth2"] = true
			if info.TokenPath == "" {
				info.TokenPath = r.Path
			}
		} else if strings.Contains(lower, "/login") || strings.Contains(lower, "/signin") {
			info.LoginPaths = appendUnique(info.LoginPaths, r.Path, 5)
		}
	}

	for k, vals := range headerValues {
		switch k {
		case "cookie":
			modeSet["cookie"] = true
		case "authorization":
			for _, v := range vals {
				if strings.HasPrefix(strings.ToLower(strings.TrimSpace(v)), "bearer") {
					modeSet["bearer"] = true
				}
			}
		}
	}

	total := len(recs)
	fixed := map[string]string{}
	if total > 0 {
		for k, vals := range headerValues {
			if standardHeaders[k] || headerCount[k]*10 < total*9 {
				continue // 排除标准头，以及出现率不足 90% 的头
			}
			v := vals[0]
			allSame := true
			for _, x := range vals {
				if x != v {
					allSame = false
					break
				}
			}
			if allSame && v != "" {
				fixed[k] = v
			}
		}
	}
	if len(fixed) > 0 {
		info.FixedHeaders = fixed
	}

	for m := range modeSet {
		info.Modes = append(info.Modes, m)
	}
	sort.Strings(info.Modes)
	return info
}

// standardHeaders 是标准 HTTP 头，不属于"业务必带头"的范畴。
var standardHeaders = map[string]bool{
	"accept":          true,
	"accept-charset":  true,
	"cache-control":   true,
	"content-type":    true,
	"origin":          true,
	"pragma":          true,
	"x-requested-with": false, // 这个是业务特征，保留
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// containsAll 判断 list 是否包含 want 中的每一个元素。
func containsAll(list, want []string) bool {
	for _, w := range want {
		if !contains(list, w) {
			return false
		}
	}
	return true
}

func coverageText(n, total int) string {
	return itoa(n) + "/" + itoa(total) + " 个接口共享"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}