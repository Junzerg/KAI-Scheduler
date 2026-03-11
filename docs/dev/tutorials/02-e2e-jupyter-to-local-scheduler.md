# 阶段二：端到端 Kubeflow TrainJob 到本地调度器的完整链路

在本篇教程中，我们将解析并打通从 Kubeflow Jupyter Notebook 发起训练任务（TrainJob），一直到你本地 IDE 中的 KAI-Scheduler 进行成功拦截与调度的完整生命周期。

## 1. 核心链路架构解析

KAI-Scheduler 并不是直接负责解析 `TrainJob` 的，它只认 `PodGroup`（一组需要被协同调度的 Pod的集合）。将 Notebook 里的代码变成 KAI-Scheduler 能理解的任务，需要经历以下完整链路：

```text
[1. 提交] Jupyter Notebook / Python SDK
   │
   ▼ (创建 TrainJob CR 资源)
[2. 翻译] Training Operator (Kubeflow 组件，监听 TrainJob)
   │
   ▼ (将其翻译转为 1 个或多个基础的 Pod，其中带有 schedulerName: kai-scheduler 和特定 label)
[3. 打包] KAI PodGrouper (KAI 组件，监听此类 Pod)
   │
   ▼ (自动收集同一批 Pod，为它们创建一个 PodGroup CR 资源)
[4. 调度] KAI-Scheduler (本地 IDE Debug 实例正在运行)
   │
   ▼ (发现未调度的 PodGroup，执行排序、分配资源、节点打分等核心算法)
[5. 运行] Node 节点
```

## 2. 环境诊断：缺失的 Training Operator Controller

**诊断结果：** 如果你的测试集群只安装了 `TrainJob` 的 CRD（Custom Resource Definition），这是**不够的**。CRD 只是在 Kubernetes 里注册了“有这么一种配置格式”，但缺乏实际干活的"工人"（Controller组件），这些配置只是僵死的数据，永远不会被转化为实际的 Pod。

要让 `TrainJob` 真正跑起来，测试集群**必须**运行着从属于 `kubeflow/training-operator` 的 Controller Pod。

### 生产环境平移部署方案 (离线)

你的同事反馈生产环境（也是离线的）已经跑通了。通过查看生产环境，我们发现 `training-operator` 运行在 `kubeflow` 命名空间下。最稳妥、最保证版本一致性的做法就是**直接从生产环境把配置和镜像平移到测试环境**。

> **提示:** 你可以在生产环境执行 `kubectl get deployments -n kubeflow training-operator` 来确认它的存在。

#### 步骤 A：从生产环境导出物料

在连通**生产环境**的终端（如你截图所在的 Node）执行：

1. **导出依赖资源的 YAML (并清理特定集群的乱码标签)**:

   ```bash
   # 1. 导出主体 Deployment，去除多余的状态字段
   kubectl get deployment training-operator -n kubeflow -o yaml | grep -v "creationTimestamp\|resourceVersion\|uid\|generation" > training-operator-deploy.yaml

   # 2. 获取 ConfigMap (可选资源，如果这里提示 Not Found，直接忽略跳过即可)
   kubectl get configmap training-operator-features -n kubeflow -o yaml | grep -v "creationTimestamp\|resourceVersion\|uid" > training-operator-cm.yaml || true

   # 3. 获取 ServiceAccount
   kubectl get serviceaccount training-operator -n kubeflow -o yaml | grep -v "creationTimestamp\|resourceVersion\|uid" > training-operator-sa.yaml

   # 4. 获取 ClusterRole (注：即使不在 kubeflow 命名空间，生产环境通常名字也叫 training-operator)
   kubectl get clusterrole training-operator -o yaml | grep -v "creationTimestamp\|resourceVersion\|uid" > training-operator-cr.yaml

   # 5. 获取 ClusterRoleBinding
   kubectl get clusterrolebinding training-operator -o yaml | grep -v "creationTimestamp\|resourceVersion\|uid" > training-operator-crb.yaml
   # 6. 获取 Webhook 证书对应的 Secret (非常关键，缺少会导致 Pod 挂载 Volume 失败)
   kubectl get secret training-operator-webhook-cert -n kubeflow -o yaml | grep -v "creationTimestamp\|resourceVersion\|uid" > training-operator-secret.yaml

   # 7. 获取 Service (供集群内部通信，Webhook 调用使用)
   kubectl get service training-operator -n kubeflow -o yaml | grep -v "creationTimestamp\|resourceVersion\|uid\|clusterIPs\|clusterIP" > training-operator-svc.yaml
   ```

