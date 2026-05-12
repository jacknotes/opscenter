#!/bin/bash

AUTH_PASSWORD='homsom.com'
ScaleDownValue='0'
CHECK_INTERVAL=5			# 检查Pod状态的间隔时间（秒）
MAX_RETRIES=18				# 最大重试次数（5秒*12=60秒超时） 
GRACE_TIME=$((CHECK_INTERVAL * 7))	# pod有30秒优雅关闭时间，所以预留35秒
DATETIME='date +"%F %T"'

# 只针对rollout控制器
rolloutlist(){
        echo '
pro-java pro-java-flightrefund-order-service-hs-com-rollout 2
pro-java pro-java-hotelbusiness-service-hs-com-rollout 2
pro-java pro-java-budget-manageservice-hs-com-rollout 2
        ' | grep -v '^#'
}

# 只针对不受argocd管理的deployment控制器，因为argocd管理deployment更改不了副本
deploymentlist(){
        echo '
middleware middleware-xxl-job-admin-deployment 2
        ' | grep -v '^#'
}

# 依赖的rollout列表
requirelist(){
        echo '
pro-dotnet pro-dotnet-domaineventserviceapi-hs-com-rollout 4
        ' | grep -v '^#'
}

# 并行执行缩放任务的函数
parallel_scale() {
    local action=$1
    local namespace=$2
    local resource_type=$3
    local resource_name=$4
    local target_replicas=$5
    
    # 执行缩放操作并捕获输出
    output=$(scale_resource "$action" "$namespace" "$resource_type" "$resource_name" "$target_replicas" 2>&1)

    # 使用锁机制确保输出不会交错，-x表示独占锁，200表示文件描述符
    (
        flock -x 200
        echo "$output"
        echo "----------------------------------------"
    ) 200>/tmp/scale_output.lock
}

# 增强版 Kubernetes 环境验证函数
check_k8s_environment() {
    echo "----------------------------------------"
    # 参数说明：
    # $1 - 预期 server 地址（默认 192.168.13.90）
    # $2 - 自定义 kubeconfig 路径（可选）
    local expected_server="${1:-192.168.13.90}"
    local user_kubeconfig="${2:-}"

    # 获取当前用户名
    local current_user=$(whoami)
    local kubeconfig_path=""

    # 确定 kubeconfig 路径
    if [ -n "$user_kubeconfig" ]; then
        # 使用用户指定的配置文件
        kubeconfig_path="$user_kubeconfig"
    else
        # 自动检测默认路径
        if [ "$current_user" = "root" ]; then
            kubeconfig_path="/root/.kube/config"
        else
            kubeconfig_path="/home/$current_user/.kube/config"
            # 如果普通用户路径不存在，尝试检查是否有全局配置
            [ ! -f "$kubeconfig_path" ] && kubeconfig_path="/etc/kubernetes/admin.conf"
        fi
    fi

    # 检查配置文件是否存在且可读
    if [ ! -f "$kubeconfig_path" ]; then
        echo >&2 "[ERROR] Kubernetes 配置文件不存在: $kubeconfig_path"
        echo >&2 "请确保:"
        echo >&2 "1. 已配置 kubectl"
        echo >&2 "2. 或者通过参数指定正确路径"
        return 2
    fi

    if [ ! -r "$kubeconfig_path" ]; then
        echo >&2 "[ERROR] 无权限读取配置文件: $kubeconfig_path"
        return 3
    fi

    # 提取 server 地址（兼容多种方法）
    local current_server=""
    if command -v yq &>/dev/null; then
        # 方法1：使用 yq（最准确）
        current_server=$(yq eval '.clusters[0].cluster.server' "$kubeconfig_path")
    elif command -v kubectl &>/dev/null; then
        # 方法2：使用 kubectl（原生支持）
        current_server=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null)
    else
        # 方法3：使用 grep/awk（基础方法）
        current_server=$(grep "server:" "$kubeconfig_path" | 
                        awk '{print $2}' | 
                        sed -e "s/'//g" -e 's/"//g')
    fi

    # 验证是否成功获取 server
    if [ -z "$current_server" ]; then
        echo >&2 "[ERROR] 无法从配置文件中解析 server 地址"
        return 4
    fi

    # 标准化 server 地址（去除协议和端口）
    local normalized_server=$(echo "$current_server" | 
                            sed -e 's#^https\?://##' -e 's#:[0-9]\+$##')

    # 验证环境
    if [[ "$normalized_server" != *"$expected_server"* ]]; then
        echo >&2 "[ERROR] 环境验证失败"
        echo >&2 "当前环境: $current_server"
        echo >&2 "期望包含: $expected_server"
        echo >&2 "配置文件: $kubeconfig_path"
        exit 5
    fi

    echo "[OK] 环境验证通过"
    echo "用户: $current_user"
    echo "配置文件: $kubeconfig_path"
    echo "Server: $current_server"
    echo "----------------------------------------"
}

