# Tutorial 07: 结合 PVC 为不同调度任务配置 TensorBoard 可视化

> **前置条件**: 你已完成 Tutorial 01–06，了解 KAI-Scheduler 调度链路与端到端调试方法。
> **目标**: 使用共享 PVC 和 Kubeflow TensorBoard CRD，为多个 KAI-Scheduler 调度的训练任务配置统一的可视化监控。

---

## 1. 背景

在真实的分布式训练集群中，监控模型训练的各项指标（如 Loss、Accuracy）至关重要。Kubeflow 提供了 `TensorBoard` CRD，让我们可以轻松拉起可视化的 TensorBoard 实例。

这里我们将演示如何使用 **单一的 PVC (共享存储)**，通过将不同训练任务的日志按 `job_id` （或任务名称）隔离，实现一次配置即可查看不同 KAI-Scheduler 调度任务结果的方法。

**完整架构链路：**

```
Jupyter Notebook (提交训练代码)
    ↓ TrainerClient.train()
KAI-Scheduler (调度 Pod)
    ↓ 分配节点资源
训练 Pod 运行
    ↓ tensorboardX 写日志到 /mnt/logs/{job_id}
PVC (kai-tensorboard-pvc) 持久化存储
    ↑ TensorBoard Pod 挂载读取
TensorBoard Dashboard (可视化曲线)
```

---

## 2. 准备共享 PVC

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

> **💡 关于 PVC 状态：** 如果你的 StorageClass 使用 `WaitForFirstConsumer` 绑定模式（如 `local-path`），PVC 创建后会处于 `Pending` 状态，这是**正常的**——它会在第一个 Pod 使用它时自动绑定。

---

## 3. 改造 Runtime：在训练任务中挂载 PVC

修改 `toy-kai` ClusterTrainingRuntime，将被调度的 Pod 连上我们在第一步建立的 PVC。

```bash
kubectl edit clustertrainingruntime toy-kai
```

在 `spec.template.spec.replicatedJobs[0].template.spec.template.spec` 中添加挂载：

```yaml
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

**验证修改生效：**

```bash
kubectl get clustertrainingruntime toy-kai -o yaml | grep -A 3 "tb-logs"
```

应该能看到 `tb-logs` 相关的 volume 和 volumeMount。

---

## 4. 改造训练代码：按 Job 目录写入日志

在 Jupyter Notebook 提交的训练代码 `train_fn` 中，获取当前 Job 的 ID（通过环境变量 `HOSTNAME` 获取，Kubernetes 会自动将其设为 Pod 名称），并用它作为 TensorBoard 的子目录。因为所有任务的日志都写入了同一个 PVC 的 `/mnt/logs/` 根目录里，我们必须用子目录把它们隔离开。

> **⚠️ 注意事项：**
> - 测试环境的 `jupyter-scipy` 镜像没有 PyTorch，因此不能使用 `torch.utils.tensorboard.SummaryWriter`。我们使用 `tensorboardX` 作为替代，它不依赖 PyTorch 且完全兼容 TensorBoard 格式。
> - 获取 Job ID 应使用 `HOSTNAME` 而非自定义环境变量 `POD_NAME`，因为 `HOSTNAME` 是 Kubernetes 自动设置的。
> - 离线或网络受限环境中 `pip install` 可能因 SSL 失败，需指定国内镜像源。

### 示例 1：Iris 鸢尾花分类（RandomForest）

```python
def train_fn():
    """Iris 鸢尾花分类 - RandomForest, 调参 n_estimators"""
    import subprocess, os, time
    subprocess.run([
        "pip", "install", "-q",
        "--trusted-host", "pypi.tuna.tsinghua.edu.cn",
        "-i", "https://pypi.tuna.tsinghua.edu.cn/simple/",
        "tensorboardX", "scikit-learn"
    ])

    from sklearn.datasets import load_iris
    from sklearn.ensemble import RandomForestClassifier
    from sklearn.model_selection import train_test_split
    from tensorboardX import SummaryWriter

    # HOSTNAME 由 Kubernetes 自动设置为 Pod 名称 (如 zae9d225d430-node-0-0-xxxxx)
    # 取 "-node" 前面的部分作为 Job ID
    job_id = os.environ.get("HOSTNAME", "unknown").split("-node")[0]
    log_dir = f"/mnt/logs/{job_id}"
    print(f"📊 TensorBoard 日志目录: {log_dir}")

    writer = SummaryWriter(log_dir)

    iris = load_iris()
    X_train, X_test, y_train, y_test = train_test_split(
        iris.data, iris.target, test_size=0.3, random_state=42
    )

    for epoch, n_est in enumerate(range(10, 60, 5)):
        clf = RandomForestClassifier(n_estimators=n_est, random_state=42)
        clf.fit(X_train, y_train)
        acc = clf.score(X_test, y_test)

        writer.add_scalar("Accuracy/test", acc, epoch)
        writer.add_scalar("HyperParam/n_estimators", n_est, epoch)
        writer.flush()  # 立即写入磁盘，TensorBoard 实时可见
        print(f"  Epoch {epoch}: n_estimators={n_est}, accuracy={acc:.4f}")
        time.sleep(5)   # 每个 epoch 等 5 秒，方便在 TensorBoard 观察实时曲线

    writer.close()
    print(f"✅ 训练完成！日志已写入 {log_dir}")
