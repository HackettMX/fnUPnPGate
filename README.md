# 端口转发 · fnPortGate

> 一款可上架飞牛（fnOS）应用商店的 **UPnP 端口转发管理** 应用。
> 通过 UPnP IGD 协议，轻松管理路由器的 NAT 端口映射——查看公网 IP、网关状态，一键增删端口转发规则。

<p align="center">
  <img src="https://img.shields.io/badge/version-1.0.0-orange" alt="version">
  <img src="https://img.shields.io/badge/platform-fnOS-blue" alt="platform">
  <img src="https://img.shields.io/badge/python-3.12-3776AB" alt="python">
  <img src="https://img.shields.io/badge/deploy-Docker%20(host)-2496ED" alt="docker">
  <img src="https://img.shields.io/badge/protocol-UPnP%20IGD-green" alt="upnp">
</p>

---

## ✨ 功能特性

- 🌐 **状态总览** —— 公网 IP、路由器网关型号、连接状态、映射数量，四张卡片一目了然
- ➕ **端口映射管理** —— 支持 TCP / UDP，一键添加、删除、重新启用
- 🔍 **自动发现网关** —— 通过 SSDP 组播自动发现支持 UPnP 的路由器，也可手动刷新
- 🎯 **生效状态标注** —— 明确区分「已生效 / 未生效」的映射规则
- 🎨 **简洁美观** —— 原生前端，风格贴近飞牛系统，零依赖、零构建
- 💾 **本地持久化** —— 配置以 JSON 文件保存在本机，不依赖外部数据库

---

## 🚀 快速开始

### 安装到飞牛 NAS

