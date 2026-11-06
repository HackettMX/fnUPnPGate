# 飞牛应用中心上架申请 · 填表稿

> 表单地址：<https://trim-nas.feishu.cn/share/base/form/shrcn5m4MsyeqvEh6c4tm97pQKb>
> 来源：飞牛帮助中心「联系我们 → 应用生态」公开入口。
> 表单只有 6 个字段（5 个必填），**没有上传安装包的入口**，审核会通过下面填的链接取包，请确保链接公网可访问。

## 一、逐字段填写

| 字段 | 必填 | 填写内容 |
| --- | --- | --- |
| 怎么称呼您？ | 是 | `（填你的姓名或常用昵称，建议与 GitHub 账号一致：Hackett）` |
| 手机号码 | 是 | `（你本人手机号，用于审核联系，不公开）` |
| 应用叫什么名字？ | 是 | `端口转发（fnPortGate）` |
| 是否该应用开发者本人？ | 是 | `是` |
| 简单描述您的应用 | 是 | 见下方「应用描述」 |
| 应用官网/Github/文档 链接 | 否 | `https://github.com/HackettMX/fnUPnPGate/releases/latest` |

链接建议直接给 Releases 最新页，审核点开就能下 `.fpk`，少一轮来回。如果想让人先看 README，可换成仓库首页 `https://github.com/HackettMX/fnUPnPGate`，但要在描述里补一句「安装包见 Releases」。

## 二、应用描述（直接复制）

```
端口转发（fnPortGate）是给飞牛 fnOS 做的原生 UPnP 端口转发管理工具。安装包约 6 MB，同一个包同时支持 x86 与 ARM 机型，不依赖 Docker，也不需要额外容器或运行时。

打开即用：自动发现局域网内支持 UPnP IGD 的路由器，直接展示公网 IP、网关型号与连接状态；填好外部端口和内部端口（内部 IP 自动取本机）就能一键添加 TCP/UDP 映射，之后可随时删除或重新启用。

映射列表按来源分成两块：「本应用添加」只显示由本应用创建的规则，「其他设备添加」列出路由器上由其他设备或程序创建的规则，方便排查端口占用和冲突，避免与已有服务打架。

全部数据只保存在本机（一个 JSON 文件），不上传任何外部服务器；已适配飞牛统一网关同源访问，在 HTTPS 反向代理下也不会白屏。
```

约 250 字。若表单有字数限制，可删掉最后一段之外的分句，保留前两段即核心。

## 三、审核可能追问 · 预备答案

| 追问 | 回答 |
| --- | --- |
| 应用需要访问外网吗？ | 不需要。只做局域网通信：SSDP 组播（UDP 1900）发现网关 + 向网关 IP 发 HTTP 执行 UPnP 动作（AddPortMapping / DeletePortMapping / GetGenericPortMappingEntry）。没有任何外部服务器，不上报任何数据。 |
| 为什么 manifest 里 `checkport=false`？ | 桌面入口走飞牛统一网关（Unix Socket），不依赖固定端口；`cmd/main start` 会自行检测 8090 是否被占用，被占用时向用户输出明确的中文错误而不是"启动成功"。 |
| 需要 root 权限吗？ | 不需要。`config/privilege` 声明 `run-as: package`，以应用专属用户运行。 |
| 数据存在哪里？卸载会残留吗？ | `/vol{n}/@appdata/fnportgate` 下的 `mappings.json`，记录本应用添加过的映射，用于重启后对账。卸载时随应用数据目录一并清理。 |
| 支持哪些架构？ | 2.1.3 起同一个 `.fpk` 内置 `bin/fnportgate-amd64` 与 `bin/fnportgate-arm64` 两份静态二进制，`manifest` 声明 `platform="all"`，`cmd/main` 启动时按 `uname -m` 选用对应文件；x86 与 ARM 机型装同一个包，无需区分。 |
| 路由器不开 UPnP 能用吗？ | 不能，界面会明确提示"未发现支持 UPnP IGD 的路由器"，并给出重新发现按钮。这是产品的边界，描述里没有夸大成"通用端口映射"。 |

## 四、提交后

1. 审核通常会给一份修改意见（字段、图标、启动逻辑、端口行为等），改完重新提交同一表单即可，不需要重新排队描述历史。
2. 通过后应用进入 fnOS 应用中心，用户可搜索「端口转发」一键安装。
3. 后续发版只需在 GitHub 打新 tag，Actions 自动产出 Release；商店版本更新走开发者平台/同渠道提交新版本，`manifest` 的 `version` 与 `changelog` 必须同步更新（本项目已在 `fpk/manifest` 维护）。

## 五、随附材料

- 安装包：`dist/fnportgate-2.1.3.fpk`（6.1 MB，含 amd64 + arm64）
- 图标：`fpk/ICON.PNG`（64×64）、`fpk/ICON_256.PNG`（256×256）
- 商店截图（1280 宽）：`docs/screenshots/01-overview.png`、`02-add.png`、`03-external.png`
- 源码：<https://github.com/HackettMX/fnUPnPGate>