2. **确认使用的具体镜像并导出**:

   ```bash
   # 查看生产使用的确切镜像名称（带地址前缀，如果是私有仓库）
   # 如果报错，可以手动打开前面导出的 training-operator-deploy.yaml 查找 `image:` 这行
   kubectl get deployment training-operator -n kubeflow -o=jsonpath='{.spec.template.spec.containers[0].image}'

   # 由于现代 Kubernetes(1.24+) 均已废弃 Docker 转用 Containerd，所以你机器上存在的 docker 命令可能只是个客户端空壳。
   # 我们需要使用 containerd 的客户端工具进行导出打包：

   # 方法 A (推荐): 使用 ctr 工具直接从 k8s.io 命名空间打包
   ctr -n k8s.io images export training-operator.tar <上面查到的镜像完整名称>

   # 方法 B: 或者使用更上层的 crictl 工具
   # crictl images | grep training-operator
   ```

#### 步骤 B：导入测试集群

1. 将导出的 `.yaml` 配置文件和 `training-operator.tar` 镜像包通过内网 SCP 等物理形式传输**测试环境集群**节点。
2. **导入镜像** (注意：必须在测试环境能调度 Pod 的真实宿主机节点(Node)上执行，而不仅仅是跳板机):
   ```bash
   # 使用 Containerd 导入（必须指定 -n k8s.io 否则 Kubernetes 无法加载）
   ctr -n k8s.io images import training-operator.tar
   ```
3. **应用配置**:

   ```bash
   # 确保 kubeflow 命名空间存在
   kubectl create namespace kubeflow --dry-run=client -o yaml | kubectl apply -f -

   # 依次拉起依赖和组件
   kubectl apply -f training-operator-sa.yaml
   # kubectl apply -f training-operator-cm.yaml # (如果上一步没导出则跳过这行)
   kubectl apply -f training-operator-secret.yaml
   kubectl apply -f training-operator-svc.yaml
   # ... 其他关联的 RBAC 资源
   kubectl apply -f training-operator-cr.yaml
   kubectl apply -f training-operator-crb.yaml

   # 最后应用部署本体
   kubectl apply -f training-operator-deploy.yaml
   ```

4. **验证状态**:
   ```bash
   kubectl get pods -n kubeflow | grep training-operator
   # 确保 Pod 是 Running 状态
   ```

---

## 3. 本地调度器接管检查清单

当上述链路组件（Training Operator 提供翻译，PodGrouper 提供打包）均已就绪并跑在集群上时，为了让**本地 IDE 里的 KAI-Scheduler 独占接管任务**，你只需做**两件事**：

> **核心原则：保留所有的"翻译官"和"打包员"，只干掉集群上的"正式调度组长"。**

### ✔️ 保留（必须 Running 的组件）

- **Training Operator** (负责生 Pod)
- **KAI PodGrouper** (负责生 PodGroup)
- **KAI Admission/Webhook 组件**

### ❌ 关停（必须缩容到 0 的组件）

- **KAI Operator** (防止它把下面的调度器进程又拉起来)
  ```bash
  kubectl scale deployment kai-operator -n kai-scheduler --replicas=0
  ```
- **KAI-Scheduler 默认部署** (避免和本地抢生意)
  ```bash
  kubectl scale deployment kai-scheduler-default -n kai-scheduler --replicas=0
  ```

只要集群里的官方调度器被干掉，你本地跑的 IDE KAI-Scheduler（带上了相同的 `--scheduler-name=kai-scheduler` 启动参数且连向了对应集群的 Kubeconfig）就会平滑地接管随后生成的一切带标签的 PodGroup 和 Pod，并成功触发你代码中打下的断点。

---

## 4. 端到端实战流程复现

接下来你可以在本地走通完整链路进行 Debug：

1. **环境停止**：确认正式环境的 `kai-scheduler-default` 和 `kai-operator` 已停止（参照上一节）。
2. **部署依赖 Runtime**：
   确保你的集群里已经部署了供调度器识别的 Runtime （例如项目目录中的 `toy-kai-runtime.yaml` ）。
   ```bash
   kubectl apply -f docs/dev/examples/toy-kai-runtime.yaml
   ```
   _（这个文件配置了 `schedulerName: kai-scheduler` 和相关的 `default-queue` 队列标签）_
