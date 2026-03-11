# 教程 04：Kubernetes Controller 搬运实战 — 从"盲目复制"到"完全理解"

> **学习目标：** 理解 Kubernetes Controller 的完整组成部分，掌握将 Controller 从一个集群迁移到另一个集群的方法论，以及排查迁移问题的系统化思路。
>
> **适用读者：** K8s 初学者、需要在离线环境搭建 Kubeflow 测试环境的同学。
>
> **前置教程：**
>
> - [01-local-debug-trainjob.md](./01-local-debug-trainjob.md) — 本地断点调试 TrainJob
> - [02-e2e-jupyter-to-local-scheduler.md](./02-e2e-jupyter-to-local-scheduler.md) — 端到端：从 Jupyter 提交到本地调度器
> - [03-rbac-permissions-for-training.md](./03-rbac-permissions-for-training.md) — RBAC 权限体系详解

---

## 目录

1. [真实故事：一次 Controller 搬运的全过程](#1-真实故事一次-controller-搬运的全过程)
2. [三层架构：搬的东西属于哪一层？](#2-三层架构搬的东西属于哪一层)
3. [Controller 模式详解：一个 Controller 的 9 件套](#3-controller-模式详解一个-controller-的-9-件套)
4. [TrainJob 完整生命周期：从提交到执行](#4-trainjob-完整生命周期从提交到执行)
5. [搬运实战：完整清单与操作步骤](#5-搬运实战完整清单与操作步骤)
6. [踩坑实录与排查方法论](#6-踩坑实录与排查方法论)
7. [参考命令速查表](#7-参考命令速查表)

---

## 1. 真实故事：一次 Controller 搬运的全过程

> 这不是编的教材，这是我们把 Kubeflow v2 Controller 从生产搬到测试环境时的真实经历。

### 1.1 背景

为了在本地 IDE 中调试 KAI-Scheduler，我们需要一个能提交 `TrainJob` 的测试集群。但测试集群只有**旧版** Training Operator (v1)，而我们的 SDK 是 v2 的。于是决定：从生产环境把 v2 的 Controller 搬过来。

### 1.2 我们需要搬的两个 Controller

| Controller | 职责 | 原因 |
|---|---|---|
| `kubeflow-trainer-controller-manager` | 监听 `TrainJob`，翻译成 `JobSet` | v2 新增的，测试环境没有 |
| `jobset-controller-manager` | 监听 `JobSet`，翻译成 `Job` → `Pod` | v2 依赖的，测试环境也没有 |

### 1.3 搬运结果（剧透）

> **搬一个 Controller 绝不只是复制一个 Deployment YAML！**

最终我们为**每个** Controller 搬运了以下全部组件：

```
┌─────────────────────────────────────────────────┐
│              一个 Controller 的完整组成             │
├─────────────────────────────────────────────────┤
│  1. CRD (资源类型定义)                             │
│  2. Deployment (Controller 本体)                  │
│  3. Container Image (容器镜像)                    │
│  4. ServiceAccount (身份)                         │
│  5. ClusterRole + ClusterRoleBinding (集群级权限) │
│  6. Role + RoleBinding (命名空间级权限)            │
│  7. Secret (Webhook TLS 证书)                     │
│  8. Service (Webhook 通信端点)                    │
│  9. ConfigMap (运行时配置)                         │
└─────────────────────────────────────────────────┘
```

而且每样东西漏了都会报不同的错——我们全踩了一遍。

---

## 2. 三层架构：搬的东西属于哪一层？

在搬运过程中最大的困惑是：**这些东西到底是 K8s 自带的，还是 Kubeflow 的，还是 KAI-Scheduler 的？** 答案是：它们分属三个不同的层次。

```
┌───────────────────────────────────────────────────────────────────┐
│                                                                   │
│  第三层：KAI-Scheduler（你在二开的部分）                             │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  PodGroup CRD ← 定义"一组 Pod"的概念                        │  │
│  │  Queue CRD    ← 定义调度队列                                │  │
│  │  kai-scheduler     ← 调度器本体，决定任务排到哪               │  │
│  │  PodGrouper        ← 自动把 Pod 打包成 PodGroup             │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  第二层：Kubeflow + JobSet（上层应用）                               │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  TrainJob CRD               ← 用户提交的"训练任务"           │  │
│  │  ClusterTrainingRuntime CRD ← 共享的"运行配方"               │  │
│  │  JobSet CRD                 ← K8s SIG 提供的"任务编排"       │  │
│  │                                                              │  │
│  │  trainer-controller-manager ← 翻译器: TrainJob → JobSet     │  │
│  │  jobset-controller-manager  ← 翻译器: JobSet → Job → Pod   │  │
│  │                                                              │  │
│  │  这些是"应用软件"层面的东西，定义了训练任务怎么跑               │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  第一层：Kubernetes 原生（K8s 自带的"操作系统"）                     │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  运行载体: Pod, Deployment, Job                              │  │
│  │  网络通信: Service                                           │  │
│  │  配置存储: Secret, ConfigMap                                 │  │
│  │  身份权限: ServiceAccount, Role, ClusterRole,               │  │
│  │           RoleBinding, ClusterRoleBinding                   │  │
│  │  扩展机制: CRD (允许注册自定义资源类型)                       │  │
│  │  协调机制: Leases (Leader Election 选主)                     │  │
│  │  安全机制: Webhook (验证/变更准入请求)                        │  │
│  │                                                              │  │
│  │  这些是所有 K8s 集群都自带的基础能力                           │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

### 2.1 每一样搬运物的归属表

下面是我们在迁移中实际搬运的每一个资源，标注了它**属于哪一层**以及**为什么需要搬**：

| 搬运的资源 | 属于哪层 | 为什么需要搬 |
|---|---|---|
| `jobsets.jobset.x-k8s.io` CRD | 第一层 K8s 机制 + 第二层定义 | CRD 是 K8s 的扩展机制，但内容定义了 JobSet 这个第二层概念。没有它，Controller 不认识 `JobSet` 类型 |
| `kubeflow-trainer-controller-manager` Deployment | 第二层 Kubeflow | Controller 本体。没有它就没人翻译 TrainJob |
| `jobset-controller-manager` Deployment | 第二层 JobSet | Controller 本体。没有它就没人翻译 JobSet |
| Container Images (`.tar` 文件) | 第二层 | Deployment 里指定的镜像。离线环境拉不到就启动不了 |
| `kubeflow-trainer-controller-manager` ServiceAccount | 第一层 K8s | Controller Pod 的"工牌"。没有它 Pod 无法认证 |
| `jobset-controller-manager` ServiceAccount | 第一层 K8s | 同上 |
| `kubeflow-trainer-clusterrole-*` ClusterRole | 第一层 K8s | 权限菜单。没有它 Controller 不被允许操作资源 |
| `jobset-manager-role` 等 ClusterRole | 第一层 K8s | 同上 |
| ClusterRoleBinding (多个) | 第一层 K8s | 把权限菜单绑到工牌上。没有它有菜单也用不了 |
| `jobset-leader-election-role` Role | 第一层 K8s | 命名空间内权限，用于 Leader Election |
| `jobset-leader-election-rolebinding` RoleBinding | 第一层 K8s | 把 Leader Election 权限绑到 SA 上 |
| `kubeflow-trainer-config` ConfigMap | 第一层 K8s 机制 + 第二层配置 | Controller 的运行时配置参数 |
| `jobset-manager-config` ConfigMap | 第一层 K8s 机制 + 第二层配置 | 同上 |
| Webhook Secret (TLS 证书) | 第一层 K8s | Controller 的 Webhook 服务需要 TLS 证书 |
| Webhook Service | 第一层 K8s | API Server 通过 Service 找到 Webhook 端点 |

> **核心洞察：** 搬运一个"第二层"的 Controller，其实 80% 的工作都是在搬"第一层"K8s 的配套资源。Controller 本体只是一个 Deployment，但它走不了路——需要第一层的"身份证"(SA)、"许可证"(RBAC)、"设计图"(CRD)、"配置文件"(ConfigMap)、"安全证书"(Secret) 才能正常运转。

---

## 3. Controller 模式详解：一个 Controller 的 9 件套

### 3.1 什么是 Controller？

Controller 是 Kubernetes 世界里的**自动化翻译官**。它不停地做三件事：

```
┌──────────────────────────────────────────────┐
│          Controller 的核心循环（Reconcile）     │
│                                               │
│   ┌──────────┐                                │
│   │  Watch   │  ① 监听: 盯着某种资源的变化      │
│   │  (监听)   │     (比如监听 TrainJob 的创建)  │
│   └────┬─────┘                                │
│        ↓                                      │
│   ┌──────────┐                                │
│   │ Compare  │  ② 比较: 当前状态 vs 期望状态    │
│   │  (比较)   │     (是否已经创建了对应的 Pod?)  │
│   └────┬─────┘                                │
│        ↓                                      │
│   ┌──────────┐                                │
│   │   Act    │  ③ 行动: 让当前状态趋向期望状态   │
│   │  (行动)   │     (如果没有 Pod，就创建它)     │
│   └────┬─────┘                                │
│        │                                      │
│        └──────→ 回到 ① 继续监听                │
└──────────────────────────────────────────────┘
```

### 3.2 为什么 Controller 需要这么多"配套"？

想象一下：你要去一个新公司上班（= 部署到新集群）。光有"你这个人"(Deployment) 是不够的：

| 你需要什么 | Controller 需要什么 | K8s 概念 |
|---|---|---|
| 入职体检证明 | 自己是什么身份 | **ServiceAccount** |
| 工位和电脑 | 运行的容器环境 | **Container Image** |
| 门禁卡（全公司通用） | 集群级别的权限 | **ClusterRole + ClusterRoleBinding** |
| 部门门禁（只能进自己部门） | 命名空间级别的权限 | **Role + RoleBinding** |
| 公司系统账号 | 安全通信证书 | **Secret** (Webhook TLS) |
| 办公电话号码 | 让别人能找到你 | **Service** |
| 工作手册和流程文档 | 运行时配置 | **ConfigMap** |
| 你管辖的项目列表 | 要管理的资源类型定义 | **CRD** |

### 3.3 九件套详解

#### ① CRD — 资源类型注册

**属于：** 第一层 K8s 机制

**是什么：** CRD (Custom Resource Definition) 就是往 K8s 的 API Server "注册"一个新的资源类型。就像在公司 OA 系统里新增了一种"表单"。

**不搬会怎样：** Controller 启动后会疯狂报错 `no matches for kind "JobSet"`，因为它试图监听一种集群根本不认识的资源类型。

```bash
# 查看集群已注册的 CRD
kubectl get crd | grep jobset
# 预期: jobsets.jobset.x-k8s.io

# 导出 CRD
kubectl get crd jobsets.jobset.x-k8s.io -o yaml > jobset-crd.yaml
```

> **注意：** CRD 文件通常非常大（数千行），因为它包含了完整的 OpenAPI Schema。

#### ② Deployment — Controller 本体

**属于：** 第二层 Kubeflow / JobSet

**是什么：** 就是 Controller 的"程序"。一个 Deployment 描述了"运行几个副本、用什么镜像、挂载什么文件"。

```bash
kubectl get deployment -n kubeflow-system
# NAME                                      READY
# kubeflow-trainer-controller-manager        1/1
# jobset-controller-manager                  1/1
```

> **关键排查技巧：** 用以下命令一次性找出 Deployment 依赖的所有 Secret 和 ConfigMap：
> ```bash
> kubectl get deployment <name> -n <ns> -o json | grep -E "configMap|secret"
> ```

#### ③ Container Image — 容器镜像

**属于：** 第二层

**是什么：** Deployment 里指定的 Docker/Containerd 镜像。在离线环境必须手动搬运。

```bash
# 在生产环境导出
ctr -n k8s.io images export trainer-ctrl.tar <镜像全名>

# 在测试环境导入
ctr -n k8s.io images import trainer-ctrl.tar
```

> **为什么不用 `docker`？** 因为现代 K8s (≥1.24) 使用 Containerd 作为容器运行时，不再需要 Docker。详见 [教程 03 的 2.4 节](./03-rbac-permissions-for-training.md#24-什么是容器运行时docker-vs-containerd)。

#### ④ ServiceAccount — 身份

**属于：** 第一层 K8s

**是什么：** Controller Pod 在集群中的"身份证"。API Server 通过它判断"你是谁"。

```bash
kubectl get sa -n kubeflow-system | grep -E "jobset|trainer"
```

#### ⑤ ClusterRole + ClusterRoleBinding — 集群级权限

**属于：** 第一层 K8s

**是什么：** ClusterRole 定义了"能对哪些资源做哪些操作"（权限菜单），ClusterRoleBinding 把这个菜单绑给 ServiceAccount。

**为什么 Controller 需要集群级权限？** 因为 Controller 通常需要跨命名空间监听和操作资源。比如 trainer-controller 需要在任意命名空间创建 Pod。

```bash
# 查看 Controller 的 ClusterRole
kubectl get clusterrole | grep jobset
# jobset-manager-role          ← 主要的操作权限
# jobset-metrics-reader        ← 读取 metrics 的权限
# jobset-proxy-role            ← kube-rbac-proxy 用的权限
```

> **一个 Controller 可能有多个 ClusterRole！** 不要只导出第一个就以为完事了。

#### ⑥ Role + RoleBinding — 命名空间级权限

**属于：** 第一层 K8s

**是什么：** 跟 ClusterRole 类似，但**只在特定命名空间内**生效。

**为什么还需要 Role？** 最典型的用途是 **Leader Election（选主）**。

##### Leader Election 是什么？

当你的 Controller 部署了多个副本时（高可用），不能所有副本都同时干活——会重复处理资源。所以需要一个机制选出"领导"，只让领导干活：

```
┌───────────────────────────────────────────────────┐
│             Leader Election 选主机制                │
│                                                    │
│   Controller 副本 A ──→ 尝试获取 Lease 锁          │
│                          ↓                         │
│                       获取成功! 我是 Leader ✅        │
│                       开始执行 Reconcile 循环        │
│                                                    │
│   Controller 副本 B ──→ 尝试获取 Lease 锁          │
│                          ↓                         │
│                       获取失败! 锁已被 A 占用 ❌      │
│                       待机等待…                      │
│                                                    │
│   如果 A 崩溃 ──→ Lease 过期 ──→ B 抢到锁成为新     │
│                                   Leader           │
└───────────────────────────────────────────────────┘
```

这个 Lease（租约）对象存储在 Controller 所在的命名空间内（如 `kubeflow-system`），所以需要一个 **Role**（命名空间级别权限）来授权访问 `leases` 资源。

```bash
# 查看 Leader Election 用的 Role
kubectl get role -n kubeflow-system | grep -E "jobset|trainer"
# jobset-leader-election-role

kubectl get rolebinding -n kubeflow-system | grep -E "jobset|trainer"
# jobset-leader-election-rolebinding → Role/jobset-leader-election-role
```

**不搬会怎样：** Controller 日志里会疯狂报错：
```
error retrieving resource lock: leases.coordination.k8s.io "xxx" is forbidden:
User "system:serviceaccount:kubeflow-system:jobset-controller-manager"
cannot get resource "leases" in API group "coordination.k8s.io"
```

#### ⑦ Secret — Webhook TLS 证书

**属于：** 第一层 K8s

**是什么：** Controller 通常会运行一个 **Webhook Server**，用来在资源创建/修改时做验证或自动注入。这个 Server 需要 TLS 证书才能跟 API Server 安全通信。

```
┌──────────────────────────────────────────────┐
│            Webhook 工作流程                    │
│                                               │
│  用户创建 TrainJob                             │
│       ↓                                       │
│  API Server 收到请求                           │
│       ↓                                       │
│  API Server: "等等，有个 Webhook 说要审查这个"  │
│       ↓                                       │
│  API Server ──HTTPS──→ Controller Webhook     │
│              需要 TLS!   ↓                    │
│                        "我检查一下格式对不对…"   │
│                        "OK，放行 ✅" 或 "拒绝 ❌"│
│       ↓                                       │
│  API Server 写入 etcd（如果放行）               │
└──────────────────────────────────────────────┘
```

**不搬会怎样：** Pod 卡在 `ContainerCreating`，报错 `MountVolume.SetUp failed: secret "xxx" not found`。

#### ⑧ Service — Webhook 通信端点

**属于：** 第一层 K8s

**是什么：** API Server 需要知道 Webhook 在哪里——通过 Service 的 DNS 名称找到它。

**不搬会怎样：** Webhook 注册了但找不到后端，所有相关 API 调用都会超时或拒绝。

#### ⑨ ConfigMap — 运行时配置

**属于：** 第一层 K8s 机制 + 第二层配置内容

**是什么：** Controller 的运行时参数。比如 trainer-controller 的配置可能指定了默认的运行时名称、日志级别等。

**不搬会怎样：** Pod 卡在 `ContainerCreating`，报错 `MountVolume.SetUp failed: configmap "xxx" not found`。跟 Secret 缺失的报错格式完全一样。

---

## 4. TrainJob 完整生命周期：从提交到执行

当你在 Jupyter Notebook 里运行 `client.train(...)` 时，以下是背后发生的完整故事：

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        TrainJob 完整生命周期                                 │
│                                                                             │
│  ① 用户在 Jupyter 中提交                                                     │
│     client.train(trainer=..., runtime_ref="toy-kai")                        │
│          │                                                                  │
│          ↓ (创建 TrainJob 资源到用户命名空间)                                 │
│                                                                             │
│  ② kubeflow-trainer-controller-manager [第二层 Kubeflow]                    │
│     监听到 TrainJob 被创建                                                   │
│          │                                                                  │
│          │ 翻译: 读取 ClusterTrainingRuntime "toy-kai" 的模板                 │
│          │       根据模板生成 JobSet 规格                                     │
│          ↓                                                                  │
│                                                                             │
│  ③ jobset-controller-manager [第二层 JobSet / K8s SIG]                      │
│     监听到 JobSet 被创建                                                     │
│          │                                                                  │
│          │ 翻译: 根据 JobSet 中定义的 ReplicatedJob                          │
│          │       创建 Job → Job 再创建 Pod                                   │
│          │       (Pod 带着 schedulerName: kai-scheduler)                     │
│          ↓                                                                  │
│                                                                             │
│  ④ PodGrouper [第三层 KAI-Scheduler]                                        │
│     监听到带有 kai-scheduler 标签的 Pod                                      │
│          │                                                                  │
│          │ 打包: 把属于同一个训练任务的多个 Pod 打包成一个 PodGroup            │
│          ↓                                                                  │
│                                                                             │
│  ⑤ kai-scheduler [第三层 KAI-Scheduler]                                     │
│     监听到 PodGroup 和 Pending Pods                                          │
│          │                                                                  │
│          │ 调度: 检查队列配额、节点资源，做调度决策                            │
│          │       把 Pod 绑定到具体节点                                        │
│          ↓                                                                  │
│                                                                             │
│  ⑥ kubelet [第一层 K8s]                                                      │
│     收到绑定通知，启动容器                                                   │
│          │                                                                  │
│          └──→ Pod Running ✅ 训练开始执行                                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 每一次"翻译"的具体变化

| 步骤 | 输入资源 | 输出资源 | 负责的 Controller | 属于哪层 |
|---|---|---|---|---|
| ① → ② | `TrainJob` | `JobSet` | trainer-controller-manager | 第二层 Kubeflow |
| ② → ③ | `JobSet` | `Job` → `Pod` | jobset-controller-manager | 第二层 JobSet |
| ③ → ④ | `Pod` (pending) | `PodGroup` | PodGrouper | 第三层 KAI |
| ④ → ⑤ | `PodGroup` + `Pod` | Pod binding | kai-scheduler | 第三层 KAI |

> **为什么要经过这么多层翻译？这不是过度设计吗？**
>
> 不是！每一层都解耦了不同的关注点：
> - **TrainJob** — 用户只关心"我要训练什么模型"
> - **JobSet** — 编排器只关心"这个任务需要几组 worker"
> - **Job/Pod** — K8s 只关心"要运行什么容器"
> - **PodGroup** — 调度器只关心"这组 Pod 需要同时调度"
>
> 这种设计让每一层都可以独立替换。比如你可以用 KAI-Scheduler 替换掉默认调度器，而不需要修改 Kubeflow 的任何代码。

---

## 5. 搬运实战：完整清单与操作步骤

### 5.1 搬运前：确认依赖

在搬运任何 Controller 之前，先用这条命令列出所有依赖：

```bash
# 找出 Deployment 依赖的 Secret 和 ConfigMap
kubectl get deployment <controller-name> -n <namespace> -o json \
  | grep -E "configMapKeyRef|configMapRef|secretKeyRef|secretRef|configMap|secretName"

# 找出关联的 ServiceAccount
kubectl get deployment <controller-name> -n <namespace> \
  -o jsonpath='{.spec.template.spec.serviceAccountName}'

# 找出 ServiceAccount 关联的所有 RBAC
SA_NAME=$(kubectl get deployment <controller-name> -n <namespace> \
  -o jsonpath='{.spec.template.spec.serviceAccountName}')

# ClusterRoleBinding
kubectl get clusterrolebindings -o json \
  | jq -r ".items[] | select(.subjects[]? | .name == \"$SA_NAME\" and .namespace == \"<namespace>\") | .metadata.name"

# RoleBinding
kubectl get rolebindings -n <namespace> -o json \
  | jq -r ".items[] | select(.subjects[]? | .name == \"$SA_NAME\") | .metadata.name"
```

### 5.2 Complete Checklist: 以 jobset-controller-manager 为例

以下是实际迁移 `jobset-controller-manager` 的完整清单（按推荐的 apply 顺序排列）：

#### 步骤 A: 导出（在生产环境执行）

```bash
# ===== 第 1 步: CRD（必须最先 apply）=====
kubectl get crd jobsets.jobset.x-k8s.io -o yaml > jobset-crd.yaml

# ===== 第 2 步: 容器镜像 =====
# 先确认镜像名称
kubectl get deployment jobset-controller-manager -n kubeflow-system \
  -o jsonpath='{.spec.template.spec.containers[*].image}'
# 导出
ctr -n k8s.io images export jobset-image.tar <镜像全名>

# ===== 第 3 步: RBAC =====
# ServiceAccount
kubectl get sa jobset-controller-manager -n kubeflow-system -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-sa.yaml

# ClusterRoles (可能有多个!)
kubectl get clusterrole jobset-manager-role -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-cr-manager.yaml
kubectl get clusterrole jobset-metrics-reader -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-cr-metrics.yaml
kubectl get clusterrole jobset-proxy-role -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-cr-proxy.yaml

# ClusterRoleBindings
kubectl get clusterrolebinding jobset-manager-rolebinding -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-crb-manager.yaml
kubectl get clusterrolebinding jobset-metrics-reader -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-crb-metrics.yaml
kubectl get clusterrolebinding jobset-proxy-rolebinding -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-crb-proxy.yaml

# Role + RoleBinding (Leader Election!)
kubectl get role jobset-leader-election-role -n kubeflow-system -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-role-leader.yaml
kubectl get rolebinding jobset-leader-election-rolebinding -n kubeflow-system -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-rb-leader.yaml

# ===== 第 4 步: Secret + Service + ConfigMap =====
kubectl get secret jobset-webhook-server-cert -n kubeflow-system -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-secret.yaml
kubectl get configmap jobset-manager-config -n kubeflow-system -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > jobset-configmap.yaml
kubectl get service jobset-controller-manager-metrics-service -n kubeflow-system -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid\|clusterIP\|clusterIPs" > jobset-svc-metrics.yaml
kubectl get service jobset-webhook-service -n kubeflow-system -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid\|clusterIP\|clusterIPs" > jobset-svc-webhook.yaml

# ===== 第 5 步: Deployment =====
kubectl get deployment jobset-controller-manager -n kubeflow-system -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid\|generation\|revision" > jobset-deploy.yaml
```

#### 步骤 B: 传输

```bash
# 使用 scp 或物理方式传输到测试服务器
scp jobset-*.yaml jobset-image.tar root@<测试IP>:/tmp/
```

#### 步骤 C: 导入（在测试环境执行，注意顺序！）

```bash
# ===== 1. 导入镜像 =====
ctr -n k8s.io images import jobset-image.tar

# ===== 2. 确保命名空间存在 =====
kubectl create namespace kubeflow-system --dry-run=client -o yaml | kubectl apply -f -

# ===== 3. 先 apply CRD (大文件需要特殊处理) =====
sed '/uid:/d' jobset-crd.yaml | kubectl apply --server-side -f -

# ===== 4. RBAC (身份 → 权限) =====
kubectl apply -f jobset-sa.yaml
kubectl apply -f jobset-cr-manager.yaml -f jobset-cr-metrics.yaml -f jobset-cr-proxy.yaml
kubectl apply -f jobset-crb-manager.yaml -f jobset-crb-metrics.yaml -f jobset-crb-proxy.yaml
kubectl apply -f jobset-role-leader.yaml
kubectl apply -f jobset-rb-leader.yaml

# ===== 5. Secret + ConfigMap + Service =====
kubectl apply -f jobset-secret.yaml
kubectl apply -f jobset-configmap.yaml
kubectl apply -f jobset-svc-metrics.yaml -f jobset-svc-webhook.yaml

# ===== 6. 最后 apply Deployment =====
kubectl apply -f jobset-deploy.yaml

# ===== 7. 验证 =====
kubectl get pods -n kubeflow-system
# 期望: jobset-controller-manager-xxx  1/1  Running
```

### 5.3 Apply 顺序为什么重要？

```
CRD        ← 必须第一个，否则 Controller 不认识自己要管的资源类型
  ↓
SA         ← Deployment 需要引用 ServiceAccount
  ↓
Role/CR    ← 被 Binding 引用
  ↓
RB/CRB     ← 绑定 SA 和 Role/CR
  ↓
Secret     ← Deployment 的 Volume 引用
ConfigMap  ← Deployment 的 Volume 引用
Service    ← Webhook 配置引用
  ↓
Deployment ← 最后部署，此时所有依赖都就绪
```

如果顺序反了，最常见的现象是 Pod 卡在 `ContainerCreating`，因为 kubelet 找不到要挂载的 Secret 或 ConfigMap。

### 5.4 验证：全链路断点命中

搬运完所有组件、两个 Controller 都 `1/1 Running` 之后，用以下步骤验证迁移成功：

1. **确认 Controller 正常运行：**
   ```bash
   kubectl get pods -n kubeflow-system
   # jobset-controller-manager-xxx           1/1  Running  0
   # kubeflow-trainer-controller-manager-xxx  1/1  Running  0

   kubectl logs deployment/jobset-controller-manager -n kubeflow-system --tail=5
   # 应该看到 "Starting Controller" 和 "Starting workers"，没有 error
   ```

2. **在本地 IDE 启动 KAI-Scheduler（debug 模式）** — 设好断点（详见 [教程 01](./01-local-debug-trainjob.md)）

3. **从 Jupyter 提交 TrainJob** — 详见 [教程 02](./02-e2e-jupyter-to-local-scheduler.md)

4. **观察翻译链：**
   ```bash
   # 查看 TrainJob 是否被创建
   kubectl get trainjob -n kubeflow-user-example-com

   # 查看 JobSet 是否被 trainer-controller 创建
   kubectl get jobset -n kubeflow-user-example-com

   # 查看 Pod 是否被 jobset-controller 创建
   kubectl get pods -n kubeflow-user-example-com
   ```

5. **本地 IDE 断点命中 ✅** — 迁移成功！整条链路完全打通：
   ```
   Jupyter → TrainJob → trainer-controller → JobSet
     → jobset-controller → Job → Pod → KAI-Scheduler (你的 IDE) 🎯
   ```

> **🎉 如果你走到了这一步，恭喜！** 你已经完成了一次完整的 Kubernetes Controller 跨集群迁移，并且理解了每一个组件的作用。这不再是"盲目复制粘贴"——你现在知道搬的每一样东西是什么、属于哪一层、为什么需要搬。

---

## 6. 踩坑实录与排查方法论

### 6.1 `grep -v` 破坏 YAML 结构

**场景：** 导出 Service 时用 `grep -v "clusterIP"` 过滤动态字段。

**问题：** `clusterIP` 的值 `10.x.x.x` 在下一行，`grep -v` 只删了 key 行，留下了孤立的 value：

```yaml
# 期望的结果            # 实际结果（坏的）
spec:                   spec:
  ports:                  10.233.52.171    ← ??? 这是什么
  - name: https             ports:
```

**修复方式：** 用 `sed` 精确删除报错的行：
```bash
sed -i '7d' broken-service.yaml  # 删除第 7 行的孤立 IP
```

**教训：** `grep -v` 是逐行过滤，不理解 YAML 的层级结构。对于嵌套值跨行的字段，要用更智能的工具（如 `yq`）或者直接不过滤。

### 6.2 CRD 太大导致 `kubectl apply` 失败

**场景：** `kubectl apply -f jobset-crd.yaml`

**报错：**
```
metadata.annotations: Too long: may not be more than 262144 bytes
```

**原因：** `kubectl apply` 会在 annotation 里存储一份"上次配置"的副本（`last-applied-configuration`）。CRD 文件有几千行，副本超过了 262KB 限制。

**修复：** 使用 `--server-side` 参数，该模式不存储副本：
```bash
kubectl apply --server-side -f jobset-crd.yaml
```

### 6.3 UID 冲突

**场景：** `kubectl apply --server-side -f jobset-crd.yaml`

**报错：**
```
uid mismatch: the provided object specified uid cb9fe579-...
and no existing object was found
```

**原因：** 从生产 `kubectl get -o yaml` 导出的 YAML 包含了生产集群的 `uid` 字段。新集群里没有这个 UID 对应的对象。

**修复：** 删除 `uid` 行再 apply：
```bash
sed '/uid:/d' jobset-crd.yaml | kubectl apply --server-side -f -
```

### 6.4 ConfigMap 遗漏导致 ContainerCreating

**场景：** 两个 Controller Pod 都卡在 `ContainerCreating`。

**报错：**
```
MountVolume.SetUp failed for volume "manager-config": configmap "jobset-manager-config" not found
MountVolume.SetUp failed for volume "kubeflow-trainer-config": configmap "kubeflow-trainer-config" not found
```

**原因：** 搬运时只关注了 RBAC 和 Secret，忘了 Deployment 还依赖 ConfigMap。

**排查方法：**
```bash
kubectl describe pod <pod-name> -n kubeflow-system | grep -A 3 "Warning"
```

**教训：** 搬运 Deployment 前一定要检查 `volumes` 部分引用的所有外部资源。

### 6.5 Role/RoleBinding 遗漏导致 Leader Election 失败

**场景：** JobSet Controller Pod 显示 Running，但日志里不停报错。

**报错：**
```
error retrieving resource lock: leases.coordination.k8s.io "xxx" is forbidden:
User "system:serviceaccount:kubeflow-system:jobset-controller-manager"
cannot get resource "leases"
```

**原因：** 只搬了 ClusterRole/ClusterRoleBinding，忘了还有命名空间内的 Role/RoleBinding（用于 Leader Election）。

**排查方法：**
```bash
# 在生产环境查找 Controller 的命名空间级权限
kubectl get role -n kubeflow-system | grep jobset
kubectl get rolebinding -n kubeflow-system | grep jobset
```

### 6.6 排查方法论总结

当 Controller Pod 出问题时，按以下顺序排查：

```
Pod 状态异常?
  │
  ├─ ContainerCreating → kubectl describe pod → 看 Events
  │   ├─ "secret xxx not found"    → 补 Secret
  │   ├─ "configmap xxx not found" → 补 ConfigMap
  │   └─ "image xxx not found"     → 补镜像
  │
  ├─ CrashLoopBackOff → kubectl logs → 看日志
  │   ├─ "no matches for kind"     → 补 CRD
  │   └─ "is forbidden"            → 补 RBAC (Role/ClusterRole)
  │
  └─ Running 但功能异常 → kubectl logs → 看日志
      ├─ "cannot get leases"       → 补 Role (Leader Election)
      └─ "connection refused"      → 检查 Service 或 Secret
```

---

## 7. 参考命令速查表

### 搬运前的依赖分析

```bash
# 找出 Deployment 的所有 Secret/ConfigMap 依赖
kubectl get deployment <name> -n <ns> -o json | grep -E "configMap|secret"

# 找出 ServiceAccount
kubectl get deployment <name> -n <ns> -o jsonpath='{.spec.template.spec.serviceAccountName}'

# 找出 SA 关联的 ClusterRoleBinding
kubectl get clusterrolebindings -o json | grep -B 5 "<sa-name>"

# 找出命名空间内的 Role/RoleBinding
kubectl get role -n <ns> | grep <keyword>
kubectl get rolebinding -n <ns> | grep <keyword>
```

### 导出资源（生产环境）

```bash
# 普通资源（清理运行时字段）
kubectl get <type> <name> [-n <ns>] -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" > output.yaml

# Service（额外清理动态 IP）
kubectl get service <name> -n <ns> -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid\|clusterIP\|clusterIPs" > output.yaml

# CRD（不需要清理，太大了用 --server-side apply）
kubectl get crd <name> -o yaml > output.yaml

# 容器镜像
ctr -n k8s.io images export output.tar <image-name>
```

### 导入资源（测试环境）

```bash
# 镜像
ctr -n k8s.io images import input.tar

# CRD（大文件专用）
sed '/uid:/d' crd.yaml | kubectl apply --server-side -f -

# 普通资源
kubectl apply -f resource.yaml

# 验证
kubectl get pods -n <ns>
kubectl logs deployment/<name> -n <ns> --tail=20
```

### 排查命令

```bash
# 查看 Pod 事件（ContainerCreating 排查）
kubectl describe pod <name> -n <ns> | grep -A 3 "Warning\|Error"

# 查看 Controller 日志
kubectl logs deployment/<name> -n <ns> --tail=20

# 测试权限
kubectl auth can-i <verb> <resource> \
  --as=system:serviceaccount:<ns>:<sa-name>
```

---

## 附录：本教程涉及的所有搬运文件清单

### kubeflow-trainer-controller-manager

| 文件 | 资源类型 | 必要性 |
|---|---|---|
| `trainer-ctrl-sa.yaml` | ServiceAccount | ✅ 必须 |
| `trainer-ctrl-cr.yaml` | ClusterRole | ✅ 必须 |
| `trainer-ctrl-crb.yaml` | ClusterRoleBinding | ✅ 必须 |
| `trainer-ctrl-secret.yaml` | Secret (Webhook TLS) | ✅ 必须 |
| `trainer-ctrl-svc.yaml` | Service (Webhook) | ✅ 必须 |
| `trainer-configmap.yaml` | ConfigMap | ✅ 必须 |
| `trainer-controller-deploy.yaml` | Deployment | ✅ 必须 |
| `trainer-ctrl-image.tar` | Container Image | ✅ 必须 (离线环境) |

### jobset-controller-manager

| 文件 | 资源类型 | 必要性 |
|---|---|---|
| `jobset-crd.yaml` | CRD | ✅ 必须 |
| `jobset-ctrl-sa.yaml` | ServiceAccount | ✅ 必须 |
| `jobset-cr-manager.yaml` | ClusterRole | ✅ 必须 |
| `jobset-cr-metrics.yaml` | ClusterRole | ✅ 必须 |
| `jobset-cr-proxy.yaml` | ClusterRole | ✅ 必须 |
| `jobset-crb-manager.yaml` | ClusterRoleBinding | ✅ 必须 |
| `jobset-crb-metrics.yaml` | ClusterRoleBinding | ✅ 必须 |
| `jobset-crb-proxy.yaml` | ClusterRoleBinding | ✅ 必须 |
| `jobset-role-leader.yaml` | Role (Leader Election) | ✅ 必须 |
| `jobset-rb-leader.yaml` | RoleBinding (Leader Election) | ✅ 必须 |
| `jobset-ctrl-secret.yaml` | Secret (Webhook TLS) | ✅ 必须 |
| `jobset-configmap.yaml` | ConfigMap | ✅ 必须 |
| `jobset-svc-metrics.yaml` | Service (Metrics) | ✅ 必须 |
| `jobset-svc-webhook.yaml` | Service (Webhook) | ✅ 必须 |
| `jobset-controller-deploy.yaml` | Deployment | ✅ 必须 |
| `jobset-ctrl-image.tar` | Container Image | ✅ 必须 (离线环境) |
