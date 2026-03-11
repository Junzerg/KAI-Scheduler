# 教程 03：Kubeflow Training + KAI-Scheduler 的 RBAC 权限体系详解

> **学习目标：** 深入理解从 Kubeflow Jupyter Notebook 提交训练任务到 KAI-Scheduler 调度的全链路中，每一层 RBAC（基于角色的访问控制）权限是如何生效的。掌握如何从生产环境安全地迁移权限配置到测试环境。
>
> **适用读者：** K8s 初学者、刚接手 KAI-Scheduler 二开任务的同学。
>
> **前置教程：**
>
> - [01-local-debug-trainjob.md](./01-local-debug-trainjob.md) — 本地断点调试 TrainJob
> - [02-e2e-jupyter-to-local-scheduler.md](./02-e2e-jupyter-to-local-scheduler.md) — 端到端：从 Jupyter 提交到本地调度器

---

## 目录

1. [真实故事：一次权限排查的全过程](#1-真实故事一次权限排查的全过程)
2. [K8s 基础概念：给完全不懂的人](#2-k8s-基础概念给完全不懂的人)
3. [Kubernetes RBAC 核心概念详解](#3-kubernetes-rbac-核心概念详解)
4. [生产环境权限链路全分析](#4-生产环境权限链路全分析)
5. [权限拓扑图：完整链路](#5-权限拓扑图从-jupyter-提交训练到-kai-scheduler-调度)
6. [从生产环境迁移权限到测试环境](#6-从生产环境迁移权限到测试环境)
7. [实战中踩过的其他坑](#7-实战中踩过的其他坑)
8. [常见问题排查](#8-常见问题排查)
9. [KAI-Scheduler 整合涉及的所有权限资源清单](#9-kai-scheduler-整合涉及的所有权限资源清单)
10. [参考命令速查表](#10-参考命令速查表)

---

## 1. 真实故事：一次权限排查的全过程

> 这不是编的教材，这是我们实际部署 KAI-Scheduler 测试环境时亲身经历的排查过程。

### 1.1 目标

在离线测试集群中，通过 Kubeflow Jupyter Notebook 提交一个鸢尾花训练任务（TrainJob），让本地 IDE 中运行的 KAI-Scheduler 拦截并调度该任务。

### 1.2 报错现场

在 Jupyter Notebook 中执行以下代码：

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

client = TrainerClient()
runtime = client.get_runtime("toy-kai")  # ← 这里爆炸了
```

报错信息：

```
RuntimeError: Failed to get clustertrainingruntimes: toy-kai
```

### 1.3 直觉 vs 真相

| 🤔 第一直觉                                               | ✅ 真相                                                                            |
| --------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| "是不是 `toy-kai` 这个 Runtime 没部署？"                  | 不是！在终端 `kubectl get clustertrainingruntimes` 可以看到 `toy-kai` 好好地存在着 |
| "是不是 SDK 版本不对？"                                   | 不是！SDK 安装成功，能正常 import                                                  |
| "是不是网络隔离？"                                        | 不是！Jupyter Pod 跟 API Server 在同一个集群里                                     |
| **"是不是这个 Jupyter 的身份没有权限读集群级别的资源？"** | **✅ 就是这个！**                                                                  |

### 1.4 排查路径

我们在生产环境（可以正常运行的同一套系统）中做了如下精确定位：

```bash
# 第 1 步: 确认 Jupyter Pod 用的身份
kubectl get serviceaccounts -n kubeflow-user-example-com
# 发现: default-editor (这就是 Jupyter 的 Pod 使用的 ServiceAccount)

# 第 2 步: 查看全集群谁给过它权限
kubectl get clusterrolebindings -o json | grep -B 5 "kubeflow-user-example-com"
# 发现: 有一个 ClusterRoleBinding 把 ClusterRole "kubeflow-trainer-user" 授给了它

# 第 3 步: 看看这个 ClusterRole 里到底写了什么权限
kubectl get clusterrole kubeflow-trainer-user -o yaml
# 发现: 授权了对 clustertrainingruntimes 的 get/list/watch 权限

# 第 4 步: 测试集群里也存在同名的 ServiceAccount，但唯独缺少这个 ClusterRoleBinding！
# 根因确认！
```

### 1.5 修复

把生产环境里的这对 ClusterRole + ClusterRoleBinding 导出，搬运到测试环境 `kubectl apply` 即可。无需重启任何组件，权限实时生效，Jupyter 立刻就能正常提交任务了。

---

## 2. K8s 基础概念：给完全不懂的人

> 如果你已经熟悉 K8s，可以跳过本节。

### 2.1 什么是命名空间（Namespace）？

把 Kubernetes 集群想象成一栋写字楼。**命名空间**就是楼里的一间间独立办公室。

```
┌────────────── Kubernetes 集群（大楼）──────────────┐
│                                                     │
│  ┌─────────────────┐  ┌──────────────────────────┐  │
│  │   kubeflow       │  │  kubeflow-user-example   │  │
│  │   (系统办公室)    │  │  (用户专属办公室)          │  │
│  │                  │  │                           │  │
│  │  training-       │  │  Jupyter Notebook Pod     │  │
│  │  operator Pod    │  │  TrainJob                 │  │
│  │                  │  │                           │  │
│  └─────────────────┘  └──────────────────────────┘  │
│                                                     │
│  ┌──────────────────┐  ┌─────────────────────────┐  │
│  │  kai-scheduler    │  │  kube-system            │  │
│  │  (调度器办公室)   │  │  (大楼物业)              │  │
│  │                  │  │                           │  │
│  │  scheduler Pod   │  │  coredns, etcd, etc.     │  │
│  │  pod-grouper     │  │                           │  │
│  └──────────────────┘  └─────────────────────────┘  │
│                                                     │
│  ※ 大楼公告栏（集群级别资源，不属于任何房间）：         │
│     ClusterTrainingRuntime, ClusterRole, Queue ...   │
└─────────────────────────────────────────────────────┘
```

- **命名空间内资源（Namespace-scoped）：** Pod、TrainJob、Service、Secret → 像是放在办公室里的文件，只有进入这间办公室才能拿到
- **集群级别资源（Cluster-scoped）：** ClusterTrainingRuntime、ClusterRole、Node、Queue → 像是贴在大楼公告栏上的通知，不属于任何一间办公室

### 2.2 什么是 ServiceAccount？

每个 Pod 启动时，K8s 都会给它发一张"工牌"，这就是 **ServiceAccount**。

```
┌──────────────────────────────┐
│  Jupyter Notebook Pod        │
│                              │
│  工牌: default-editor        │
│  所在办公室: kubeflow-user-* │
│                              │
│  当它去 API Server 办事时：  │
│  "你好，我是 kubeflow-user-  │
│   example-com 的             │
│   default-editor，我要查     │
│   公告栏上的 Runtime 信息"   │
│                              │
│  API Server: "让我查查你有   │
│   没有这个权限……"             │
└──────────────────────────────┘
```

### 2.3 什么是 CRD（Custom Resource Definition）？

K8s 原生只认识 Pod、Service、Deployment 等资源。但你可以通过 CRD 教它认识新的资源类型。

| CRD 注册的资源类型       | 谁注册的                   | 干什么的                           |
| ------------------------ | -------------------------- | ---------------------------------- |
| `TrainJob`               | Kubeflow Training Operator | 代表一次训练任务                   |
| `ClusterTrainingRuntime` | Kubeflow Training Operator | 训练任务的运行时模板（共享的配方） |
| `PodGroup`               | KAI-Scheduler              | 把多个 Pod 打包成一个调度组        |
| `Queue`                  | KAI-Scheduler              | 调度队列，控制资源分配             |

> **重要：** CRD 只是在集群里"注册"了资源类型的定义（相当于登记表的表头），但光有 CRD 不够。你还需要有一个 **Controller（控制器）** 在后台运行来监听和处理这些资源。例如：
>
> - `TrainJob` CRD + **Training Operator** Controller → 才能让训练任务真正跑起来
> - `PodGroup` CRD + **PodGrouper** Controller → 才能自动打包 Pod

### 2.4 什么是容器运行时？Docker vs Containerd

你可能习惯了 `docker` 命令，但现代 K8s（≥1.24）已经弃用了 Dockershim。

```
旧时代（K8s < 1.24）:
  kubelet → Dockershim → Docker Engine → 容器

新时代（K8s ≥ 1.24）:
  kubelet → CRI → Containerd → 容器
```

这就是为什么我们在导出镜像时 `docker save` 会报错 `Cannot connect to Docker daemon`——因为根本就没有 Docker 在跑！

| 操作         | Docker 命令                  | Containerd 命令                                |
| ------------ | ---------------------------- | ---------------------------------------------- |
| 查看镜像列表 | `docker images`              | `crictl images` 或 `ctr -n k8s.io images list` |
| 导出镜像     | `docker save -o x.tar image` | `ctr -n k8s.io images export x.tar image`      |
| 导入镜像     | `docker load -i x.tar`       | `ctr -n k8s.io images import x.tar`            |

> **踩坑经验：** 如果机器上仍然装着 `docker` 客户端，敲 `docker` 命令不会报"command not found"，而是报"Cannot connect to Docker daemon"。这非常容易让人误判，以为是 Docker 服务挂了。其实你根本不需要启动 Docker——底层用的压根就不是它。

---

## 3. Kubernetes RBAC 核心概念详解

### 3.1 RBAC 四件套

```
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes RBAC 四件套                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. ClusterRole / Role                                          │
│     "权限菜单" - 定义了能对哪些资源做哪些操作                       │
│     · ClusterRole = 全集群通用的菜单                              │
│     · Role = 仅在某个命名空间内有效的菜单                          │
│                                                                 │
│  2. ClusterRoleBinding / RoleBinding                            │
│     "授权凭证" - 把某个菜单授权给某个账号                          │
│     · ClusterRoleBinding = 全集群生效的授权                       │
│     · RoleBinding = 仅在某个命名空间生效的授权                     │
│                                                                 │
│  3. ServiceAccount                                              │
│     "身份证/工牌" - Pod 在集群中的身份标识                         │
│     · 每个 Pod 启动时会自动挂载一个 ServiceAccount 的令牌          │
│     · Jupyter Notebook Pod 通常使用 `default-editor` 这个身份     │
│                                                                 │
│  组合公式:                                                       │
│  ServiceAccount + (Cluster)RoleBinding + (Cluster)Role          │
│  = "谁" + "被授权了" + "能做什么"                                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 ClusterRole vs Role，ClusterRoleBinding vs RoleBinding 的关键区别

这是最容易搞混的地方，用一张表说清楚：

| 组合方式                                 | 效果                                         | 典型场景                              |
| ---------------------------------------- | -------------------------------------------- | ------------------------------------- |
| **ClusterRole** + **ClusterRoleBinding** | 对**全集群所有资源**生效                     | 读取集群级别的 ClusterTrainingRuntime |
| **ClusterRole** + **RoleBinding**        | ClusterRole 的权限**只在某个命名空间内**生效 | 用户只能在自己命名空间内创建 TrainJob |
| **Role** + **RoleBinding**               | 对**某个命名空间内的资源**生效               | 最常规的用法                          |
| **Role** + **ClusterRoleBinding**        | ❌ **非法组合！** 不允许                     | —                                     |

> **核心记忆点：**
>
> - 要访问**集群级别**资源 → 必须用 **ClusterRoleBinding**（因为 RoleBinding 管不到大楼公告栏）
> - **ClusterRole** 既可以用 ClusterRoleBinding 全集群授权，也可以用 RoleBinding 限定到某个命名空间内
> - 这也是 Kubeflow 的巧妙设计：`kubeflow-edit` 是一个 ClusterRole，但通过 RoleBinding 绑定，让每个用户只能在**自己的**命名空间内编辑东西

### 3.3 什么是 API Group？

在 K8s 中，每种资源都属于一个 **API Group**（API 分组）。就像公司里不同部门管不同的事。

| API Group                   | 管辖的资源                                | 是谁注册的                 |
| --------------------------- | ----------------------------------------- | -------------------------- |
| `""` (核心组)               | Pod, Service, ConfigMap, Secret, Node ... | Kubernetes 自己            |
| `apps`                      | Deployment, StatefulSet, DaemonSet ...    | Kubernetes 自己            |
| `rbac.authorization.k8s.io` | Role, ClusterRole, RoleBinding ...        | Kubernetes 自己            |
| `trainer.kubeflow.org`      | TrainJob, ClusterTrainingRuntime ...      | Kubeflow Training Operator |
| `scheduling.run.ai`         | PodGroup, Queue ...                       | KAI-Scheduler              |

在 RBAC 规则中，你必须指定 `apiGroups` 来告诉 K8s 你要管的是哪个"部门"的资源。

### 3.4 Verbs（动作）速查

| Verb     | HTTP 方法    | 含义             |
| -------- | ------------ | ---------------- |
| `get`    | GET (单个)   | 获取单个资源详情 |
| `list`   | GET (列表)   | 列出所有资源     |
| `watch`  | GET (长连接) | 实时监听资源变更 |
| `create` | POST         | 创建新资源       |
| `update` | PUT          | 更新已有资源     |
| `patch`  | PATCH        | 部分更新资源     |
| `delete` | DELETE       | 删除资源         |

---

## 4. 生产环境权限链路全分析

通过在生产环境执行排查命令，我们精确还原了完整的权限授权链条：

### 4.1 涉及的 ServiceAccount

在 `kubeflow-user-example-com` 命名空间下：

```bash
kubectl get serviceaccounts -n kubeflow-user-example-com
```

| ServiceAccount 名称     | 用途                                   |
| ----------------------- | -------------------------------------- |
| `default`               | 默认账号（几乎不直接使用）             |
| `default-editor`        | **Jupyter Notebook Pod 使用的身份** ⭐ |
| `default-viewer`        | 只读权限的账号                         |
| `model-registry-server` | 模型注册服务专用                       |

### 4.2 命名空间级别的 RoleBinding

```bash
kubectl get rolebindings -n kubeflow-user-example-com
```

| RoleBinding 名称 | 绑定的 ClusterRole | 效果                                                     |
| ---------------- | ------------------ | -------------------------------------------------------- |
| `default-editor` | `kubeflow-edit`    | 允许在该命名空间内创建/编辑 Kubeflow 资源（如 TrainJob） |
| `default-viewer` | `kubeflow-view`    | 允许在该命名空间内只读查看 Kubeflow 资源                 |
| `namespaceAdmin` | `kubeflow-admin`   | 命名空间管理员权限                                       |

> **注意：** 这里用的是 **RoleBinding**（不是 ClusterRoleBinding），所以虽然绑定的是 ClusterRole，但权限**仅限于** `kubeflow-user-example-com` 这个命名空间内部。这解释了为什么 `default-editor` 可以在自己的命名空间内创建 `TrainJob`，但无法读取集群级别的 `ClusterTrainingRuntime`。

### 4.3 集群级别的 ClusterRoleBinding（🔑 关键钥匙）

```bash
kubectl get clusterrolebindings -o json | grep -B 5 "kubeflow-user-example-com"
```

发现了一个至关重要的绑定：

| ClusterRoleBinding 名称                       | 绑定的 ClusterRole      | 授权给谁                                       |
| --------------------------------------------- | ----------------------- | ---------------------------------------------- |
| `kubeflow-trainer-user-ai-lab-default-editor` | `kubeflow-trainer-user` | `default-editor` @ `kubeflow-user-example-com` |

> **🎯 这就是那把"缺失的钥匙"！** 正是这个 ClusterRoleBinding 让 Jupyter 的 `default-editor` 获得了访问 `ClusterTrainingRuntime`（集群级别资源）的合法权限。测试环境中因为缺少了这个绑定，所以 SDK 调用被 API Server 拒绝了。

### 4.4 ClusterRole `kubeflow-trainer-user` 的权限规则

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubeflow-trainer-user
rules:
  # 规则 1: 只读访问运行时模板（集群级别资源）
  - apiGroups: ["trainer.kubeflow.org"]
    resources: ["clustertrainingruntimes", "trainingruntimes"]
    verbs: ["get", "list", "watch"]
  # 规则 2: 完整操作训练任务（命名空间级别资源）
  - apiGroups: ["trainer.kubeflow.org"]
    resources: ["trainjobs", "trainjobs/status"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]
```

> **设计亮点：** 这是一个典型的"最小权限原则"实践。
>
> - 对 `ClusterTrainingRuntime` 只授予 **只读** 权限（`get/list/watch`），因为运行时模板由管理员统一维护，用户只需引用
> - 对 `TrainJob` 授予 **完整 CRUD** 权限，因为用户需要自主提交、查看和管理自己的训练任务
> - 所有资源都限定在 `trainer.kubeflow.org` 这一个 API Group 下，不会越界访问其他系统资源

### 4.5 权限链条总结

把上述分析按层级串起来，整个"许可证"的颁发过程就像这样：

```
ServiceAccount: default-editor
│
├── 通过 RoleBinding "default-editor"
│   └── 绑定 ClusterRole "kubeflow-edit"
│       └── 权限范围: kubeflow-user-example-com 命名空间内
│           └── 可以: 创建/编辑 TrainJob 等命名空间级别的资源 ✅
│
└── 通过 ClusterRoleBinding "kubeflow-trainer-user-ai-lab-default-editor"
    └── 绑定 ClusterRole "kubeflow-trainer-user"
        └── 权限范围: 全集群
            ├── 可以: 读取 ClusterTrainingRuntime ✅
            └── 可以: 操作 TrainJob (集群级别的补充) ✅
```

---

## 5. 权限拓扑图：从 Jupyter 提交训练到 KAI-Scheduler 调度

```
┌───────────────────────────────────────────────────────────────────────────────┐
│                          完整权限链路与数据流向                                 │
│                                                                               │
│  ① Jupyter Notebook Pod                                                       │
│     (身份: default-editor)                                                    │
│       │                                                                       │
│       │ client.get_runtime("toy-kai")                                         │
│       ├──→ 读取 ClusterTrainingRuntime (集群级别) ──── 需要 ────┐              │
│       │                                             ClusterRoleBinding        │
│       │                                              ↓                        │
│       │                                     kubeflow-trainer-user-*           │
│       │                                              ↓                        │
│       │                                     ClusterRole:                      │
│       │                                     kubeflow-trainer-user             │
│       │                                     [get, list, watch]                │
│       │                                                                       │
│       │ client.train(trainer=..., runtime=...)                                │
│       ├──→ 创建 TrainJob (命名空间级别) ──────── 需要 ────┐                    │
│       │                                         RoleBinding                   │
│       │                                          ↓                            │
│       │                                     default-editor                    │
│       │                                          ↓                            │
│       │                                     ClusterRole: kubeflow-edit        │
│       │                                     [create, get, list, watch, ...]   │
│       │                                                                       │
│  ② Training Operator Controller (监听 TrainJob)                               │
│       │                                                                       │
│       ├──→ 创建 Pod (带 schedulerName: kai-scheduler)                         │
│       │    使用自己专属的 ServiceAccount 和权限                                 │
│       │                                                                       │
│  ③ KAI PodGrouper (监听带标签的 Pod)                                          │
│       │                                                                       │
│       ├──→ 创建 PodGroup (打包多个 Pod 为一组)                                │
│       │                                                                       │
│  ④ KAI-Scheduler (监听 PodGroup 和 Pending Pods)                              │
│       │                                                                       │
│       └──→ 调度: 分配节点、绑定 Pod → Running                                 │
│                                                                               │
└───────────────────────────────────────────────────────────────────────────────┘
```

---

## 6. 从生产环境迁移权限到测试环境

### 步骤 A：在生产环境导出权限资源

```bash
# 1. 导出 ClusterRole（权限菜单）
kubectl get clusterrole kubeflow-trainer-user -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" \
  > kubeflow-trainer-user-cr.yaml

# 2. 导出 ClusterRoleBinding（授权凭证）
kubectl get clusterrolebinding kubeflow-trainer-user-ai-lab-default-editor -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" \
  > kubeflow-trainer-user-crb.yaml

# 3. (可选) 导出 kubeflow-edit ClusterRole（命名空间级别的编辑权限）
kubectl get clusterrole kubeflow-edit -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" \
  > kubeflow-edit-cr.yaml
```

> **为什么要 `grep -v` 过滤？**
> 导出的 YAML 里会包含当前集群的运行时状态字段（如 `creationTimestamp`、`resourceVersion`、`uid`），这些是旧集群专属的标识。如果不清理，直接在新集群 `apply` 时可能会因为 ID 冲突报错。

### 步骤 B：将文件传输到测试环境

```bash
# 使用 scp 或者 U 盘等物理方式传输（离线环境就是这么朴素）
scp kubeflow-trainer-user-cr.yaml kubeflow-trainer-user-crb.yaml root@<测试环境IP>:/tmp/
```

### 步骤 C：在测试环境导入权限

```bash
# 1. 导入 ClusterRole
kubectl apply -f kubeflow-trainer-user-cr.yaml

# 2. 导入 ClusterRoleBinding
kubectl apply -f kubeflow-trainer-user-crb.yaml

# 3. 验证权限是否生效（无需重启 Jupyter！RBAC 实时生效）
kubectl auth can-i get clustertrainingruntimes \
  --as=system:serviceaccount:kubeflow-user-example-com:default-editor
# 期望输出: yes

kubectl auth can-i create trainjobs \
  --as=system:serviceaccount:kubeflow-user-example-com:default-editor \
  -n kubeflow-user-example-com
# 期望输出: yes
```

---

## 7. 实战中踩过的其他坑

在整个部署和调试过程中，除了 RBAC 权限问题，我们还遇到了以下值得记录的坑：

### 7.1 Training Operator 卡在 ContainerCreating

**现象：** `training-operator` Pod 状态一直是 `ContainerCreating`，不进入 `Running`。

**报错：**

```
MountVolume.SetUp failed for volume "cert": secret "training-operator-webhook-cert" not found
```

**原因：** 从生产环境迁移 Training Operator 时，只复制了 Deployment、ServiceAccount、ClusterRole 等资源，遗漏了 Webhook 使用的 TLS 证书 Secret。Deployment 里定义了要挂载 `training-operator-webhook-cert` 这个 Secret 作为 Volume，kubelet 在启动 Pod 时发现找不到它就一直死等。

**修复：**

```bash
# 在生产环境导出
kubectl get secret training-operator-webhook-cert -n kubeflow -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" \
  > training-operator-secret.yaml

# 搬运到测试环境后 apply
kubectl apply -f training-operator-secret.yaml
# Pod 会在数秒内自动检测到 Volume 可用，立刻变为 Running
```

**学到的：** 迁移 Deployment 时，一定要检查它的 `volumes` 部分是否引用了 Secret 或 ConfigMap。这些依赖资源必须一起迁移。

### 7.2 docker save 报错 "Cannot connect to Docker daemon"

**现象：** 在生产节点上执行 `docker save` 导出镜像时报错。

**原因：** 生产集群使用的是 Containerd 作为容器运行时（K8s ≥1.24 的标配），而不是 Docker。机器上的 `docker` 客户端只是一个空壳，背后没有 Docker Daemon 在跑。

**修复：** 使用 Containerd 的原生命令：

```bash
# 查看可用镜像
crictl images | grep training-operator

# 导出镜像
ctr -n k8s.io images export training-operator.tar <镜像全名>

# 在测试节点导入
ctr -n k8s.io images import training-operator.tar
```

### 7.3 pip install kubeflow-training 之后仍然 ModuleNotFoundError

**现象：** 在 Jupyter 中 `!pip install kubeflow-training` 成功后，`from kubeflow.trainer import TrainerClient` 仍报 `ModuleNotFoundError`。

**原因解析：** 这里踩了两个坑——

1. **Kernel 缓存问题：** Jupyter 的 Kernel 在内存中缓存了模块列表。新装的包需要重启 Kernel 才能被识别。
2. **包名问题：** 在某些 Kubeflow 版本体系中，`kubeflow-training` 包不一定包含 `kubeflow.trainer` 这个新版 API 模块。正确的做法是安装更上层的 `kubeflow` 元包。

**修复：**

```python
# 正确的安装命令
!pip install kubeflow

# 安装后务必: 菜单栏 → Kernel → Restart Kernel
# 然后再执行 from kubeflow.trainer import TrainerClient
```

### 7.4 离线环境中镜像不存在

**现象：** `ClusterTrainingRuntime` 中指定了 `python:3.9-slim` 镜像，但离线测试集群拉不到这个公网镜像。

**修复：** 替换为集群中已确认存在的镜像。一个万无一失的办法是使用 Kubeflow Jupyter 创建 Notebook 时所使用的那个镜像（因为它必然已经被拉取到了所有节点上）：

```yaml
# 将 toy-kai-runtime.yaml 中的 image 替换为:
image: kubeflow/kubeflow/notebook-servers/jupyter-scipy:v1.10.0
```

> **小技巧：** 在 Kubeflow Dashboard 的 "New Notebook" 页面就能看到可用的镜像列表。

---

## 8. 常见问题排查

### Q: 为什么 `ClusterTrainingRuntime` 需要 ClusterRoleBinding 而不是 RoleBinding？

**A:** 因为 `ClusterTrainingRuntime` 是一个**集群级别（Cluster-scoped）**的资源，它不属于任何命名空间。RoleBinding 只能授予命名空间内的权限，无法触及集群级别的资源。因此必须使用 ClusterRoleBinding 来授权。

### Q: 为什么 `TrainJob` 用 RoleBinding 就够了？

**A:** 因为 `TrainJob` 是一个**命名空间级别（Namespace-scoped）**的资源，创建在用户自己的命名空间内。命名空间内的 `default-editor` RoleBinding（绑定 `kubeflow-edit` ClusterRole）已经包含了对 `TrainJob` 的操作权限。

### Q: 修改权限后需要重启 Jupyter Notebook 吗？

**A:** **不需要！** Kubernetes 的 RBAC 权限变更是**实时生效**的。一旦 ClusterRoleBinding 被创建，下一次 API 请求就会立即通过权限检查。

### Q: 如何检查某个 ServiceAccount 是否有某个权限？

```bash
# 语法: kubectl auth can-i <动词> <资源> --as=system:serviceaccount:<命名空间>:<SA名称>
kubectl auth can-i get clustertrainingruntimes \
  --as=system:serviceaccount:kubeflow-user-example-com:default-editor
# 输出 "yes" 或 "no"
```

### Q: 如何查看一个 ServiceAccount 拥有的所有权限？

```bash
kubectl auth can-i --list \
  --as=system:serviceaccount:kubeflow-user-example-com:default-editor
```

### Q: 如何快速定位某个 ServiceAccount 关联了哪些绑定？

```bash
# 查找命名空间级别的 RoleBinding
kubectl get rolebindings -n <namespace> -o json | jq '.items[] | select(.subjects[]?.name == "<sa-name>")'

# 查找集群级别的 ClusterRoleBinding
kubectl get clusterrolebindings -o json | grep -B 5 "<namespace>"
```

### Q: 我可以给 Jupyter 开 cluster-admin 吗？

**A:** 从技术上当然可以（`kubectl create clusterrolebinding ... --clusterrole=cluster-admin`），但这是**反模式**。在生产环境中绝对不允许，因为它意味着任何在 Jupyter 中运行的代码都能删除整个集群的任何资源。即使在测试环境中，也建议用最小权限原则，方便将来发现潜在的权限依赖问题。

---

## 9. KAI-Scheduler 整合涉及的所有权限资源清单

| 资源类型                 | 示例名称                         | 作用域       | 需要的最低权限                                       | 由谁操作                         |
| ------------------------ | -------------------------------- | ------------ | ---------------------------------------------------- | -------------------------------- |
| `ClusterTrainingRuntime` | `toy-kai`                        | 集群级别     | `get`, `list`, `watch`                               | Jupyter 用户 (TrainerClient SDK) |
| `TrainJob`               | 用户创建                         | 命名空间级别 | `create`, `get`, `list`, `watch`, `update`, `delete` | Jupyter 用户 (TrainerClient SDK) |
| `TrainJob/status`        | —                                | 命名空间级别 | `get`, `list`, `watch`                               | Jupyter 用户 (查看任务状态)      |
| `Pod`                    | 由 Training Operator 创建        | 命名空间级别 | `create`, `get`, `list`, `watch`                     | Training Operator Controller     |
| `PodGroup`               | 由 PodGrouper 创建               | 命名空间级别 | `create`, `get`, `list`, `watch`, `update`           | KAI PodGrouper                   |
| `Queue`                  | `default-queue`                  | 集群级别     | `get`, `list`, `watch`                               | KAI-Scheduler                    |
| `Secret` (Webhook cert)  | `training-operator-webhook-cert` | 命名空间级别 | — (由 kubelet 挂载)                                  | 系统组件                         |

### Training Operator 自身的权限体系

Training Operator 作为一个 Controller，它也有自己的一套 RBAC：

| 资源                 | 关联的 ServiceAccount                        |
| -------------------- | -------------------------------------------- |
| `ServiceAccount`     | `training-operator` (在 `kubeflow` 命名空间) |
| `ClusterRole`        | `training-operator`                          |
| `ClusterRoleBinding` | `training-operator`                          |

迁移 Training Operator 到测试环境时，这些也需要一并迁移（详见 [教程 02](./02-e2e-jupyter-to-local-scheduler.md) 的步骤 A）。

---

## 10. 参考命令速查表

### RBAC 排查类

```bash
# 查看某个命名空间下所有 ServiceAccount
kubectl get serviceaccounts -n <namespace>

# 查看命名空间内的 RoleBinding
kubectl get rolebindings -n <namespace>

# 查看全集群的 ClusterRoleBinding（按关键词过滤）
kubectl get clusterrolebindings | grep <keyword>

# 查看某个 ClusterRole 的详细权限规则
kubectl describe clusterrole <name>
# 或者
kubectl get clusterrole <name> -o yaml

# 测试某个 ServiceAccount 是否有某个权限
kubectl auth can-i <verb> <resource> --as=system:serviceaccount:<ns>:<sa>

# 查看某个 ServiceAccount 的所有权限
kubectl auth can-i --list --as=system:serviceaccount:<ns>:<sa>
```

### 资源迁移类

```bash
# 导出任意资源并清理集群特定字段
kubectl get <resource-type> <name> [-n <namespace>] -o yaml \
  | grep -v "creationTimestamp\|resourceVersion\|uid" \
  > <output-file>.yaml

# 在测试集群导入
kubectl apply -f <output-file>.yaml

# 验证资源是否存在
kubectl get <resource-type> <name> [-n <namespace>]
```

### 容器镜像管理类（Containerd 环境）

```bash
# 查看节点上的镜像列表
crictl images

# 导出镜像为 tar 文件
ctr -n k8s.io images export <output>.tar <image-name>

# 导入镜像 tar 文件
ctr -n k8s.io images import <input>.tar
```

### Pod 排查类

```bash
# 查看 Pod 状态
kubectl get pods -n <namespace>

# 查看 Pod 详细事件（排查 ContainerCreating 等异常）
kubectl describe pod <pod-name> -n <namespace>

# 查看 Pod 的日志
kubectl logs <pod-name> -n <namespace>

# 查看 Pod 使用的 ServiceAccount
kubectl get pod <pod-name> -n <namespace> -o jsonpath='{.spec.serviceAccountName}'

# 查看 Jupyter Notebook Pod 所在的命名空间（在 Jupyter 内部执行）
# !cat /var/run/secrets/kubernetes.io/serviceaccount/namespace
```

---

## 附录：本教程涉及的全部 YAML 文件列表

| 文件名                           | 来源       | 用途                                        |
| -------------------------------- | ---------- | ------------------------------------------- |
| `kubeflow-trainer-user-cr.yaml`  | 从生产导出 | 定义 Jupyter 用户对 Training 资源的权限规则 |
| `kubeflow-trainer-user-crb.yaml` | 从生产导出 | 将上述权限授予 `default-editor`             |
| `training-operator-deploy.yaml`  | 从生产导出 | Training Operator 的部署清单                |
| `training-operator-sa.yaml`      | 从生产导出 | Training Operator 的身份账号                |
| `training-operator-cr.yaml`      | 从生产导出 | Training Operator 自身的操作权限            |
| `training-operator-crb.yaml`     | 从生产导出 | Training Operator 权限绑定                  |
| `training-operator-secret.yaml`  | 从生产导出 | Webhook TLS 证书                            |
| `training-operator-svc.yaml`     | 从生产导出 | Webhook 通信用 Service                      |
| `toy-kai-runtime.yaml`           | 项目代码库 | 轻量级测试用 ClusterTrainingRuntime         |
