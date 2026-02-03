# NodeSets

## 概述
本文档概述了对调度过程的增强，通过引入 NodeSets 概念，通过挂钩集群节点划分逻辑并在生成的集合上执行调度尝试，实现集群节点上 Pod 调度的新粒度。

拓扑感知调度是此类需求的良好示例——尝试在相同拓扑域下的节点集上调度作业。虽然此类功能可以在前置谓词和谓词钩子中实现，但多域不可行，限制了拓扑调度能力。

在本文档中，我们引入 `NodeSet` 的新概念，即调度器尝试在其上调度作业的节点集合。

## 动机
当前拓扑调度的实现仅限于单域尝试。这意味着当工作负载需要具有拓扑约束进行调度时，插件会搜索能够容纳的域（不保证作业可在其上调度）并尝试在该域上分配作业。如果尝试失败，它将不会寻找能够调度此[工作负载](https://github.com/NVIDIA/KAI-Scheduler/tree/main/docs/developer/designs/topology-awareness#stage-2-simulation-based-evaluation)的不同域。

不确定性来自拓扑插件只能做出尽力而为的决策，因为它不知道所有调度约束，如 NodeAffinity、NodeSelector、PVC 等。

## 目标
- 设计一个框架钩子，在拓扑调度时允许多域尝试
- 为未来的划分逻辑提供抽象
- 在多插件环境中定义清晰的行为
- 在 API（Kubernetes 事件）中定义用户体验

## 非目标
- 声明哪些插件将实现此类钩子
- 定义拓扑插件的内部逻辑

## 关键概念

- 调度框架——一组 API，供开发者在调度周期的多个决策点实现指定逻辑以扩展 KAI 的行为
- 插件——通过实现自定义逻辑扩展 KAI 调度器原生行为的代码片段
- 钩子——为插件开发者提供的扩展点，提供影响调度决策的多个点
- Session——表示调度周期的运行时对象，暴露用于激活已注册插件的函数
- 拓扑调度插件——实现拓扑调度逻辑的插件

## 高层设计

### NodeSet
引入新类型 `NodeSet`

```go
type NodeSet []*NodeInfo
```

此类型定义为零个或多个节点（单个 NodeSet 等于多个节点分组在一起）。

### SubsetNodes 钩子
引入新钩子，为给定工作负载返回 NodeSet 列表
```go
type SubsetNodesFn func(PodGroup) []NodeSet
```

此类函数接受工作负载作为参数，并提供多个 NodeSet 以尝试在其上调度。
通过实现此类函数，您可以将代码注册为 SubsetNodes 插件。

一个节点可以位于零个或多个 NodeSet 中。

## 定义的行为

### 注册
与任何其他钩子一样，开发者需要在编译前注册插件。
```go
framework.RegisterPluginBuilder("myPlugin", myPlugin.New)
```

此外，用户应将插件名称添加到调度器配置中。

### Session 函数
Session 对象暴露用于激活实现该钩子的插件的 API。

```go
func (ssn *Session) SubsetNodes(PodGroup) []NodeSet
```

此类函数在调度周期内可访问，只要 Session 存在，并暴露所有实现插件的逻辑。

在多个插件实现该钩子的情况下，预期行为是多层 SubSetting。
例如，如果您有 2 个插件 `P` 和 `R`。集群节点的划分分 2 步进行：

第一个插件 `P` 将所有集群节点划分为子集：
```
AllNodes -> P1, P2, P3
```
然后插件 R 划分每个子集：
```
P1 -> P1R1, P1R2
P2 -> P2R1, P2R2, P2R3
P3 -> P3R1, P3R2
```
因此子集的最终结果是：
```
P1R1, P1R2, P2R1, P2R2, P2R3, P3R1, P1R2
```


### 调度期间

在调度期间，当选择工作负载进行调度时，调度器算法在尝试分配任何任务之前调用该钩子，将集群节点划分为 NodeSet。

与传统行为相反，我们现在将尝试在每个 NodeSet 上调度工作负载，直到找到可以调度的 NodeSet，然后停止迭代。

```go
for podGroup := range PodGroups {
	nodeSets := ssn.SubsetNodes(podGroup)
	for nodeSet := range nodeSets {
		if attemptToAllocatePodGroup(podGroup, nodeSet) {
			break
        }   
    }
}
```
这意味着对于每个 PodGroup，我们执行划分，然后尝试每个 NodeSet，直到找到匹配的。

## 低层设计

```go
package NodeInfo

type NodeSet []*NodeInfo
```

```go
package api

type SubsetNodesFn func(job *podgroup_info.PodGroupInfo, tasks []*pod_info.PodInfo, nodeSet node_info.NodeSet) (bool, []node_info,NodeSet, error)
```

`tasks` 参数允许开发者在实际分配的情况下执行不同的逻辑（例如，拓扑插件需要仅计算待处理 Pod，或模拟期间虚拟驱逐的 Pod）。

返回值是一个布尔值，表示该插件是否与该作业相关（例如，拓扑要求仅在用户指定拓扑约束时相关）、NodeSet 列表和错误。

## 用户体验

### 开发者日志

在子集划分逻辑上，我们应记录（v4）每个插件子集划分结果的长度，在扩展日志（v7）中还应记录节点名称
```go
func SubsetNodes(PodGroup) {
	for plugin := range registeredPlugins {
		log.v4("Performing {plugin} logic")
		subsets = plugin()
        log.v4("Result of {plugin} logic is {len(subsets)} subsets")
        log.v7("Result of {plugin} logic is {for each subset [{for each node in subset.Nodes}]}")
    }
}
```

例如：
```
V4: Performing topology logic
V4: Result of topology logic is 3 subsets
V7: Result of topology logic is [Node1, Node2], [Node3, Node6], [Node4, Node5, Node7]
```

在运行插件失败时（`err != nil`），我们应记录失败以及插件名称和错误消息。

记录插件内部逻辑是开发者的责任。

### Kubernetes 事件

用户体验无变化，我们不想向用户暴露 NodeSets 概念。如果我们未能通过所有 NodeSet 调度作业，用户应看到 "unschedulable on cluster"。
