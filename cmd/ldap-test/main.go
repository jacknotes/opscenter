package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/go-ldap/ldap/v3"
	"golang.org/x/term"
)

const (
	ldapHost = "192.168.10.110"
	ldapPort = 389
	baseDN   = "DC=hs,DC=com"
	bindDN   = "CN=域管理员,OU=Services,OU=Headquarter,DC=hs,DC=com"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	// 获取绑定密码
	fmt.Print("请输入域管理员密码: ")
	bindPassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalf("读取密码失败: %v", err)
	}
	fmt.Println()

	// 测试连接
	fmt.Println("\n========== 测试 1: LDAP 连接 ==========")
	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", ldapHost, ldapPort))
	if err != nil {
		log.Fatalf("连接 LDAP 服务器失败: %v", err)
	}
	defer conn.Close()
	fmt.Println("✓ LDAP 连接成功")

	// 测试绑定
	fmt.Println("\n========== 测试 2: 管理员绑定 ==========")
	if err := conn.Bind(bindDN, string(bindPassword)); err != nil {
		log.Fatalf("管理员绑定失败: %v", err)
	}
	fmt.Println("✓ 管理员绑定成功")

	// 获取测试用户名
	fmt.Print("\n请输入测试用户名 (sAMAccountName): ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)
	if username == "" {
		log.Fatal("用户名不能为空")
	}

	// 测试 3: 基础用户搜索
	fmt.Println("\n========== 测试 3: 基础用户搜索 ==========")
	filter := fmt.Sprintf("(sAMAccountName=%s)", ldap.EscapeFilter(username))
	fmt.Printf("过滤器: %s\n\n", filter)

	searchReq := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		10,
		0,
		false,
		filter,
		[]string{"dn", "sAMAccountName", "displayName", "mail"},
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil {
		log.Printf("搜索失败: %v", err)
	} else if len(sr.Entries) == 0 {
		fmt.Printf("✗ 未找到用户 '%s'\n", username)
	} else {
		fmt.Printf("✓ 找到 %d 个用户:\n", len(sr.Entries))
		for _, entry := range sr.Entries {
			fmt.Printf("  DN: %s\n", entry.DN)
			fmt.Printf("  sAMAccountName: %s\n", entry.GetAttributeValue("sAMAccountName"))
			fmt.Printf("  displayName: %s\n", entry.GetAttributeValue("displayName"))
			fmt.Printf("  mail: %s\n", entry.GetAttributeValue("mail"))
			fmt.Println()
		}
	}

	// 保存用户 DN
	var userDN string
	if len(sr.Entries) > 0 {
		userDN = sr.Entries[0].DN
	}

	// 测试 4: user_filter 测试
	fmt.Println("========== 测试 4: user_filter 测试 ==========")
	fmt.Println("说明: AD 中不能用 distinguishedName 通配符过滤，需要用 base_dn 限制 OU")
	fmt.Print("\n请输入 user_filter (留空跳过，可使用 %s 作为用户名占位符): ")
	userFilter, _ := reader.ReadString('\n')
	userFilter = strings.TrimSpace(userFilter)

	if userFilter != "" {
		actualFilter := strings.ReplaceAll(userFilter, "%s", ldap.EscapeFilter(username))
		fmt.Printf("实际过滤器: %s\n\n", actualFilter)

		searchReq2 := ldap.NewSearchRequest(
			baseDN,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			10,
			0,
			false,
			actualFilter,
			[]string{"dn", "sAMAccountName", "displayName"},
			nil,
		)

		sr2, err := conn.Search(searchReq2)
		if err != nil {
			log.Printf("搜索失败: %v", err)
		} else if len(sr2.Entries) == 0 {
			fmt.Printf("✗ user_filter 未匹配到用户 '%s'\n", username)
		} else {
			fmt.Printf("✓ user_filter 匹配到 %d 个用户:\n", len(sr2.Entries))
			for _, entry := range sr2.Entries {
				fmt.Printf("  DN: %s\n", entry.DN)
			}
		}
	} else {
		fmt.Println("跳过 user_filter 测试")
	}

	// 测试 5: 通过 base_dn 限制 OU
	fmt.Println("\n========== 测试 5: 通过 base_dn 限制 OU ==========")
	fmt.Println("说明: 正确的方式是把 base_dn 设置为特定 OU")
	fmt.Print("\n请输入要限制的 OU (如: OU=技术研发中心,OU=部门员工,OU=Users,OU=Headquarter,DC=hs,DC=com): ")
	ouBaseDN, _ := reader.ReadString('\n')
	ouBaseDN = strings.TrimSpace(ouBaseDN)

	if ouBaseDN != "" {
		fmt.Printf("搜索 base_dn: %s\n", ouBaseDN)
		fmt.Printf("过滤器: %s\n\n", filter)

		searchReq3 := ldap.NewSearchRequest(
			ouBaseDN,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			10,
			0,
			false,
			filter,
			[]string{"dn", "sAMAccountName", "displayName"},
			nil,
		)

		sr3, err := conn.Search(searchReq3)
		if err != nil {
			log.Printf("搜索失败: %v", err)
		} else if len(sr3.Entries) == 0 {
			fmt.Printf("✗ 在该 OU 下未找到用户 '%s'\n", username)
		} else {
			fmt.Printf("✓ 在该 OU 下找到 %d 个用户:\n", len(sr3.Entries))
			for _, entry := range sr3.Entries {
				fmt.Printf("  DN: %s\n", entry.DN)
			}
		}
	} else {
		fmt.Println("跳过 OU 测试")
	}

	// 测试 6: 用户密码认证
	fmt.Println("\n========== 测试 6: 用户密码认证 ==========")
	if userDN != "" {
		fmt.Printf("使用 DN: %s\n", userDN)
		fmt.Print("请输入该用户的密码: ")
		userPassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			log.Printf("读取密码失败: %v", err)
		} else {
			fmt.Println()
			testConn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", ldapHost, ldapPort))
			if err != nil {
				log.Printf("创建测试连接失败: %v", err)
			} else {
				defer testConn.Close()
				if err := testConn.Bind(userDN, string(userPassword)); err != nil {
					fmt.Printf("✗ 用户密码认证失败: %v\n", err)
				} else {
					fmt.Println("✓ 用户密码认证成功")
				}
			}
		}
	} else {
		fmt.Println("未找到测试用户的 DN，跳过密码认证测试")
	}

	fmt.Println("\n========== 测试完成 ==========")
}
