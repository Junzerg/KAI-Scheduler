# Tutorial 06: 端到端调试排障指南 — 从七个 Bug 到鸢尾花绽放

> **前置条件**: 你已完成 Tutorial 01–05，了解 KAI-Scheduler 调度链路。
> **目标**: 通过一次真实的端到端调试历程，掌握 Kubeflow + KAI-Scheduler 链路中最常见的 7 类故障的诊断与修复方法。

---

## 1. 背景与架构回顾

本教程记录了一次从 Kubeflow Jupyter Notebook 提交 `TrainJob`，到 KAI-Scheduler 成功调度并完成鸢尾花分类训练任务的完整调试历程。整个过程中，我们依次遇到并解决了 **7 个不同层级的 Bug**，覆盖了控制器、资源管理、镜像、运行时框架、文件权限和环境变量等多个维度。

### 完整链路回顾

```
[Jupyter Notebook]  ──TrainJob──►  [Training Operator]  ──JobSet──►  [jobset-controller]
                                                                           │
                                                                     ──Pod──►  [KAI PodGrouper]
                                                                                     │
                                                                               ──PodGroup──►  [KAI-Scheduler]
                                                                                                    │
                                                                                              ──Bind──►  [Node 执行]
                                                                                                              │
                                                                                                        ✅ Completed
```

### Bug 全景图

| # | 层级 | 现象 | 根因 |
|---|------|------|------|
| 1 | 控制器 | jobset-controller panic 重启 | `spec.network` 字段为 nil |
| 2 | 调度器 | Pod 卡在 Pending (OutOfCpu) | CPU 请求量超过集群可用 |
| 3 | 容器运行时 | ImagePullBackOff | 镜像 registry 前缀不匹配 |
| 4 | 框架注入 | `torchrun: command not found` | torch 框架强制注入 torchrun |
| 5 | 解释器 | `python: not found` | 镜像内只有 `python3` |
| 6 | 文件系统 | `Permission denied` 脚本无法创建 | 默认用户无写权限 |
| 7 | 环境变量 | `pip/sklearn not found` | PATH 覆盖遗漏 conda 路径 |

---

## 2. Bug #1: jobset-controller Nil Pointer Panic

### 现象

`jobset-controller` Pod 反复 CrashLoopBackOff，日志中出现 panic：

```
goroutine 598 [running]:
sigs.k8s.io/jobset/pkg/webhooks.dnsHostnamesEnabled(...)
    /workspace/pkg/webhooks/jobset_webhook.go:267
panic: runtime error: invalid memory address or nil pointer dereference
```

提交的 `TrainJob` 永远无法创建出对应的 Pod。

### 诊断过程

```bash
# 1. 查看 jobset-controller 状态
kubectl get pods -n jobset-system
# → CrashLoopBackOff

# 2. 查看 panic 日志
kubectl logs -n jobset-system deployment/jobset-controller-manager --previous

# 3. 定位到 dnsHostnamesEnabled 函数源码
# → 发现它直接访问 jobSet.Spec.Network.EnableDNSHostnames 而没有做 nil 检查
```

### 根因分析

`jobset-controller` v0.10.1 的 `dnsHostnamesEnabled()` 函数存在 Bug：当 `JobSet.Spec.Network` 字段为 `nil` 时，直接解引用导致 panic。

**生产环境不出问题的原因**：生产环境部署了 `jobset-mutating-webhook`，每个 JobSet 创建时会被自动注入默认的 `network` 字段。测试环境缺少了这个 Webhook。

```
生产环境:  TrainJob → JobSet (webhook 自动注入 network:{}) → jobset-controller ✅
测试环境:  TrainJob → JobSet (没有 webhook, network=nil)   → jobset-controller 💥 panic
```

### 修复方案

**长期方案**: 从生产环境迁移 Webhook 配置到测试环境。

```bash
# 在生产环境导出
kubectl get mutatingwebhookconfigurations jobset-mutating-webhook-configuration -o yaml > jobset-mwh.yaml
kubectl get validatingwebhookconfigurations jobset-validating-webhook-configuration -o yaml > jobset-vwh.yaml
kubectl get secret jobset-webhook-server-cert -n jobset-system -o yaml > jobset-cert.yaml

# 在测试环境导入
kubectl apply -f jobset-cert.yaml
kubectl apply -f jobset-mwh.yaml
kubectl apply -f jobset-vwh.yaml

# 重启 controller 使其注册 webhook endpoint
kubectl rollout restart deployment/jobset-controller-manager -n jobset-system
```

