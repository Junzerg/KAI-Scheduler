# Kubernetes / Helm 常用命令与参数说明

面向不熟悉 K8s 的开发者：本文档解释在本项目二开、调试过程中会用到的 K8s/Helm 命令及参数含义。

---

## 一、核心概念（先建立直觉）

| 概念 | 简单理解 |
|------|----------|
| **Cluster** | 一整套 K8s：多台机器（或 Docker 里的多节点）组成一个集群，由 API Server 统一管理。 |
| **Node** | 集群里的一台“工作机”，上面会跑 Pod。`kubectl get nodes` 看的就是这些。 |
| **Namespace** | 命名空间，把资源“分文件夹”放。例如 `kai-scheduler`、`default`、`kube-system`。 |
| **Pod** | 最小调度单元，通常一个 Pod 里跑一个或多个容器。你看到的 “Running” 一般是 Pod 状态。 |
| **Deployment** | 声明“要跑多少份、用什么镜像”的控制器，会帮你维持指定数量的 Pod。 |
| **CRD** | 自定义资源类型。例如 KAI 的 Queue、PodGroup、SchedulingShard，都是 CRD。先有 CRD，才能创建这种资源。 |
| **Helm** | K8s 的“安装包工具”。一个 Chart = 一套 YAML + 默认配置，`helm install` 会往集群里创建这些资源。 |

---

## 二、kubectl 命令与参数

### 1. 查看：`kubectl get`

**作用**：列出某种资源（节点、Pod、Namespace 等）。

```bash
kubectl get nodes
kubectl get pods -n kai-scheduler
kubectl get ns
```

| 参数 / 写法 | 含义 |
|-------------|------|
| `nodes` | 资源类型：节点。 |
| `pods` | 资源类型：Pod。 |
| `ns` 或 `namespaces` | 资源类型：命名空间。 |
| `-n <命名空间>` 或 `--namespace=<命名空间>` | 只查这个命名空间里的资源。不写则默认用 `default`。 |
| `-A` 或 `--all-namespaces` | 查**所有**命名空间下的该资源。例如 `kubectl get pods -A`。 |
| `-o wide` | 输出更多列（如节点名、IP）。例如 `kubectl get pods -n kai-scheduler -o wide`。 |
| `-o yaml` / `-o json` | 以 YAML 或 JSON 形式输出资源内容，便于排查或复制。 |
| `-w` 或 `--watch` | 持续刷新，不退出。例如 `kubectl get pods -n kai-scheduler -w`。 |
| `-l <标签选择器>` | 只显示带某标签的资源。例如 `-l app.kubernetes.io/name=kai-scheduler`。 |

**常见资源类型简写**：`po`(pods)、`deploy`(deployments)、`svc`(services)、`ns`(namespaces)、`crd`(customresourcedefinitions)。

---

### 2. 用文件创建/更新资源：`kubectl apply -f`

**作用**：根据 YAML/JSON 文件里的声明，在集群里“创建或更新”资源（声明式）。

```bash
kubectl apply -f docs/quickstart/pods/cpu-only-pod.yaml
kubectl apply -f deployments/kai-scheduler/crds/
```

| 参数 | 含义 |
|------|------|
| `-f <文件或目录>` 或 `--filename=` | 从该文件或该目录下所有 YAML 文件读取资源定义。目录会递归处理。 |
| `-n <命名空间>` | 若 YAML 里没写 `metadata.namespace`，用这个命名空间。 |
| `--server-side` | 用服务端应用（Server-Side Apply），不写大的 `last-applied-configuration` annotation，适合很大的资源（如某些 CRD）。 |
| `--dry-run=client -o yaml` | 只生成要提交的 YAML，不真正发到集群，用来检查或导出。 |

---

### 3. 看日志：`kubectl logs`

**作用**：看某个 Pod 里容器的标准输出/错误，用于调试。

```bash
kubectl logs -n kai-scheduler kai-operator-866599df4d-b4h72
kubectl logs -n kai-scheduler -l app.kubernetes.io/name=kai-scheduler -f
```

