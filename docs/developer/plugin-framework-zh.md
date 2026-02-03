# 调度器插件机制

## 概述
调度器使用基于插件的架构，允许通过各种扩展点扩展其功能。核心机制围绕维护调度上下文和插件注册回调的 `Session` 对象构建。

## 核心组件

### 插件接口
```go
type Plugin interface {
    Name() string
    OnSessionOpen(ssn *Session)
    OnSessionClose(ssn *Session)
}
```

插件使用构建器模式注册：
```go
type PluginBuilder func(map[string]string) Plugin

// 注册新插件
RegisterPluginBuilder("my-plugin", func(args map[string]string) Plugin {
    return &MyPlugin{}
})
```

## 关键扩展点

### 1. Session 生命周期钩子

- **OnSessionOpen**：在调度会话开始时调用
- **OnSessionClose**：在调度会话结束时调用

OnSessionOpen 用于初始化状态并注册回调函数。OnSessionClose 可用于清理或指标报告。

### 2. Session 扩展点

Session 对象为插件提供多个扩展点，插件可以为其注册回调。例如：

#### 调度顺序
- `AddJobOrderFn`：定义队列内作业的排序——例如，作业优先级
- `AddTaskOrderFn`：定义作业内任务的优先级——例如，尝试先调度 leader pod
- `AddQueueOrderFn`：定义队列优先级排序——例如，公平份额或严格优先级
- `AddNodeOrderFn`：为任务放置对节点评分——例如，装箱、节点亲和性


#### 谓词（Predicates）

```go
// 前置谓词函数在主谓词之前运行
type PrePredicateFn func(task *pod_info.PodInfo, job *podgroup_info.PodGroupInfo) error

// 主谓词函数确定 Pod 是否可以在节点上运行
type PredicateFn func(task *pod_info.PodInfo, job *podgroup_info.PodGroupInfo, node *node_info.NodeInfo) error

// 注册谓词
ssn.AddPrePredicateFn(myPrePredicate)
ssn.AddPredicateFn(myPredicate)
```

谓词示例：
```go
func GPUPredicate(task *pod_info.PodInfo, job *podgroup_info.PodGroupInfo, node *node_info.NodeInfo) error {
    if task.RequiresGPU && !node.HasAvailableGPUs() {
        return fmt.Errorf("node %s has no available GPUs", node.Name)
    }
    return nil
}
```

### 3. 评分函数

#### 节点评分
```go
type NodeOrderFn func(task *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error)

// 节点评分函数示例
func GPUUtilizationScore(task *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error) {
    return float64(node.GetGPUUtilization()), nil
}
```

#### GPU 评分
```go
type GpuOrderFn func(task *pod_info.PodInfo, node *node_info.NodeInfo, gpuIdx string) (float64, error)
```

#### 作业/任务排序
```go
// CompareFn 返回：
// -1 如果 left 应在 right 之前调度
//  0 如果优先级相等
//  1 如果 right 应在 left 之前调度
type CompareFn func(left, right interface{}) int
```

### 4. AllocateFunc/DeallocateFunc 回调函数

插件可以注册在关键调度事件期间触发的回调，允许插件在模拟调度决策发生时进行跟踪。

**回调：**
- **AllocateFunc**：当 Pod 被虚拟分配到节点时调用
- **DeallocateFunc**：当 Pod 被虚拟从节点驱逐时调用

这些回调使插件能够：
- 根据调度决策更新内部状态
- 收集有关分配和驱逐的指标
- 触发副作用（例如，通知、日志记录）

插件如何注册回调的示例：
```go
// NamespaceAllocationTracker 是一个跟踪每个命名空间已分配 Pod 数量的插件。
type NamespaceAllocationTracker struct {
	podsPerNamespace map[string]int
}

func (nat *NamespaceAllocationTracker) OnSessionOpen(ssn *framework.Session) {
	nat.podsPerNamespace = map[string]int{}
	// 注册事件处理器。
	ssn.AddEventHandler(&framework.EventHandler{
		AllocateFunc: func(event *framework.Event) {
			if _, found := nat.podsPerNamespace[event.Task.Namespace]; !found {
				nat.podsPerNamespace[event.Task.Namespace] = 0
			}
			nat.podsPerNamespace[event.Task.Namespace]++
		},
		DeallocateFunc: func(event *framework.Event) {
			if _, found := nat.podsPerNamespace[event.Task.Namespace]; !found {
				nat.podsPerNamespace[event.Task.Namespace] = 0
			}
			nat.podsPerNamespace[event.Task.Namespace]--
		},
	})
}

func (nat *NamespaceAllocationTracker) OnSessionClose(ssn *framework.Session) {
	// 记录日志或发布到指标
}
```