```

### 示例 2: 手写数字识别（KNN 分类器）

```python
def train_digits_fn():
    """手写数字识别 - KNN 分类器，调参 n_neighbors（实时 TensorBoard 可视化版）"""
    import subprocess, os, time
    subprocess.run([
        "pip", "install", "-q",
        "--trusted-host", "pypi.tuna.tsinghua.edu.cn",
        "-i", "https://pypi.tuna.tsinghua.edu.cn/simple/",
        "tensorboardX", "scikit-learn"
    ])

    from sklearn.datasets import load_digits
    from sklearn.model_selection import train_test_split
    from sklearn.neighbors import KNeighborsClassifier
    from sklearn.metrics import accuracy_score, f1_score
    from tensorboardX import SummaryWriter

    # HOSTNAME 由 Kubernetes 自动设置为 Pod 名称
    job_id = os.environ.get("HOSTNAME", "unknown").split("-node")[0]
    log_dir = f"/mnt/logs/{job_id}"
    print(f"📊 TensorBoard 日志目录: {log_dir}")

    writer = SummaryWriter(log_dir)

    # 加载手写数字数据集 (8x8 图像, 0-9 共 10 类)
    digits = load_digits()
    X_train, X_test, y_train, y_test = train_test_split(
        digits.data, digits.target, test_size=0.3, random_state=42
    )

    # 遍历不同的 K 值
    for epoch, k in enumerate(range(1, 16)):
        clf = KNeighborsClassifier(n_neighbors=k)
        clf.fit(X_train, y_train)

        train_acc = accuracy_score(y_train, clf.predict(X_train))
        test_acc = accuracy_score(y_test, clf.predict(X_test))
        f1 = f1_score(y_test, clf.predict(X_test), average='weighted')

        writer.add_scalar("Accuracy/train", train_acc, epoch)
        writer.add_scalar("Accuracy/test", test_acc, epoch)
        writer.add_scalar("Metrics/f1_score", f1, epoch)
        writer.add_scalar("HyperParam/n_neighbors", k, epoch)
        writer.flush()  # 立即写入磁盘，TensorBoard 实时可见

        print(f"  Epoch {epoch}: K={k}, train_acc={train_acc:.4f}, "
              f"test_acc={test_acc:.4f}, f1={f1:.4f}")
        time.sleep(5)   # 每个 epoch 等 5 秒，方便在 TensorBoard 观察实时曲线

    writer.close()
    print(f"✅ 训练完成！日志已写入 {log_dir}")
```

### 提交任务（两个示例通用代码）

```python
from kubeflow.trainer import TrainerClient, CustomTrainer

client = TrainerClient()
runtime = client.get_runtime("toy-kai")

trainer = CustomTrainer(
    func=train_fn,           # 或 train_digits_fn
    num_nodes=1,
    resources_per_node={"cpu": "100m"},
)

print("🚀 提交 TrainJob...")
job_id = client.train(trainer=trainer, runtime=runtime)
print(f"✅ 任务提交成功！Job ID: {job_id}")
```

---

## 5. 拉起 TensorBoard 实例

日志准备就绪后，我们只需通过 Kubeflow 创建一个指向该 PVC 的 TensorBoard 实例。

### 方法一：通过 Kubeflow Dashboard（推荐）

1. 在 Kubeflow 左侧菜单点击 **TensorBoards**
2. 点击右上角 **+ New TensorBoard**
3. 填写 Name（如 `tb-kai-job-viewer`）和 PVC（选择 `kai-tensorboard-pvc`）
4. 状态变为绿色 ✅ 后，点击 **CONNECT**

### 方法二：通过 kubectl

创建 `my-tensorboard.yaml`：

```yaml
apiVersion: tensorboard.kubeflow.org/v1alpha1
kind: Tensorboard
metadata:
  name: tb-kai-job-viewer
  namespace: kubeflow-user-example-com
spec:
  logspath: "pvc://kai-tensorboard-pvc"
```

> **⚠️ 注意 `logspath` 末尾不能有多余的斜杠 `/`！**
> - ✅ 正确：`pvc://kai-tensorboard-pvc`
> - ❌ 错误：`pvc://kai-tensorboard-pvc/` 或 `pvc://kai-tensorboard-pvc//`
>
> 多余的 `/` 会导致 Kubernetes volumeMount 的 subPath 变成 `/`（绝对路径），报错：
> `Invalid value: "/": must be a relative path`

```bash
kubectl apply -f my-tensorboard.yaml
```

---

## 6. 效果展示