auth(){
        read -s -t 30 -n 16 -p 'please input password:' CMD_PASSWORD
        if [ "${CMD_PASSWORD}" != "${AUTH_PASSWORD}" ];then
                echo -e '\n[ERROR]: password error!'
                exit 10
        else
                echo -e '\n'
        fi
}

# 检查Pod是否全部停止
check_pods_stopped() {
    local namespace=$1
    local resource_type=$2
    local resource_name=$3
    
    local retries=0
    local running_pods=1
    
    echo "`eval "$DATETIME"` [INFO] 正在检查 ${namespace}/${resource_name} 的Pod状态..."
    
    while [ $retries -lt $MAX_RETRIES ] && [ $running_pods -gt 0 ]; do
        running_pods=$(kubectl get pods -n "$namespace" | grep "$resource_name" | grep 'Running' | wc -l)
        
        if [ $running_pods -gt 0 ]; then
	    echo "`eval "$DATETIME"` [INFO] 仍有 ${running_pods} 个Pod在运行中，等待 ${CHECK_INTERVAL} 秒后重试..."
            sleep $CHECK_INTERVAL
            retries=$((retries + 1))
        fi
    done
    
    if [ $running_pods -gt 0 ]; then
        echo "[ERROR] 超时: ${namespace}/${resource_name} 仍有Pod在运行"
        return 1
    else
        echo "`eval "$DATETIME"` [INFO] ${namespace}/${resource_name} 的所有Pod已停止"
        return 0
    fi
}

# 检查Pod是否成功启动
check_pods_started() {
    local namespace=$1
    local resource_type=$2
    local resource_name=$3
    local expected_replicas=$4
    
    local retries=0
    local ready_pods=0
    
    echo "`eval "$DATETIME"` [INFO] 正在检查 ${namespace}/${resource_name} 的Pod启动状态..."
    
    while [ $retries -lt $MAX_RETRIES ] && [ $ready_pods -lt $expected_replicas ]; do
        # 获取就绪Pod数量
        ready_pods=$(kubectl get "$resource_type" -n "$namespace" "$resource_name" \
            -o jsonpath='{.status.readyReplicas}')
        ready_pods=${ready_pods:-0}  # 如果为空则设为0
        
        # 获取总Pod数量
        total_pods=$(kubectl get "$resource_type" -n "$namespace" "$resource_name" \
            -o jsonpath='{.status.replicas}')
        total_pods=${total_pods:-0}  # 如果为空则设为0
        
        if [ $ready_pods -lt $expected_replicas ]; then
            echo "`eval "$DATETIME"` [INFO] Pod启动中 (就绪 ${ready_pods}/${expected_replicas})，等待 ${CHECK_INTERVAL} 秒后重试..."
            sleep $CHECK_INTERVAL
            retries=$((retries + 1))
        fi
    done
    
    if [ $ready_pods -lt $expected_replicas ]; then
        echo "[ERROR] 超时: ${namespace}/${resource_name} 只有 ${ready_pods}/${expected_replicas} 个Pod就绪"
        return 1
    else
        echo "[SUCCESS] ${namespace}/${resource_name} 的所有Pod已就绪 (${ready_pods}/${expected_replicas})"
        return 0
    fi
}

scale_resource() {
    local action=$1
    local namespace=$2
    local resource_type=$3
    local resource_name=$4
    local target_replicas=$5
    
    echo "正在${action}命名空间: $namespace, 资源: $resource_name, 目标副本: $target_replicas"
    
    # 获取当前副本
    current_replicas=$(kubectl get "$resource_type" -n "$namespace" "$resource_name" -o json | jq -r '.spec.replicas' 2>&1)
    
    if [ "$current_replicas" == "$target_replicas" ]; then
        echo "`eval "$DATETIME"` [INFO] ${namespace}/${resource_name} 已经是目标副本数 ${target_replicas}，无需操作"
        return 0
    fi
    
    # 执行缩放操作
    result=$(kubectl scale "$resource_type" --replicas="$target_replicas" -n "$namespace" "$resource_name" 2>&1)
    
    if [[ $? -eq 0 ]]; then
        echo "[SUCCESS] ${action}成功: $result"
        
        # 如果是缩容操作，检查Pod状态
        if [ "$action" == "scaledown" ] && [ "$target_replicas" == "0" ]; then
            check_pods_stopped "$namespace" "$resource_type" "$resource_name" || return 1
        fi
        
        # 如果是扩容操作，检查Pod状态
        if [ "$action" == "scaleup" ] && [ "$target_replicas" -gt 0 ]; then
            check_pods_started "$namespace" "$resource_type" "$resource_name" "$target_replicas" || return 1
        fi
        
        return 0
    else
        echo "[ERROR] ${action}失败: $result"
        return 1
    fi
}

