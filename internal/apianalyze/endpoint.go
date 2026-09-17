package apianalyze

import (
	"regexp"
	"sort"
	"strings"

	"cyberstrike-ai/internal/trafficparse"
)

// Endpoint 是接口清单里的一条接口。
type Endpoint struct {
	Method      string      `json:"method"`
	PathPattern string      `json:"pathPattern"`
	PathParams  []string    `json:"pathParams,omitempty"`
	Module      string      `json:"module"`
	Summary     string      `json:"summary"`
	CallCount   int         `json:"callCount"`
	StatusCodes []int       `json:"statusCodes"`
	Query       []Field     `json:"query,omitempty"`
	Request     *Schema     `json:"request,omitempty"`
	Response    *Schema     `json:"response,omitempty"`
	Pagination  *Pagination `json:"pagination,omitempty"`
	SampleIDs   []string    `json:"sampleIds,omitempty"`
}

// Group 是按路径前缀归纳出的功能模块。
type Group struct {
	Name      string   `json:"name"`
	Prefix    string   `json:"prefix"`
	Endpoints []string `json:"endpoints"`
}

// idSegmentRe 匹配形如 UUID 或长十六进制的路径段。
var idSegmentRe = regexp.MustCompile(`^[0-9a-fA-F]{16,}$|^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// allDigitsRe 匹配纯数字段。
var allDigitsRe = regexp.MustCompile(`^\d+$`)

// generalizePath 把路径里的实例标识替换为 {id}，并返回是否发生泛化。
//
// 只替换"明显是实例值"的段（纯数字 / UUID / 长十六进制），
// 不做基于命名规律的外推——猜错会把不同接口错误合并。
func generalizePath(p string) (string, []string) {
	segs := strings.Split(strings.Trim(p, "/"), "/")
	var params []string
	changed := false
	for i, s := range segs {
		if allDigitsRe.MatchString(s) || idSegmentRe.MatchString(s) {
			segs[i] = "{id}"
			changed = true
		}
	}
	if !changed {
		return p, nil
	}
	params = []string{"id"}
	return "/" + strings.Join(segs, "/"), params
}

// endpointKey 是端点归并键。
type endpointKey struct {
	method  string
	pattern string
}

// endpointAccum 在归并过程中累积同一端点的观察结果。
type endpointAccum struct {
	ep          *Endpoint
	queryValues map[string][]string
	reqBuilder  *schemaBuilder
	respBuilder *schemaBuilder
	reqMIME     string
	respMIME    string
}

// mergeEndpoints 把流量记录归并为端点列表。
func mergeEndpoints(recs []*trafficparse.Record) map[endpointKey]*endpointAccum {
	out := make(map[endpointKey]*endpointAccum)

	for _, r := range recs {
		pattern, params := generalizePath(r.Path)
		key := endpointKey{method: r.Method, pattern: pattern}

		acc, ok := out[key]
		if !ok {
			acc = &endpointAccum{
				ep: &Endpoint{
					Method:      r.Method,
					PathPattern: pattern,
					PathParams:  params,
					Summary:     summarize(pattern),
					Module:      moduleOf(pattern),
				},
				queryValues: map[string][]string{},
				reqBuilder:  newSchemaBuilder(),
				respBuilder: newSchemaBuilder(),
			}
			out[key] = acc
		}

		acc.ep.CallCount++
		acc.ep.SampleIDs = appendUnique(acc.ep.SampleIDs, r.ID, 5)
		acc.ep.StatusCodes = appendUniqueInt(acc.ep.StatusCodes, r.Status)
		for k, v := range r.Query {
			acc.queryValues[k] = append(acc.queryValues[k], v)
		}
		if r.ReqMIME != "" {
			acc.reqMIME = r.ReqMIME
		}
		if r.RespMIME != "" {
			acc.respMIME = r.RespMIME
		}
		acc.reqBuilder.add(r.ReqBody)
		acc.respBuilder.add(r.RespBody)
	}
	return out
}

// summarize 取路径最后一段作为业务名（接口方法名通常比路径更有信息量）。
func summarize(pattern string) string {
	segs := strings.Split(strings.Trim(pattern, "/"), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		s := segs[i]
		if s == "" || s == "{id}" || isVersionOrAPISegment(s) {
			continue
		}
		return s
	}
	return pattern
}

// moduleOf 取路径中的"资源名"作为模块，即倒数第二段。
//
// 接口路径通常形如 .../<资源>/<动作>，以资源名分组最贴近人的理解：
//
//	/tsgz/nspt-tsgz-alarmquery/api/v1/alarms/queryAdvPage → alarms
//	/tsgz/nspt-tsgz-system/sysDict/options                → sysDict
//	/tsgz/oauth2/token                                    → oauth2
func moduleOf(pattern string) string {
	segs := splitPath(pattern)
	for i := len(segs) - 1; i >= 0; i-- {
		s := segs[i]
		if s == "" || s == "{id}" || isVersionOrAPISegment(s) {
			continue
		}
		// 最后一段是动作名，模块取它前面那段；只有单段路径时才用本身
		if i == len(segs)-1 && len(segs) >= 2 {
			continue
		}
		return s
	}
	if len(segs) > 0 {
		return segs[0]
	}
	return "未分组"
}

func splitPath(p string) []string {
	return strings.Split(strings.Trim(p, "/"), "/")
}

func isVersionOrAPISegment(s string) bool {
	switch strings.ToLower(s) {
	case "api", "rest", "graphql", "rpc", "v", "v1", "v2", "v3", "v4", "v5", "v6":
		return true
	}
	return false
}

// buildGroups 按模块聚合端点。
func buildGroups(eps []*Endpoint) []Group {
	byModule := map[string]*Group{}
	for _, ep := range eps {
		g, ok := byModule[ep.Module]
		if !ok {
			g = &Group{Name: ep.Module, Prefix: prefixOf(ep.PathPattern)}
			byModule[ep.Module] = g
		}
		g.Endpoints = append(g.Endpoints, ep.Method+" "+ep.PathPattern)
	}
	groups := make([]Group, 0, len(byModule))
	for _, g := range byModule {
		sort.Strings(g.Endpoints)
		groups = append(groups, *g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if len(groups[i].Endpoints) != len(groups[j].Endpoints) {
			return len(groups[i].Endpoints) > len(groups[j].Endpoints)
		}
		return groups[i].Name < groups[j].Name
	})
	return groups
}

// prefixOf 取路径中版本段之前的部分作为前缀展示。
func prefixOf(pattern string) string {
	segs := strings.Split(strings.Trim(pattern, "/"), "/")
	var keep []string
	for _, s := range segs {
		if isVersionOrAPISegment(s) {
			break
		}
		keep = append(keep, s)
	}
	if len(keep) == 0 {
		return "/"
	}
	return "/" + strings.Join(keep, "/")
}

func appendUnique(list []string, v string, max int) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	if len(list) >= max {
		return list
	}
	return append(list, v)
}

func appendUniqueInt(list []int, v int) []int {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}