在打开的 TensorBoard 界面左侧的 `Runs` (运行) 列表中，你就会看到不同的 `job_id` 子目录。
- **对比与切换**：你可以方便地勾选/取消勾选不同 `job_id` 的指标，在同一张图表上对比超参搜索实验或者多次调度的结果。
- **自动刷新**：只要保证 PVC 的空间充足，后续每通过 KAI-Scheduler 下发一个新的训练任务，只需确保代码仍然向 `/mnt/logs/新的job_id` 中写入，TensorBoard 实例会**自动扫描**并发现新的日志文件，无需你重新创建 TensorBoard 资源。

---

## 7. 常见问题排查（踩坑记录）

### 7.1 PVC 状态一直是 Pending

**现象：** PVC 创建后状态显示 `Pending`。

**原因：** 使用 `local-path` StorageClass 的 `WaitForFirstConsumer` 绑定模式，PVC 只有在 Pod 第一次使用时才会绑定。

**处理：** 这是**正常行为**，无需处理。提交训练任务后，Pod 启动时 PVC 会自动变成 `Bound`。

### 7.2 SDK 包冲突：kubeflow-training vs kubeflow-trainer

**现象：** `from kubeflow.trainer import TrainerClient` 报 `ImportError`。

**原因：** 系统中同时安装了旧版 `kubeflow-training`（v1 SDK）和新版 `kubeflow-trainer`（v2 SDK），两者冲突。

**修复：**

```bash
pip uninstall kubeflow-training -y
pip install kubeflow
```

> `pip install kubeflow` 会自动安装 `kubeflow-trainer`（v2 SDK），其中包含 `TrainerClient` 和 `CustomTrainer`。

### 7.3 typing_extensions 版本不兼容

**现象：** `ImportError: cannot import name 'Sentinel' from 'typing_extensions'`

**原因：** `pydantic` v2.12+ 需要较新版本的 `typing_extensions`（包含 `Sentinel` 类型），而环境中的版本过旧。

**修复：**

```bash
pip install typing_extensions --upgrade
```

⚠️ 升级后**必须重启 Notebook Kernel**（菜单 → Kernel → Restart Kernel），否则旧版模块缓存不会刷新。

### 7.4 TensorBoard Controller 镜像拉取失败

**现象：** TensorBoard CR 创建后，Dashboard 上一直转圈（不变成绿色 ✅），且没有对应的 Pod 被创建。

**排查步骤：**

```bash
# 1. 检查 TensorBoard Controller 状态
kubectl get pods -A | grep -i tensorboard

# 2. 如果 controller pod 显示 ImagePullBackOff，查看缺失的镜像
kubectl describe pod <controller-pod-name> -n kubeflow | grep -E "Image:|Back-off"
```

**常见缺失镜像：** `quay.io/brancz/kube-rbac-proxy:v0.8.0`

**修复（离线环境）：** 从生产环境导出镜像并导入到测试环境：

```bash
# 生产环境
ctr -n k8s.io images export kube-rbac-proxy.tar quay.io/brancz/kube-rbac-proxy:v0.8.0

# 传输到测试环境后导入
ctr -n k8s.io images import kube-rbac-proxy.tar

# 删除坏的 Pod，让 Deployment 自动重建
kubectl delete pod <controller-pod-name> -n kubeflow
```

### 7.5 TensorBoard logspath 格式错误

**现象：** Controller 日志报错 `volumeMounts.subPath: Invalid value: "/": must be a relative path`

**原因：** `logspath` 末尾有多余的 `/`（如 `pvc://kai-tensorboard-pvc//`），导致 subPath 被解析为 `/`。

**修复：** 删除并重新创建 TensorBoard CR，去掉末尾斜杠：

```bash
kubectl delete tensorboard <name> -n kubeflow-user-example-com

cat <<'EOF' | kubectl apply -f -
apiVersion: tensorboard.kubeflow.org/v1alpha1
kind: Tensorboard
metadata:
  name: tb-kai-job-viewer
  namespace: kubeflow-user-example-com
spec:
  logspath: "pvc://kai-tensorboard-pvc"
EOF
```

### 7.6 训练 Pod 卡在 Pending

**现象：** 训练 Pod 一直处于 `Pending` 状态。

**可能原因：**
1. **调度器未运行：** 如果在 Tutorial 02 中将 `kai-scheduler-default` 缩容到 0，需要恢复：
   ```bash
   kubectl scale deployment kai-scheduler-default -n kai-scheduler --replicas=1
   ```
2. **资源不足：** 集群资源被其他任务占用，需等待或清理。

---

## 下一步

- 探索: 尝试使用 `ReadWriteMany (RWX)` 的 PVC，支持多节点分布式训练同时写入日志
- 探索: 配置 TensorBoard 的 `--samples_per_plugin` 参数，优化大规模日志加载性能
- 探索: 结合 Kubeflow Pipelines，在 Pipeline 中自动创建 TensorBoard 实例
