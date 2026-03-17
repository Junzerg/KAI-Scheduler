# CoPaw 连接飞书操作指南

CoPaw 已安装好后，按以下步骤在飞书开放平台创建应用并配置 CoPaw，即可在飞书中与 CoPaw 对话。飞书使用 **WebSocket 长连接** 收消息，**不需要公网 IP 或 webhook**。

---

## 一、飞书开放平台创建应用

1. 打开 **[飞书开放平台](https://open.feishu.cn/app)**，使用你的飞书账号登录。
2. 点击 **「创建企业自建应用」**。
3. 填写应用名称（如「CoPaw 助手」）、描述等，创建完成。
4. 进入应用后，在 **「凭证与基础信息」** 页面：
   - 复制 **App ID**（形如 `cli_xxxxxxxxxx`）。
   - 复制 **App Secret**（点击「显示」后复制）。  
   **先不要关闭页面**，后面填到 CoPaw 配置里要用。

---

## 二、配置 CoPaw 的 config.json

1. 打开 CoPaw 的配置文件（默认路径）：
   - **Windows**：`%USERPROFILE%\.copaw\config.json`（即 `C:\Users\你的用户名\.copaw\config.json`）
   - **macOS/Linux**：`~/.copaw/config.json`
   - 若安装时执行过 `copaw init`，该文件应已存在；若没有，可先执行一次 `copaw init` 生成。

2. 在 `config.json` 里找到 **`channels`** → **`feishu`**，改成下面这样（把 `app_id` 和 `app_secret` 换成你在飞书后台复制的值）：

   ```json
   "feishu": {
     "enabled": true,
     "bot_prefix": "[BOT]",
     "app_id": "cli_你的AppID",
     "app_secret": "你的AppSecret"
   }
   ```

3. 保存文件。  
   **注意**：`encrypt_key`、`verification_token`、`media_dir` 等在 WebSocket 长连接模式下可省略，不用填。

---

## 三、安装飞书依赖并启动 CoPaw

1. 安装飞书通道所需的 Python 包（若未装过）：
   ```bash
   pip install lark-oapi
   ```
   若本机使用 SOCKS 代理，再装：
   ```bash
   pip install python-socks
   ```

2. **先启动 CoPaw**（重要：必须先启动，后面飞书后台才能完成「长连接」配置）：
   ```bash
   copaw app
   ```
   保持该进程运行，不要关闭。

---

## 四、在飞书后台启用 Bot 与权限

回到 **飞书开放平台** → 你的应用：

1. **启用 Bot**  
   - 左侧进入 **「添加能力」** 或 **「功能」**，找到并开启 **「机器人」**（Bot）能力。

2. **配置权限与范围**  
   - 进入 **「权限管理」** 或 **「权限与范围」**。  
   - 找到 **「批量导入/导出权限」** 或类似入口，选择 **「批量导入」**，粘贴下面整段 JSON 后确认：

   ```json
   {
     "scopes": {
       "tenant": [
         "aily:file:read",
         "aily:file:write",
         "aily:message:read",
         "aily:message:write",
         "corehr:file:download",
         "im:chat",
         "im:message",
         "im:message.group_msg",
         "im:message.p2p_msg:readonly",
         "im:message.reactions:read",
         "im:resource",
         "contact:user.base:readonly"
       ],
       "user": []
     }
   }
   ```
   - 其中 `contact:user.base:readonly` 用于在会话和日志里显示用户昵称（否则只显示 open_id）。保存后等待权限生效。

3. **配置事件与回调（长连接）**  
   - 进入 **「事件与回调」** 或 **「事件订阅」**。  
   - 将 **订阅模式** 选为 **「通过长连接接收事件」**（无需公网 IP，也无需填 URL）。  
   - 点击 **「添加事件」**，搜索 **「接收消息」**，订阅 **「接收消息 v2.0」**（Message received v2.0）。  
   - 保存。

4. **发布版本**  
   - 进入 **「版本管理与发布」** 或 **「应用发布」**。  
   - 创建新版本，填写版本说明，提交审核或直接发布（企业自建应用一般可立即发布）。  
   - 发布成功后，机器人才会在飞书里可用。

---

## 五、在飞书中找到机器人并对话

1. 打开 **飞书客户端**（桌面或手机）。
2. 在顶部搜索框搜索你创建的应用名称（如「CoPaw 助手」）。
3. 在 **「功能」** 或 **「机器人」** 下找到该应用，点击进入即可与 CoPaw 对话。
4. （可选）在 **「工作台」** 里点 **「添加」** → 搜索机器人并加入收藏，方便以后从工作台直接打开。

---

## 常见问题

| 现象 | 处理建议 |
|------|----------|
| 收不到消息 / 机器人无响应 | 确认 **顺序**：先填好 `config.json` → 启动 `copaw app` → 再在飞书后台配置「长连接」并订阅「接收消息 v2.0」。顺序反了容易连不上。 |
| 提示未授权或权限不足 | 检查飞书应用是否已 **发布版本**，且权限里已包含上述 `im:message`、`im:message.p2p_msg:readonly` 等。 |
| 连接/网络错误 | 若使用代理，安装 `python-socks`；检查本机网络能否访问飞书 API。 |
| 仍报错 | 可先停止 `copaw app`，再重新执行一次 `copaw app` 后，再在飞书后台保存一次「长连接」配置。 |

---

---

## 附录 A：能否修改飞书 channel 的链接地址（API 域名）？

若你希望连接**飞书国际版（Lark）**（`open.larksuite.com`）或自建/代理的飞书兼容 API，需要改的是飞书开放平台的 **API 基础地址**。

### 当前情况

- CoPaw 官方 **config.json** 里飞书 channel 只有：`enabled`、`bot_prefix`、`app_id`、`app_secret`、`encrypt_key`、`verification_token`、`media_dir`，**没有** `domain`、`base_url` 或「链接地址」配置项。
- 飞书 channel 源码里把 `https://open.feishu.cn` 写死在鉴权、消息、联系人等请求里，**不能通过 config 或环境变量直接改**。

因此，**目前无法在 CoPaw 里通过配置「修改飞书 channel 的链接地址」**；默认只能连国内版飞书（open.feishu.cn）。

### 可行做法

1. **向 CoPaw 提需求**  
   在 [CoPaw GitHub Issues](https://github.com/agentscope-ai/CoPaw/issues) 提一个 Feature Request：为飞书 channel 增加可选配置项（例如 `domain` 或 `base_url`），用于指定开放平台地址（如 `https://open.feishu.cn` 或 `https://open.larksuite.com`），便于支持国际版或自定义端点。

2. **本地改源码（Fork）**  
   - 在 CoPaw 的 `FeishuConfig`（或等价配置）里增加字段，例如：`domain: str = "https://open.feishu.cn"`。  
   - 在飞书 channel 实现里，把所有写死的 `"https://open.feishu.cn"` 改为使用该配置（或从环境变量如 `FEISHU_DOMAIN` 读取，再写回 config）。  
   - 安装时用 `pip install -e /path/to/your/copaw-fork` 使用自己改过的版本。

3. **环境变量（若后续版本支持）**  
   若 CoPaw 后续版本支持通过环境变量覆盖飞书域名，可能会是类似 `FEISHU_DOMAIN=https://open.larksuite.com` 的形式；当前版本尚未提供，需以官方文档或源码为准。

**参考**：飞书国内版默认 `https://open.feishu.cn`，国际版（Lark）为 `https://open.larksuite.com`；底层 lark-oapi SDK 支持在创建 Client 时指定 `.domain()`，CoPaw 需在 channel 层暴露该能力。

### 只把协议改成 http（https → http）

若你只是要把 `https://open.feishu.cn` 改成 `http://open.feishu.cn`（例如走本地代理或内网 HTTP），可以**直接改 CoPaw 飞书 channel 源码**，无需加配置项。

1. **找到 CoPaw 安装位置**  
   - 用 pip 安装时，飞书 channel 一般在：  
     `Python 的 site-packages/copaw/app/channels/feishu/channel.py`  
   - 例如：`%USERPROFILE%\AppData\Local\Programs\Python\Python3xx\Lib\site-packages\copaw\app\channels\feishu\channel.py`（Windows），或 `venv/Lib/site-packages/copaw/app/channels/feishu/channel.py`。

2. **全文替换**  
   打开 `channel.py`，用编辑器的「全部替换」：  
   - **查找**：`https://open.feishu.cn`  
   - **替换为**：`http://open.feishu.cn`  
   保存。

3. **重启 CoPaw**  
   重启 `copaw app` 后，飞书相关请求会走 `http://open.feishu.cn`。

**注意**：pip 升级 CoPaw（`pip install -U copaw`）会覆盖修改，若需长期使用 http，建议 fork CoPaw 仓库，在 `src/copaw/app/channels/feishu/channel.py` 里做上述替换并用自己的 fork 安装（如 `pip install -e /path/to/copaw-fork`）。

---

## 附录 B：CoPaw 能否在 KAI-Scheduler 目录下「完整」跑 Cursor？

分两层看：

### 1. Cursor 侧：一旦被调用，能力是完整的

当执行  
`agent -p "<你的请求>" --workspace "D:\Projects\Fork\KAI-Scheduler"`  
时，Cursor CLI 会：

- 在 **KAI-Scheduler 目录** 下工作；
- 自动加载该目录的 **`.cursor/rules`**、**`AGENTS.md`**；
- 使用**完整能力**：读文件、改文件、跑命令、语义搜索、Plan/Ask 模式等。

所以，**在 KAI-Scheduler 目录下，Cursor 本身的能力是完整的**，和你在 Cursor IDE 里开发时一致。

### 2. CoPaw 侧：取决于能否「执行这条命令」

我们写的 **cursor_cli** 技能是**描述型**的：告诉 CoPaw「当用户说用 Cursor 做某事时，应该执行  
`agent -p "..." --workspace "<项目路径>"`  
并把输出返回」。

CoPaw 要真正在 KAI-Scheduler 下跑 Cursor，必须能**在你这台机器上执行上述 shell 命令**并拿到 stdout。这取决于：

- CoPaw 的 Agent 是否具备 **「运行系统命令」** 类工具（如 run_shell、bash、execute_command）；
- 若没有，CoPaw 只能“知道该这么做”，但无法真正执行，也就**不能**在飞书里完整触发 Cursor。

### 3. 如何验证

在飞书里对 CoPaw 说一句，例如：

- 「用 Cursor 在 KAI-Scheduler 目录下运行 `go test ./pkg/scheduler/...` 并把结果发给我」

并确保：

- 已安装本仓库的 **cursor_cli** 技能（复制到 `~/.copaw/customized_skills/`）；
- 已设置 **`KAI_SCHEDULER_ROOT`**（或 `COPAW_PROJECT_ROOT`）为 KAI-Scheduler 的绝对路径；
- 本机已安装并登录 **Cursor CLI**（`agent` 在 PATH 中）。

若 CoPaw **能执行并返回 Cursor 的测试输出**，说明当前环境已经可以在 KAI-Scheduler 目录下完整跑 Cursor；若 CoPaw 只回复“我无法执行命令”或仅给出口头说明而不实际运行，则说明 CoPaw 当前没有暴露「执行命令」的工具，需要查 CoPaw 文档/社区是否支持，或改用其他方式（例如用 **Cron 技能** 配置定时/手动任务，在任务里执行 `agent -p "..." --workspace "..."` 并把结果发到飞书）。

---

## 附录 C：修改 API 地址（OpenAI / 自定义接口）

若你选择了 **OpenAI 接口** 或 **自定义（Custom）** 作为模型提供商，需要改成自己的 API 地址（如代理、自建兼容 OpenAI 的接口）时，可以用下面两种方式。

### 方式一：通过控制台（推荐）

1. 确保 **CoPaw 已启动**：执行 `copaw app` 并保持运行。
2. 浏览器打开 **http://127.0.0.1:8088**（若启动时改了端口，用对应端口）。
3. 进入 **设置（Settings）** → **模型（Models）**。
4. 找到你当前使用的 **OpenAI / 自定义（custom）** 提供商，点击编辑。
5. 在 **Base URL**（或「API 基础地址」「base_url」）一栏填入你的 API 地址，例如：
   - 官方：`https://api.openai.com/v1`
   - 代理/自建：`https://your-proxy.com/v1` 或 `http://localhost:8080/v1`
6. 保存后，CoPaw 会使用新地址发起请求；无需重启（若未生效可重启一次 `copaw app`）。

### 方式二：通过命令行

在终端执行：

```bash
copaw models config-key custom
```

按提示依次输入：

- **Base URL**：你的 API 地址（如 `https://api.openai.com/v1` 或自建/代理地址，末尾通常带 `/v1`）。
- **API Key**：对应的密钥。

完成后，当前激活的模型若已是 custom 提供商，会直接使用新地址；若尚未激活 custom，可再执行 `copaw models set-llm` 选择该 custom 下的模型。

### 说明

- CoPaw 内置的「自定义」提供商 ID 为 `custom`，可设置任意 **base_url** 和 **api_key**，兼容 OpenAI 格式的接口（包括 Azure OpenAI、本地部署、国内代理等）。
- 修改后若仍报错，请检查：Base URL 是否可访问、是否以 `/v1` 结尾（视接口要求而定）、API Key 是否有效。

---

## 参考

- [CoPaw 官方文档 - Channels（飞书）](https://copaw.bot/docs/channels.html)
- [CoPaw 配置与工作目录（含 LLM Providers）](https://copaw.bot/docs/config.html)
- [CoPaw CLI - models](https://copaw.bot/docs/cli.html)
- [飞书开放平台](https://open.feishu.cn/app)
- 本仓库 [AI 助手集成方案 - CoPaw + 飞书](ai-assistant-integration.md#方案-a-1copaw--飞书推荐飞书--钉钉--qq-等国内渠道)
