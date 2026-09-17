package apianalyze

import (
	"strings"
	"testing"

	"cyberstrike-ai/internal/trafficparse"
)

// 这批用例守住三个曾经出过错的不变量：
//  1. 数值不能被当成"小数末尾补零"处理（200 曾变成 2）
//  2. 统一外壳要在多个接口间被识别出来
//  3. 分页的 list/total 藏在外壳 data 里面也要能识别
const fixtureHAR = `{
  "log": {
    "version": "1.2",
    "entries": [
      {
        "startedDateTime": "2026-09-17T10:00:00.000Z",
        "time": 12,
        "request": {
          "method": "POST",
          "url": "https://demo.local/api/v1/alarms/queryAdvPage",
          "headers": [{"name": "X-BFF-Mode", "value": "true"}, {"name": "Content-Type", "value": "application/json"}],
          "postData": {"mimeType": "application/json", "text": "{\"pageNo\":1,\"pageSize\":20,\"keyword\":\"\"}"}
        },
        "response": {
          "status": 200,
          "headers": [{"name": "Content-Type", "value": "application/json"}],
          "content": {
            "size": 100,
            "mimeType": "application/json",
            "text": "{\"code\":200,\"msg\":\"ok\",\"data\":{\"total\":137,\"list\":[{\"alarmId\":\"a1\",\"level\":2},{\"alarmId\":\"a2\",\"level\":3}]}}"
          }
        }
      },
      {
        "startedDateTime": "2026-09-17T10:00:01.000Z",
        "time": 8,
        "request": {
          "method": "GET",
          "url": "https://demo.local/api/v1/user/me",
          "headers": [{"name": "X-BFF-Mode", "value": "true"}]
        },
        "response": {
          "status": 200,
          "headers": [{"name": "Content-Type", "value": "application/json"}],
          "content": {
            "size": 40,
            "mimeType": "application/json",
            "text": "{\"code\":200,\"msg\":\"ok\",\"data\":{\"userName\":\"admin\",\"level\":150}}"
          }
        }
      },
      {
        "startedDateTime": "2026-09-17T10:00:02.000Z",
        "time": 3,
        "request": {
          "method": "GET",
          "url": "https://demo.local/static/app.js",
          "headers": []
        },
        "response": {
          "status": 200,
          "headers": [{"name": "Content-Type", "value": "application/javascript"}],
          "content": {"size": 900, "mimeType": "application/javascript", "text": "console.log(1)"}
        }
      }
    ]
  }
}`

func analyzeFixture(t *testing.T) *Inventory {
	t.Helper()
	records, err := trafficparse.ParseHAR(strings.NewReader(fixtureHAR), "demo")
	if err != nil {
		t.Fatalf("解析 HAR 失败: %v", err)
	}
	return Analyze("demo", records)
}

func TestFilterDropsStaticAssets(t *testing.T) {
	inv := analyzeFixture(t)
	if inv.SourceCount != 3 {
		t.Fatalf("输入记录数应为 3，实际 %d", inv.SourceCount)
	}
	if inv.APICount != 2 {
		t.Fatalf("静态资源应被过滤，接口候选应为 2，实际 %d", inv.APICount)
	}
	if len(inv.Endpoints) != 2 {
		t.Fatalf("接口数应为 2，实际 %d", len(inv.Endpoints))
	}
}

func TestEnvelopeDetected(t *testing.T) {
	inv := analyzeFixture(t)
	if inv.Envelope == nil || !inv.Envelope.Detected {
		t.Fatal("应识别出 {code,data,msg} 外壳")
	}
	if inv.Envelope.SuccessField != "code" {
		t.Errorf("成功字段应为 code，实际 %q", inv.Envelope.SuccessField)
	}
	if inv.Envelope.DataField != "data" {
		t.Errorf("数据字段应为 data，实际 %q", inv.Envelope.DataField)
	}
	// 数值 200 必须原样保留——曾经因为去掉末尾 0 变成 2
	if got := inv.Envelope.SuccessCheck; got != "code == 200" {
		t.Errorf("成功判定应为 %q，实际 %q", "code == 200", got)
	}
}

func TestPaginationFoundInsideEnvelope(t *testing.T) {
	inv := analyzeFixture(t)
	var adv *Endpoint
	for _, ep := range inv.Endpoints {
		if ep.Summary == "queryAdvPage" {
			adv = ep
		}
	}
	if adv == nil {
		t.Fatal("未找到 queryAdvPage 接口")
	}
	if adv.Pagination == nil || !adv.Pagination.Detected {
		t.Fatal("分页藏在外壳 data 里，也必须被识别")
	}
	if adv.Pagination.ListField != "list" || adv.Pagination.TotalField != "total" {
		t.Errorf("列表/总数字段识别错误: %+v", adv.Pagination)
	}
	if adv.Pagination.ObservedSize != "20" {
		t.Errorf("样本页大小应为 20（不能被截成 2），实际 %q", adv.Pagination.ObservedSize)
	}
}

func TestNestedSchemaExpanded(t *testing.T) {
	inv := analyzeFixture(t)
	for _, ep := range inv.Endpoints {
		if ep.Summary != "queryAdvPage" {
			continue
		}
		var data *Field
		for i := range ep.Response.Fields {
			if ep.Response.Fields[i].Name == "data" {
				data = &ep.Response.Fields[i]
			}
		}
		if data == nil || len(data.Fields) == 0 {
			t.Fatal("外壳 data 内部结构必须递归展开")
		}
		var list *Field
		for i := range data.Fields {
			if data.Fields[i].Name == "list" {
				list = &data.Fields[i]
			}
		}
		if list == nil || list.Items == nil || len(list.Items.Fields) == 0 {
			t.Fatal("列表元素结构必须展开")
		}
		names := map[string]bool{}
		for _, f := range list.Items.Fields {
			names[f.Name] = true
		}
		if !names["alarmId"] || !names["level"] {
			t.Errorf("列表元素应含 alarmId 与 level，实际 %v", names)
		}
	}
}

func TestFixedHeaderAndAuth(t *testing.T) {
	inv := analyzeFixture(t)
	if inv.Auth == nil {
		t.Fatal("应产出鉴权信息")
	}
	if got := inv.Auth.FixedHeaders["x-bff-mode"]; got != "true" {
		t.Errorf("业务必带头 x-bff-mode 应为 true，实际 %q", got)
	}
}

func TestPathParamGeneralized(t *testing.T) {
	recs := []*trafficparse.Record{
		{ID: "1", Method: "GET", Path: "/api/v1/alarms/4c456157226143ffa1b4576cf38176c6", Host: "d", Status: 200, RespMIME: "application/json", RespBody: "{}"},
	}
	inv := Analyze("demo", recs)
	if len(inv.Endpoints) != 1 {
		t.Fatalf("应有 1 个接口，实际 %d", len(inv.Endpoints))
	}
	if got := inv.Endpoints[0].PathPattern; got != "/api/v1/alarms/{id}" {
		t.Errorf("路径参数应泛化为 {id}，实际 %q", got)
	}
}