> 💡 **经验教训**: 当控制器 panic 时，优先查看 panic 堆栈中的函数名和行号。生产环境能跑通但测试环境 panic，通常是缺少了 Webhook、Admission Controller 等「隐形组件」。

---

## 3. Bug #2: OutOfCpu — 资源请求超额

### 现象

Pod 提交后一直停留在 `Pending` 状态，KAI-Scheduler 日志中可以看到：

```
OutOfCpu: Insufficient cpu: task requires 1100m, but node only has 900m available
```

### 诊断过程

```bash
# 1. 查看 Pod 事件
kubectl describe pod <pod-name> -n kubeflow-user-example-com
# → Events 中有 OutOfCpu 相关信息

# 2. 查看节点可用资源
kubectl describe node <node-name> | grep -A 5 "Allocatable"
kubectl describe node <node-name> | grep -A 20 "Allocated resources"

# 3. 分析资源请求来源
# → 发现 Notebook 中的 Python SDK 代码请求了 cpu: 1（1000m）
# → 加上 Istio sidecar 的 100m overhead → 总共 1100m
# → 超过了节点可用的 CPU 余量
```

### 根因分析

资源请求涉及**两层配置**，而且后者会覆盖前者：

```
Layer 1: toy-kai-runtime.yaml 中定义的默认资源  →  cpu: "100m"  ✅ 合理
Layer 2: Python SDK 中 resources_per_node 参数   →  cpu: 1     ❌ 覆盖为 1000m！
```

Kubeflow SDK 的 `CustomTrainer(resources_per_node={"cpu": 1})` 优先级高于 ClusterTrainingRuntime 中的默认值。再加上 Istio sidecar 注入带来的额外开销（~100m），最终请求量超过了测试集群节点的可用余量。

### 修复方案

**同时修改两处**，确保不会互相覆盖：

1. **Runtime 侧** (`toy-kai-runtime.yaml`):

```yaml
resources:
  requests:
    cpu: "100m"      # 从 "1" 降到 "100m"
    memory: "256Mi"  # 从 "1Gi" 降到 "256Mi"
  limits:
    cpu: "200m"
    memory: "512Mi"
```

2. **SDK 侧** (Jupyter Notebook Cell):

```python
trainer = CustomTrainer(
    func=train_fn,
    num_nodes=1,
    resources_per_node={
        "cpu": "100m",  # 从 1 改为 "100m"，0.1 个核足够运行鸢尾花
    }
)
```

> 💡 **经验教训**: 资源不足时，用 `kubectl describe node` 查看真实的可用余量，用 `kubectl describe pod` 查看实际请求量。注意 Istio sidecar 等自动注入组件带来的隐性开销。**当 Runtime 和 SDK 都指定了资源时，SDK 的值优先。**

---

## 4. Bug #3: ImagePullBackOff — 镜像拉取失败

### 现象

Pod 被成功调度到节点上（状态变为 `Scheduled`），但随后进入 `ImagePullBackOff`：

```
Warning  Failed  kubelet  Failed to pull image "kubeflow/kubeflow/notebook-servers/jupyter-scipy:v1.10.0":
  failed to resolve reference "docker.io/kubeflow/kubeflow/notebook-servers/jupyter-scipy/manifests/v1.10.0":
  dial tcp 31.13.94.10:443: i/o timeout
```

### 诊断过程

```bash
# 1. 查看 Pod 事件
kubectl describe pod <pod-name> -n kubeflow-user-example-com | tail -n 15
# → Failed to pull image ... i/o timeout

# 2. 查看当前集群中 Notebook Pod 使用的真实镜像名
kubectl get pod <notebook-pod> -n kubeflow-user-example-com -o jsonpath='{.spec.containers[0].image}'
# → ghcr.io/kubeflow/kubeflow/notebook-servers/jupyter-scipy:v1.10.0
#   ^^^^^^^^ 注意这个 registry 前缀！
```

### 根因分析

问题出在 `toy-kai-runtime.yaml` 中的镜像名：

```yaml
# ❌ 错误: 缺少 registry 前缀
image: kubeflow/kubeflow/notebook-servers/jupyter-scipy:v1.10.0
# Docker 默认解析为 docker.io/kubeflow/kubeflow/...，但这个路径在 Docker Hub 上不存在

# ✅ 正确: 完整的 GHCR 路径
image: ghcr.io/kubeflow/kubeflow/notebook-servers/jupyter-scipy:v1.10.0
```

