# 优先级与可抢占性分离（P0）

*状态：草案*

## 目录
- [背景](#背景)
- [问题陈述](#问题陈述)
- [目标 / 非目标](#目标--非目标)
   * [目标](#目标)
   * [非目标](#非目标)
- [提案](#提案)
   * [API 变更](#api-变更)
      + [1. PodGroup Spec 字段](#1-podgroup-spec-字段)
      + [2. Pod 标签](#2-pod-标签)
   * [调度器逻辑变更](#调度器逻辑变更)
      + [1. 可抢占性判定](#1-可抢占性判定)
   * [向后兼容性](#向后兼容性)
      + [1. 默认行为支持](#1-默认行为支持)
      + [2. 配置验证](#2-配置验证)
- [示例](#示例)
   * [示例 1：高优先级可抢占 PodGroup](#示例-1高优先级可抢占-podgroup)
   * [示例 2：低优先级不可抢占 PodGroup](#示例-2低优先级不可抢占-podgroup)
   * [示例 3：具有显式可抢占性的外部工作负载](#示例-3具有显式可抢占性的外部工作负载)
   * [示例 4：默认行为 PodGroup（向后兼容）](#示例-4默认行为-podgroup向后兼容)


---

## 背景

目前，KAI-scheduler 用户可以提交具有关联优先级类的工作负载，这些优先级类应用于所有从属 Pod。调度器隐式假设使用低于 100 的优先级类的所有工作负载是可抢占的，而使用大于或等于 100 的优先级类的工作负载是不可抢占的。

## 问题陈述

优先级与可抢占性之间的这种耦合限制了多个重要用例的使用灵活性：

1. **高优先级可抢占工作负载**：用户可能希望提交仍然可抢占的高优先级工作负载（例如，高优先级训练工作负载）
2. **低优先级不可抢占工作负载**：用户可能希望提交低优先级工作负载（例如，数据处理），在获得资源后运行至完成
3. **半可抢占工作负载**：用户可能希望提交具有 min-members 计数的复合工作负载，其中 min-members 不可抢占，但 min-members 之上的额外 Pod 可抢占

本文档将处理情况 1 和 2。情况 3 将在单独文档中处理。

当前实现创建了人为约束，阻止用户表达其真正的调度需求：

- **优先级**应控制队列中考虑工作负载调度的顺序
- **可抢占性**应控制工作负载在运行后是否可被中断
- 这些是正交的关注点，应可独立配置

当前基于优先级的可抢占性判定（优先级 >= 100 = 不可抢占）过于简单，无法满足现代 AI/ML 工作负载的多样化调度需求。

## 目标 / 非目标

### 目标
- **将优先级与可抢占性分离**：允许工作负载优先级和可抢占性的独立配置
- **保持向后兼容性**：没有显式可抢占性配置的现有工作负载应继续使用默认的基于优先级的判定
- **支持两种可抢占性模式**：可抢占、不可抢占

### 非目标
- **P1 功能**：半可抢占工作负载（单独处理）


## 提案

在 pod/podgroup 级别添加新的 `preemptibility` 参数，具有以下可能值：
- `preemptible`：PodGroup 可被更高优先级工作负载抢占
- `non-preemptible`：PodGroup 一旦调度则运行至完成
- `semi-preemptible`：PodGroup 同时具有可抢占和不可抢占组件（P1 功能）

当 `preemptibility` 未显式设置时，系统默认为基于优先级的可抢占性判定。

### API 变更

#### 1. PodGroup Spec 字段
向 PodGroup spec 添加 preemptibility 字段：

```yaml
spec:
  preemptibility: "preemptible"  # 或 "non-preemptible"
  priorityClassName: "train"     # 现有字段，现与可抢占性独立
```

```go
type PodGroupSpec struct {
    // ... 现有字段 ...
    
    // Preemptibility 定义此 PodGroup 是否可被抢占
    // 默认为基于优先级的可抢占性判定（优先级 < 100 则为可抢占）
    // +kubebuilder:validation:Enum=preemptible;non-preemptible;semi-preemptible
    // +optional
    Preemptibility string `json:"preemptibility,omitempty"`
}
```

#### 2. Pod 标签
Pod 也可以通过 `kai.scheduler/preemptibility` 标签指定可抢占性，这对于外部工作负载或无法修改 PodGroup spec 的情况很有用（与优先级相同）：

```yaml
metadata:
  labels:
    kai.scheduler/preemptibility: "non-preemptible"
```

使用无效的可抢占性值创建 Pod 将回退到默认的基于优先级的判定。

### 调度器逻辑变更

#### 1. 可抢占性判定
调度器将使用以下优先级顺序判定可抢占性：

1. PodGroup 上的**显式 preemptibility spec 字段**
2. Pod 顶层 owner 上的**显式 preemptibility 标签**（用于外部工作负载）
3. Pod 上的**显式 preemptibility 标签**（用于外部工作负载）
4. **传统基于优先级的判定**（优先级 < 100 则为可抢占）以保持向后兼容性

相同的逻辑将用于 PodGroupController 以在 PodGroup 上发布可抢占性状态（用于向后兼容性）。

### 向后兼容性

#### 1. 默认行为支持
没有显式可抢占性配置的工作负载将继续使用基于优先级的判定作为默认行为：
- 优先级 < 100 → 可抢占
- 优先级 >= 100 → 不可抢占

#### 2. 配置验证
可抢占性值将被验证，无效值将回退到默认的基于优先级的判定。

## 示例

### 示例 1：高优先级可抢占 PodGroup
```yaml
apiVersion: scheduling.kai.nvidia.com/v2alpha2
kind: PodGroup
metadata:
  name: high-priority-training
spec:
  preemptibility: "preemptible"
  priorityClassName: "inference"  # 高优先级 (125)
  # ... podgroup spec 其余部分
```

### 示例 2：低优先级不可抢占 PodGroup
```yaml
apiVersion: scheduling.kai.nvidia.com/v2alpha2
kind: PodGroup
metadata:
  name: data-processing
spec:
  preemptibility: "non-preemptible"
  priorityClassName: "train"  # 低优先级 (50)
  # ... podgroup spec 其余部分
```

### 示例 3：具有显式可抢占性的外部工作负载
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-workload
spec:
  template:
    metadata:
      labels:
        kai.scheduler/preemptibility: "non-preemptible"
        priorityClassName: "train"
    spec:
      # ... pod spec
```

### 示例 4：默认行为 PodGroup（向后兼容）
```yaml
apiVersion: scheduling.kai.nvidia.com/v2alpha2
kind: PodGroup
metadata:
  name: default-behavior-workload
spec:
  priorityClassName: "build"  # 优先级 100 → 不可抢占（默认行为）
  # 无 preemptibility 字段 → 使用默认基于优先级的判定
  # ... podgroup spec 其余部分
```
