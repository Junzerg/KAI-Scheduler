# KAI Scheduler 离线环境开发准备指南

本文档说明如何在**有网环境**预先准备依赖，并带入**离线环境**进行 KAI Scheduler 的开发、构建和调试。  
若在 **Mac** 上准备、目标环境为 **Linux x64**，需特别注意 Docker 镜像架构，见第二部分。

> 若目标环境在**中国大陆**且可联网，可参考 [china-mainland-mirrors.md](./china-mainland-mirrors.md) 配置国内镜像源。

---

## 一、需要预先准备并带进离线环境的内容

### 1. Docker 镜像（拉取并导出）

在有网环境执行以下命令。**若在 Mac 上准备给 Linux x64 用，必须加 `--platform linux/amd64`**（见第二部分）：

```bash
# 构建用
docker pull golang:1.24.4-bullseye

# 构建产物运行时
docker pull registry.access.redhat.com/ubi9/ubi-minimal

# 调试目标（DEBUG=1 时）
docker pull golang:1.22

# Lint
docker pull golangci/golangci-lint:v1.64.8

# Helm chart 测试
docker pull helmunittest/helm-unittest:3.17.2-0.8.1
```

导出为 tar 便于拷贝：

```bash
mkdir -p offline-docker-images
docker save golang:1.24.4-bullseye -o offline-docker-images/golang-1.24.4-bullseye.tar
docker save registry.access.redhat.com/ubi9/ubi-minimal -o offline-docker-images/ubi9-minimal.tar
docker save golang:1.22 -o offline-docker-images/golang-1.22.tar
docker save golangci/golangci-lint:v1.64.8 -o offline-docker-images/golangci-lint.tar
docker save helmunittest/helm-unittest:3.17.2-0.8.1 -o offline-docker-images/helm-unittest.tar
```

离线环境导入：

```bash
for f in offline-docker-images/*.tar; do docker load -i "$f"; done
```

### 2. Go 模块缓存

在有网环境项目根目录执行：

```bash
cd /path/to/KAI-Scheduler
go mod download
```

缓存目录：`$GOPATH/pkg/mod` 或 `$HOME/go/pkg/mod`。  
把整个 `go/pkg/mod`（或 `$GOPATH/pkg/mod`）目录拷贝到离线环境，并保证离线环境的 `GOPATH` 或 `GOMODCACHE` 指向该目录。

更稳妥的方式是使用 vendor：

```bash
go mod vendor
```

这样会生成 `vendor/` 目录，把整个 `vendor/` 和 `go.mod`、`go.sum` 一起拷贝到离线环境，构建时加上 `-mod=vendor` 即可。

### 3. Go 工具（controller-gen、mockgen、addlicense、setup-envtest）

在有网环境执行：

```bash
cd /path/to/KAI-Scheduler
mkdir -p bin

GOBIN=$(pwd)/bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.1
GOBIN=$(pwd)/bin go install go.uber.org/mock/mockgen@v0.5.0
GOBIN=$(pwd)/bin go install github.com/google/addlicense@v1.1.1
GOBIN=$(pwd)/bin go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.20
```

把整个 `bin/` 目录拷贝到离线环境，并确保 `PATH` 中包含该目录，或 Makefile 中的 `LOCALBIN` 指向该路径。

### 4. Kustomize

Makefile 通过 curl 下载安装脚本，离线环境需要提前装好：

- 下载：<https://github.com/kubernetes-sigs/kustomize/releases>（版本 v5.0.0）
- 或：`go install sigs.k8s.io/kustomize/kustomize/v5@v5.0.0`，然后把生成的二进制放到 `bin/` 中

离线环境需保证 `kustomize` 在 `PATH` 中，或 `$(LOCALBIN)/kustomize` 存在。

### 5. envtest 二进制（用于单元测试）

在有网环境执行：

```bash
# 需要先有 setup-envtest
KUBEBUILDER_ASSETS="$(bin/setup-envtest use 1.34.0 -p path --bin-dir bin)" echo "done"
```

会下载 envtest 到 `bin/`。把 `bin/` 中相关文件一并拷贝到离线环境。

### 6. crd-upgrader 的 kubectl 下载问题

