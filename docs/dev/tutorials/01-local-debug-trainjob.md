# 阶段一：本地 Debug 拦截 KAI-Scheduler 任务指南

本教程旨在帮助初学者从零开始，实现在本地开发环境中拦截并调试提交给 KAI-Scheduler 的训练任务。

## 前置条件

- 一台能连通测试 Kubernetes 集群的本地开发机
- KAI-Scheduler 的 Go 源码（入口在 `cmd/scheduler/main.go`）
- 测试集群已部署 KAI-Scheduler 全套组件（含 Operator、PodGrouper、Binder 等）
- 测试集群已有 `default-queue` 队列

---

## 第一步：配置 IDE 本地 Debug 环境

### VSCode 配置

在项目根目录下创建 `.vscode/launch.json`：

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug KAI-Scheduler",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/cmd/scheduler/main.go",
      "env": {
        "KUBECONFIG": "${env:HOME}/.kube/config"
      },
      "args": ["-v=3", "--scheduler-name=kai-scheduler"],
      "showLog": true
    }
  ]
}
```

### GoLand 配置

1. `Edit Configurations...` → 添加 `Go Build`
2. **Run kind**: `File`，**Files**: `cmd/scheduler/main.go`
3. **Environment**: `KUBECONFIG=/Users/junzerg/.kube/config;`
4. **Program arguments**: `-v=3 --scheduler-name=kai-scheduler`

### 参数解释

- `-v=3`：日志级别。3 会输出详细的调度过程日志，方便排查。
- `--scheduler-name=kai-scheduler`：**关键参数**。调度器只会处理声明了 `schedulerName: kai-scheduler` 的 Pod。

---

## 第二步：在核心源码中打下断点

### 拦截点 1：调度循环入口

- **文件**：`pkg/scheduler/scheduler.go`
- **函数**：`runOnce()`
- **断点位置**：`log.InfraLogger.V(1).Infof("Start scheduling ...")`
- **目的**：确认调度器的主循环是否在正常运转

### 拦截点 2：任务出队与分配

- **文件**：`pkg/scheduler/actions/allocate/allocate.go`
- **函数**：`Execute()` 和 `attemptToAllocateJob()`
- **断点位置**：`job := jobsOrderByQueues.PopNextJob()`
- **目的**：观察 PodGroup 如何从队列中被取出并尝试分配

### 拦截点 3：节点筛选与打分

- **文件**：`pkg/scheduler/plugins/predicates/predicates.go`
- **函数**：`evaluateTaskOnPredicates()`
- **断点位置**：`originalPodInfoNodeName := task.NodeName`
- **目的**：观察 Pod 如何被匹配到具体 Node

---

## 第三步：停掉集群正式调度器（⚠️ 必须做）

> **踩坑经验**：测试集群里通常已经部署了一套完整的 KAI-Scheduler。如果不停掉它，集群的"正式工"会抢先处理所有任务，你本地的 Debug 实例永远拿不到活。
>
> 另外，KAI-Operator 会自动拉起被停掉的组件，所以**必须先关 Operator，再关调度器**。

```bash
# 1. 先关管家（Operator），不让它自动恢复调度器
kubectl scale deployment kai-operator -n kai-scheduler --replicas=0

# 2. 再关调度器
kubectl scale deployment kai-scheduler-default -n kai-scheduler --replicas=0

# 3. 确认调度器已停
kubectl get pods -n kai-scheduler | grep kai-scheduler-default
# 应该没有任何输出
```

---

## 第四步：通过 IDE 启动本地调度器

按下 IDE 的 Debug 按钮（F5）。正常启动后，你会在控制台看到类似如下的循环日志：

```
Enter Allocate ...
There are <0> PodGroupInfos and <1> Queues in total for scheduling
Leaving Allocate ...
Enter Consolidation ...
...
End scheduling ...
```

这表示调度器已成功连接集群，正在每秒巡逻一次。`<0> PodGroupInfos` 表示还没有待处理的任务。

---

## 第五步：发射测试任务

### 核心架构知识（⚠️ 重要）

KAI-Scheduler 的调度逻辑**不直接认识 TrainJob**。真实链路是：

```
TrainJob → Training Operator(翻译成Pod) → PodGrouper(创建PodGroup) → Scheduler(处理PodGroup)
```

因为我们只启动了调度器，所以**必须手动创建 PodGroup 和 Pod**，才能触发断点。

### Python 脚本（`test_submit_pod.py`）

```python
from kubernetes import client, config