| 参数 | 含义 |
|------|------|
| `-n <命名空间>` | Pod 所在命名空间。 |
| `<Pod 名>` | 要看的 Pod 名称（可用 `kubectl get pods -n <ns>` 查）。 |
| `-l <标签选择器>` | 不写 Pod 名时，选一组 Pod（例如按 app 标签），会看其中一个。 |
| `-f` 或 `--follow` | 持续跟踪输出，类似 `tail -f`。 |
| `--tail=N` | 只显示最后 N 行。 |
| `-c <容器名>` | Pod 里有多容器时，指定看哪个容器。 |

---

### 4. 查看资源详情：`kubectl describe`

**作用**：看某个资源的详细状态、事件（Events），排查为什么 Pod 没起来等。

```bash
kubectl describe pod -n kai-scheduler kai-operator-866599df4d-b4h72
kubectl describe node desktop-worker
```

| 参数 | 含义 |
|------|------|
| `<资源类型> <资源名>` | 例如 `pod <名字>`、`node <名字>`。 |
| `-n <命名空间>` | 对 namespaced 资源（如 pod）必须指定命名空间（除非在 default）。 |

---

### 5. 删除资源：`kubectl delete`

**作用**：按名字或按文件删除资源。

```bash
kubectl delete pod -n kai-scheduler kai-operator-866599df4d-b4h72
kubectl delete -f docs/quickstart/pods/cpu-only-pod.yaml
```

| 参数 | 含义 |
|------|------|
| `<资源类型> <资源名>` | 删除指定资源。 |
| `-f <文件>` | 删除文件里声明的资源。 |
| `-n <命名空间>` | 指定命名空间。 |
| `--all -n <ns>` | 删除该命名空间下该类型的所有资源（慎用）。 |

---

### 6. 配置与上下文：`kubectl config`

**作用**：管理 kubeconfig（连接哪个集群、用哪个用户、当前 context）。

```bash
kubectl config get-contexts
kubectl config use-context docker-desktop
kubectl config current-context
kubectl config view
```

| 子命令 / 参数 | 含义 |
|---------------|------|
| `get-contexts` | 列出所有 context（每个 context = 一个集群 + 用户 组合）。`*` 表示当前。 |
| `use-context <名称>` | 切换当前使用的 context，后续 kubectl 都针对该集群。 |
| `current-context` | 显示当前 context 名称。 |
| `view` | 以 YAML 形式显示 kubeconfig 内容。 |
| `set-cluster` / `set-credentials` | 修改 kubeconfig 里某个 cluster 或 user 的配置（例如改 server 地址）。 |

---

### 7. 等待条件：`kubectl wait`

**作用**：等某资源达到某条件再返回，常用于脚本里“等 CRD/ Pod 就绪再继续”。

```bash
kubectl wait --for=condition=Established crd/queues.scheduling.run.ai --timeout=60s
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=kai-scheduler -n kai-scheduler --timeout=300s
```

| 参数 | 含义 |
|------|------|
| `--for=condition=<条件>` | 例如 `Established`（CRD 已建立）、`Ready`（Pod 就绪）。 |
| `crd/<crd 名>` 或 `pod -l <标签>` | 要等的资源。 |
| `-n <命名空间>` | 对 Pod 等需要指定命名空间。 |
| `--timeout=` | 最多等多久，超时则命令失败。 |

---

## 三、Helm 命令与参数

### 1. 安装/升级 Release：`helm upgrade -i`

**作用**：安装或升级一个“Release”（一次 Helm 部署的名字）。`-i` 表示若不存在就安装，存在就升级。

```bash
helm upgrade -i kai-scheduler ./kai-scheduler -n kai-scheduler --create-namespace --skip-crds
```

