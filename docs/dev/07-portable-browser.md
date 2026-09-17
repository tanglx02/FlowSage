# 便携浏览器下载

FlowSage 需要一个能被 CDP 驱动的 Chromium 内核浏览器。运行时按
`系统 Chrome → 系统 Edge → 项目内便携版` 顺序探测，**只有在前两者都没有时才需要手动下载**。

本文的下载地址均已实测可访问（2026-09-18 验证）。

## 1. 放置位置

下载解压后放到项目根目录的 `browser/` 下（该目录已被 `.gitignore` 排除，不进版本库）：

```
FlowSage/
└── browser/
    └── chrome-win/            ← 解压后含 chrome.exe
        └── chrome.exe
```

Windows 版压缩包解开后是 `chrome-win/` 目录，可整体移动；Linux/macOS 同理。

## 2. 三种获取途径

| 途径 | 适合 | 版本特性 | 体积 |
|---|---|---|---|
| **A. 官方 Chromium 快照** | 要真正的 Chromium，仓库最全 | 滚动快照（每次提交都出包） | ~340 MB |
| **B. Chrome for Testing** | 要稳定版本、要能长期复现 | Google 官方标记的稳定版 | ~150 MB |
| **C. npmmirror 国内镜像** | 访问 Google 存储受限时 | 与 A 同源，但有滞后 | ~340 MB |

三种产物都能被 chromedp 驱动，功能等价。**推荐 B**（版本稳定、体积小、有 JSON 索引可脚本化）。

## 3. A. 官方 Chromium 快照

基址 `https://storage.googleapis.com/chromium-browser-snapshots/`

取当前版本号（对每个平台各有一个 `LAST_CHANGE` 文件）：

| 平台 | 版本号地址 | 下载地址（把 `{rev}` 换成版本号） | 包内目录 |
|---|---|---|---|
| Windows x64 | `{基址}Win_x64/LAST_CHANGE` | `{基址}Win_x64/{rev}/chrome-win.zip` | `chrome-win/` |
| Windows x86 | `{基址}Win/LAST_CHANGE` | `{基址}Win/{rev}/chrome-win.zip` | `chrome-win/` |
| Linux x64 | `{基址}Linux_x64/LAST_CHANGE` | `{基址}Linux_x64/{rev}/chrome-linux.zip` | `chrome-linux/` |
| macOS Intel | `{基址}Mac/LAST_CHANGE` | `{基址}Mac/{rev}/chrome-mac.zip` | `chrome-mac/` |
| macOS Apple Silicon | `{基址}Mac_Arm/LAST_CHANGE` | `{基址}Mac_Arm/{rev}/chrome-mac.zip` | `chrome-mac/` |

> 实测（2026-09-18）：`Win_x64/LAST_CHANGE` = `1699683`，
> `Win_x64/1699683/chrome-win.zip` 可下载，340.1 MB。

PowerShell 一步取版本号 + 下载（Windows x64）：

```powershell
$base = 'https://storage.googleapis.com/chromium-browser-snapshots'
$rev  = (Invoke-WebRequest "$base/Win_x64/LAST_CHANGE" -UseBasicParsing).Content.Trim()
Invoke-WebRequest "$base/Win_x64/$rev/chrome-win.zip" -OutFile chrome-win.zip
```

## 4. B. Chrome for Testing（推荐）

基址 `https://storage.googleapis.com/chrome-for-testing-public/{版本}/{平台}/chrome-{平台}.zip`

JSON 索引（可用于脚本自动取最新稳定版）：
`https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions-with-downloads.json`

实测 Stable 版本 `153.0.8010.47`，各平台地址：

| 平台 | 下载地址 |
|---|---|
| Windows x64 | `https://storage.googleapis.com/chrome-for-testing-public/153.0.8010.47/win64/chrome-win64.zip` |
| Windows x86 | `https://storage.googleapis.com/chrome-for-testing-public/153.0.8010.47/win32/chrome-win32.zip` |
| Linux x64 | `https://storage.googleapis.com/chrome-for-testing-public/153.0.8010.47/linux64/chrome-linux64.zip` |
| Linux arm64 | `https://storage.googleapis.com/chrome-for-testing-public/153.0.8010.47/linux-arm64/chrome-linux-arm64.zip` |
| macOS Intel | `https://storage.googleapis.com/chrome-for-testing-public/153.0.8010.47/mac-x64/chrome-mac-x64.zip` |
| macOS Apple Silicon | `https://storage.googleapis.com/chrome-for-testing-public/153.0.8010.47/mac-arm64/chrome-mac-arm64.zip` |

脚本取最新稳定版（跨平台，需 PowerShell 7 / `curl`）：

```powershell
$idx = Invoke-RestMethod 'https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions-with-downloads.json'
$url = ($idx.channels.Stable.downloads.chrome | Where-Object platform -eq 'win64').url
Invoke-WebRequest $url -OutFile chrome-win64.zip
```

## 5. C. npmmirror 国内镜像

基址 `https://registry.npmmirror.com/-/binary/chromium-browser-snapshots/`

目录结构与 A 完全一致，**但 `LAST_CHANGE` 文件未镜像**，需要先列目录取最新版本号：

```powershell
$base = 'https://registry.npmmirror.com/-/binary/chromium-browser-snapshots'
$rev  = (Invoke-WebRequest "$base/Win_x64/" -UseBasicParsing).Content |
        ConvertFrom-Json | Select-Object -Last 1 | ForEach-Object { $_.name.TrimEnd('/') }
Invoke-WebRequest "$base/Win_x64/$rev/chrome-win.zip" -OutFile chrome-win.zip
```

> 实测（2026-09-18）：镜像最新版本号为 `1698291`（官方为 `1699683`，**有滞后**），
> 该版本 `chrome-win.zip` 可下载，339.7 MB。

## 6. 下载后自检

```powershell
# 1) 确认能启动并打印版本
.\browser\chrome-win\chrome.exe --version

# 2) 确认 CDP 端口可用（启动后访问 http://127.0.0.1:9222/json/version 应返回 JSON）
.\browser\chrome-win\chrome.exe --headless --remote-debugging-port=9222 --user-data-dir=.\tmp\profile-test
```

第 2 步能返回 `webSocketDebuggerUrl` 即说明该浏览器可被 FlowSage 驱动。
若返回空或连接被拒，通常是企业策略禁用了远程调试，需换其他浏览器来源。

## 7. 常见问题

| 现象 | 原因 | 处理 |
|---|---|---|
| 解压后找不到 `chrome.exe` | 压缩包内还有一层目录 | 把 `chrome-win/` 整体放到 `browser/` 下即可，无需再拆 |
| 提示缺少 DLL | 下载不完整 | 重新下载，注意核对体积（Windows x64 约 340 MB） |
| 能启动但 CDP 连不上 | 企业策略禁用远程调试 | 改用系统 Edge，或联系管理员放行 `--remote-debugging-port` |
| Linux 下启动报缺库 | 缺少系统依赖 | `apt install -y libnss3 libatk-bridge2.0-0 libgtk-3-0 libasound2` |
| macOS 提示"已损坏" | 未签名/隔离属性 | `xattr -dr com.apple.quarantine browser/chrome-mac/` |