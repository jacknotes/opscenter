#!/bin/sh
set -e

# 配置文件选择逻辑：
#   1. CONFIG_FILE 环境变量直接指定配置文件路径（最高优先级）
#   2. GO_ENV 环境变量选择对应配置文件：config-${GO_ENV}.yaml
#   3. 默认使用 config.yaml

if [ -n "$CONFIG_FILE" ]; then
    # 直接指定配置文件路径
    CONFIG_PATH="$CONFIG_FILE"
elif [ -n "$GO_ENV" ]; then
    # 根据环境变量选择配置文件
    CONFIG_PATH="/app/config/config-${GO_ENV}.yaml"
else
    # 默认配置文件
    CONFIG_PATH="/app/config/config.yaml"
fi

# 检查配置文件是否存在
if [ ! -f "$CONFIG_PATH" ]; then
    echo "错误: 配置文件不存在: $CONFIG_PATH"
    echo "可用的配置文件:"
    ls -la /app/config/*.yaml 2>/dev/null || echo "  (无)"
    exit 1
fi

echo "使用配置文件: $CONFIG_PATH"

exec ./opscenter -config "$CONFIG_PATH"
