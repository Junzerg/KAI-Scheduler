# 实施计划 Phase 2.1: 前端工程搭建与仪表盘 (Dashboard) - Angular 14
**Status: COMPLETED** ✅

> **注意**: 本计划参考 Kubeflow 子项目依赖配置，锁定使用 **Angular 14** 技术栈，以确保与 Kubeflow 原生组件（如 Jupyter Web App）的完全兼容性。

---

## 1. 阶段目标

- [x] **环境锁定**: 使用 Angular CLI 14 初始化项目，确保依赖版本与 Kubeflow 官方组件一致。
- [x] **工程初始化**: 搭建 `web` 项目，配置 SCSS 和 Strict Mode。
- [x] **依赖同步**: 安装 `package.json` 中列出的核心依赖 (Angular Material 14, RxJS 7.4, etc.)。
- [x] **样式集成**: 配置 Angular Material 主题，尽量复用 Kubeflow 风格 (Indigo/Pink)。
- [x] **网络连通**: 配置 Proxy 转发 `/api` 请求。

---

## 2. 技术栈详细 (Reference Compliance)

所有版本号严格参考用户提供的 Reference `package.json`：

- **Framework**: Angular `~14.3.0`
- **CLI**: Angular CLI `~14.2.13`
- **UI Library**: Angular Material `~14.2.7` / CDK `~14.2.7`
- **RxJS**: `~7.4.0`
- **TypeScript**: `~4.7.0`
- **Icons**: `@fortawesome/angular-fontawesome`, `material-icons`
- **Utils**: `lodash` (`^4.17.21`), `date-fns` (`1.29.0`)

---

## 3. 详细实施步骤

### 3.1 步骤 1: Angular 14 项目初始化

1.  **使用特定版本 CLI 创建项目**:
    由于本地 Node 版本较新 (v22)，我们需要通过 `npx` 指定 CLI 版本来创建项目，避免使用最新版 CLI。
    ```bash
    # 使用 Angular CLI 14 创建项目
    npx -p @angular/cli@14.2.13 ng new web --routing --style=scss --strict --skip-git
    ```
2.  **降级/调整依赖 (如果需要)**:
    初始化后，检查 `package.json`，确保 `@angular/core` 等核心库版本并未意外升级到 15+。

### 3.2 步骤 2: 安装参考依赖

1.  **安装核心 UI 库**:
    ```bash
    npm install @angular/material@14.2.7 @angular/cdk@14.2.7
    npm install @angular/flex-layout@14.0.0-beta.41 # 布局辅助 (可选，Kubeflow 常用)
    ```
2.  **安装图标与工具库**:
    ```bash
    npm install @fortawesome/fontawesome-svg-core @fortawesome/free-solid-svg-icons @fortawesome/angular-fontawesome
    npm install material-icons
    npm install lodash @types/lodash date-fns
    ```
3.  **安装 K8s Client (可选)**:
    参考配置中有 `@kubernetes/client-node`，但在前端项目中通常使用 API 代理。如果是用于 Node.js 中间件层则安装，纯前端暂不安装，避免 Polyfill 问题。

### 3.3 步骤 3: 样式与主题配置

1.  **引入 Material 主题**:
    在 `src/styles.scss` 中配置：

    ```scss
    @use "@angular/material" as mat;
    @include mat.core();

    // 定义 Kubeflow 风格调色板 (Indigo/Pink)
    $console-primary: mat.define-palette(mat.$indigo-palette);
    $console-accent: mat.define-palette(mat.$pink-palette, A200, A100, A400);
    $console-warn: mat.define-palette(mat.$red-palette);

    $console-theme: mat.define-light-theme(
      (
        color: (
          primary: $console-primary,
          accent: $console-accent,
          warn: $console-warn,
        ),
      )
    );

    @include mat.all-component-themes($console-theme);
    ```

2.  **引入全局图标**:
    在 `src/styles.scss` 或 `angular.json` 中引入 Material Icons。

### 3.4 步骤 4: 网络层配置 (Proxy)

1.  **创建 Proxy 配置**:
    `src/proxy.conf.json`:
    ```json
    {
      "/api": {
        "target": "http://localhost:8081",
        "secure": false,
        "changeOrigin": true
      }
    }
    ```
2.  **更新 Scripts**:
    ```json
    "start": "ng serve --proxy-config src/proxy.conf.json"
    ```

### 3.5 步骤 5: 仪表盘开发

1.  **生成 Dashboard 组件**:
    ```bash
    npx -p @angular/cli@14.2.13 ng generate component pages/dashboard
    ```
2.  **实现布局**:
    模仿 Kubeflow Jupyter Web App 的布局结构。

---

## 4. 交付物与验收标准

1.  **版本一致性**: 项目 `package.json` 中的 Angular 版本必须为 `~14.x`。
2.  **成功运行**: `npm start` 能正常启动开发服务器，无版本冲突报错。
3.  **UI 风格**: 默认组件样式与 Kubeflow 一致 (Material Design)。