3. **启动 Debug**：在 VSCode 或 GoLand 点击 Debug 运行本地 `cmd/scheduler/main.go`。
4. **在 Kubeflow Jupyter 中发射模型任务**：
   现在我们不再使用底层脚本，而是模拟最真实的算法工程师工作流：从 Kubeflow 的 Jupyter Notebook 中直接提交这个任务，让本地调度器跨越网络进行接管调度！由于我们没有测试厚重的镜像，还是使用轻量级的鸢尾花测试逻辑。

   **第一步：进入 Notebook 环境**
   1. 登录你的测试环境 Kubeflow Dashboard。
   2. 选择命名空间，导航到 `Notebooks` 菜单，连接并打开一个现有的 JupyterLab 实例。

   **第二步：安装依赖（在 Notebook Cell 中执行）**
   你需要安装 Kubernetes 与 Kubeflow SDK 的基础集成包，包含最新的 Trainer 模块：

   ```python
   !pip install kubeflow
   ```

   **第三步：编写训练策略与任务（在新的 Cell 中执行）**

   ```python
   from kubeflow.trainer import TrainerClient, CustomTrainer
   import time

   def train_fn():
       """将在 Kubernetes 集群内的独立 Pod 中实际执行的训练逻辑"""
       import os
       print("⚡️ 正在静默安装轻量级机器学习依赖...")
       os.system("pip install scikit-learn -q")

       from sklearn.datasets import load_iris
       from sklearn.model_selection import train_test_split
       from sklearn.ensemble import RandomForestClassifier
       from sklearn.metrics import accuracy_score

       print("1. 加载 Iris 鸢尾花数据集...")
       iris = load_iris()
       X, y = iris.data, iris.target
       X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)

       print("2. 训练 RandomForest 模型...")
       clf = RandomForestClassifier(n_estimators=100)
       clf.fit(X_train, y_train)

       print("3. 模型评估...")
       predictions = clf.predict(X_test)
       acc = accuracy_score(y_test, predictions)
       print(f"✅ 训练完成！准确率为: {acc:.2%}")

   # 初始化 Kubeflow Training 客户端
   # 注意：在 Notebook 内执行时，它会自动使用当前集群分配的 ServiceAccount 权限，无需做本地 ~/.kube/config 挂载映射！
   client = TrainerClient()

   # 使用极简容器环境 (需要你提前在集群层面应用过 docs/dev/examples/toy-kai-runtime.yaml)
   runtime = client.get_runtime("toy-kai")

   trainer = CustomTrainer(
       func=train_fn,
       num_nodes=1,
       resources_per_node={
           "cpu": "100m", # 对于小数据集测试，0.1 个核（100m）足够，避免集群资源耗尽
       }
   )

   print("🚀 提交 TrainJob 到集群...")
   job_id = client.train(
       trainer=trainer,
       runtime=runtime,
   )
   print(f"✅ 任务提交成功! 获取到的 Job ID: {job_id}")
   ```

   **第四步：查看日志阻塞与打点拦截（在另一个 Cell 中执行）**
   在同一个 Jupyter 中，执行后续查看状态和日志的单元格：

   ```python
   time.sleep(2)
   print("\n📊 --- 任务运行状态 ---")
   for s in client.get_job(name=job_id).steps:
       print(f"Step: {s.name}, Status: {s.status}, Devices: {s.device} x {s.device_count}")

   print("\n📝 --- 任务流式日志 ---")
   # 获取日志 (这里会阻塞等待，因为你的 IDE 断点拦截了集群进度！)
   for logline in client.get_job_logs(job_id, follow=True):
       print(logline)
   ```

   **第五步：见证奇迹的时刻**
   当你执行完第三个步骤后，Kubernetes 集群侧会发生：
   - -> [Jupyter Notebook 将 `train_fn` 代码打包并隐式拉起了一个 `TrainJob`]
   - -> [集群上的 Training Operator 监听到该任务]
   - -> [它创建了一个携带 `toy-kai` 标签且带 `schedulerName: kai-scheduler` 标识的空白 Pod]
   - -> [集群上的 KAI-PodGrouper 监听到该事件，敏锐地把它打包成 PodGroup CR]
   - -> **此时切回你的本地 IDE！原本一片寂静的控制台中断点将被高亮拦截！**

   你在本地 IDE 放行断点，为该请求算好分数调度到某个节点后，Jupyter Notebook 里刚才一直执行（阻塞）中状态的日志单元格，就会瞬间喷涌出刚刚真正的鸢尾花分类的执行日志。完成了一次完美的端到端接力调教！

祝 Debug 顺利！