此外，即使节点上已经缓存了正确名字的镜像，Kubernetes 默认的 `imagePullPolicy` 也可能尝试去 registry 验证，在无公网的环境下导致超时。

### 修复方案

```yaml
containers:
  - name: node
    image: ghcr.io/kubeflow/kubeflow/notebook-servers/jupyter-scipy:v1.10.0  # 完整路径
    imagePullPolicy: IfNotPresent  # 本地有就不再拉取
```

**快速定位正确镜像名的方法**：

```bash
# 列出集群中所有正在使用的镜像
kubectl get pods -A -o jsonpath='{range .items[*]}{.spec.containers[*].image}{"\n"}{end}' | sort -u

# 或者直接抄一个正在运行的同类型 Pod 的镜像名
kubectl get pod <running-pod> -o jsonpath='{.spec.containers[0].image}'
```

> 💡 **经验教训**: 镜像名必须包含完整的 registry 域名前缀（如 `ghcr.io/`、`gcr.io/`、`registry.k8s.io/`）。在离线/无公网环境中，务必设置 `imagePullPolicy: IfNotPresent`。

---

## 5. Bug #4: `torchrun: command not found`

### 现象

Pod 启动后很快 CrashLoopBackOff，容器日志显示：

```
bash: line 36: 1152095799.py: Permission denied
bash: line 37: torchrun: command not found
```

### 诊断过程

```bash
# 查看容器日志
kubectl logs <pod-name> -n kubeflow-user-example-com -c node

# 查看 Training Operator 注入的启动命令
kubectl describe pod <pod-name> | grep -A 5 "Command:"
# → Command: bash -c read -r -d '' SCRIPT << EOM ...
# → 最终调用: torchrun --nproc_per_node=auto /path/to/script.py
```

### 根因分析

`toy-kai-runtime.yaml` 中标记了框架类型为 `torch`：

```yaml
metadata:
  labels:
    trainer.kubeflow.org/framework: torch  # ← 告诉 Training Operator: 这是 PyTorch 任务
spec:
  mlPolicy:
    torch:
      numProcPerNode: auto  # ← 配置分布式参数
```

Training Operator 看到 `framework: torch` 后，会**强制注入** `torchrun` 作为启动器。它会把你在 Notebook 里写的 `train_fn` 代码序列化成一个 `.py` 文件，然后用 `torchrun --nproc_per_node=auto /path/to/script.py` 来执行。

但我们选的 `jupyter-scipy` 镜像是一个**纯数学计算镜像，根本没有安装 PyTorch**，自然找不到 `torchrun` 命令。

**为什么不直接换 PyTorch 镜像？** 因为测试集群没有 PyTorch 镜像的本地缓存，且无公网下载。搜索结果为空：

```bash
kubectl get pods -A -o jsonpath='{.items[*].spec.containers[*].image}' | tr ' ' '\n' | sort -u | grep pytorch
# (无输出)
```

### 修复方案：狸猫换太子 — 伪造 torchrun

既然我们改不了 Training Operator 的注入行为，也拿不到 PyTorch 镜像，那就用一个**假的 `torchrun` 脚本**来拦截调用，偷偷把 `torchrun xxx.py` 替换成 `python3 xxx.py`。

在 `toy-kai-runtime.yaml` 中添加 `initContainer` 和 Volume：

```yaml
spec:
  # 1. 共享 Volume：存放伪造的 torchrun
  volumes:
    - name: fake-bin
      emptyDir: {}

  # 2. initContainer：生成假 torchrun 脚本
  initContainers:
    - name: fake-torchrun
      image: ghcr.io/kubeflow/kubeflow/notebook-servers/jupyter-scipy:v1.10.0
      imagePullPolicy: IfNotPresent
      command: ['bash', '-c']
      args:
        - |
          cat << 'EOF' > /opt/fake/torchrun
          #!/bin/bash
          # 忽略 torchrun 的分布式参数，直接找到最后一个 .py 文件并用 python3 运行
          SCRIPT=""
          for arg in "$@"; do
            if [[ "$arg" == *.py ]]; then
              SCRIPT="$arg"
            fi
          done
          echo "🔥 Faked torchrun intercepted! Executing: python3 $SCRIPT"
          chmod +x "$SCRIPT" 2>/dev/null || true
          exec python3 "$SCRIPT"
          EOF
          chmod +x /opt/fake/torchrun
      volumeMounts:
        - name: fake-bin
          mountPath: /opt/fake

  # 3. 主容器：将假 torchrun 目录放在 PATH 最前面
  containers:
    - name: node
      env:
        - name: PATH
          value: /opt/fake:/opt/conda/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
      volumeMounts:
        - name: fake-bin
          mountPath: /opt/fake
```

