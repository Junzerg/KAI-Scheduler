# 中国大陆网络环境：需替换或配置的网站与镜像源

在中国大陆构建和运行 KAI Scheduler 时，以下站点可能访问缓慢或被限制，需要替换为国内镜像或提前配置代理。

---

## 一、Go 模块代理 (GOPROXY)

**原始**：`proxy.golang.org`、`sum.golang.org`（国内访问不稳定）

**替换为**（任选其一）：

```bash
# 七牛云 goproxy.cn（推荐，会代理 sum.golang.org）
export GOPROXY=https://goproxy.cn,direct

# 阿里云
export GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

# 腾讯云
export GOPROXY=https://mirrors.tencent.com/go/,direct
```

goproxy.cn 会代理校验数据库，一般无需单独设置 `GOSUMDB`。若校验失败，可尝试 `export GOSUMDB=off`（会跳过校验，仅建议在可信环境使用）。

**生效方式**：

- 构建时：`GOPROXY=https://goproxy.cn,direct make build`
- 或写入 `~/.bashrc` / `~/.zshrc` 持久生效
- Docker 构建时：在 `build/makefile/golang.mk` 的 `DOCKER_GO_CACHING_VOLUME_AND_ENV` 中已支持 `GOPROXY` 环境变量，构建前设置即可

---

## 二、Docker 镜像源

### 2.1 项目用到的原始镜像

| 原始地址 | 用途 | 国内情况 |
|----------|------|----------|
| `docker.io/library/golang:*` | 构建 | Docker Hub 慢 |
| `registry.access.redhat.com/ubi9/ubi-minimal` | 运行时 | Red Hat 可能慢 |
| `ghcr.io/*` | Helm OCI、CI | GitHub 可能不稳定 |
| `gcr.io/*` | 测试镜像 | 需代理 |
| `registry.k8s.io/*` | LWS 等 | 可能慢 |

### 2.2 Docker Daemon 镜像加速

编辑 `/etc/docker/daemon.json`（Linux）或 Docker Desktop 设置：

```json
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com",
    "https://mirror.ccs.tencentyun.com"
  ]
}
```

重启 Docker 后，拉取 `docker.io` 的镜像会走镜像站。

**注意**：`ghcr.io`、`gcr.io`、`registry.access.redhat.com` 等非 Docker Hub 的 registry 通常**不会**被上述镜像加速，需要单独处理（见下文）。

### 2.3 替代方案

- **ghcr.io**：可尝试配置 HTTP 代理，或使用 `--local-images-build` 本地构建后 `kind load`，避免在线拉取
- **registry.access.redhat.com**：可预先在有代理环境拉取并导出，再导入目标环境
- **gcr.io**：测试用镜像，可改为使用国内可访问的镜像，或提前导出导入

---

## 三、构建时下载的 URL

### 3.1 Kustomize 安装脚本

**位置**：`Makefile` 第 103–107 行

**原始**：`https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh`

**替换**：

- 将脚本下载到本地，或从国内可访问的镜像获取
- 或直接安装二进制，跳过 curl：
  ```bash
  # 使用 Go 安装（走 GOPROXY）
  GOBIN=$(pwd)/bin go install sigs.k8s.io/kustomize/kustomize/v5@v5.0.0
  ```
- 或设置变量覆盖：
  ```bash
  export KUSTOMIZE_INSTALL_SCRIPT="https://your-mirror/install_kustomize.sh"
  ```

### 3.2 kubectl 下载（crd-upgrader）

**位置**：`deployments/crd-upgrader/Dockerfile`

**原始**：

- `https://dl.k8s.io/release/stable.txt`
- `https://dl.k8s.io/release/.../bin/linux/$TARGETARCH/kubectl`

**国内镜像**（需自行验证可用性）：

- 阿里云：`https://mirrors.aliyun.com/kubernetes/` 主要提供 yum/apt 包，二进制结构与 dl.k8s.io 可能不同，需查看其文档
- 可尝试：`https://mirrors.aliyun.com/kubernetes/` 或 `https://mirrors.tuna.tsinghua.edu.cn/kubernetes/` 等镜像站
- 稳妥做法：在有代理环境下载 kubectl，通过 `COPY` 放入构建上下文

**替换方式**：修改 Dockerfile，将 `RUN curl ...` 改为从上述镜像下载，或 `COPY` 本地已下载的 kubectl。

---

## 四、apt 源（builder 镜像构建）

**位置**：`build/builder/Dockerfile`

**原始**：Debian 默认 `deb.debian.org`

**替换**：在 `apt-get update` 前添加国内源，例如：

```dockerfile
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update && apt-get install -y \
    g++-x86-64-linux-gnu \
    ...
```

或使用 `mirrors.ustc.edu.cn`、`mirrors.tuna.tsinghua.edu.cn` 等。

---

## 五、Helm OCI 仓库

**原始**：

- `oci://ghcr.io/nvidia/kai-scheduler/kai-scheduler`
- `oci://ghcr.io/run-ai/fake-gpu-operator/fake-gpu-operator`
- `oci://registry.k8s.io/lws/charts/lws`

**说明**：Helm OCI 直接连容器仓库，一般不走 Docker 镜像加速。

**替换**：

- 使用 `--local-images-build` 本地构建并安装，避免从 ghcr.io 拉 chart 和镜像
- 或将 chart 和镜像预先导出，在目标环境导入后使用本地 tgz / 本地镜像

---

## 六、Kind 集群镜像

**原始**：`kindest/node:v1.34.0`（来自 Docker Hub）

**替换**：若已配置 Docker 镜像加速，拉取会走镜像站；否则可指定国内镜像，例如：

```bash
# 若某镜像站提供 kindest/node 的同步
KIND_IMAGE=your-mirror/kindest/node:v1.34.0 kind create cluster ...
```

需确认该镜像站是否同步了 `kindest/node`。

---

## 七、快速检查清单

| 类别 | 配置项 | 建议 |
|------|--------|------|
| Go | `GOPROXY` | `https://goproxy.cn,direct` 或阿里云 |
| Go | `GOSUMDB` | `sum.golang.google.cn`（可选） |
| Docker | `registry-mirrors` | 配置 Docker Hub 加速 |
| Kustomize | 安装方式 | 用 `go install` 或本地脚本 |
| kubectl | crd-upgrader | 修改 Dockerfile 使用国内镜像或 COPY |
| apt | builder | 使用阿里云/清华/中科大 Debian 源 |
| Helm/镜像 | 安装方式 | 优先本地构建 + `kind load` |

---

## 八、最小改动方案（不修改代码）

若暂时不想改 Dockerfile 或 Makefile，可优先做：

1. **设置 GOPROXY**：`export GOPROXY=https://goproxy.cn,direct`
2. **配置 Docker 镜像加速**：编辑 `daemon.json` 添加 `registry-mirrors`
3. **使用本地构建**：`make build` 后 `helm upgrade -i ... ./charts/kai-scheduler-0.0.0.tgz`，避免从 ghcr.io 拉 chart
4. **crd-upgrader**：在有代理环境构建好镜像并导出，在目标环境 `docker load` 使用

若仍有拉取失败，再按上文逐项替换 URL 或使用离线包（参见 [offline-bundle.md](./offline-bundle.md)）。
