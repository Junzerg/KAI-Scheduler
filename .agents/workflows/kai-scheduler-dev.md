---
description: KAI-Scheduler 项目开发上下文 — 环境、架构、沟通风格的持久化记忆
---

# KAI-Scheduler 项目开发上下文

## 用户背景

- **身份：** K8s 初学者，正在学习 Kubeflow，接手了 KAI-Scheduler 的二开任务
- **学习风格：** 偏好"先实战再理解"，但需要在实战后补充系统化的知识梳理
- **操作风格：** 重度 Never AFK 选手，能用快捷键的绝不用鼠标。开发环境是 **macOS + Antigravity**
- **沟通偏好：**
  - 中文沟通
  - 喜欢用类比（"入职新公司" = "部署到新集群"）
  - 喜欢 ASCII 图解和表格来理解复杂概念
  - 不要假设用户懂 K8s 术语，第一次出现时要解释
  - 实战中踩的坑要记录成教程，放到 `docs/dev/tutorials/` 下
  - 教程用真实故事开头，不要纯理论
  - 涉及 IDE / 终端操作时，优先给出快捷键方案（Mac 键位：⌘ ⌥ ⌃ ⇧）

## 环境架构

```
┌─────────────────────────────────────────────────────────────┐
│                        环境拓扑                               │
│                                                              │
│  🏭 生产环境 (kubeflow-master)                                │
│  ├── 完整的 Kubeflow v2 部署（含所有 Controller）              │
│  ├── 离线环境，无法访问公网                                    │
│  ├── 容器运行时: Containerd（不是 Docker！）                   │
│  ├── 命令: ctr / crictl（不是 docker）                        │
│  └── 用途: 导出 Controller 资源、镜像、CRD 等                 │
│                                                              │
│  🧪 测试环境 (test-172-28-2-242)                              │
│  ├── 精简的 Kubeflow 部署（需要手动补全组件）                   │
│  ├── 离线环境，无法访问公网                                    │
│  ├── 容器运行时: Containerd                                   │
│  ├── 已部署: trainer-controller, jobset-controller (从生产搬来) │
│  └── 用途: 提交 TrainJob，让本地 IDE 中的调度器拦截             │
│                                                              │
│  💻 本地开发机 (macOS)                                        │
│  ├── IDE: GoLand / VSCode                                    │
│  ├── 通过 kubeconfig 连接到测试集群                            │
│  ├── 本地运行 KAI-Scheduler（debug 模式）                     │
│  └── 用途: 断点调试调度器代码                                  │
│                                                              │
│  📋 资源传输方式: scp 或物理拷贝（两个服务器都离线）             │
└─────────────────────────────────────────────────────────────┘
```

## 三层架构速查

```
第三层 KAI-Scheduler (你在二开的)
  PodGroup CRD, Queue CRD, kai-scheduler, PodGrouper

第二层 Kubeflow + JobSet (上层应用)
  TrainJob CRD, ClusterTrainingRuntime CRD, JobSet CRD
  trainer-controller-manager, jobset-controller-manager

第一层 K8s 原生 (操作系统)
  Pod, Deployment, Service, Secret, ConfigMap
  ServiceAccount, Role, ClusterRole, RoleBinding, ClusterRoleBinding
  CRD机制, Leases, Webhook
```

## TrainJob 生命周期

```
Jupyter 提交 TrainJob
  → trainer-controller (第二层) 翻译成 JobSet
    → jobset-controller (第二层) 翻译成 Job → Pod
      → PodGrouper (第三层) 打包成 PodGroup
        → kai-scheduler (第三层, 本地 IDE) 调度
```

## 关键命名空间

| 命名空间 | 用途 |
|---|---|
| `kubeflow-system` | Controller 运行的地方 (trainer-ctrl, jobset-ctrl) |
| `kubeflow-user-example-com` | 用户 Jupyter 和 TrainJob 所在的地方 |
| `kai-scheduler` | KAI-Scheduler 组件所在的地方 |
| `kube-system` | K8s 系统组件 |

## 已完成的教程体系

| 教程 | 路径 | 主题 |
|---|---|---|
| 01 | `docs/dev/tutorials/01-local-debug-trainjob.md` | 本地断点调试 TrainJob |
| 02 | `docs/dev/tutorials/02-e2e-jupyter-to-local-scheduler.md` | 端到端：Jupyter → 本地调度器 |
| 03 | `docs/dev/tutorials/03-rbac-permissions-for-training.md` | RBAC 权限体系详解 |
| 04 | `docs/dev/tutorials/04-controller-migration-guide.md` | Controller 搬运实战 |

## 沟通风格指南

1. **先动手后讲解：** 用户倾向于先看到命令和结果，再理解为什么
2. **踩坑即教材：** 每次遇到问题都是学习机会，要记录到教程中
3. **分层讲解：** 总是标注"这个东西属于第几层"，帮助建立认知框架
4. **不要省略步骤：** 用户在离线环境操作，每个命令都要完整可执行
5. **类比优先：** 用生活化的类比解释 K8s 概念（工牌=SA, 门禁卡=RBAC, 公告栏=集群级资源）
6. **表格总结：** 复杂的对比信息用表格呈现
7. **ASCII 图解：** 架构和流程用 ASCII 图画出来

## 常见踩坑备忘

- `docker` 命令在生产/测试环境不可用，要用 `ctr -n k8s.io` 或 `crictl`
- `grep -v` 会破坏 YAML 多行结构，大文件慎用
- CRD 文件太大需要 `kubectl apply --server-side`
- 从生产导出的 YAML 包含 `uid`，apply 到新集群会冲突，用 `sed '/uid:/d'` 清理
- Controller 搬运不只是 Deployment，还有 SA + RBAC + Secret + ConfigMap + Service + CRD（9件套）
- Leader Election 需要 Role（不是 ClusterRole），因为 Lease 是命名空间级资源
- Service 导出时要额外过滤 `clusterIP` 和 `clusterIPs`