`deployments/crd-upgrader/Dockerfile` 在构建时通过 `curl` 下载 kubectl，离线构建会失败。

可选做法：

- **方案 A**：在有网环境先构建好 crd-upgrader 镜像，导出并拷贝到离线环境，离线只使用该镜像，不再重新构建。
- **方案 B**：修改 Dockerfile，改为 `COPY` 本地的 kubectl 二进制，而不是 `RUN curl`。需要提前下载对应架构的 kubectl 并放入构建上下文。

### 7. hack/update-client.sh（代码生成）

该脚本会执行 `go mod vendor`，并从 `vendor/k8s.io/code-generator` 生成客户端代码。  
离线环境需要：

- 先在有网环境执行一次 `go mod vendor`，把完整 `vendor/` 目录拷贝到离线环境；
- 离线环境执行 `hack/update-client.sh` 时，确保使用 `-mod=vendor`（例如通过 `GOFLAGS=-mod=vendor`）。

---

## 二、Mac 准备、目标 Linux x64 时的 Docker 注意事项

在 Mac 上准备给 **Linux x64** 用的离线 Docker 资源时，主要要注意**架构**。

### 架构差异

| 你的 Mac | 默认拉取的镜像架构 | Linux x64 需要 |
|----------|--------------------|----------------|
| **Apple Silicon (M1/M2/M3)** | `linux/arm64` | `linux/amd64` |
| **Intel Mac** | `linux/amd64` | `linux/amd64` |

如果是 **Apple Silicon Mac**，不指定平台时拉到的镜像是 arm64，在 Linux x64 上无法直接运行。

### 拉取和保存时指定平台

在 Mac 上准备离线包时，**显式指定 `--platform linux/amd64`**：

```bash
# 拉取 amd64 版本（适配 Linux x64）
docker pull --platform linux/amd64 golang:1.24.4-bullseye
docker pull --platform linux/amd64 registry.access.redhat.com/ubi9/ubi-minimal
docker pull --platform linux/amd64 golang:1.22
docker pull --platform linux/amd64 golangci/golangci-lint:v1.64.8
docker pull --platform linux/amd64 helmunittest/helm-unittest:3.17.2-0.8.1

# 保存（此时保存的是 amd64 镜像）
docker save golang:1.24.4-bullseye -o golang-builder.tar
docker save registry.access.redhat.com/ubi9/ubi-minimal -o ubi9.tar
# ... 其余同理
```

### 构建 builder 镜像

`make builder` 会构建 `builder:1.24.4-bullseye`，默认会按你 Mac 的架构来构建。

在 Mac 上为 Linux x64 准备时，需要指定平台：

```bash
# 构建 amd64 的 builder 镜像
DOCKER_BUILDKIT=1 docker buildx build \
  --platform linux/amd64 \
  -f build/builder/Dockerfile \
  --load \
  -t builder:1.24.4-bullseye \
  .
```

### 构建 KAI 服务镜像

项目 Makefile 里 `DOCKER_BUILD_PLATFORM` 默认是 `linux/$(ARCH)`，在 Mac 上 `ARCH` 可能是 `arm64`。

为 Linux x64 构建时，需要显式指定：

```bash
# 构建 amd64 的 Docker 镜像
make build-go SERVICE_NAME=scheduler
DOCKER_BUILD_PLATFORM=linux/amd64 make docker-build-generic SERVICE_NAME=scheduler
```

### 验证镜像架构

在 Mac 上导出前，确认镜像确实是 amd64：

```bash
docker image inspect golang:1.24.4-bullseye --format '{{.Architecture}}'
# 期望输出: amd64
```

### 在 Linux x64 上加载

在 Linux x64 上：

```bash
docker load -i golang-builder.tar
```

只要 tar 里是 amd64 镜像，就可以正常使用。

### 可选：用 buildx 直接导出 tar

不加载到本地 Docker，直接导出 amd64 镜像：

```bash
docker buildx build --platform linux/amd64 \
  -f build/builder/Dockerfile \
  -o type=docker,dest=./builder-amd64.tar \
  .
```

这样得到的 `builder-amd64.tar` 在 Linux x64 上 `docker load -i builder-amd64.tar` 即可使用。

