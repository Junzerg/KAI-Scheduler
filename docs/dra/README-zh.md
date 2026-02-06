# 动态资源分配 (DRA)

KAI Scheduler 通过其 [Binder 控制器](https://github.com/NVIDIA/KAI-Scheduler/blob/main/docs/developer/binder.md#binder) 支持 DRA，用于处理资源声明 (resourceclaims)。调度器和 Binder 之间通过名为 `BindRequest` 的自定义资源进行通信。当调度器决定 Pod 应该在哪里运行时，它会创建一个包含以下内容的 BindRequest 对象：

- 待调度的 Pod
- 选定的节点
- 资源分配信息（包括 GPU 资源）
- DRA（动态资源分配）绑定信息
- 重试设置

### 先决条件

- 必须按照 [此处的说明](https://github.com/NVIDIA/k8s-dra-driver-gpu/discussions/249) 安装 [NVIDIA k8s-dra-driver-gpu](https://github.com/NVIDIA/k8s-dra-driver-gpu)。该驱动程序利用上游 DRA API，通过 ComputeDomain CRD 支持 GB200 GPU 中可用的 NVIDIA Multi-Node NVLink，允许您定义可在工作负载中引用的资源模板。

- DRA 默认是禁用的。要启用它，请将以下标志添加到 helm install 命令中：

```
 --set scheduler.additionalArgs[0]=--feature-gates=DynamicResourceAllocation=true --set binder.additionalArgs[0]=--feature-gates=DynamicResourceAllocation=true
```

### DRA IMEX POD

要提交一个请求 IMEX 通道的 Pod，请运行此命令：

```
kubectl apply -f gpu-imex-pod.yaml
```

调度器将自动为请求的 IMEX 通道生成此 BindRequest

```bash
kubectl get BindRequest gpu-imex-pod-8g6vlrjxpp -o yaml

apiVersion: scheduling.run.ai/v1alpha2
kind: BindRequest
metadata:
  creationTimestamp: "2025-04-08T21:55:32Z"
  generation: 1
  labels:
    pod-name: gpu-imex-pod
    selected-node: NODE_NAME
  name: gpu-imex-pod-8g6vlrjxpp
  namespace: default
  ownerReferences:
  - apiVersion: v1
    kind: Pod
    name: gpu-imex-pod
    uid: 6306ffe2-a348-467a-b9aa-7176f2e95f53
  resourceVersion: "17426791"
  uid: 3c56aca5-027c-4bcd-9e9a-755e4c61ee8b
spec:
  podName: gpu-imex-pod
  receivedGPU:
    count: 1
    portion: "1.00"
  receivedResourceType: Regular
  resourceClaimAllocations:
  - allocation:
      devices:
        config:
        - opaque:
            driver: compute-domain.nvidia.com
            parameters:
              apiVersion: resource.nvidia.com/v1beta1
              domainID: 83479f70-e292-43d6-aa67-dd4ba8adab8f
              kind: ComputeDomainChannelConfig
          requests:
          - channel
          source: FromClaim
        results:
        - device: channel-0
          driver: compute-domain.nvidia.com
          pool: NODE_NAME
          request: channel
      nodeSelector:
        nodeSelectorTerms:
        - matchFields:
          - key: metadata.name
            operator: In
            values:
            - NODE_NAME
    name: imex-channel-0
  selectedNode: NODE_NAME
status:
  phase: Succeeded
```
