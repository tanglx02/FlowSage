package apianalyze

import (
	"encoding/json"
	"sort"
	"strings"
)

// Field 描述一个请求/响应字段。
//
// 对象与数组会递归展开：生成的客户端需要知道 data.list[].alarmId 这一级的结构，
// 只到顶层是不够的。
type Field struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Enum     []string `json:"enum,omitempty"`  // 取值集合很小且出现重复时的候选值
	Fixed    bool     `json:"fixed,omitempty"` // 所有样本取值恒定（疑似固定值）
	Sample   string   `json:"sample,omitempty"`
	Fields   []Field  `json:"fields,omitempty"` // Type == object 时的子字段
	Items    *Schema  `json:"items,omitempty"`  // Type == array 时的元素结构
}

// Schema 描述一个 JSON 结构。
type Schema struct {
	Type   string  `json:"type"`
	Fields []Field `json:"fields,omitempty"` // Type == object
	Items  *Schema `json:"items,omitempty"`  // Type == array
	MIME   string  `json:"mime,omitempty"`
}

const (
	maxDistinctValues = 10  // 超过此数量不再当作枚举
	maxValueSamples   = 20  // 每个字段最多记录多少个不同取值
	maxSampleLen      = 120 // 样本值截断长度
)

// schemaBuilder 跨多条流量累积同一个结构。
//
// 单个样本无法判断"是否必填"与"是否固定值"，必须累积多个样本后统计。
type schemaBuilder struct {
	objSamples   int
	arraySamples int
	scalarType   string
	objFields    map[string]*fieldAccum
	item         *schemaBuilder
	rawSamples   int
}

type fieldAccum struct {
	types   map[string]int
	values  []string
	present int
	nested  *schemaBuilder // 该字段为对象或数组时，累积其内部结构
}

func newSchemaBuilder() *schemaBuilder {
	return &schemaBuilder{objFields: map[string]*fieldAccum{}}
}

// add 喂入一个样本（响应体或请求体的原始文本）。
func (b *schemaBuilder) add(body string) {
	s := strings.TrimSpace(body)
	if s == "" {
		return
	}
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		b.rawSamples++
		return
	}
	b.addValue(v)
}

func (b *schemaBuilder) addValue(v interface{}) {
	switch t := v.(type) {
	case map[string]interface{}:
		b.objSamples++
		for k, val := range t {
			fa, ok := b.objFields[k]
			if !ok {
				fa = &fieldAccum{types: map[string]int{}}
				b.objFields[k] = fa
			}
			fa.present++
			fa.types[jsonTypeName(val)]++
			if s, ok := scalarString(val); ok {
				fa.values = addDistinct(fa.values, s)
				continue
			}
			// 对象或数组：递归累积内部结构
			if fa.nested == nil {
				fa.nested = newSchemaBuilder()
			}
			fa.nested.addValue(val)
		}
	case []interface{}:
		b.arraySamples++
		if b.item == nil {
			b.item = newSchemaBuilder()
		}
		for _, el := range t {
			b.item.addValue(el)
		}
	default:
		if b.scalarType == "" {
			b.scalarType = jsonTypeName(v)
		}
	}
}

// build 产出结构描述。
func (b *schemaBuilder) build() *Schema {
	if b == nil {
		return nil
	}
	switch {
	case b.objSamples > 0:
		return &Schema{Type: "object", Fields: b.buildFields()}
	case b.arraySamples > 0:
		var items *Schema
		if b.item != nil {
			items = b.item.build()
		}
		return &Schema{Type: "array", Items: items}
	case b.scalarType != "":
		return &Schema{Type: b.scalarType}
	default:
		return nil
	}
}

func (b *schemaBuilder) buildFields() []Field {
	fields := make([]Field, 0, len(b.objFields))
	for name, fa := range b.objFields {
		f := Field{
			Name:     name,
			Type:     dominantType(fa.types),
			Required: fa.present == b.objSamples && b.objSamples > 0,
		}
		if len(fa.values) > 0 {
			f.Sample = truncate(fa.values[0], maxSampleLen)
			if len(fa.values) == 1 {
				// 所有出现过的取值都一样，疑似固定值（如 pushOmsFlag: "0"）
				f.Fixed = true
			} else if len(fa.values) <= maxDistinctValues {
				f.Enum = append([]string(nil), fa.values...)
			}
		}
		if fa.nested != nil {
			if s := fa.nested.build(); s != nil {
				switch f.Type {
				case "object":
					f.Fields = s.Fields
				case "array":
					f.Items = s.Items
				}
			}
		}
		fields = append(fields, f)
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	return fields
}

// FieldsFromValues 由字符串取值集合构造字段列表，用于 Query 参数这类非 JSON 来源。
func FieldsFromValues(values map[string][]string, total int) []Field {
	names := make([]string, 0, len(values))
	for k := range values {
		names = append(names, k)
	}
	sort.Strings(names)

	out := make([]Field, 0, len(names))
	for _, name := range names {
		vals := dedupe(values[name])
		f := Field{Name: name, Type: "string", Required: len(values[name]) == total && total > 0}
		if len(vals) > 0 {
			f.Sample = truncate(vals[0], maxSampleLen)
			if len(vals) == 1 {
				f.Fixed = true
			} else if len(vals) <= maxDistinctValues {
				f.Enum = vals
			}
		}
		out = append(out, f)
	}
	return out
}

func jsonTypeName(v interface{}) string {
	switch v.(type) {
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func scalarString(v interface{}) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		return trimFloat(t), true
	case bool:
		if t {
			return "true", true
		}
		return "false", true
	case nil:
		return "null", true
	default:
		return "", false
	}
}

// trimFloat 去掉小数末尾多余的 0；整数不做任何裁剪。
//
// 注意：不能对整数用 TrimRight(s, "0")——那会把 200 变成 2、150 变成 15。
func trimFloat(f float64) string {
	s := formatFloat(f)
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func formatFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// dominantType 取出现次数最多的类型；数量相同时按名称排序保证结果稳定。
func dominantType(types map[string]int) string {
	best, bestN := "", -1
	for t, n := range types {
		if n > bestN || (n == bestN && t < best) {
			best, bestN = t, n
		}
	}
	if best == "" {
		return "unknown"
	}
	return best
}

func addDistinct(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	if len(list) >= maxValueSamples {
		return list
	}
	return append(list, v)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		if len(out) >= maxValueSamples {
			break
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}