**工作原理**：

```
Training Operator 注入命令:  torchrun --nproc_per_node=auto /tmp/script.py
                                  │
                                  ▼
PATH 查找顺序:  /opt/fake/torchrun  ← 命中我们的假脚本！
                                  │
                                  ▼
假脚本执行: python3 /tmp/script.py  ← 直接用 python3 运行
```

> 💡 **经验教训**: 当框架的启动器不匹配实际环境时，可以通过 PATH 劫持的方式注入替代脚本。`initContainer + emptyDir Volume` 是 Kubernetes 中注入文件到主容器的经典模式。

---

## 6. Bug #5: `python: not found`

### 现象

假 torchrun 拦截成功了，但执行时报错：

```
🔥 Faked torchrun intercepted! Executing: python 1152095799.py
/opt/fake/torchrun: line 11: exec: python: not found
```

### 根因分析

`jupyter-scipy` 基于 `conda`，系统中只安装了 `python3`，没有创建 `python` 这个符号链接。这是现代 Linux 发行版的常见做法（PEP 394）。

```bash
# 在镜像内部
which python   → 找不到
which python3  → /opt/conda/bin/python3 ✅
```

### 修复方案

把假 torchrun 脚本中的 `python` 改为 `python3`：

```bash
# 修复前
exec python "$SCRIPT"

# 修复后
exec python3 "$SCRIPT"
```

> 💡 **经验教训**: 永远使用 `python3` 而不是 `python`。很多现代 Docker 镜像不再建 `python` → `python3` 的符号链接。

---

## 7. Bug #6: `Permission denied` — 脚本文件写入失败

### 现象

```
bash: line 36: 1152095799.py: Permission denied
🔥 Faked torchrun intercepted! Executing: python3 1152095799.py
python3: can't open file '//1152095799.py': [Errno 2] No such file or directory
```

观察到两个问题：
1. bash 尝试写文件时 `Permission denied`
2. 由于文件创建失败，python3 自然找不到

### 根因分析

Training Operator 生成的启动脚本大致如下：

```bash
# 将用户的 train_fn 代码写入文件
read -r -d '' SCRIPT << EOM
[用户代码]
EOM
echo "$SCRIPT" > 1152095799.py     # ← 写入当前工作目录
torchrun --nproc_per_node=auto 1152095799.py
```

关键在 `echo "$SCRIPT" > 1152095799.py` 这一步：它把代码写入**当前工作目录**。

`jupyter-scipy` 镜像的默认用户是 `jovyan` (UID 1000)，默认工作目录是 `/home/jovyan`。而在容器化环境中，`/home/jovyan` 目录可能是只读的、或者不存在的，导致写入失败。

### 修复方案

给容器指定一个确定有写权限的工作目录：

```yaml
containers:
  - name: node
    workingDir: /tmp  # /tmp 目录对所有用户都可写
```

> 💡 **经验教训**: 当容器内发生 `Permission denied` 文件写入错误时，优先检查容器的默认用户和工作目录。`/tmp` 是 Linux 中所有用户都可写的安全落脚点。

---

## 8. Bug #7: `pip/sklearn not found` — PATH 覆盖遗漏

### 现象

脚本终于能创建和执行了，但运行时报：

```
🔥 Faked torchrun intercepted! Executing: python3 1152095799.py
sh: 1: pip: not found
正在安装轻量级机器学习依赖...
ModuleNotFoundError: No module named 'sklearn'
```

明明 `jupyter-scipy` 镜像预装了 `sklearn` 和 `pip`，怎么会找不到？

### 根因分析

为了注入假 `torchrun`，我们手动覆盖了容器的 `PATH` 环境变量：

```yaml
env:
  - name: PATH
    # ❌ 遗漏了 /opt/conda/bin
    value: /opt/fake:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
```