| 参数 | 含义 |
|------|------|
| `upgrade` | 升级（或第一次安装时等价于 install）。 |
| `-i` 或 `--install` | 若 Release 不存在则先安装，存在则升级。 |
| `<Release 名>` | 例如 `kai-scheduler`，以后用这个名字做 upgrade/uninstall。 |
| `<Chart 路径或 OCI 地址>` | 本地目录如 `./kai-scheduler`，或 OCI 如 `oci://ghcr.io/nvidia/kai-scheduler/kai-scheduler`。 |
| `-n <命名空间>` 或 `--namespace=` | 资源安装到哪个命名空间。 |
| `--create-namespace` | 若该命名空间不存在则自动创建。 |
| `--version <版本号>` | 从 OCI/仓库拉指定版本的 Chart（例如 `--version v0.12.10`）。 |
| `--skip-crds` | 不安装/升级 Chart 里的 CRD，适合 CRD 已单独用 kubectl apply 装好的情况。 |
| `--set key=value` | 覆盖 values 里某一项。可多次写。例如 `--set global.tag=dev`。 |
| `-f <values 文件>` | 用额外 values 文件覆盖默认 values.yaml。 |

---

### 2. 拉取 Chart：`helm pull`

**作用**：从 OCI 或仓库下载 Chart 包（tgz），或解压到目录。

```bash
helm pull oci://ghcr.io/nvidia/kai-scheduler/kai-scheduler --version v0.12.10 --untar
```

| 参数 | 含义 |
|------|------|
| `oci://<仓库>/<项目>/<Chart 名>` | OCI 镜像地址。 |
| `--version <版本>` | 指定版本，不写则默认最新。 |
| `--untar` | 下载后解压成目录（目录名一般为 Chart 名），便于查看或本地 `helm upgrade -i ... ./目录`。 |
| 不写 `--untar` | 只下载一个 `.tgz` 文件。 |

---

### 3. 查看 Release / 历史：`helm list`、`helm history`

```bash
helm list -n kai-scheduler
helm history kai-scheduler -n kai-scheduler
```

| 参数 | 含义 |
|------|------|
| `list` | 列出该命名空间下的 Helm Release。 |
| `-n <命名空间>` | 指定命名空间。 |
| `-A` | 所有命名空间。 |
| `history <Release 名>` | 该 Release 的每次 upgrade 历史（REVISION、STATUS 等）。 |

---

### 4. 卸载：`helm uninstall`

```bash
helm uninstall kai-scheduler -n kai-scheduler
```

| 参数 | 含义 |
|------|------|
| `<Release 名>` | 要删的 Helm Release。 |
| `-n <命名空间>` | Release 所在的命名空间。 |
| `--keep-history` | 保留历史记录，便于以后 rollback。 |

---

## 四、本项目中常见命令速查

| 目的 | 命令 |
|------|------|
| 看集群节点 | `kubectl get nodes` |
| 看所有命名空间 | `kubectl get ns` |
| 看 KAI 的 Pod | `kubectl get pods -n kai-scheduler` |
| 看 KAI 的 Pod（持续刷新） | `kubectl get pods -n kai-scheduler -w` |
| 看某 Pod 日志 | `kubectl logs -n kai-scheduler <Pod 名> -f` |
| 看调度器相关日志 | `kubectl logs -n kai-scheduler -l app.kubernetes.io/name=kai-scheduler -f` |
| 提交示例 Pod | `kubectl apply -f docs/quickstart/pods/cpu-only-pod.yaml` |
| 看默认队列 | `kubectl get queue -A` 或 `kubectl get -f docs/quickstart/default-queues.yaml` |
| 当前连的集群 | `kubectl config current-context` |
| 切换集群/context | `kubectl config use-context docker-desktop` |

---

## 五、环境变量说明（你当前 WSL 配置）

你在 `~/.bashrc` 里配置了：

- **KUBECONFIG**：指定 kubeconfig 文件路径（Windows 下的 `C:\Users\Newera\.kube\config`），这样在 WSL 里用的“kubectl”会通过 Windows 的 kubectl.exe 连到 Docker Desktop 的集群，无需端口转发。
- **kubectl 函数**：实际执行的是 Windows 的 `kubectl.exe`，所以 127.0.0.1 指向的是 Windows 本机，能连上 Docker Desktop 的 API。

以上命令在“二开、调试、测试”场景下足够用；更完整的 K8s 概念可参考 [Kubernetes 官方文档](https://kubernetes.io/docs/home/)。
