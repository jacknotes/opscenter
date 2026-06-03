package service

import (
	"crypto/tls"
	"fmt"
	"log"

	"github.com/go-ldap/ldap/v3"

	"opscenter/internal/config"
)

// LDAPUserInfo 从 LDAP 获取的用户信息。
type LDAPUserInfo struct {
	Username string // 用户名（sAMAccountName）
	Name     string // 姓名（displayName）
	Email    string // 邮箱（mail）
	DN       string // 用户 DN
}

// LDAPService 负责 LDAP 认证。
type LDAPService struct {
	config *config.LDAPConfig
}

// NewLDAPService 创建 LDAP 服务实例。
func NewLDAPService(cfg *config.LDAPConfig) *LDAPService {
	return &LDAPService{config: cfg}
}

// Authenticate 使用 LDAP 验证用户凭据，返回用户信息。
func (s *LDAPService) Authenticate(username, password string) (*LDAPUserInfo, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("LDAP 未启用")
	}

	// 1. 使用管理员绑定账号搜索用户 DN
	userDN, err := s.getUserDN(username)
	if err != nil {
		return nil, fmt.Errorf("LDAP 搜索用户失败: %w", err)
	}

	// 2. 使用用户 DN 和密码进行绑定验证
	if err := s.bindUser(userDN, password); err != nil {
		return nil, fmt.Errorf("LDAP 认证失败: %w", err)
	}

	// 3. 重新使用管理员绑定获取用户详细信息
	userInfo, err := s.getUserInfo(username)
	if err != nil {
		// 如果获取信息失败，返回基本信息
		return &LDAPUserInfo{
			Username: username,
			DN:       userDN,
		}, nil
	}

	return userInfo, nil
}

// connect 创建 LDAP 连接。
func (s *LDAPService) connect() (*ldap.Conn, error) {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	if s.config.StartTLS {
		conn, err := ldap.DialTLS("tcp", addr, &tls.Config{
			InsecureSkipVerify: true, // 内网 LDAP 可能使用自签名证书
		})
		if err != nil {
			return nil, fmt.Errorf("连接 LDAP 服务器失败: %w", err)
		}
		return conn, nil
	}

	conn, err := ldap.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("连接 LDAP 服务器失败: %w", err)
	}

	// 如果配置了 StartTLS，升级连接
	if s.config.StartTLS {
		if err := conn.StartTLS(&tls.Config{
			InsecureSkipVerify: true,
		}); err != nil {
			conn.Close()
			return nil, fmt.Errorf("StartTLS 失败: %w", err)
		}
	}

	return conn, nil
}

// bindAdmin 使用管理员账号绑定。
func (s *LDAPService) bindAdmin(conn *ldap.Conn) error {
	if s.config.BindDN == "" {
		return nil // 匿名绑定
	}
	if err := conn.Bind(s.config.BindDN, s.config.BindPassword); err != nil {
		return fmt.Errorf("管理员绑定失败: %w", err)
	}
	return nil
}

// getUserDN 根据用户名搜索用户 DN。
func (s *LDAPService) getUserDN(username string) (string, error) {
	conn, err := s.connect()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if err := s.bindAdmin(conn); err != nil {
		return "", err
	}

	// 构建搜索过滤器
	filter := s.config.UserFilter
	if filter == "" {
		attr := s.config.Attributes.Username
		if attr == "" {
			attr = "sAMAccountName"
		}
		// 对用户名进行转义，防止 LDAP 注入
		escaped := ldap.EscapeFilter(username)
		filter = fmt.Sprintf("(%s=%s)", attr, escaped)
	}

	searchReq := ldap.NewSearchRequest(
		s.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, // 只需要一个结果
		0,
		false,
		filter,
		[]string{"dn"},
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil {
		return "", fmt.Errorf("LDAP 搜索失败: %w", err)
	}

	if len(sr.Entries) == 0 {
		return "", fmt.Errorf("用户 '%s' 不存在", username)
	}

	return sr.Entries[0].DN, nil
}

// bindUser 使用用户 DN 和密码进行绑定验证。
func (s *LDAPService) bindUser(userDN, password string) error {
	conn, err := s.connect()
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.Bind(userDN, password); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			return fmt.Errorf("用户名或密码错误")
		}
		return fmt.Errorf("绑定失败: %w", err)
	}

	return nil
}

// getUserInfo 获取用户详细信息。
func (s *LDAPService) getUserInfo(username string) (*LDAPUserInfo, error) {
	conn, err := s.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := s.bindAdmin(conn); err != nil {
		return nil, err
	}

	attrUsername := s.config.Attributes.Username
	if attrUsername == "" {
		attrUsername = "sAMAccountName"
	}
	attrName := s.config.Attributes.Name
	if attrName == "" {
		attrName = "displayName"
	}
	attrEmail := s.config.Attributes.Email
	if attrEmail == "" {
		attrEmail = "mail"
	}

	// 构建搜索过滤器
	filter := s.config.UserFilter
	if filter == "" {
		escaped := ldap.EscapeFilter(username)
		filter = fmt.Sprintf("(%s=%s)", attrUsername, escaped)
	}

	searchReq := ldap.NewSearchRequest(
		s.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1,
		0,
		false,
		filter,
		[]string{"dn", attrUsername, attrName, attrEmail},
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("LDAP 搜索失败: %w", err)
	}

	if len(sr.Entries) == 0 {
		return nil, fmt.Errorf("用户 '%s' 不存在", username)
	}

	entry := sr.Entries[0]
	return &LDAPUserInfo{
		Username: entry.GetAttributeValue(attrUsername),
		Name:     entry.GetAttributeValue(attrName),
		Email:    entry.GetAttributeValue(attrEmail),
		DN:       entry.DN,
	}, nil
}

// TestConnection 测试 LDAP 连接。
func (s *LDAPService) TestConnection() error {
	if !s.config.Enabled {
		return fmt.Errorf("LDAP 未启用")
	}

	conn, err := s.connect()
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := s.bindAdmin(conn); err != nil {
		return err
	}

	log.Printf("[LDAP] 连接测试成功: %s:%d", s.config.Host, s.config.Port)
	return nil
}

// ListUsers 获取 LDAP 用户列表。
func (s *LDAPService) ListUsers() ([]LDAPUserInfo, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("LDAP 未启用")
	}

	conn, err := s.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := s.bindAdmin(conn); err != nil {
		return nil, err
	}

	attrUsername := s.config.Attributes.Username
	if attrUsername == "" {
		attrUsername = "sAMAccountName"
	}
	attrName := s.config.Attributes.Name
	if attrName == "" {
		attrName = "displayName"
	}
	attrEmail := s.config.Attributes.Email
	if attrEmail == "" {
		attrEmail = "mail"
	}

	// 搜索所有用户
	filter := "(&(objectClass=user)(objectCategory=person))"
	if s.config.UserFilter != "" {
		filter = s.config.UserFilter
	}

	searchReq := ldap.NewSearchRequest(
		s.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, // 不限制数量
		0,
		false,
		filter,
		[]string{"dn", attrUsername, attrName, attrEmail},
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("LDAP 搜索失败: %w", err)
	}

	var users []LDAPUserInfo
	for _, entry := range sr.Entries {
		username := entry.GetAttributeValue(attrUsername)
		if username == "" {
			continue
		}
		users = append(users, LDAPUserInfo{
			Username: username,
			Name:     entry.GetAttributeValue(attrName),
			Email:    entry.GetAttributeValue(attrEmail),
			DN:       entry.DN,
		})
	}

	return users, nil
}
