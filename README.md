# Restify

macOS 菜单栏休息与喝水提醒应用：工作计时到期强制全屏休息，定时提醒喝水并记录每日进度。基于 [mygo](https://github.com/egoist/mygo)（Go 原生 UI）与 [Lobe MyGo](https://github.com/lobehub/lobe-mygo)（Lobe UI 设计语言）构建，单二进制，无第三方运行时依赖。

## 功能

- **工作计时**：可配时长（默认 25 分钟）与休息时长（默认 5 分钟）；到期全屏休息遮罩倒计时，结束自动恢复；空闲 5 分钟自动暂停
- **喝水提醒**：固定间隔提醒（默认 1 小时），每日目标（默认 8 杯）、达标庆祝、午夜自动结算
- **菜单栏**：template 图标 + 实时倒计时，点击弹出操作面板，右键快捷菜单
- **统计**：今日/近 7 天/当月饮水图表，连续天数、最高纪录与成就徽章
- **全局快捷键**：⌘⇧T 开始/暂停计时，⌘⇧H 记录喝水，⌘, 打开设置
- **智能工作提醒**：未计时时检测持续工作并提醒开启计时器
- **界面**：全部视图由 Lobe MyGo 组件与 Lobe tokens 绘制，跟随浅色/深色/系统主题
- 通知、自定义提醒音（mp3/wav/ogg）、提醒文案库、登录自启、数据导出

## 运行与构建

```bash
go tool mygo dev       # 开发运行（live reload）
go test ./...          # 全部测试（引擎单测 + 视图无头测试）
go tool mygo build     # 打包 .app 与 .dmg 到 build/
go run ./tools/preview -out /tmp/preview   # 无头渲染全部视图为 PNG（-view / -dark / -measure）
```

需要 Go ≥ 1.27.3（`go` 会自动切换工具链）。mygo CLI 由 `go.mod`
的 tool 依赖提供（`tool github.com/egoist/mygo/cmd/mygo`），无需单独安装。

界面组件与属性以 Lobe MyGo 文档（<https://mygo.lobehub.com>）为准。依赖版本在 `go.mod` 中固定：
mygo v0.8.4-0.20261108082913-9233663736ca（Lobe MyGo 要求的版本）与
lobe-mygo v0.0.0-20261109063003-6081a16a09f2，两者需同步升级。

## 结构

```
main.go                  入口：单实例、通知路由、生命周期
internal/
  store/                 settings.json / hydration-records.json / timer-state.json
  core/                  纯逻辑引擎（可单测）：工作计时、喝水循环、休息会话、文案
  gui/                   Lobe MyGo 界面：theme.go（Lobe tokens）、components.go（共享组件）、
                         icons.go（Lucide 图标）、model.go 与各视图（view_popover/settings/stats/alerts）
  app/                   装配：托盘、flyout 面板、各窗口、通知、声音、快捷键、自启
tools/preview            无头渲染全部视图为 PNG（浅色 / 深色）
tools/gen_icons.sh       由 remixicon npm 包生成菜单栏字形 glyphs_gen.go
tools/gen_tray           将托盘字形无头渲染为 template PNG（internal/app/trayicons/）
tools/gen_icon           生成应用图标（resources/icon.png，SVG 无头渲染）
```

数据存于 `~/Library/Application Support/Restify/`。

## 与 Swift 版的差异

本应用原为 SwiftUI 实现（macOS 13+），已完全重写为 Go，旧版代码可在 git
历史中找到（`Restify/` 目录）。与旧版的行为差异：

- 界面已重建为 Lobe MyGo（Lobe UI 设计语言）：颜色、字号、圆角与动效均取自
  Lobe tokens，不再由本项目自定义设计系统
- 通知无操作按钮（mygo 限制）：点击通知打开喝水弹窗代替
- Dock 图标取消（`ActivationPolicyAccessory`，纯菜单栏应用）
- LaunchAgent 常驻与 LSSharedFileList 登录项机制由
  `mygo.App.SetOpenAtLogin` 取代
- 菜单栏图标状态由 template 图形切换表达（不再是绿/橙着色）
- 计时器暂停状态跨启动保留（旧版仅恢复"运行中"状态）
- 旧版 UserDefaults 数据不迁移

## 验证

- `go vet ./...` 与 `go tool mygo vet .`
- `go test ./... -race`
- 视图/组件无头断言（`internal/gui`）；PNG 样张二选一：
  - 组件色板：`RESTIFY_GALLERY_DIR=/tmp/gallery go test ./internal/gui/`
  - 全部真实视图：`go run ./tools/preview -out /tmp/preview`
    （`-view` 按名称前缀筛选、`-dark` 只渲染深色、`-measure` 打印面板高度）
- 手动清单：托盘倒计时、面板交互、休息遮罩、喝水弹窗、⌘⇧T/⌘⇧H
  快捷键、深浅色主题、通知权限（需 `mygo build` 的打包版）