`jupyter-scipy` 镜像中，`pip`、`python3`、`sklearn` 等所有包都安装在 `/opt/conda/bin/` 和 `/opt/conda/lib/` 下。我们覆盖 PATH 后把 conda 的路径从搜索列表中删除了，系统因此找不到 conda 环境中的任何命令。

### 修复方案

在 PATH 中加入 `/opt/conda/bin`：

```yaml
env:
  - name: PATH
    # ✅ 加入 /opt/conda/bin
    value: /opt/fake:/opt/conda/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    #               ^^^^^^^^^^^^^^ 加上这个！
```

> 💡 **经验教训**: 覆盖容器的 `PATH` 环境变量时，务必先查看镜像原始的 PATH 包含哪些目录。可以通过以下命令获取：
> ```bash
> kubectl exec <running-pod> -- env | grep PATH
> # 或在 Dockerfile 中查看 ENV PATH=...
> ```

---

## 9. 最终成功的完整配置

经过 7 轮迭代修复后，最终可用的 `toy-kai-runtime.yaml`：

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: toy-kai
  labels:
    trainer.kubeflow.org/framework: torch
spec:
  mlPolicy:
    numNodes: 1
    torch:
      numProcPerNode: auto
  template:
    spec:
      replicatedJobs:
        - groupName: default
          name: node
          replicas: 1
          template:
            metadata:
              labels:
                kai.scheduler/queue: default-queue
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  schedulerName: kai-scheduler

                  # [Bug #4 修复] 共享 Volume: 存放伪造的 torchrun
                  volumes:
                    - name: fake-bin
                      emptyDir: {}

                  # [Bug #4 修复] initContainer: 生成假 torchrun 脚本
                  initContainers:
                    - name: fake-torchrun
                      image: ghcr.io/kubeflow/kubeflow/notebook-servers/jupyter-scipy:v1.10.0
                      imagePullPolicy: IfNotPresent        # [Bug #3 修复]
                      command: ['bash', '-c']
                      args:
                        - |
                          cat << 'EOF' > /opt/fake/torchrun
                          #!/bin/bash
                          SCRIPT=""
                          for arg in "$@"; do
                            if [[ "$arg" == *.py ]]; then
                              SCRIPT="$arg"
                            fi
                          done
                          echo "🔥 Faked torchrun intercepted! Executing: python3 $SCRIPT"
                          chmod +x "$SCRIPT" 2>/dev/null || true
                          exec python3 "$SCRIPT"            # [Bug #5 修复] python → python3
                          EOF
                          chmod +x /opt/fake/torchrun
                      volumeMounts:
                        - name: fake-bin
                          mountPath: /opt/fake

                  containers:
                    - name: node
                      image: ghcr.io/kubeflow/kubeflow/notebook-servers/jupyter-scipy:v1.10.0  # [Bug #3 修复]
                      imagePullPolicy: IfNotPresent        # [Bug #3 修复]
                      workingDir: /tmp                     # [Bug #6 修复]
                      env:
                        - name: PATH                       # [Bug #4 + #7 修复]
                          value: /opt/fake:/opt/conda/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
                      volumeMounts:
                        - name: fake-bin
                          mountPath: /opt/fake
                      resources:                           # [Bug #2 修复]
                        requests:
                          cpu: "100m"
                          memory: "256Mi"
                        limits:
                          cpu: "200m"
                          memory: "512Mi"
                  restartPolicy: OnFailure
```

### Jupyter Notebook 提交代码

```python
trainer = CustomTrainer(
    func=train_fn,
    num_nodes=1,
    resources_per_node={
        "cpu": "100m",  # [Bug #2 修复] 从 1 降低到 100m
    }
)
```

### 成功结果

```
🔥 Faked torchrun intercepted! Executing: python3 1152095799.py
正在安装轻量级机器学习依赖...
1. 加载鸢尾花数据集...
2. 开始训练测试模型 (RandomForest)...
3. 模型评估...
✅ 训练完成！模型在测试集上的准确率为: 100.00%
```

---

## 10. 通用排障清单

当 TrainJob 提交后没有如预期运行时，按以下顺序逐步排查：

### Step 1: 确定 Pod 停在哪个阶段

```bash
kubectl get pods -n <namespace>
```

| Pod 状态 | 含义 | 跳转 |
|----------|------|------|
| 没有 Pod 出现 | TrainJob → JobSet → Pod 的翻译链路断了 | → 检查 Training Operator / jobset-controller 日志 |
| `Pending` | Pod 已创建但未被调度 | → Step 2 |
| `ImagePullBackOff` | 调度成功但镜像拉取失败 | → Step 3 |
| `Init:CrashLoopBackOff` | initContainer 崩溃 | → Step 4 |
| `CrashLoopBackOff` | 主容器启动后崩溃 | → Step 5 |
| `Running` 但无输出 | 容器在运行但卡住 | → Step 6 |
| `Completed` | 成功完成 ✅ | 🎉 |

### Step 2: Pending — 调度排查

```bash
# 查看调度失败原因
kubectl describe pod <pod> -n <ns> | grep -A 5 "Events"

