from kubeflow.trainer import TrainerClient, CustomTrainer
import time


def train_fn():
    """典型的鸢尾花(Iris)数据集训练任务"""
    import os

    print("正在安装轻量级机器学习依赖...")
    os.system("pip install scikit-learn -q")

    from sklearn.datasets import load_iris
    from sklearn.model_selection import train_test_split
    from sklearn.ensemble import RandomForestClassifier
    from sklearn.metrics import accuracy_score

    print("1. 加载鸢尾花数据集...")
    iris = load_iris()
    X, y = iris.data, iris.target
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42
    )

    print("2. 开始训练测试模型 (RandomForest)...")
    clf = RandomForestClassifier(n_estimators=100)
    clf.fit(X_train, y_train)

    print("3. 模型评估...")
    predictions = clf.predict(X_test)
    acc = accuracy_score(y_test, predictions)
    print(f"✅ 训练完成！模型在测试集上的准确率为: {acc:.2%}")


def main():
    # 会自动读取你本地 ~/.kube/config 连上测试集群
    client = TrainerClient()

    # 这里不再需要笨重的 swift 图层资源，改为最轻量的通用 python Runtime
    # 即之前准备的 docs/dev/examples/toy-kai-runtime.yaml
    runtime_name = "toy-kai"
    print(f"🔍 获取轻量级 Runtime: {runtime_name} ...")
    runtime = client.get_runtime(runtime_name)

    # 构造 Trainer
    trainer = CustomTrainer(
        func=train_fn,
        num_nodes=1,
        resources_per_node={
            # 普通测试机只需要申请哪怕 1 个或者更少 CPU 核心即可完成调度
            "cpu": 1,
        },
    )

    # 提交 Job
    print("🚀 正在提交鸢尾花模型测试流水线...")
    job_id = client.train(
        trainer=trainer,
        runtime=runtime,
    )
    print(f"✅ 任务提交成功! 获取到的 Job ID: {job_id}")

    # 稍微等待一下生成状态
    time.sleep(2)

    # 查询任务运行状态
    print("\n📊 --- 任务运行状态 ---")
    try:
        for s in client.get_job(name=job_id).steps:
            print(
                f"Step: {s.name}, Status: {s.status}, Devices: {s.device} x {s.device_count}"
            )
    except Exception as e:
        print(f"获取状态遇到问题: {e}")

    # 跟随日志输出
    print("\n📝 --- 任务日志流 ---")
    print(
        "(注意: 若此时调度器在 IDE 断点处卡住，将会长时间无任何输出！因为 Pod 尚处于 Pending 状态。)\n"
    )
    try:
        for logline in client.get_job_logs(job_id, follow=True):
            print(logline)
    except Exception as e:
        print(f"捕获日志挂起: {e}")


if __name__ == "__main__":
    main()
