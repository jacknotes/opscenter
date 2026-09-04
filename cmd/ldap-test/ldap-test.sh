#!/bin/bash

# LDAP 测试脚本
# 使用方法: ./ldap-test.sh

# ========== 配置 ==========
LDAP_HOST="192.168.10.110"
LDAP_PORT="389"
BASE_DN="DC=hs,DC=com"
BIND_DN="CN=域管理员,OU=Services,OU=Headquarter,DC=hs,DC=com"
ATTR_USERNAME="sAMAccountName"
ATTR_NAME="displayName"
ATTR_EMAIL="mail"

# ========== 颜色输出 ==========
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ========== 函数 ==========
check_ldapsearch() {
    if ! command -v ldapsearch &> /dev/null; then
        echo -e "${RED}错误: ldapsearch 未安装${NC}"
        echo "请安装: yum install openldap-clients (CentOS) 或 apt-get install ldap-utils (Ubuntu)"
        exit 1
    fi
}

# ========== 主流程 ==========
echo "=========================================="
echo "       LDAP 连接测试脚本"
echo "=========================================="
echo ""

# 检查 ldapsearch
check_ldapsearch

# 获取密码
read -sp "请输入域管理员密码: " BIND_PASSWORD
echo ""
echo ""

# 测试 1: LDAP 连接
echo -e "${YELLOW}========== 测试 1: LDAP 连接 ==========${NC}"
if ldapsearch -x -H "ldap://${LDAP_HOST}:${LDAP_PORT}" -D "$BIND_DN" -w "$BIND_PASSWORD" -b "$BASE_DN" -s base "(objectClass=*)" dn > /dev/null 2>&1; then
    echo -e "${GREEN}✓ LDAP 连接成功${NC}"
else
    echo -e "${RED}✗ LDAP 连接失败${NC}"
    exit 1
fi
echo ""

# 测试 2: 管理员绑定
echo -e "${YELLOW}========== 测试 2: 管理员绑定 ==========${NC}"
if ldapsearch -x -H "ldap://${LDAP_HOST}:${LDAP_PORT}" -D "$BIND_DN" -w "$BIND_PASSWORD" -b "$BASE_DN" -s base "(objectClass=*)" dn > /dev/null 2>&1; then
    echo -e "${GREEN}✓ 管理员绑定成功${NC}"
else
    echo -e "${RED}✗ 管理员绑定失败，请检查 bind_dn 和密码${NC}"
    exit 1
fi
echo ""

# 获取测试用户名
read -p "请输入测试用户名 (${ATTR_USERNAME}): " TEST_USERNAME
if [ -z "$TEST_USERNAME" ]; then
    echo -e "${RED}用户名不能为空${NC}"
    exit 1
fi
echo ""

# 测试 3: 基础用户搜索
echo -e "${YELLOW}========== 测试 3: 基础用户搜索 ==========${NC}"
echo "过滤器: (${ATTR_USERNAME}=${TEST_USERNAME})"
echo ""
SEARCH_RESULT=$(ldapsearch -x -H "ldap://${LDAP_HOST}:${LDAP_PORT}" \
    -D "$BIND_DN" -w "$BIND_PASSWORD" \
    -b "$BASE_DN" \
    "(${ATTR_USERNAME}=${TEST_USERNAME})" \
    dn ${ATTR_USERNAME} ${ATTR_NAME} ${ATTR_EMAIL} 2>&1)

if echo "$SEARCH_RESULT" | grep -q "numEntries"; then
    ENTRY_COUNT=$(echo "$SEARCH_RESULT" | grep -c "dn:")
    echo -e "${GREEN}✓ 找到 ${ENTRY_COUNT} 个用户${NC}"
    echo "$SEARCH_RESULT" | grep -E "^(dn:|dn::|${ATTR_USERNAME}:|${ATTR_NAME}:|${ATTR_NAME}::|${ATTR_EMAIL}:)" | head -10
else
    echo -e "${RED}✗ 未找到用户 '${TEST_USERNAME}'${NC}"
fi
echo ""

# 保存用户 DN 用于后续测试
DN_LINE=$(echo "$SEARCH_RESULT" | grep "^dn:" | head -1)
if echo "$DN_LINE" | grep -q "^dn::"; then
    # DN 是 Base64 编码，需要解码
    DN_BASE64=$(echo "$DN_LINE" | sed 's/^dn:: //' | tr -d '[:space:]')
    SAVED_USER_DN=$(echo "$DN_BASE64" | base64 --decode 2>/dev/null)
    if [ -z "$SAVED_USER_DN" ]; then
        # 尝试用 openssl 解码
        SAVED_USER_DN=$(echo "$DN_BASE64" | openssl base64 -d 2>/dev/null)
    fi
    if [ -z "$SAVED_USER_DN" ]; then
        echo -e "${YELLOW}警告: DN 解码失败，使用原始值${NC}"
        SAVED_USER_DN="$DN_BASE64"
    fi
else
    SAVED_USER_DN=$(echo "$DN_LINE" | sed 's/^dn: //')
fi
echo "解码后的 DN: ${SAVED_USER_DN}"