# 查看节点资源余量
kubectl describe node <node> | grep -A 20 "Allocated resources"

# 查看 KAI-Scheduler 日志
kubectl logs deployment/kai-scheduler-default -n kai-scheduler | grep <pod-name>
```

常见原因: `OutOfCpu`、`OutOfMemory`、`NoMatchingNode`、`IsJobOverQueueCapacity`

### Step 3: ImagePullBackOff — 镜像排查

```bash
# 查看拉取错误详情
kubectl describe pod <pod> -n <ns> | tail -n 15

# 查找集群中已有的镜像名
kubectl get pods -A -o jsonpath='{range .items[*]}{.spec.containers[*].image}{"\n"}{end}' | sort -u | grep <keyword>
```

三板斧: (1) 补全 registry 前缀 (2) 设置 `imagePullPolicy: IfNotPresent` (3) 确认镜像在目标节点上存在

### Step 4: initContainer 崩溃

```bash
kubectl logs <pod> -n <ns> -c <init-container-name>
```

### Step 5: CrashLoopBackOff — 主容器排查

```bash
# 查看容器日志
kubectl logs <pod> -n <ns> -c node

# 查看注入的启动命令
kubectl describe pod <pod> -n <ns> | grep -A 10 "Command:"
```

常见原因: 命令找不到（`torchrun`）、权限不足（`Permission denied`）、依赖缺失（`ModuleNotFoundError`）

### Step 6: Running 但无输出

```bash
# 实时跟踪日志
kubectl logs <pod> -n <ns> -c node -f

# 进入容器内部排查
kubectl exec -it <pod> -n <ns> -c node -- bash
```

---

## 经验总结

1. **从现象到根因的调试顺序**: Pod 状态 → Events → 容器日志 → Describe 详情 → 控制器日志
2. **生产与测试环境差异**: 永远要检查 Webhook、Admission Controller、镜像 registry 等「隐形组件」是否对齐
3. **资源请求层级**: SDK 参数 > Runtime 默认值，注意 sidecar 的额外开销
4. **镜像兼容性**: 框架标签（如 `torch`）会强制注入特定启动器，镜像必须包含对应的工具
5. **环境变量覆盖陷阱**: 覆盖 `PATH` 时必须保留原镜像的关键路径（如 `/opt/conda/bin`）
6. **文件权限**: 容器默认用户可能不是 root，`workingDir` 必须指向可写目录

---

## 11. 实战：结合 PVC 为不同调度任务配置 TensorBoard 可视化

在真实的分布式训练集群中，监控模型训练的各项指标（如 Loss、Accuracy）至关重要。Kubeflow 提供了 `TensorBoard` CRD，让我们可以轻松拉起可视化的 TensorBoard 实例。

这里我们将演示如何使用 **单一的 PVC (共享存储)**，通过将不同训练任务的日志按 `job_id` （或任务名称）隔离，实现一次配置即可查看不同 KAI-Scheduler 调度任务结果的方法。

### 11.1 准备共享 PVC

首先，在 Kubernetes 中创建一个支持持久化的 PVC。如果在本地 K3d/Kind 测试环境，可以使用默认 StorageClass（比如 `local-path`）。

创建一个名为 `tb-logs-pvc.yaml` 的文件：

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: kai-tensorboard-pvc
  namespace: kubeflow-user-example-com
spec:
  accessModes:
    - ReadWriteOnce # 本地测试用 RWO 即可，生产环境如果是分布式存储可用 RWX
  resources:
    requests:
      storage: 5Gi
```

```bash
kubectl apply -f tb-logs-pvc.yaml
```

### 11.2 改造 Runtime：在训练任务中挂载 PVC

修改我们的 `toy-kai-runtime.yaml`，将被调度的 Pod 连上我们在第一步建立的 PVC。

