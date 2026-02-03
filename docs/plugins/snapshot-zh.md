# KAI Scheduler Snapshot 插件与工具

## 概述

KAI Scheduler 提供 snapshot 插件和工具，用于捕获和分析调度器及集群资源的状态。本文档涵盖 snapshot 插件和 snapshot 工具。

## Snapshot 插件

Snapshot 插件是一个框架插件，提供 HTTP 端点以捕获调度器和集群资源的当前状态。

### 功能

- 捕获调度器配置和参数
- 收集调度器用于执行其操作的原始 Kubernetes 对象，包括：
  - Pods
  - Nodes
  - Queues
  - PodGroups
  - BindRequests
  - PriorityClasses
  - ConfigMaps
  - PersistentVolumeClaims
  - CSIStorageCapacities
  - StorageClasses
  - CSIDrivers
  - ResourceClaims
  - ResourceSlices
  - DeviceClasses

### 使用方法

插件注册 HTTP 端点 `/get-snapshot`，返回包含集群状态 JSON 快照的 ZIP 文件。
在 `kai` 命名空间中部署的调度器 Pod 示例：
```bash
kubectl port-forward -n kai deployment/scheduler 8081 &
curl -vv "localhost:8081/get-snapshot"  > snapshot.gzip
./bin/snapshot-tool-amd64 --filename snapshot.gzip --verbosity 8
```

### 响应格式

快照以 ZIP 文件形式返回，包含单个 JSON 文件（`snapshot.json`），结构如下：

```json
{
  "config": {
    // 调度器配置
  },
  "schedulerParams": {
    // 调度器参数
  },
  "rawObjects": {
    // 原始 Kubernetes 对象
  }
}
```

## Snapshot 工具

Snapshot 工具是一个命令行实用程序，可以加载和分析由 snapshot 插件捕获的快照。

### 功能

- 从 ZIP 文件加载快照
- 从快照重建调度器环境
- 支持在快照数据上运行调度器动作
- 提供详细的操作日志

### 使用方法

```bash
snapshot-tool --filename <snapshot-file> [--verbosity <log-level>]
```

#### 参数

- `--filename`：快照 ZIP 文件路径（必填）
- `--verbosity`：日志详细级别（默认：4）

### 示例

```bash
# 加载并分析快照
snapshot-tool --filename snapshot.zip

# 以更高详细级别加载并分析快照
snapshot-tool --filename snapshot.zip --verbosity 5
```

## 实现细节

### Snapshot 插件

Snapshot 插件（`pkg/scheduler/plugins/snapshot/snapshot.go`）实现以下关键组件：

1. `RawKubernetesObjects`：包含所有捕获的 Kubernetes 对象的结构
2. `Snapshot`：包含配置、参数和原始对象的主结构
3. `snapshotPlugin`：带有 HTTP 端点处理器的插件实现

### Snapshot 工具

Snapshot 工具（`cmd/snapshot-tool/main.go`）实现：

1. 快照加载和解析
2. 使用快照数据创建 Fake client
3. 调度器缓存初始化
4. Session 管理
5. 动作执行

## 限制

- Snapshot 工具在模拟环境中运行
- 某些实时集群功能可能不可用
- 资源约束可能与原始集群不同
