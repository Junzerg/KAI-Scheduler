# Phase 2.8: Go Embed 生产打包 (Single Binary Distribution) — 实施计划

**日期**: 2026-02-11
**前置依赖**: Phase 2.7 完成 (或可独立执行)
**范围**: 后端构建流程，独立闭环

---

## 概述

使用 Go 1.16+ 的 `embed` 特性，将 Angular 前端构建产物打包进 Scheduler 二进制文件中。
部署时只需分发一个可执行文件，无需单独部署前端静态资源。

**交付物**: 执行 `make build` 后生成包含前端 UI 的单一二进制文件，访问 `http://<scheduler>:8081/ui/` 即可使用控制台。

---

## 1. 前端构建配置

### 1.1 确认 Angular 输出路径

文件: `web/angular.json`

确认 `outputPath` 为 `dist/kai-scheduler-console`（或类似路径），构建产物将被嵌入此目录。

### 1.2 Base Href 配置

Angular 构建时需设置 `--base-href /ui/`，使 SPA 路由在子路径下正常工作：

```bash
ng build --configuration=production --base-href /ui/
```

---

## 2. Go Embed 实现

### 2.1 创建 embed 包

新建文件: `pkg/scheduler/webui/embed.go`

```go
package webui

import (
    "embed"
    "io/fs"
    "net/http"
)

//go:embed all:dist
var staticAssets embed.FS

// Handler returns an http.Handler that serves the embedded frontend assets.
// It also handles SPA fallback (serves index.html for unmatched routes).
func Handler() http.Handler {
    sub, _ := fs.Sub(staticAssets, "dist/kai-scheduler-console")
    fileServer := http.FileServer(http.FS(sub))

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Try serving the file; if not found, serve index.html for SPA routing
        fileServer.ServeHTTP(w, r)
    })
}
```

### 2.2 注册路由

文件: Scheduler HTTP server 注册处 (如 `pkg/scheduler/visualizer/visualizer_service.go` 或 `cmd/scheduler/main.go`)

```go
import "github.com/NVIDIA/kube-scheduler/pkg/scheduler/webui"

// 在已有的 API 路由注册处添加：
mux.Handle("/ui/", http.StripPrefix("/ui/", webui.Handler()))
```

---

## 3. 构建脚本

### 3.1 创建 Makefile target

文件: `Makefile` (新增或修改)

```makefile
.PHONY: build-frontend build-all

build-frontend:
	cd web && npm ci && npm run build -- --configuration=production --base-href /ui/
	rm -rf pkg/scheduler/webui/dist
	cp -r web/dist pkg/scheduler/webui/dist

build-all: build-frontend
	go build -o bin/kai-scheduler ./cmd/scheduler/

clean:
	rm -rf bin/ pkg/scheduler/webui/dist/ web/dist/
```

---

## 4. 文件变更清单

| 操作 | 文件 |
|:---|:---|
| 新增 | `pkg/scheduler/webui/embed.go` — embed.FS + HTTP handler |
| 新增 | `pkg/scheduler/webui/dist/` — 前端构建产物 (gitignore) |
| 修改 | HTTP server 路由注册 — 添加 `/ui/` 路径 |
| 修改 | `web/angular.json` — 确认 outputPath |
| 新增 | `Makefile` — `build-frontend` / `build-all` targets |
| 修改 | `.gitignore` — 添加 `pkg/scheduler/webui/dist/` |

---

## 5. 验收标准

- [ ] `make build-frontend` 成功构建前端并复制到 embed 目录
- [ ] `make build-all` 成功编译出包含前端的单一 Go 二进制文件
- [ ] 启动二进制后，浏览器访问 `http://localhost:8081/ui/` 可正常加载控制台
- [ ] SPA 路由正常工作 (如直接访问 `/ui/jobs` 不返回 404)
- [ ] `pkg/scheduler/webui/dist/` 目录已加入 `.gitignore`