# 测试 4: user_filter 测试
echo -e "${YELLOW}========== 测试 4: user_filter 测试 ==========${NC}"
echo "说明: AD 中不能用 distinguishedName 通配符过滤，需要用 base_dn 限制 OU"
echo ""
read -p "请输入 user_filter (留空跳过，可使用 %s 作为用户名占位符): " USER_FILTER

if [ -n "$USER_FILTER" ]; then
    ACTUAL_FILTER="${USER_FILTER//%s/$TEST_USERNAME}"
    echo "实际过滤器: ${ACTUAL_FILTER}"
    echo ""

    FILTER_RESULT=$(ldapsearch -x -H "ldap://${LDAP_HOST}:${LDAP_PORT}" \
        -D "$BIND_DN" -w "$BIND_PASSWORD" \
        -b "$BASE_DN" \
        "$ACTUAL_FILTER" \
        dn ${ATTR_USERNAME} ${ATTR_NAME} ${ATTR_EMAIL} 2>&1)

    if echo "$FILTER_RESULT" | grep -q "numEntries"; then
        ENTRY_COUNT2=$(echo "$FILTER_RESULT" | grep -c "dn:")
        echo -e "${GREEN}✓ user_filter 匹配到 ${ENTRY_COUNT2} 个用户${NC}"
        echo "$FILTER_RESULT" | grep -E "^(dn:|dn::|${ATTR_USERNAME}:|${ATTR_NAME}:|${ATTR_NAME}::|${ATTR_EMAIL}:)" | head -10
    else
        echo -e "${RED}✗ user_filter 未匹配到用户 '${TEST_USERNAME}'${NC}"
    fi
else
    echo "跳过 user_filter 测试"
fi
echo ""

# 测试 5: 通过 base_dn 限制 OU
echo -e "${YELLOW}========== 测试 5: 通过 base_dn 限制 OU ==========${NC}"
echo "说明: 正确的方式是把 base_dn 设置为特定 OU"
echo ""
read -p "请输入要限制的 OU (如: OU=技术研发中心,OU=部门员工,OU=Users,OU=Headquarter,DC=hs,DC=com): " OU_BASE_DN

if [ -n "$OU_BASE_DN" ]; then
    echo "搜索 base_dn: ${OU_BASE_DN}"
    echo "过滤器: (${ATTR_USERNAME}=${TEST_USERNAME})"
    echo ""

    OU_RESULT=$(ldapsearch -x -H "ldap://${LDAP_HOST}:${LDAP_PORT}" \
        -D "$BIND_DN" -w "$BIND_PASSWORD" \
        -b "$OU_BASE_DN" \
        "(${ATTR_USERNAME}=${TEST_USERNAME})" \
        dn ${ATTR_USERNAME} ${ATTR_NAME} 2>&1)

    if echo "$OU_RESULT" | grep -q "numEntries"; then
        ENTRY_COUNT3=$(echo "$OU_RESULT" | grep -c "dn:")
        echo -e "${GREEN}✓ 在该 OU 下找到 ${ENTRY_COUNT3} 个用户${NC}"
        echo "$OU_RESULT" | grep -E "^(dn:|dn::|${ATTR_USERNAME}:|${ATTR_NAME}:|${ATTR_NAME}::)" | head -10
    else
        echo -e "${RED}✗ 在该 OU 下未找到用户 '${TEST_USERNAME}'${NC}"
        echo -e "${YELLOW}提示: 可能用户不在这个 OU 下，或者 OU 路径不正确${NC}"
    fi
else
    echo "跳过 OU 测试"
fi
echo ""

# 测试 6: 用户密码认证
echo -e "${YELLOW}========== 测试 6: 用户密码认证 ==========${NC}"

if [ -n "$SAVED_USER_DN" ]; then
    echo "使用 DN: ${SAVED_USER_DN}"
    read -sp "请输入该用户的密码: " USER_PASSWORD
    echo ""
    echo ""

    AUTH_RESULT=$(ldapsearch -x -H "ldap://${LDAP_HOST}:${LDAP_PORT}" -D "$SAVED_USER_DN" -w "$USER_PASSWORD" -b "$BASE_DN" -s base "(objectClass=*)" dn 2>&1)
    AUTH_EXIT_CODE=$?

    if [ $AUTH_EXIT_CODE -eq 0 ] && echo "$AUTH_RESULT" | grep -q "dn:"; then
        echo -e "${GREEN}✓ 用户密码认证成功${NC}"
    else
        echo -e "${RED}✗ 用户密码认证失败${NC}"
        echo -e "${YELLOW}可能原因:${NC}"
        echo "  1. 密码错误"
        echo "  2. 账户被锁定"
        echo "  3. 账户被禁用"
        echo ""
        echo -e "${YELLOW}详细错误:${NC}"
        echo "$AUTH_RESULT" | grep -i "error\|result\|comment" | head -5
    fi
else
    echo -e "${RED}未找到测试用户的 DN，跳过密码认证测试${NC}"
fi

echo ""
echo "=========================================="
echo "           测试完成"
echo "=========================================="
