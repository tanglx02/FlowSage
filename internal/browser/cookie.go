package browser

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Cookie 是从浏览器提取出的单条 Cookie，只保留业务需要的字段。
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires,omitempty"` // Unix 时间戳；0 表示会话 Cookie
	HTTPOnly bool    `json:"http_only,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
}

// CookieHeader 拼成可直接放入 HTTP 请求头的单行 Cookie 字符串，
// 形如 name1=value1; name2=value2 —— 与 alertctl 的 config.toml::cookie 用法一致。
func CookieHeader(cookies []Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// EarliestExpiry 返回所有非会话 Cookie 中最早的过期时间。
// 全部为会话 Cookie 时返回零值。
func EarliestExpiry(cookies []Cookie) time.Time {
	var earliest time.Time
	for _, c := range cookies {
		if c.Expires <= 0 {
			continue
		}
		t := time.Unix(int64(c.Expires), 0)
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
	}
	return earliest
}

// ExtractCookies 连接到已运行的实例，取出目标 URL 可见的 Cookie。
//
// targetURL 为空时返回该 profile 下的全部 Cookie；否则只返回对指定 URL 生效的那些。
//
// 实现说明：chromedp 会在实例中临时开一个标签页用于执行 CDP 命令，
// 命令结束即随上下文取消而关闭，不会干扰用户正在操作的窗口。
func ExtractCookies(ctx context.Context, inst *Instance, targetURL string) ([]Cookie, error) {
	wsURL, err := inst.DebuggerURL()
	if err != nil {
		return nil, fmt.Errorf("实例不可用（可能已关闭）: %w", err)
	}

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, wsURL)
	defer cancelAlloc()

	tabCtx, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()

	runCtx, cancelRun := context.WithTimeout(tabCtx, 30*time.Second)
	defer cancelRun()

	var got []Cookie
	err = chromedp.Run(runCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		req := network.GetCookies()
		if targetURL != "" {
			req = req.WithURLs([]string{targetURL})
		}
		cs, err := req.Do(ctx)
		if err != nil {
			return err
		}
		got = make([]Cookie, 0, len(cs))
		for _, c := range cs {
			got = append(got, Cookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Expires:  float64(c.Expires),
				HTTPOnly: c.HTTPOnly,
				Secure:   c.Secure,
			})
		}
		return nil
	}))
	if err != nil {
		return nil, fmt.Errorf("通过 CDP 读取 Cookie 失败: %w", err)
	}

	// 稳定输出顺序，便于人工核对与 diff。
	sort.Slice(got, func(i, j int) bool {
		if got[i].Domain != got[j].Domain {
			return got[i].Domain < got[j].Domain
		}
		return got[i].Name < got[j].Name
	})
	return got, nil
}