1. 从本仓库 [Releases](https://github.com/HackettMX/fnUPnPGate/releases) 下载最新的 `.fpk` 文件（或按下方说明自行打包）
2. 在飞牛「应用中心 → 设置 → 手动安装」中上传该 `.fpk`
3. 打开应用，进入 Web 界面即可自动发现路由器并管理端口映射

> ⚠️ 安装前请确认路由器已开启 **UPnP** 功能（部分路由器默认关闭）。

---

## 📋 功能一览

| 功能 | 说明 |
| --- | --- |
| 状态总览 | 公网 IP、网关设备、连接状态、映射数量四张卡片 |
| 端口映射列表 | 描述 / 协议 / 外部端口 / 内部地址 / 状态 / 操作 |
| 添加映射 | 描述、协议、外部端口、内部端口、内部 IP、租期 |
| 删除 / 重新启用 | 对已管理条目；外部条目也可一键清理 |
| 自动刷新 | 每 20 秒自动刷新，也可手动「刷新」重新发现网关 |

---

## 🛠 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Python 3.12 · FastAPI · Uvicorn · `async-upnp-client`（IGD 支持成熟） |
| 前端 | 原生 HTML / CSS / JS（无框架、无构建步骤） |
| 存储 | JSON 文件（`/data/fn-upnp.json`，映射到 `TRIM_PKGVAR/data`） |
| 部署 | Docker 单容器，`network_mode: host` |

### 为什么必须 host 网络？

UPnP 发现依赖 SSDP 组播（`239.255.255.250:1900`），且端口映射要指向 **NAS 自己的局域网 IP**。桥接网络下组播无法到达路由器、映射会错误指向容器 IP，因此必须使用 host 网络（这也是飞牛上 ONVIF / UPnP 类多播应用的通行做法）。

---

## 📁 目录结构

```
fnPortGate/
├── .github/workflows/         # GitHub Actions 自动构建 + 打包
├── src/                       # 应用源码
│   ├── app/                   # FastAPI 后端（配置/模型/存储/UPnP/路由/入口）
│   ├── static/                # 前端（原生 HTML/CSS/JS，零构建）
│   ├── requirements.txt
│   └── Dockerfile
├── fpk/                       # 飞牛 fnpack 打包工程
│   ├── manifest               # 应用元数据（INI）
│   ├── ICON.PNG / ICON_256.PNG
│   ├── config/                # privilege + resource
│   ├── app/docker/            # docker-compose.yaml（host 网络）
│   ├── app/ui/                # 桌面入口 + 图标
│   ├── cmd/                   # 生命周期脚本
│   └── wizard/
├── scripts/
│   ├── gen_icons.py           # 图标生成
│   ├── build.sh               # 构建镜像 + 打包
│   └── dev.sh                 # 本地开发
└── README.md
```

---

## 💻 本地开发

```bash
# 1. 安装依赖
python -m venv .venv
.venv/Scripts/python -m pip install -r src/requirements.txt    # Windows
# .venv/bin/pip install -r src/requirements.txt                 # Linux/macOS

# 2. 启动（本地测试）
cd src
FN_UPNP_DATA_DIR=./data FN_UPNP_WEB_PORT=8090 \
  .venv/bin/python -m uvicorn app.main:app --host 0.0.0.0 --port 8090
```

浏览器打开 `http://localhost:8090` 即可看到界面。若本机无路由器 / UPnP，界面会显示「未连接」及友好提示，这是预期行为。

### 环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `FN_UPNP_WEB_PORT` | 8090 | Web 服务端口 |
| `FN_UPNP_DATA_DIR` | /data | 配置数据目录 |
| `FN_UPNP_DISCOVERY_TIMEOUT` | 5 | SSDP 发现等待秒数 |
| `FN_UPNP_DISCOVERY_TTL` | 300 | 网关信息缓存秒数 |
| `FN_UPNP_LOG_LEVEL` | INFO | 日志级别 |

---

## 📦 构建与发布

### 方式一：GitHub Actions 自动打包（推荐）

推送一个 `v*` 标签即可自动完成「构建镜像 → 推送 ghcr → 打包 .fpk → 发布 Release」全流程：

```bash
git tag v1.0.0
git push origin v1.0.0
```

也可在仓库的 **Actions** 页面手动触发 `Build & Package` 工作流，并指定版本号。

### 方式二：手动构建

镜像仓库地址为 `ghcr.io/hackettmx/fnupnpgate:1.0.0`（对应仓库 `HackettMX/fnUPnPGate`）。

```bash
# 1. 构建镜像
cd src
docker build -t ghcr.io/hackettmx/fnupnpgate:1.0.0 .

# 2. 登录 ghcr（需要一个带 packages 读写权限的 GitHub PAT）
echo "<你的 GitHub Token>" | docker login ghcr.io -u HackettMX --password-stdin

# 3. 推送
docker push ghcr.io/hackettmx/fnupnpgate:1.0.0
```

> ⚠️ **关键**：ghcr 镜像包默认是 **Private**。推送成功后，请到 GitHub 仓库的 `Packages` 页面 → 找到 `fnupnpgate` 包 → `Package settings` → `Danger Zone` → `Change visibility` 改为 **Public**。否则飞牛应用中心拉取时会报 `denied`。

然后打包：

```bash
# 安装 fnpack 后（见 https://developer.fnnas.com/docs/cli/fnpack/）
cd fpk
fnpack build     # 产出 fnportgate.fpk
```

---

## 🔌 API 一览

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/health` | 健康检查 |
| GET | `/api/status` | 网关状态 / 公网 IP |
| POST | `/api/discover` | 强制重新发现网关 |
| GET | `/api/mappings` | 端口映射列表（含生效状态） |
| POST | `/api/mappings` | 添加端口映射 |
| DELETE | `/api/mappings/{id}` | 删除映射（按 ID） |
| POST | `/api/mappings/{id}/enable` | 重新启用映射 |
| DELETE | `/api/mappings/port/{external_port}/{protocol}` | 按端口删除外部映射 |

---

## ⚠️ 注意事项

1. **版本必须与镜像 tag 严格一致**：`manifest` 里的 `version=1.0.0` 必须等于 compose 中镜像 tag 的 `1.0.0`，否则拉取镜像会报 `manifest unknown`。
2. **host 网络端口冲突**：默认占用 8090 端口。如与其它应用冲突，需同步修改三处：`manifest` 的 `service_port`、`app/ui/config` 的 `port`、`docker-compose.yaml` 的 `FN_UPNP_WEB_PORT`（以及 `cmd/main` 中的健康检查端口）。
3. **路由器需开启 UPnP**：部分路由器默认关闭 UPnP，需在路由器管理界面开启，否则会提示「未发现支持 UPnP 的路由器」。
4. **安全**：端口映射会把内网服务暴露到公网，请只转发必要端口，并对内网服务做好访问控制。

---

## 📄 License

本项目为个人项目，暂未指定开源许可证。如需使用或二次分发，请先联系作者。
