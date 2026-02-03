# MinRuntime 插件

## 概述

KAI-Scheduler 的 MinRuntime 插件为作业提供运行时保护，在作业运行达到指定的最短时间之前，防止其被抢占或资源被回收。这确保工作负载在被打断之前能够完成关键的初始化或检查点保存。

## 主要特性

- **抢占保护**：在作业运行达到指定的最短时间之前，防止其被抢占
- **回收保护**：在弹性作业运行达到指定的最短时间之前，防止其资源被回收
- **基于队列的配置**：在调度层次结构中为不同队列配置不同的最短运行时间
- **层次继承**：最短运行时间设置从父队列向下级联到叶队列
- **灵活的解析方法**：两种回收最短运行时间解析方法：
  - 基于队列：根据受害者的队列配置评估 min-runtime 保护
  - LCA（最近公共祖先）：使用回收方和受害者队列在队列层次结构中的公共祖先来确定 min-runtime 保护

## 使用方法

minruntime 插件有两种配置方式：

1. **队列定义**：在 Queue 配置中设置最短运行时间值
2. **插件参数**：在调度器配置中配置默认值和解析方法

### 队列配置

队列可以指定以下最短运行时间参数：

- `preemptMinRuntime`：此队列中的作业可被抢占前的最短运行时间
- `reclaimMinRuntime`：此队列中的作业可被回收资源前的最短运行时间

队列定义示例：

```yaml
apiVersion: scheduling.run.ai/v2
kind: Queue
metadata:
  name: production
spec:
  preemptMinRuntime: "20s"
  reclaimMinRuntime: "30s"
```

### 插件配置

在调度器配置（`scheduler-config` ConfigMap）中，添加 minruntime 插件及其参数：

```yaml
tiers:
- plugins:
  # 其他插件...
  - name: minruntime
    arguments:
      defaultPreemptMinRuntime: "10m"
      defaultReclaimMinRuntime: "10m"
      reclaimResolveMethod: "lca"  # 或 "queue"
```

### 配置参数

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `defaultPreemptMinRuntime` | 队列未指定时，抢占前默认最短运行时间 | "0s" |
| `defaultReclaimMinRuntime` | 队列未指定时，资源回收前默认最短运行时间 | "0s" |
| `reclaimResolveMethod` | 解析回收最短运行时间的方法（"lca" 或 "queue"） | "lca" |

0s 表示工作负载可立即被回收/抢占。

## 解析方法

### 抢占解析

对于抢占，最短运行时间从受害者的队列开始，沿队列层次结构向上查找，直到找到 `preemptMinRuntime` 值。

### 回收解析

对于回收，支持两种方法：

1. **基于队列的解析**（`reclaimResolveMethod: "queue"`）：
   - 与抢占类似，查看受害者的队列并沿层次结构向上查找，直到找到 `reclaimMinRuntime` 值。
   - 如果完全找不到值，则使用插件的默认值。

2. **LCA 解析**（`reclaimResolveMethod: "lca"`）：
   - 在队列层次结构中识别抢占方和受害者之间的最近公共祖先（LCA）
   - 从 LCA 向受害者方向下走一步，使用该队列的 `reclaimMinRuntime` 值
   - 如果未找到值，则向根队列方向向上查找，直到找到为止。
   - 如果完全找不到值，则使用插件的默认值。

如果未指定方法，LCA 方法为默认方法。LCA 方法的目的是遵循队列层次结构的策略设置方式，允许子队列中的用户设置 min-runtime 值，这些值会被其兄弟队列遵守，同时不影响堂兄弟队列。

## 弹性作业处理

对于弹性作业（其中 `MinAvailable < 作业中的 Pod 数量`），插件：

1. 在过滤阶段允许作业被考虑进行抢占/回收
2. 在场景验证阶段验证：如果尚未达到 min-runtime，作业将保持其最小 Pod 数量。

## 实现细节

插件实现以下函数：

- `preemptFilterFn`：过滤因最短运行时间而不应被抢占的受害者
- `reclaimFilterFn`：过滤因最短运行时间而不应被回收资源的受害者
- `preemptScenarioValidatorFn`：验证弹性作业的抢占场景
- `reclaimScenarioValidatorFn`：验证弹性作业的回收场景

## 示例工作流

1. 作业在配置了 `reclaimMinRuntime: "30s"` 的队列中开始运行
2. 运行 20 秒时，另一个作业尝试回收资源
3. minruntime 插件识别出尚未达到最短运行时间
4. 受害者作业受到保护，直到运行至少 30 秒后才可被回收

## 缓存

插件维护缓存以提高性能：
- `preemptMinRuntimeCache`：按队列缓存抢占最短运行时间值
- `reclaimMinRuntimeCache`：按队列对缓存回收最短运行时间值
- `preemptProtectionCache`：跟踪受抢占保护的作业
- `reclaimProtectionCache`：跟踪受回收保护的作业

这些缓存在每个调度会话开始时重置。
