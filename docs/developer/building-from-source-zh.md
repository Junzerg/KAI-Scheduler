# 从源码构建

要从源码构建和部署 KAI Scheduler，请按以下步骤操作：

1. 克隆仓库：
   ```sh
   git clone git@github.com:NVIDIA/KAI-scheduler.git
   cd KAI-scheduler
   ```

2. 构建容器镜像，这些镜像将在本地构建（不会推送到远程仓库）
   ```sh
   make build
   ```
   如果要将镜像推送到私有 Docker 仓库，可以设置 DOCKER_REPO_BASE 变量：
   ```sh
   DOCKER_REPO_BASE=<REGISTRY-URL> make build
   ```

3. 打包 Helm Chart：
   ```sh
   helm package ./deployments/kai-scheduler -d ./charts
   ```
   
4. 确保镜像可从集群节点访问，可以通过将镜像推送到私有仓库或加载到节点缓存来实现。
   例如，可以使用以下命令将镜像加载到 kind 集群：
   ```sh
   for img in $(docker images --format '{{.Repository}}:{{.Tag}}' | grep kai-scheduler); 
      do kind load docker-image $img --name <KIND-CLUSTER-NAME>; done
   ```

5. 在集群上安装：
   ```sh
   helm upgrade -i kai-scheduler -n kai-scheduler --create-namespace ./charts/kai-scheduler-0.0.0.tgz
   ```