---

## 三、建议的离线准备脚本（有网环境执行）

```bash
#!/bin/bash
# 在联网环境执行，生成 offline-bundle 目录
# 若在 Mac 上准备给 Linux x64 用，取消下面 PLATFORM 的注释

PLATFORM=""  # 本机架构，直接拉取
# PLATFORM="--platform linux/amd64"  # Mac 准备给 Linux x64 时使用

OFFLINE_DIR=offline-bundle
mkdir -p $OFFLINE_DIR/docker-images $OFFLINE_DIR/bin $OFFLINE_DIR/go-cache

# 1. 保存 Docker 镜像
docker pull $PLATFORM golang:1.24.4-bullseye
docker pull $PLATFORM registry.access.redhat.com/ubi9/ubi-minimal
docker pull $PLATFORM golang:1.22
docker pull $PLATFORM golangci/golangci-lint:v1.64.8
docker pull $PLATFORM helmunittest/helm-unittest:3.17.2-0.8.1

docker save golang:1.24.4-bullseye -o $OFFLINE_DIR/docker-images/golang-builder.tar
docker save registry.access.redhat.com/ubi9/ubi-minimal -o $OFFLINE_DIR/docker-images/ubi9.tar
docker save golang:1.22 -o $OFFLINE_DIR/docker-images/golang-debug.tar
docker save golangci/golangci-lint:v1.64.8 -o $OFFLINE_DIR/docker-images/golangci.tar
docker save helmunittest/helm-unittest:3.17.2-0.8.1 -o $OFFLINE_DIR/docker-images/helm-unittest.tar

# 2. Go vendor
go mod vendor

# 3. Go 工具
GOBIN=$(pwd)/$OFFLINE_DIR/bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.1
GOBIN=$(pwd)/$OFFLINE_DIR/bin go install go.uber.org/mock/mockgen@v0.5.0
GOBIN=$(pwd)/$OFFLINE_DIR/bin go install github.com/google/addlicense@v1.1.1
GOBIN=$(pwd)/$OFFLINE_DIR/bin go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.20

# 4. envtest 资源
$OFFLINE_DIR/bin/setup-envtest use 1.34.0 -p path --bin-dir $OFFLINE_DIR/bin

# 5. kustomize（若用 go install）
GOBIN=$(pwd)/$OFFLINE_DIR/bin go install sigs.k8s.io/kustomize/kustomize/v5@v5.0.0

# 6. 拷贝项目源码（不含 .git 可减小体积）
rsync -a --exclude='.git' --exclude='offline-bundle' . $OFFLINE_DIR/source/
cp -r vendor $OFFLINE_DIR/source/  # 若上面 go mod vendor 在项目根执行
```

---

## 四、离线环境使用时的注意点

| 项目 | 说明 |
|------|------|
| **GOPROXY** | 离线时设为 `GOPROXY=off` 或 `GOPROXY=direct`，并配合 `-mod=vendor` 使用 |
| **Go 构建** | 使用 `GOFLAGS=-mod=vendor make build-go SERVICE_NAME=scheduler` 等 |
| **Docker 构建** | 构建时通过 `-e GOPROXY=off -e GOFLAGS=-mod=vendor` 传入，或确保挂载的目录中已有 vendor |
| **crd-upgrader** | 建议在有网环境预先构建并导出镜像，离线直接 load 使用 |
| **Kubernetes 集群** | 若离线环境也要部署 KAI，需要提前准备集群镜像（如 Kind 的 node 镜像等） |

---

## 五、最小离线开发集合（不跑完整 make test）

如果只做代码修改、构建和基础验证，可以只准备：

1. Docker 镜像：`golang:1.24.4-bullseye`、`registry.access.redhat.com/ubi9/ubi-minimal`
2. `go mod vendor` 后的 `vendor/` 目录
3. `controller-gen`、`mockgen`、`addlicense` 的二进制（放在 `bin/`）
4. 项目源码

这样可以在离线环境执行 `make build`、`make validate`（需保证 kustomize 可用）。  
需要跑 `make test` 时，再额外准备 golangci-lint、helm-unittest 镜像和 envtest 相关文件。
