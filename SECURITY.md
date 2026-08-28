# Security Policy

## Reporting a Vulnerability

请不要在公开 Issue 中提交可利用细节、凭据、内部地址或用户数据。请通过 GitHub Security Advisories 的私密漏洞报告功能联系维护者。

报告中请包含受影响版本、复现条件、影响范围和建议修复方式。维护者确认问题前，请勿公开漏洞细节。

## Deployment Baseline

- 首次初始化只监听本机地址。
- 对外开放前启用 LDAP，并使用 HTTPS 反向代理。
- 数据库、LDAP Bind、GitLab、钉钉和云账号凭据必须通过受限配置注入，不得提交到 Git。
- 定期备份 PostgreSQL，并限制数据库与平台端口的网络访问范围。
