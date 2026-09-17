# 环境搭建与迁移

本文档记录 FlowSage 从零搭建的完整步骤，以及**迁移到新机器时应做什么**。
照本文操作后应能直接 `run-windows.cmd` 启动服务。

## 1. 依赖清单

| 依赖 | 版本要求 | 本机实测版本 | 获取方式 |
|---|---|---|---|
| Go | **1.25+**（`go.mod` 为准） | 1.27.0 | https://go.dev/dl/ |
| C 编译器 | 支持 CGO 即可 | MSYS2 gcc 16.2.0 | MSYS2 / MinGW-w64 |
| Python | **3.10+** | 3.12.3 | https://www.python.org/downloads/ |
| Git | 任意较新版本 | 2.47.1 | https://git-scm.com/ |
| Node | 18+（仅跑前端单测） | 24.19.0 | https://nodejs.org/ |

> **CGO 是硬性要求**：项目用 `mattn/go-sqlite3`，没有 C 编译器会直接编译失败。
> 安装 MSYS2 后需把 `msys64\ucrt64\bin`（或 `mingw64\bin`）加入 PATH。

## 2. 一键重建（Windows，推荐）

```cmd
scripts\setup-windows.cmd
```

脚本会依次完成：

1. 检查 Go / Python / gcc 是否存在及版本是否达标
2. 创建 `venv\`（若不存在）
3. 用清华镜像安装 `requirements.txt`
4. 在 `venv\Scripts\` 内生成 `python3.exe`（见下方"坑 1"）
5. 若 `config.yaml` 不存在则从 `config.example.yaml` 复制一份

完成后编辑 `config.yaml` 填入 AI 通道的 `api_key`，然后 `run-windows.cmd` 启动。

## 3. 手动步骤（脚本失败时照此排查）

```powershell
# 1) Python 环境
python -m venv venv
.\venv\Scripts\python.exe -m pip install --index-url https://pypi.tuna.tsinghua.edu.cn/simple -r requirements.txt

# 2) 生成 python3 入口（关键，见坑 1）
Copy-Item .\venv\Scripts\python.exe .\venv\Scripts\python3.exe

# 3) 配置
Copy-Item config.example.yaml config.yaml
# 然后编辑 config.yaml：ai.default_channel / ai.channels.<id> 填 base_url / api_key / model

# 4) 构建
go build -o cyberstrike-ai.exe ./cmd/server

# 5) 启动
run-windows.cmd --http
```

首次启动控制台会打印 **admin 初始密码，只显示一次**，务必立即记录。
忘记密码可执行 `run-windows.cmd --reset-admin-password` 重置。

## 4. 已知环境坑

### 坑 1：Windows 的 `python3` 指向坏存根

Windows 上 `C:\Users\<用户>\AppData\Local\Microsoft\WindowsApps\python3.exe` 是微软商店的
应用执行别名（App Execution Alias）。未安装商店版 Python 时它不工作，
导致 `tools/*.yaml` 里 16 个 `command: python3` 的配方全部失败。

**解决**：在 `venv\Scripts\` 内放一份 `python.exe` 的副本并命名 `python3.exe`。
由于 venv 的解释器按所在目录解析 `pyvenv.cfg`，副本仍然是 venv 环境，能用到全部依赖。

`run-windows.cmd` 会把 `venv\Scripts` 前置到 PATH，因此服务子进程执行工具时能解析到这个 `python3`。

### 坑 2：Go 1.27 触发 sonic 降级告警

启动时会打印：

```
WARNING: sonic/ast only supports (go1.17~1.26 ...), but your environment is not suitable
and will fallback to encoding/json
```

`bytedance/sonic` 尚未支持 Go 1.27，自动降级到标准库 `encoding/json`。**功能不受影响**，
仅 JSON 序列化性能略降。介意的话装 Go 1.25 / 1.26。

### 坑 3：`web/static/vendor/` 被忽略规则误伤

上游 `.gitignore` 有 `vendor/`（Go 约定），会连带匹配前端目录 `web/static/vendor/`。
但页面运行时需要其中的 xterm / marked / cytoscape 等库，所以**本仓库用 `git add -f` 强制纳入**。

> 新拉取的仓库若发现前端样式错乱，先确认 `web/static/vendor/` 文件是否齐全。

### 坑 4：换行符

本仓库设置了 `core.autocrlf=input`，并以 LF 提交。这是为了迁到 Linux 后 `*.sh` 不被 CRLF 破坏。
新机器克隆后若 git 提示大量修改，检查 `git config core.autocrlf`。

### 坑 5：`config.yaml` 不进版本库

`config.yaml` 含 API Key，已被 `.gitignore` 排除。迁移机器时**需要手动复制或重新填写**，
不能靠 `git clone` 带过去。

## 5. Linux / macOS

上游的 `run.sh` 在这两个平台上可用（自动建 venv、装依赖、编译、启动）：

```bash
chmod +x run.sh && ./run.sh          # HTTPS（自签证书）
./run.sh --http                      # 纯 HTTP
```

Linux 上还能直接用系统包管理器装齐 `tools/*.yaml` 里的原生工具，工具链会比 Windows 完整得多。

## 6. 迁移到新机器的检查清单

- [ ] 安装 Go / Python / gcc（CGO）/ Git
- [ ] `git clone <FlowSage 仓库>`（`upstream` remote 会自动带上）
- [ ] 运行 `scripts\setup-windows.cmd`（或按第 3 节手动执行）
- [ ] 从旧机器复制 `config.yaml`（含 API Key），或用 `config.example.yaml` 重填
- [ ] `go build -o cyberstrike-ai.exe ./cmd/server` 验证编译
- [ ] `run-windows.cmd --http` 启动，浏览器访问 http://127.0.0.1:8080/
- [ ] 登录（admin / 你设置的密码）
- [ ] 阅读 `AGENTS.md` 与 `docs/dev/04-progress.md` 接上进度