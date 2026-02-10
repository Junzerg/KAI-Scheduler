# KAI Scheduler 项目总体分析

## 一、项目是什么

**KAI Scheduler** 是一个面向 AI/ML 工作负载的 **Kubernetes 调度器**，重点解决 GPU 等稀缺资源的分配和公平使用。它基于 [kube-batch](https://github.com/kubernetes-sigs/kube-batch)，支持大规模 GPU 集群和高吞吐调度。

## 二、解决什么问题

### 2.1 传统 kube-scheduler 的局限

- **单 Pod 调度**：不感知 Job、Gang 等 workload 语义
- **缺乏队列与配额**：难以做多租户、多团队资源隔离
- **GPU 支持有限**：对 GPU 共享、DRA、拓扑等支持不足
- **公平性弱**：难以实现 DRF、抢占、回收等策略

### 2.2 KAI Scheduler 的定位

- **Gang Scheduling**：同一 Job 的多个 Pod 要么一起调度，要么都不调度
- **队列与公平**：多级队列、配额、DRF、抢占、回收
- **GPU 优化**：GPU 共享、DRA、拓扑感知、binpack/spread 等
- **可扩展**：插件化、多 shard、与现有 kube-scheduler 并存

## 三、核心能力

| 能力 | 说明 |
|------|------|
| Batch / Gang 调度 | PodGroup 保证一组 Pod 同时调度 |
| 层级队列 | 父子队列、配额、优先级、limit |
| 公平分配 | DRF、抢占、回收、时间感知公平 |
| GPU 共享 | 多容器共享单卡或部分显存 |
| DRA | 通过 ResourceClaim 支持厂商 GPU |
| 拓扑感知 | 考虑 NVLink、NVSwitch 等拓扑 |
| 弹性 workload | 支持最小/最大副本数动态伸缩 |

## 四、与 Kubernetes 的关系

- 作为**扩展调度器**运行，`schedulerName: kai-scheduler`
- 不替代默认 kube-scheduler，可与其共存
- 通过 CRD（Queue、PodGroup、BindRequest 等）扩展 API
- 通过 webhook（admission）校验和改写 Pod

## 五、典型使用场景

- 多团队共享 GPU 集群，按队列分配配额
- 分布式训练（PyTorch、TensorFlow 等）需要 gang scheduling
- 推理服务需要 GPU 共享、拓扑优化
- 混合 CPU/GPU 工作负载的 binpack 或 spread 策略
