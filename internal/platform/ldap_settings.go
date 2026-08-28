package platform

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
)

type LDAPSettingsTester func(context.Context, LDAPAuthSettings) error

type LDAPIdentity struct {
	DN          string
	Username    string
	DisplayName string
	Email       string
}

type LDAPAuthenticator func(context.Context, LDAPAuthSettings, string, string) (LDAPIdentity, error)

func AuthenticateLDAP(ctx context.Context, settings LDAPAuthSettings, username, password string) (LDAPIdentity, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return LDAPIdentity{}, fmt.Errorf("用户名或密码错误")
	}
	filter := strings.ReplaceAll(settings.UserFilter, "%s", ldap.EscapeFilter(username))
	if _, err := ldap.CompileFilter(filter); err != nil {
		return LDAPIdentity{}, fmt.Errorf("LDAP 用户过滤器配置无效")
	}

	conn, timeout, err := dialLDAP(ctx, settings)
	if err != nil {
		return LDAPIdentity{}, err
	}
	defer conn.Close()
	if err := conn.Bind(settings.BindDN, settings.BindPassword); err != nil {
		return LDAPIdentity{}, fmt.Errorf("LDAP 服务账号认证失败")
	}

	attributes := uniqueLDAPAttributes(settings.UserAttribute, settings.DisplayNameAttribute, settings.EmailAttribute)
	result, err := conn.Search(ldap.NewSearchRequest(
		settings.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		2,
		int(timeout/time.Second),
		false,
		filter,
		attributes,
		nil,
	))
	if err != nil || len(result.Entries) != 1 {
		return LDAPIdentity{}, fmt.Errorf("用户名或密码错误")
	}
	entry := result.Entries[0]
	if err := conn.Bind(entry.DN, password); err != nil {
		return LDAPIdentity{}, fmt.Errorf("用户名或密码错误")
	}
	identity := LDAPIdentity{
		DN:          entry.DN,
		Username:    entry.GetAttributeValue(settings.UserAttribute),
		DisplayName: entry.GetAttributeValue(settings.DisplayNameAttribute),
		Email:       entry.GetAttributeValue(settings.EmailAttribute),
	}
	if identity.Username == "" {
		identity.Username = username
	}
	if identity.DisplayName == "" {
		identity.DisplayName = identity.Username
	}
	return identity, nil
}

func TestLDAPSettingsConnection(ctx context.Context, settings LDAPAuthSettings) error {
	filterProbe := strings.ReplaceAll(settings.UserFilter, "%s", ldap.EscapeFilter("ldap-config-probe"))
	if _, err := ldap.CompileFilter(filterProbe); err != nil {
		return fmt.Errorf("用户过滤器语法错误: %w", err)
	}

	conn, timeout, err := dialLDAP(ctx, settings)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Bind(settings.BindDN, settings.BindPassword); err != nil {
		return fmt.Errorf("Bind DN 或密码错误: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("LDAP 配置验证超时: %w", err)
	}

	baseResult, err := conn.Search(ldap.NewSearchRequest(
		settings.BaseDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		1,
		int(timeout/time.Second),
		false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	))
	if err != nil || len(baseResult.Entries) == 0 {
		if err != nil {
			return fmt.Errorf("Base DN 不存在或不可访问: %w", err)
		}
		return fmt.Errorf("Base DN 不存在或不可访问")
	}

	attributes := uniqueLDAPAttributes(
		settings.UserAttribute,
		settings.DisplayNameAttribute,
		settings.EmailAttribute,
	)
	users, err := conn.Search(ldap.NewSearchRequest(
		settings.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1,
		int(timeout/time.Second),
		false,
		"(objectClass=person)",
		attributes,
		nil,
	))
	if err != nil {
		return fmt.Errorf("在 Base DN 下搜索用户失败: %w", err)
	}
	if len(users.Entries) == 0 {
		return fmt.Errorf("Base DN 下没有找到 person 用户")
	}
	probeUser := users.Entries[0]
	loginValue := probeUser.GetAttributeValue(settings.UserAttribute)
	if loginValue == "" {
		return fmt.Errorf("用户缺少登录属性 %s", settings.UserAttribute)
	}
	if probeUser.GetAttributeValue(settings.DisplayNameAttribute) == "" {
		return fmt.Errorf("用户缺少姓名属性 %s", settings.DisplayNameAttribute)
	}
	if probeUser.GetAttributeValue(settings.EmailAttribute) == "" {
		return fmt.Errorf("用户缺少邮箱属性 %s", settings.EmailAttribute)
	}

	userFilter := strings.ReplaceAll(settings.UserFilter, "%s", ldap.EscapeFilter(loginValue))
	matched, err := conn.Search(ldap.NewSearchRequest(
		settings.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1,
		int(timeout/time.Second),
		false,
		userFilter,
		attributes,
		nil,
	))
	if err != nil {
		return fmt.Errorf("用户过滤器查询失败: %w", err)
	}
	if len(matched.Entries) == 0 {
		return fmt.Errorf("用户过滤器无法按 %s 定位用户", settings.UserAttribute)
	}
	return nil
}

func dialLDAP(ctx context.Context, settings LDAPAuthSettings) (*ldap.Conn, time.Duration, error) {
	parsedURL, err := url.Parse(settings.URL)
	if err != nil || parsedURL.Hostname() == "" {
		return nil, 0, fmt.Errorf("LDAP 地址无效")
	}
	timeout := 8 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: parsedURL.Hostname()}
	dialOptions := []ldap.DialOpt{ldap.DialWithDialer(&net.Dialer{Timeout: timeout})}
	if parsedURL.Scheme == "ldaps" {
		dialOptions = append(dialOptions, ldap.DialWithTLSConfig(tlsConfig))
	}
	conn, err := ldap.DialURL(settings.URL, dialOptions...)
	if err != nil {
		return nil, 0, fmt.Errorf("连接 LDAP 地址失败: %w", err)
	}
	conn.SetTimeout(timeout)
	if settings.StartTLS {
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, 0, fmt.Errorf("StartTLS 握手失败: %w", err)
		}
	}
	return conn, timeout, nil
}

func uniqueLDAPAttributes(attributes ...string) []string {
	seen := make(map[string]bool, len(attributes))
	result := make([]string, 0, len(attributes))
	for _, attribute := range attributes {
		key := strings.ToLower(strings.TrimSpace(attribute))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, attribute)
	}
	return result
}
