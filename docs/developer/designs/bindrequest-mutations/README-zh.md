# BindRequest 注解变更插件点

## 摘要

本设计文档概述了 KAI-Scheduler 中的一个新插件点，允许调度器插件在 BindRequest 创建之前修改其注解。此增强解决了调度器与 Binder 组件之间的同步问题。

## 动机

KAI-Scheduler 的当前架构将调度和绑定过程分离到不同的组件中，这提高了可扩展性和错误恢复能力。然而，这些组件之间的通信目前仅限于 BindRequest 规范中定义的固定字段。

我们需要一个更开放、更灵活的 API，允许调度器中的插件向 Binder 中的插件传递额外信息，而无需为每个新用例修改 BindRequest CRD。这将实现更复杂的调度和绑定行为，同时保持组件之间的清晰分离。

通过允许调度器插件向 BindRequest 添加可由相应 Binder 插件解析的注解，我们创建了一个可扩展的通信通道，可以在不修改 API 的情况下演进。这种方法在保持向后兼容性的同时，通过插件实现新功能。

## 使用场景

### 节点状态同步

调度器中的插件检测到节点适合某些 GPU 工作负载，并向 BindRequest 添加注解，指示 Binder 中应由哪个插件处理绑定。如果该插件无法检测到相同的环境，可以尝试刷新 Binder 缓存中的节点状态，或拒绝绑定请求。

### 拓扑信息传递

对于拓扑感知调度，插件可以将精确的拓扑位置信息注入到 BindRequest 注解中。这允许每个 Pod 接收有关其在拓扑中位置的信息（与 block 实现相关），而无需向 BindRequest 规范添加字段。

### 设备管理插件通信

设备管理插件可以在调度器和 Binder 之间传递参数，而无需向 BindRequest 添加自定义字段，在保持 API 简洁的同时实现丰富的功能。

## 目标

- 使调度器插件能够在创建前修改 BindRequest 注解
- 创建灵活的接口，用于从调度器插件向 Binder 插件传递信息
- 保持与现有调度器和 Binder 行为的向后兼容性

## 设计细节

### 扩展点定义

遵循调度器的插件扩展约定，我们引入用于变更 BindRequest 注解的函数类型：

```go
// 在 pkg/scheduler/api/types.go
// BindRequestMutateFn 允许插件在 BindRequest 创建之前变更注解。
type BindRequestMutateFn func(pod *pod_info.PodInfo, nodeName string) map[string]string
```

这些函数的切片被添加到 `Session` 结构体，并提供一个注册方法：

```go
// 在 pkg/scheduler/framework/session.go 和 session_plugins.go
// 在 Session 结构体中：
BindRequestMutateFns []api.BindRequestMutateFn

// 注册方法：
func (ssn *Session) AddBindRequestMutateFn(fn api.BindRequestMutateFn) {
    ssn.BindRequestMutateFns = append(ssn.BindRequestMutateFns, fn)
}
```

### 插件注册

插件在 `OnSessionOpen` 期间注册其变更函数：

```go
func (p *MyPlugin) OnSessionOpen(ssn *framework.Session) {
    ssn.AddBindRequestMutateFn(p.MyBindRequestMutateFn)
}

func (p *MyPlugin) MyBindRequestMutateFn(pod *pod_info.PodInfo, nodeName string) map[string]string {
    bindRequestAnnotations := map[string]string{}
    bindRequestAnnotations["my-plugin.kai.scheduler/some-key"] = "some-value"
    return bindRequestAnnotations
}
```

### 注解合并与 Session 职责

Session 负责在将注解传递给缓存以创建 BindRequest 之前，收集并合并所有插件提供的注解。这是通过调用所有已注册的 `BindRequestMutateFn` 函数并将其结果合并到名为 `bindRequestAnnotations` 的单个 map 中来实现的。

如果多个插件提供相同的注解键，最后注册的插件的值将覆盖该键的先前值。

### 在调度器中的使用

创建 BindRequest 时，调度器将调用所有已注册的变更函数并合并其结果：

```go
// 在 session_plugins.go 中：
bindRequestAnnotations := map[string]string{}
for _, fn := range ssn.BindRequestMutateFns {
    for k, v := range fn(pod, nodeName) {
        bindRequestAnnotations[k] = v
    }
}
// ... 将 bindRequestAnnotations 传递给缓存以创建 BindRequest
```

### Binder 插件访问

Binder 插件在 PreBind 和 PostBind 阶段已经可以访问 BindRequest 对象，因此可以读取调度器插件添加的注解：

```go
func (p *MyBinderPlugin) PreBind(ctx context.Context, pod *v1.Pod, node *v1.Node, 
                                bindRequest *v1alpha2.BindRequest, state *state.BindingState) error {
    // 读取调度器插件添加的注解
    if value, exists := bindRequest.Annotations["my-plugin.kai.scheduler/some-key"]; exists {
        // 使用注解值修改绑定行为
    }
    return nil
}
```

### 注解命名约定

为避免不同插件之间的冲突，我们建议对注解键使用命名空间方式：

```
<plugin-name>.kai.scheduler/<key>
```

例如：
```
topology-plugin.kai.scheduler/topology-level: "rack"
gpu-plugin.kai.scheduler/requires-env-vars: "true"
```