这些回调可由插件用于维护其状态并在整个调度器生命周期中执行策略。
> 注意：截至目前，这些回调无法返回值或使操作失败——插件应跟踪相关更改以供内部使用。场景可以被其他函数（如 `ssn.AddReclaimableFn`）阻止。保持事件处理器精简高效。在其他函数中处理错误和繁重计算。

## 最佳实践

1. 保持评分函数轻量高效，因为它们在调度模拟期间被频繁调用。
2. 尽可能在 `OnSessionOpen` 中初始化状态并执行预计算，因为它每个周期只调用一次。

## 示例插件：Spot 实例管理

以下示例演示了一个通过以下方式管理 Spot 实例的插件：
1. 防止不可抢占的 Pod 被调度到 Spot 实例上
2. 降低 Spot 实例的评分，以增加其被释放用于扩缩的机会
3. 使用节点标签识别 Spot 实例

```go
type SpotInstancePlugin struct {
	// 配置参数
	spotLabelKey   string
	spotLabelValue string
	nonSpotScore   float64
}

func NewSpotInstancePlugin(args map[string]string) Plugin {
	return &SpotInstancePlugin{
		spotLabelKey:   "kai.scheduler/instance-type",
		spotLabelValue: "spot",
		nonSpotScore:   1000, // 非 Spot 实例获得更高的评分。有关其他插件使用的评分参考，请查看 scheduler/pkg/plugins/scores/scores.go
	}
}

func (sp *SpotInstancePlugin) Name() string {
	return "spot-instance-manager"
}

func (sp *SpotInstancePlugin) OnSessionOpen(ssn *Session) {
	// 注册谓词以防止不可抢占的 Pod 调度到 Spot 实例
	ssn.AddPredicateFn(func(task *pod_info.PodInfo, job *podgroup_info.PodGroupInfo, node *node_info.NodeInfo) error {
		// 忽略可抢占作业
        if job.IsPreemptibleJob() {
			return nil
		}

		// 检查节点是否为 Spot 实例
		isSpot := node.Node.Labels[sp.spotLabelKey] == sp.spotLabelValue
		if !isSpot {
			return nil
		}

		return fmt.Errorf("non-preemptible pod %s cannot be scheduled on spot instance %s",
			task.Name, node.Name)
	})

	// 注册评分函数以优先选择常规实例而非 Spot 实例
	ssn.AddNodeOrderFn(func(task *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error) {
		// 检查节点是否为 Spot 实例
		isSpot := node.Node.Labels[sp.spotLabelKey] == sp.spotLabelValue
		if isSpot {
			// 返回 0 作为评分，因此不提升 Spot 节点
			return 0, nil
		}

        // 返回 1000 的评分，这将提升非 Spot 节点
		return sp.nonSpotScore, nil
	})
}


func (sp *SpotInstancePlugin) OnSessionClose(ssn *Session) {
    // 本例中无需清理
}
```

### 使用方法

1. 使用 `kai.scheduler/instance-type=spot` 标记 Spot 实例
2. 在调度器配置中注册插件：

```go
RegisterPluginBuilder("spot-instance-manager", NewSpotInstancePlugin)
```

### 工作原理

- **谓词**：检查节点是否为 Spot 实例，并防止不可抢占的 Pod 被调度到其上
- **评分**：
  - 常规实例获得 1000 的评分（GpuSharing 常量）
  - Spot 实例获得 0 的评分，使其在调度时不太受青睐
  - 调度器汇总所有插件的评分，因此常规实例将优先于 Spot 实例
- **配置**：
  - 使用节点标签识别 Spot 实例（`kai.scheduler/instance-type=spot`）

此插件通过以下方式帮助管理 Spot 实例：
- 确保只有可抢占工作负载在 Spot 实例上运行
- 通过不提升其评分使 Spot 实例在调度时不太受青睐
- 通过配置参数提供灵活性

本文档涵盖了调度器插件机制的主要扩展点。有关特定插件实现或高级功能的更多详细信息，请参阅代码库示例和测试。