scaledown(){
    echo "`eval "$DATETIME"` [INFO]: 开始缩容操作"
    auth
    
    # 创建临时文件存储进程ID
    tmpfile=$(mktemp)
    
    # 并行缩容rolloutlist和deploymentlist中的资源
    while read -r namespace rollout replicas; do
        [[ -z "$namespace" || -z "$rollout" ]] && continue
        parallel_scale "scaledown" "$namespace" "rollout" "$rollout" "$ScaleDownValue" &
        echo $! >> "$tmpfile"
    done < <(rolloutlist)
    
    while read -r namespace deployment replicas; do
        [[ -z "$namespace" || -z "$deployment" ]] && continue
        parallel_scale "scaledown" "$namespace" "deployment" "$deployment" "$ScaleDownValue" &
        echo $! >> "$tmpfile"
    done < <(deploymentlist)
    
    # 等待所有并行任务完成
    while read -r pid; do
        wait "$pid"
    done < "$tmpfile"
    rm "$tmpfile"
    
    # 额外休息35秒，因为有优雅关闭时间30秒
    echo "`eval "$DATETIME"` [INFO]: 休息 $GRACE_TIME 秒"
    sleep $GRACE_TIME

    # 确认所有Pod已停止后再缩容requirelist中的资源
    echo "`eval "$DATETIME"` [INFO]: 开始缩容requirelist中的资源:"
    while read -r namespace rollout replicas; do
        [[ -z "$namespace" || -z "$rollout" ]] && continue
        scale_resource "scaledown" "$namespace" "rollout" "$rollout" "$ScaleDownValue"
        echo "----------------------------------------"
    done < <(requirelist)
}

scaleup(){
    echo "`eval "$DATETIME"` [INFO]: 开始扩容操作"
    auth
    
    # 先扩容requirelist中的资源并等待就绪
    while read -r namespace rollout replicas; do
        [[ -z "$namespace" || -z "$rollout" || -z "$replicas" ]] && continue
        if ! scale_resource "scaleup" "$namespace" "rollout" "$rollout" "$replicas"; then
            echo "[ERROR] requirelist资源扩容失败，停止后续操作"
            exit 20
        fi
        echo "----------------------------------------"
    done < <(requirelist)
    
    # 额外休息5秒
    echo "`eval "$DATETIME"` [INFO]: 休息 $CHECK_INTERVAL 秒"
    sleep $CHECK_INTERVAL

    # requirelist资源全部就绪后，再并行扩容rolloutlist和deploymentlist中的资源
    echo "`eval "$DATETIME"` [INFO]: requirelist资源已就绪，开始并行扩容rolloutlist和deploymentlist"
    
    # 创建临时文件存储进程ID
    tmpfile=$(mktemp)
    
    while read -r namespace rollout replicas; do
        [[ -z "$namespace" || -z "$rollout" || -z "$replicas" ]] && continue
        parallel_scale "scaleup" "$namespace" "rollout" "$rollout" "$replicas" &
        echo $! >> "$tmpfile"
    done < <(rolloutlist)
    
    while read -r namespace deployment replicas; do
        [[ -z "$namespace" || -z "$deployment" || -z "$replicas" ]] && continue
        parallel_scale "scaleup" "$namespace" "deployment" "$deployment" "$replicas" &
        echo $! >> "$tmpfile"
    done < <(deploymentlist)
    
    # 等待所有并行任务完成
    while read -r pid; do
        wait "$pid"
    done < "$tmpfile"
    rm "$tmpfile"
}

list() {
    echo "`eval "$DATETIME"` [INFO]: 当前资源状态"
    
    echo "=== Rollout状态 ==="
    while read -r namespace rollout replicas; do
        [[ -z "$namespace" || -z "$rollout" ]] && continue
        kubectl get rollout -n "$namespace" "$rollout" -o wide
        echo "----------------------------------------"
    done < <(rolloutlist)
  
    echo "=== Deployment状态 ==="
    while read -r namespace deployment replicas; do
        [[ -z "$namespace" || -z "$deployment" ]] && continue
        kubectl get deployment -n "$namespace" "$deployment" -o wide
        echo "----------------------------------------"
    done < <(deploymentlist)
    
    echo "=== Require列表状态 ==="
    while read -r namespace rollout replicas; do
        [[ -z "$namespace" || -z "$rollout" ]] && continue
        kubectl get rollout -n "$namespace" "$rollout" -o wide
        echo "----------------------------------------"
    done < <(requirelist)
}

# 校验k8s环境
case "$1" in
        list)
		check_k8s_environment $2 $3;
                $1;;
        scaledown)
		check_k8s_environment $2 $3;
                $1;;
        scaleup)
		check_k8s_environment $2 $3;
                $1;;
        *)
                echo "Usage: $0 { list | scaledown | scaleup } [ KUBE_APISERVER & KUBE_CONFIG_PATH ]"
                exit 2
esac
