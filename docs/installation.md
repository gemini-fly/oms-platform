# 安装 OMS Platform

## 1. 系统要求

- Linux、macOS 或 Windows amd64/arm64
- PostgreSQL 14 或更高版本，推荐 PostgreSQL 16
- Docker 与 Docker Compose，仅在使用项目自带数据库配置时需要
- 生产环境需要 HTTPS 反向代理

运行 Release 二进制不需要 Go、Node.js 或 npm。

## 2. 下载并校验

### 自动安装

安装脚本支持 Linux 和 macOS，默认安装到 `$HOME/.local/bin`：

```bash
curl -fLO https://raw.githubusercontent.com/gemini-fly/oms-platform/main/scripts/install.sh
chmod +x install.sh
./install.sh v0.1.0
export PATH="$HOME/.local/bin:$PATH"
```

指定系统安装目录：

```bash
sudo PREFIX=/usr/local ./install.sh v0.1.0
```

省略版本号时会安装 GitHub 上的最新 Release。

### 手动安装

从 [Releases](https://github.com/gemini-fly/oms-platform/releases) 下载与系统匹配的压缩包和 `checksums.txt`。例如 Linux amd64：

```bash
curl -fLO https://github.com/gemini-fly/oms-platform/releases/download/v0.1.0/oms-platform_0.1.0_linux_amd64.tar.gz
curl -fLO https://github.com/gemini-fly/oms-platform/releases/download/v0.1.0/checksums.txt
grep 'oms-platform_0.1.0_linux_amd64.tar.gz$' checksums.txt | sha256sum -c -
tar -xzf oms-platform_0.1.0_linux_amd64.tar.gz
sudo install -m 0755 oms-platform_0.1.0_linux_amd64/oms-platform /usr/local/bin/oms-platform
oms-platform --version
```

macOS 将校验命令中的 `sha256sum -c -` 替换为 `shasum -a 256 -c -`。

## 3. 启动 PostgreSQL

进入解压目录，使用自带 Compose 文件：

```bash
docker compose up -d postgres
docker compose ps
```

默认仅用于本地体验：

```text
数据库：oms_platform
用户：oms_platform
密码：oms_platform
端口：127.0.0.1:15432
```

生产环境必须通过 `.env` 修改 PostgreSQL 密码，并同步修改应用 DSN：

```bash
POSTGRES_DB=oms_platform
POSTGRES_USER=oms_platform
POSTGRES_PASSWORD=请替换为随机强密码
```

## 4. 启动 OMS Platform

```bash
export SY_PLATFORM_DB_DSN='postgres://oms_platform:oms_platform@127.0.0.1:15432/oms_platform?sslmode=disable'
export HTTP_ADDR='127.0.0.1:8080'
oms-platform
```

打开 [http://127.0.0.1:8080](http://127.0.0.1:8080)。程序会自动创建当前版本所需的数据库结构。

首次启动只有本地初始化管理员和内置角色、Pipeline 模块、云资源类型目录，不会创建演示应用、资产、工单或云账号。

## 5. 首次安全配置

1. 保持服务只监听 `127.0.0.1`。
2. 在“设置 > 登录认证”配置 LDAP 地址、Base DN、Bind DN、用户过滤器和平台管理员 LDAP 账号。
3. 保存并测试 LDAP，启用后退出并使用真实 LDAP 用户重新登录。
4. 检查平台管理员与普通用户的菜单、服务可见范围和审计记录。
5. 配置 Nginx、Caddy 等 HTTPS 反向代理。
6. 完成以上步骤后，按需将 `HTTP_ADDR` 改为 `0.0.0.0:8080`。

不要直接把未启用 LDAP 的初始化服务暴露到公网。

## 6. systemd 部署

```bash
sudo useradd --system --home /var/lib/oms-platform --shell /usr/sbin/nologin oms-platform
sudo install -d -o oms-platform -g oms-platform /var/lib/oms-platform /etc/oms-platform
sudo install -m 0755 oms-platform /usr/local/bin/oms-platform
sudo install -m 0644 deploy/oms-platform.service /etc/systemd/system/oms-platform.service
```

创建 `/etc/oms-platform/oms-platform.env`，权限设为 `0600`：

```bash
HTTP_ADDR=127.0.0.1:8080
SY_PLATFORM_DB_DSN=postgres://oms_platform:请替换数据库密码@127.0.0.1:5432/oms_platform?sslmode=require
SY_PLATFORM_GITLAB_API_BASE=https://gitlab.example.com/api/v4
SY_PLATFORM_GITLAB_TOKEN=
```

启动服务：

```bash
sudo chmod 0600 /etc/oms-platform/oms-platform.env
sudo systemctl daemon-reload
sudo systemctl enable --now oms-platform
sudo systemctl status oms-platform
```

## 7. 升级与回滚

升级前备份 PostgreSQL，然后替换二进制：

```bash
pg_dump "$SY_PLATFORM_DB_DSN" > oms-platform-backup.sql
sudo install -m 0755 ./oms-platform /usr/local/bin/oms-platform
sudo systemctl restart oms-platform
```

验证版本、页面和登录后再删除旧二进制。需要回滚时恢复上一版二进制；如果新版本包含不兼容的数据迁移，同时恢复升级前数据库备份。
