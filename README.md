# 端口转发 · fnPortGate

> 一款可上架飞牛（fnOS）应用商店的 **UPnP 端口转发管理** 应用。
> 通过 UPnP IGD 协议管理路由器的 NAT 端口映射——查看公网 IP、网关状态，一键增删端口转发规则。

<p align="center">
  <img src="https://img.shields.io/badge/version-2.0.0-orange" alt="version">
  <img src="https://img.shields.io/badge/platform-fnOS-blue" alt="platform">
  <img src="https://img.shields.io/badge/language-Go-00ADD8" alt="go">
  <img src="https://img.shields.io/badge/deploy-Native%20(no%20Docker)-success" alt="native">
  <img src="https://img.shields.io/badge/protocol-UPnP%20IGD-green" alt="upnp">
</p>

---

## ✨ 功能特性

- 🌐 **状态总览** —— 公网 IP、路由器网关型号、连接状态、映射数量，四张卡片一目了然
- ➕ **端口映射管理** —— 支持 TCP / UDP，一键添加、删除、重新启用
- 🔍 **自动发现网关** —— 通过 SSDP 组播自动发现支持 UPnP 的路由器，也可手动刷新
- 🎯 **生效状态标注** —— 明确区分「已生效 / 未生效」的映射规则
- 🎨 **简洁美观** —— 原生前端，风格贴近飞牛系统，零依赖、零构建
- 💾 **本地持久化** —— 配置以 JSON 文件保存在应用数据目录，不依赖外部数据库

---

## 🚀 快速开始

### 安装到飞牛 NAS

