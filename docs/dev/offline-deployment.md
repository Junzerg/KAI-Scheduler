# KAI Scheduler 离线部署指南

本指南说明如何在外网环境构建 KAI Scheduler 镜像，打包传输到内网离线 K8s 集群并完成部署。

> 开发环境离线准备请参考 [offline-bundle.md](./offline-bundle.md)。

---

## 一、外网环境：构建与导出

### 1.1 构建镜像

```bash
cd /path/to/KAI-Scheduler

# 设置版本号（建议带日期，便于识别）
export VERSION=0.0.0-offline-$(date +%Y%m%d)
export DOCKER_REPO_BASE=registry/local/kai-scheduler

# 构建所有组件镜像（含 crd-upgrader）
make build DOCKER_REPO_BASE=$DOCKER_REPO_BASE VERSION=$VERSION
make docker-build-crd-upgrader DOCKER_REPO_BASE=$DOCKER_REPO_BASE VERSION=$VERSION
```

**仅构建 amd64 架构**（内网多为 x64 节点）：

```bash
make build DOCKER_BUILD_PLATFORM=linux/amd64 DOCKER_REPO_BASE=$DOCKER_REPO_BASE VERSION=$VERSION
make docker-build-crd-upgrader DOCKER_BUILD_PLATFORM=linux/amd64 DOCKER_REPO_BASE=$DOCKER_REPO_BASE VERSION=$VERSION
```

### 1.2 导出镜像为 tar 文件

```bash
# 必须设置变量，否则会报 invalid reference format
export REGISTRY="registry/local/kai-scheduler"
export VERSION="0.0.0-offline-20260211"   # 与上面构建时一致

mkdir -p offline-images
cd offline-images

for component in scheduler operator binder podgrouper podgroupcontroller queuecontroller admission nodescaleadjuster resourcereservation scalingpod crd-upgrader; do
  docker save ${REGISTRY}/${component}:${VERSION} -o kai-scheduler-${component}-${VERSION}.tar
done
```

### 1.3 打包 Helm Chart

```bash
cd /path/to/KAI-Scheduler

mkdir -p offline-package
helm package ./deployments/kai-scheduler -d ./offline-package \
  --app-version $VERSION --version $VERSION
```

### 1.4 打包传输文件

```bash
cd /path/to/KAI-Scheduler

mkdir -p offline-deploy
cp offline-images/kai-scheduler-{scheduler,operator,binder,podgrouper,podgroupcontroller,queuecontroller,admission,nodescaleadjuster,resourcereservation,scalingpod,crd-upgrader}-*.tar offline-deploy/
cp offline-package/*.tgz offline-deploy/

# 打成压缩包便于传输
tar -czvf kai-scheduler-offline-${VERSION}.tar.gz -C offline-deploy .
```

将 `kai-scheduler-offline-*.tar.gz` 或 `offline-deploy/` 目录拷贝到内网环境（U 盘、文件传输等）。

---

## 二、内网环境：导入与部署

### 2.1 解压（若传输的是 tar.gz）

```bash
mkdir -p kai-scheduler-offline
tar -xzvf kai-scheduler-offline-0.0.0-offline-20260211.tar.gz -C kai-scheduler-offline
cd kai-scheduler-offline
```

### 2.2 导入镜像

**方式 A：单节点或节点可直接访问本机 Docker**

```bash
for f in kai-scheduler-{scheduler,operator,binder,podgrouper,podgroupcontroller,queuecontroller,admission,nodescaleadjuster,resourcereservation,scalingpod,crd-upgrader}-*.tar; do
  docker load -i "$f"
done
```

**方式 B：多节点集群，使用内网私有 Registry**

```bash
VERSION="0.0.0-offline-20260211"
INTERNAL_REGISTRY="172.18.191.197:5000/kai-scheduler"  # 替换为你的内网 registry 地址

# 1. 加载镜像
for f in kai-scheduler-{scheduler,operator,binder,podgrouper,podgroupcontroller,queuecontroller,admission,nodescaleadjuster,resourcereservation,scalingpod,crd-upgrader}-*.tar; do
  docker load -i "$f"
done

# 2. 打 tag 并推送到内网 registry
for c in scheduler operator binder podgrouper podgroupcontroller queuecontroller admission nodescaleadjuster resourcereservation scalingpod crd-upgrader; do
  docker tag registry/local/kai-scheduler/${c}:${VERSION} ${INTERNAL_REGISTRY}/${c}:${VERSION}
  docker push ${INTERNAL_REGISTRY}/${c}:${VERSION}
done
```

### 2.3 部署到 K8s

**方式 A：镜像在节点本机**

```bash
VERSION="0.0.0-offline-20260211"

helm upgrade -i kai-scheduler -n kai-scheduler --create-namespace \
  ./kai-scheduler-${VERSION}.tgz \
  --set global.registry=registry/local/kai-scheduler \
  --set global.tag=${VERSION}
```

**方式 B：使用内网私有 Registry**

```bash
VERSION="0.0.0-offline-20260211"
INTERNAL_REGISTRY="172.18.191.197:5000/kai-scheduler"

helm upgrade -i kai-scheduler -n kai-scheduler --create-namespace \
  ./kai-scheduler-${VERSION}.tgz \
  --set global.registry=${INTERNAL_REGISTRY} \
  --set global.tag=${VERSION}
```

### 2.4 私有 Registry 需认证时

若内网 registry 需要登录，先创建 Secret：

```bash
kubectl create secret docker-registry regcred \
  --docker-server=172.18.191.197:5000 \
  --docker-username=<用户名> \
  --docker-password=<密码> \
  -n kai-scheduler
```

部署时追加：

```bash
--set global.imagePullSecrets[0].name=regcred
```

### 2.5 验证部署

```bash
kubectl get pods -n kai-scheduler
kubectl get queue -A
```

---

## 三、文件清单

| 类型 | 文件 |
|------|------|
| 镜像 | `kai-scheduler-<组件>-<版本>.tar`（11 个组件） |
| Helm Chart | `kai-scheduler-<版本>.tgz` |
| 组件列表 | scheduler, operator, binder, podgrouper, podgroupcontroller, queuecontroller, admission, nodescaleadjuster, resourcereservation, scalingpod, crd-upgrader |

---

## 四、注意事项

| 项目 | 说明 |
|------|------|
| **架构** | 内网多为 amd64，外网构建时建议 `DOCKER_BUILD_PLATFORM=linux/amd64` |
| **变量** | 执行 `docker save` 前必须设置 `REGISTRY` 和 `VERSION`，否则会报 `invalid reference format` |
| **K8s 版本** | 确认内网集群版本与 KAI Scheduler 兼容 |
| **CRD** | 首次安装会自动创建 CRD，如需外部 CRD 见 `deployments/external-crds/` |
| **高可用** | 多副本时部署时加 `--set global.leaderElection=true` |