```yaml
# 在 spec.template.spec.replicatedJobs[0].template.spec.template.spec 中添加挂载：
                  volumes:
                    - name: fake-bin
                      emptyDir: {}
                    # 新增: TensorBoard 日志存储卷
                    - name: tb-logs
                      persistentVolumeClaim:
                        claimName: kai-tensorboard-pvc

                  containers:
                    - name: node
                      volumeMounts:
                        - name: fake-bin
                          mountPath: /opt/fake
                        # 新增: 挂载到容器内的 /mnt/logs 目录
                        - name: tb-logs
                          mountPath: /mnt/logs
```

### 11.3 改造训练代码：按 Job 目录写入日志

在 Jupyter Notebook 提交的训练代码 `train_fn` 中，获取当前 Job 的 ID（可通过环境变量获取），并用它作为 TensorBoard 的子目录。因为所有任务的日志都写入了同一个 PVC 的 `/mnt/logs/` 根目录里，我们必须用子目录把它们隔离开。

```python
def train_fn():
    import os
    from torch.utils.tensorboard import SummaryWriter
    
    # 获取当前 Pod 的 HOSTNAME (通常形如 train-job-name-worker-0)
    # 取它的前缀作为 job_id (例如去掉后缀 -worker-0)
    pod_name = os.environ.get("HOSTNAME", "default-job")
    job_id = "-".join(pod_name.split("-")[:-2]) if "-" in pod_name else pod_name 
    
    # 日志输出路径形如 /mnt/logs/my-iris-training-job
    log_dir = f"/mnt/logs/{job_id}"
    print(f"写入 TensorBoard 日志到: {log_dir}")
    
    writer = SummaryWriter(log_dir=log_dir)
    
    # 模拟训练循环和写指标
    for epoch in range(10):
        # 你的实际训练/测试业务逻辑
        loss = 1.0 / (epoch + 1)
        writer.add_scalar('Loss/train', loss, epoch)
        
    writer.close()
```

这样，当调度 `Job A` 时，日志写入 `/mnt/logs/Job_A`；当调度 `Job B` 时，日志写入 `/mnt/logs/Job_B`。

### 11.4 拉起 TensorBoard 实例

日志准备就绪后，我们只需通过 Kubeflow 创建一个指向该 PVC 的 TensorBoard 实例。

创建 `my-tensorboard.yaml`:

```yaml
apiVersion: tensorboard.kubeflow.org/v1alpha1
kind: TensorBoard
metadata:
  name: kai-tensorboard
  namespace: kubeflow-user-example-com
spec:
  # 使用 pvc:// 协议指定 PVC 名称
  # 这里不指定具体的 job 子目录，而是直接指向 PVC 的根目录
  # 这样 TensorBoard 就能扫描该 PVC 下的所有子文件夹
  logspath: "pvc://kai-tensorboard-pvc/" 
```

```bash
kubectl apply -f my-tensorboard.yaml
```

**访问面板：**
- **通过 Dashboard**：在 Kubeflow 左侧菜单栏点击 `TensorBoards` 即可看到刚才创建的实例。点击 `CONNECT` 打开可视化页面。
- **通过命令行**：如果是本地测试没有完整的 Dashboard，也可以通过端口转发查看：
  ```bash
  kubectl port-forward svc/kai-tensorboard 6006:80 -n kubeflow-user-example-com
  ```
  然后在浏览器访问 `http://localhost:6006`。

### 11.5 效果展示
在打开的 TensorBoard 界面左侧的 `Runs` (运行) 列表中，你就会看到不同的 `job_id` 子目录。
- **对比与切换**：你可以方便地勾选/取消勾选不同 `job_id` 的指标，在同一张图表上对比超参搜索实验或者多次调度的结果。
- **自动刷新**：只要保证 PVC 的空间充足，后续每通过 KAI-Scheduler 下发一个新的训练任务，只需确保代码仍然向 `/mnt/logs/新的job_id` 中写入，TensorBoard 实例会**自动扫描**并发现新的日志文件，无需你重新创建 TensorBoard 资源。

---

## 下一步

- 练习: 尝试将 `framework: torch` 标签去掉，观察 Training Operator 的行为变化
- 练习: 在有 PyTorch 镜像的环境中，去掉假 torchrun 方案，使用原生 `torchrun` 运行真正的分布式训练
- 探索: 研究 Kubeflow SDK 的 `get_job_logs()` 为什么在自定义 Runtime 下无法流式获取日志
