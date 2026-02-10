# 本地二开 / 调试环境初始化

适用于：WSL + Docker Desktop 3 节点集群（docker-desktop），日常二开、调试、测试（不跑 run-e2e-kind.sh）。

---

## 一、前置确认

- Docker Desktop 的 Kubernetes 已启动，且为当前 context。
- 在 WSL 里执行应能列出 3 个节点：
  ```bash
  kubectl get nodes
  ```

---

## 二、初始化（按顺序执行一次）

### 1. 安装 KAI-Scheduler

**方式 A：从 OCI 安装（推荐先跑通）**

到 [Releases](https://github.com/NVIDIA/KAI-Scheduler/releases) 看最新版本号，替换下面的 `<VERSION>` 后执行：

```bash
cd /home/junzerg/projects/KAI-Scheduler
helm upgrade -i kai-scheduler oci://ghcr.io/nvidia/kai-scheduler/kai-scheduler \
  -n kai-scheduler --create-namespace --version <VERSION>
```

**方式 B：从源码构建并安装（二开改代码时用）**

```bash
cd /home/junzerg/projects/KAI-Scheduler
make build
helm package ./deployments/kai-scheduler -d ./charts
# Docker Desktop 的 K8s 若底层是 kind，需把镜像 load 进集群（集群名多为 docker-desktop 或见 Docker Desktop 界面）
# for img in $(docker images --format '{{.Repository}}:{{.Tag}}' | grep kai-scheduler); do kind load docker-image $img --name <集群名>; done
helm upgrade -i kai-scheduler -n kai-scheduler --create-namespace ./charts/kai-scheduler-0.0.0.tgz
```

### 2. 确认 KAI 组件就绪

```bash
kubectl get pods -n kai-scheduler -w
# 等所有 Pod Running 后 Ctrl+C
```

### 3. （可选）安装 Kubeflow training-operator

若要跑 PyTorchJob / TFJob 等做调度验证：

```bash
./hack/third_party_integrations/deploy_kubeflow.sh
```

---

## 三、日常：运行 / 二开 / 调试 / 测试

1. **跑示例负载**（验证调度）
   - CPU：`kubectl apply -f docs/quickstart/pods/cpu-only-pod.yaml`
   - 需 GPU 时需先装 [NVIDIA GPU-Operator](https://github.com/NVIDIA/gpu-operator)，再：`kubectl apply -f docs/quickstart/pods/gpu-pod.yaml`
   - 注意：工作负载不要提交到 `kai-scheduler` namespace，用默认或自建 namespace；Pod 需带 `kai.scheduler/queue: default-queue` 且 `spec.schedulerName: kai-scheduler`。

2. **二开流程**
   - 改代码 → `make build` → 重新打 Helm 包并 load 镜像（若用方式 B）→ `helm upgrade -i ...` 更新部署 → 看 Pod 日志 / 事件排查。

3. **调试**
   - 看调度器：`kubectl logs -n kai-scheduler -l app.kubernetes.io/name=kai-scheduler -f`
   - 看其他组件：`kubectl get pods -n kai-scheduler` 后对对应 Pod 做 `kubectl logs -n kai-scheduler <pod> -f`。

4. **单元测试（不依赖集群）**
   - `make test` 或按 AGENTS.md 跑单包：`go test -v ./pkg/scheduler/actions/allocate/...`

5. **完整 E2E（可选）**
   - 需要时再跑：`./hack/run-e2e-kind.sh`（会临时建 kind 集群并跑完删掉）。

---

## 四、常用命令速查

| 目的           | 命令 |
|----------------|------|
| 看节点         | `kubectl get nodes` |
| 看 KAI Pod     | `kubectl get pods -n kai-scheduler` |
| 看队列         | `kubectl get queue -A` |
| 看默认队列     | `kubectl get -f docs/quickstart/default-queues.yaml` |
| 提交 CPU 示例   | `kubectl apply -f docs/quickstart/pods/cpu-only-pod.yaml` |
