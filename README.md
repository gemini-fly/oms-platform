# OMS Platform

OMS Platform 是一个使用 Go 构建的企业运维平台，提供应用树、ITSM、CMDB、多云资源同步、轻量 CI/CD、Kubernetes 发布治理、LDAP 登录、组织同步、RBAC 和操作审计。

前端页面已经嵌入可执行文件。安装后只需要一个 `oms-platform` 二进制和 PostgreSQL，不需要单独安装 Node.js 或部署静态文件。

> 当前为 `v0.1` 预览版，适合试用、二次开发和验证运维流程。接入生产前请完成真实 LDAP、数据库备份、HTTPS、权限和审计验证；不要把预览版的终端能力直接作为生产堡垒机使用。

## 功能模块

- 应用树：业务中心、业务线、服务和环境
- ITSM：普通工单、审批和云资源申请
- CMDB：阿里云、腾讯云、华为云账号及通用资源模型；当前真实同步以阿里云 ECS 为主，其余资源适配器仍在完善
- CI/CD：可组合 Pipeline 模块、语言识别与 Kubernetes 发布模板
- 组织与认证：LDAP 登录、钉钉组织同步、GitLab Group 映射
- 安全治理：角色与菜单权限、服务成员可见范围、SSH/Pod 操作审计

## 快速安装

Linux 和 macOS 可以使用安装脚本下载最新 Release 并校验 SHA-256：

```bash
curl -fLO https://raw.githubusercontent.com/gemini-fly/oms-platform/main/scripts/install.sh
chmod +x install.sh
./install.sh
export PATH="$HOME/.local/bin:$PATH"
oms-platform --version
```

也可以从 [GitHub Releases](https://github.com/gemini-fly/oms-platform/releases) 手动下载 Linux、macOS 或 Windows 安装包。完整步骤见 [安装文档](docs/installation.md)。

## 启动

下载源码中的 `docker-compose.yml`，启动 PostgreSQL：

```bash
docker compose up -d postgres
```

启动平台：

```bash
export SY_PLATFORM_DB_DSN='postgres://oms_platform:oms_platform@127.0.0.1:15432/oms_platform?sslmode=disable'
oms-platform
```

浏览器访问 [http://127.0.0.1:8080](http://127.0.0.1:8080)。首次安装不会注入应用、资产、工单等演示数据。

> 安全提示：LDAP 未启用时仅适合在本机完成初始化。配置 LDAP 登录并通过 HTTPS 反向代理后，再把 `HTTP_ADDR` 改为 `0.0.0.0:8080` 对外提供服务。

## 配置

| 环境变量 | 默认值 | 用途 |
| --- | --- | --- |
| `HTTP_ADDR` | `127.0.0.1:8080` | HTTP 监听地址 |
| `SY_PLATFORM_DB_DSN` | 空 | PostgreSQL DSN，也兼容 `DATABASE_URL` |
| `SY_PLATFORM_GITLAB_API_BASE` | `https://gitlab.com/api/v4` | GitLab API 地址 |
| `SY_PLATFORM_GITLAB_TOKEN` | 空 | 私有 GitLab 项目只读 Token |
| `SY_PLATFORM_FRONTEND_DIR` | 空 | 开发时覆盖内嵌前端目录 |

LDAP、钉钉、菜单权限、组织与 GitLab 映射、云账号等配置通过平台的“设置”页面维护。敏感信息不要写入仓库。

## 从源码构建

需要 Go 1.25 或更高版本：

```bash
git clone https://github.com/gemini-fly/oms-platform.git
cd oms-platform
go test ./...
make build
./bin/oms-platform --version
```

本地前端源项目仍可用于页面开发。更新页面后执行 `make sync-frontend`，将构建结果同步到 Go 的内嵌资源，再提交 `internal/web/static/index.html`。

## 发布

发布脚本会生成：

- Linux amd64 / arm64
- macOS amd64 / arm64
- Windows amd64
- `checksums.txt`

维护者发布前运行：

```bash
make release-snapshot VERSION=v0.1.0
```

产物位于 `release/`，发布前必须重新执行测试，并将这些文件与 `checksums.txt` 一起上传到 GitHub Release。

## 许可证

[MIT License](LICENSE)
