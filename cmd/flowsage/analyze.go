package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"cyberstrike-ai/internal/apianalyze"
	"cyberstrike-ai/internal/trafficparse"
)

// runAnalyze 是内部验证命令：把 HAR 跑完整分析链路并打印接口清单。
//
// 产品交互面在 Web 控制台，本命令用于开发排障与快速验证分析逻辑。
func runAnalyze(args []string) error {
	harPath, rest := splitPositional(args)

	fs := newFlagSet("analyze")
	project := fs.String("project", "", "项目标识（默认取 HAR 文件名）")
	asJSON := fs.Bool("json", false, "输出完整 JSON 清单")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if harPath == "" {
		return fmt.Errorf("缺少 HAR 文件路径，用法: flowsage analyze <file.har> [--json]")
	}

	pid := *project
	if pid == "" {
		pid = strings.TrimSuffix(filepath.Base(harPath), filepath.Ext(harPath))
	}

	records, err := trafficparse.ParseHARFile(harPath, pid)
	if err != nil {
		return err
	}
	inv := apianalyze.Analyze(pid, records)

	if *asJSON {
		data, err := json.MarshalIndent(inv, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	printInventory(inv)
	return nil
}

func printInventory(inv *apianalyze.Inventory) {
	fmt.Printf("项目      : %s\n", inv.ProjectID)
	fmt.Printf("流量记录  : %d 条 → 接口候选 %d 条（过滤掉 %d 条静态资源）\n",
		inv.SourceCount, inv.APICount, inv.SourceCount-inv.APICount)
	fmt.Printf("主机      : %s\n", strings.Join(inv.Hosts, ", "))
	fmt.Printf("接口总数  : %d 个\n", len(inv.Endpoints))
	fmt.Println()

	if inv.Envelope != nil && inv.Envelope.Detected {
		fmt.Println("统一响应外壳")
		fmt.Printf("  成功判定  : %s\n", inv.Envelope.SuccessCheck)
		fmt.Printf("  业务数据  : %s\n", orDash(inv.Envelope.DataField))
		fmt.Printf("  消息字段  : %s\n", orDash(inv.Envelope.MessageField))
		fmt.Printf("  覆盖率    : %s\n", inv.Envelope.Coverage)
		fmt.Println()
	}

	if inv.Auth != nil {
		fmt.Println("鉴权与会话")
		fmt.Printf("  鉴权方式  : %s\n", strings.Join(inv.Auth.Modes, ", "))
		if inv.Auth.TokenPath != "" {
			fmt.Printf("  取 token  : %s\n", inv.Auth.TokenPath)
		}
		if len(inv.Auth.LoginPaths) > 0 {
			fmt.Printf("  登录端点  : %s\n", strings.Join(inv.Auth.LoginPaths, ", "))
		}
		if len(inv.Auth.FixedHeaders) > 0 {
			fmt.Println("  必带固定头（缺失会导致请求异常）:")
			for k, v := range inv.Auth.FixedHeaders {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
		fmt.Println()
	}

	if len(inv.Groups) > 0 {
		fmt.Println("功能分组")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, g := range inv.Groups {
			fmt.Fprintf(w, "  %s\t%d 个接口\t%s\n", g.Name, len(g.Endpoints), g.Prefix)
		}
		_ = w.Flush()
		fmt.Println()
	}

	fmt.Println("接口清单")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  方法\t路径\t业务名\t调用\t分页\t模块")
	for _, ep := range inv.Endpoints {
		pg := "-"
		if ep.Pagination != nil && ep.Pagination.Detected {
			pg = ep.Pagination.Mode
			if ep.Pagination.ObservedSize != "" {
				pg += "(" + ep.Pagination.ObservedSize + ")"
			}
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%d\t%s\t%s\n",
			ep.Method, ep.PathPattern, ep.Summary, ep.CallCount, pg, ep.Module)
	}
	_ = w.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}