def submit_mock_pod_and_group():
    config.load_kube_config()

    custom_api = client.CustomObjectsApi()
    core_v1 = client.CoreV1Api()

    podgroup_name = "mock-kai-podgroup"

    # 1. 创建 PodGroup（调度器认的"档案单"）
    podgroup_manifest = {
        "apiVersion": "scheduling.run.ai/v2alpha2",
        "kind": "PodGroup",
        "metadata": {
            "name": podgroup_name,
            "namespace": "default"
        },
        "spec": {
            "minMember": 1,
            "queue": "default-queue"
        }
    }

    try:
        custom_api.create_namespaced_custom_object(
            group="scheduling.run.ai",
            version="v2alpha2",
            namespace="default",
            plural="podgroups",
            body=podgroup_manifest
        )
        print(f"✅ PodGroup [{podgroup_name}] 创建成功！")
    except Exception as e:
        print("⚠️ PodGroup 创建反馈 (可能已存在):", e)

    # 2. 创建 Pod（绑定到上面的 PodGroup）
    pod_manifest = {
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
            "name": "mock-kai-pod-0",
            "namespace": "default",
            "annotations": {
                "pod-group-name": podgroup_name  # 关联到 PodGroup
            }
        },
        "spec": {
            "schedulerName": "kai-scheduler",  # 指定调度器
            "containers": [
                {
                    "name": "node",
                    "image": "registry.k8s.io/pause:3.9",  # 极简镜像
                    "resources": {
                        "requests": {"cpu": "100m", "memory": "256Mi"}
                    }
                }
            ],
            "restartPolicy": "Never"
        }
    }

    try:
        core_v1.create_namespaced_pod(namespace="default", body=pod_manifest)
        print("🚀 Pod 发射成功！")
    except Exception as e:
        print("❌ Pod 提交失败:", e)

if __name__ == "__main__":
    submit_mock_pod_and_group()
```

运行后切回 IDE，断点应该会被触发。

---

## 清理与恢复

每次 Debug 完成后执行：

```bash
# 清理测试资源
kubectl delete pod mock-kai-pod-0 --ignore-not-found
kubectl delete podgroups.scheduling.run.ai mock-kai-podgroup -n default --ignore-not-found
kubectl delete trainjobs.trainer.kubeflow.org test-toy-kai-job -n default --ignore-not-found

# 恢复集群正式调度器
kubectl scale deployment kai-operator -n kai-scheduler --replicas=1
kubectl scale deployment kai-scheduler-default -n kai-scheduler --replicas=1
```

---

## 常见踩坑总结

| 问题                            | 原因                                        | 解决                                 |
| ------------------------------- | ------------------------------------------- | ------------------------------------ |
| `PodGroupInfos` 始终为 0        | Pod 已被集群调度器抢先处理                  | 停掉集群调度器后，清理旧资源重新发射 |
| 停掉调度器后它又自动恢复        | Operator 会自动拉起被停的组件               | 先停 Operator，再停调度器            |
| 直接提交 TrainJob 不触发断点    | 调度器不认 TrainJob，需要 PodGrouper 翻译   | 手动创建 PodGroup + Pod              |
| Pod 一直 ImagePullBackOff       | 测试集群无外网，拉不到镜像                  | 使用节点已有的镜像（如 pause）       |
| Python `load_kubeconfig()` 报错 | 正确写法是 `load_kube_config()`（有下划线） | 修正函数名                           |