1. 从本仓库 [Releases](https://github.com/HackettMX/fnUPnPGate/releases) 下载最新的 `.fpk` 文件
2. 在飞牛「应用中心 → 设置 → 手动安装」中上传该 `.fpk`
3. 在应用中心点击启动，桌面出现「端口转发」图标，打开即可

> ⚠️ 请确认路由器已开启 **UPnP** 功能（部分路由器默认关闭）。
> **安装过程完全离线**：可执行文件已内置在安装包中，不依赖 Docker、不拉取任何镜像。

---

## 🧭 为什么是原生应用（无 Docker）

2.0.0 起改为 **Native 形态**：`pkg` 内直接携带静态编译的 Go 可执行文件，由应用中心的 `cmd/main` 拉起宿主机进程。

| 对比项 | Docker 形态（v1.x） | Native 形态（v2.x） |
| --- | --- | --- |
| 安装依赖 | 需要 Docker 服务 + 镜像仓库 | **无**，装完即用 |
| 安装时网络 | 必须能拉取镜像（国内常见 denied / 超时） | **完全不需要联网** |
| 安装包体积 | ~13 KB（镜像另算，约 60 MB） | ~3 MB（含二进制） |
| 运行时开销 | 容器 + 镜像层 | 单个进程，内存占用更低 |
| 网络 | 必须 `network_mode: host` 才能收到 SSDP 组播 | 直接跑在宿主机上，**天然可用** |
| 隔离性 | 容器隔离 | 无（以应用专用非 root 用户运行） |

> 旧的 Docker / Python 实现保留在 git 标签 `v1.0.0` 中。

---

## 📋 功能一览

| 功能 | 说明 |
| --- | --- |
| 状态总览 | 公网 IP、网关设备、连接状态、映射数量四张卡片 |
| 端口映射列表 | 描述 / 协议 / 外部端口 / 内部地址 / 状态 / 操作 |
| 添加映射 | 描述、协议、外部端口、内部端口、内部 IP、租期 |
| 删除 / 重新启用 | 对已管理条目；路由器上的外部条目也可一键清理 |
| 自动刷新 | 每 20 秒自动刷新，也可手动「刷新」重新发现网关 |

---

## 🛠 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go 1.23+ · 标准库 `net/http` · [`goupnp`](https://github.com/huin/goupnp)（IGD1 / IGD2 双协议） |
| 前端 | 原生 HTML / CSS / JS（无框架、无构建步骤），通过 `go:embed` 内嵌进二进制 |
| 存储 | JSON 文件（`$TRIM_PKGVAR/fnportgate.json`，原子写入） |
| 部署 | 原生进程，由 `cmd/main` 管理 PID、启停与状态 |

单个静态二进制约 **7 MB**，`CGO_ENABLED=0` 编译，无 glibc 版本依赖。

---

## 📁 目录结构

```
fnPortGate/
├── .github/workflows/build.yml   # CI：编译 → 打包 → 发布 Release
├── go/                           # 后端源码（含内嵌前端）
│   ├── main.go                   # HTTP 服务、静态资源、优雅退出
│   ├── config.go                 # 环境变量配置
│   ├── store.go                  # JSON 持久化（原子写入）
│   ├── upnp.go                   # goupnp 封装：发现 / 公网 IP / 映射增删查
│   ├── api.go                    # REST 路由与入参校验
│   ├── go.mod / go.sum
│   └── static/                   # 前端（HTML/CSS/JS）
├── fpk/                          # 飞牛 fnpack 打包工程（Native）
│   ├── manifest                  # 应用元数据（INI）
│   ├── ICON.PNG / ICON_256.PNG
│   ├── config/
│   │   ├── privilege             # 运行用户声明（run-as: package）
│   │   └── resource              # 无 docker-project，原生应用无需资源声明
│   ├── app/
│   │   ├── bin/fnportgate        # 构建产物（CI 生成，不入库）
│   │   └── ui/                   # 桌面入口 + 图标
│   ├── cmd/                      # 生命周期脚本（main 管理进程）
│   └── wizard/
├── scripts/
│   ├── gen_icons.py              # 图标生成
│   ├── fix_fpk_perms.py          # 修正 tar 权限位并重算 checksum
│   ├── build.sh                  # 交叉编译 + 打包
│   └── dev.sh                    # 本地开发
└── README.md
```

---

## 💻 本地开发

```bash
# 直接运行（前端已内嵌，改完 static 重新跑即可）
bash scripts/dev.sh

# 或手动指定参数
cd go
FNPORTGATE_WEB_PORT=8090 FNPORTGATE_DATA_DIR=./data go run .
```

浏览器打开 `http://localhost:8090`。若本机没有开启 UPnP 的路由器，界面会显示「未连接」及友好提示，属预期行为。

### 环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `FNPORTGATE_WEB_PORT` | 8090 | Web 服务端口 |
| `FNPORTGATE_WEB_HOST` | 0.0.0.0 | 监听地址 |
| `FNPORTGATE_DATA_DIR` | /data | 配置数据目录（安装后为 `TRIM_PKGVAR`） |
| `FNPORTGATE_DISCOVERY_TIMEOUT` | 5 | SSDP 发现超时（秒） |
| `FNPORTGATE_DISCOVERY_TTL` | 300 | 网关信息缓存（秒） |
| `FNPORTGATE_LOG_LEVEL` | INFO | 日志级别（DEBUG / INFO / WARN / ERROR） |

> 兼容旧前缀 `FN_UPNP_*`，便于从 1.x 平滑迁移。

---

## 📦 构建与发布

### 方式一：GitHub Actions（推荐）

推送 `v*` 标签即自动完成「交叉编译 → 打包 .fpk → 发布 Release」，**不涉及任何镜像仓库**：

```bash
git tag v2.0.0
git push origin v2.0.0
```

也可在 **Actions** 页面手动触发 `Build & Package`。

### 方式二：本地构建

依赖：Go 1.23+、Python 3（仅用于修正权限位）、[fnpack](https://developer.fnnas.com/docs/cli/fnpack/)（放到 `.tools/` 或加入 `PATH`）。

```bash
bash scripts/build.sh 2.0.0 amd64
# 产物：dist/fnportgate-2.0.0.fpk
```

### 为什么需要 `fix_fpk_perms.py`

`fnpack` 在 Windows 上打包时读不到 Unix 执行位，产出的 tar 里 `cmd/*` 与二进制都是 `0666`，飞牛应用中心执行生命周期脚本会失败。该脚本重建两层 tar 并显式写入 `0755/0644`，同时重算 `manifest` 的 `checksum`（等于 `app.tgz` 的 MD5）。在 Linux 上运行同样无害。

---

## 🔌 API 一览

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/health` | 健康检查（返回版本与运行时长） |
| GET | `/api/status` | 网关状态 / 公网 IP |
| POST | `/api/discover` | 强制重新发现网关 |
| GET | `/api/mappings` | 端口映射列表（含生效状态） |
| POST | `/api/mappings` | 添加端口映射 |
| DELETE | `/api/mappings/{id}` | 删除映射（按 ID） |
| POST | `/api/mappings/{id}/enable` | 重新启用映射 |
| DELETE | `/api/mappings/port/{port}/{protocol}` | 按端口删除外部映射 |

---

## ⚠️ 注意事项

1. **端口冲突**：默认占用 `8090`。如需修改，要同步改三处：`fpk/manifest` 的 `service_port`、`fpk/app/ui/config` 的 `port`、以及运行时环境变量 `FNPORTGATE_WEB_PORT`（`cmd/main` 已自动读取 `TRIM_SERVICE_PORT`）。
2. **路由器需开启 UPnP**：部分路由器默认关闭，需在管理界面开启，否则会提示「未发现支持 UPnP 的路由器」。
3. **架构**：当前包为 x86_64（`manifest` 中 `platform="x86"`）。若需 ARM 版，用 `bash scripts/build.sh 2.0.0 arm64` 编译后，将 `platform` 改为 `arm` 重新打包即可。
4. **安全**：端口映射会把内网服务暴露到公网，请只转发必要端口，并对内网服务做好访问控制。

---

## 📄 License

本项目为个人项目，暂未指定开源许可证。如需使用或二次分发，请先联系作者。
