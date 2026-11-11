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
- **应用内更新**：从 GitHub Release 检查并安装签名更新（每日后台检查，可在设置中关闭，支持增量下载）
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

## 发布与更新

应用通过 mygo 的签名更新机制从 GitHub Release 自更新：`mygo build` 用
Ed25519 密钥对每个平台的归档签名并生成 manifest，应用内的更新器（Sparkle
风格窗口，简体中文）每日后台检查最新 release，下载（优先增量 delta）、验签
并在重启后替换应用。更新偏好存于 `~/Library/Application Support/Restify/updater.json`。

### 一次性设置

- 签名密钥由 `go tool mygo keygen` 生成于
  `~/Library/Application Support/mygo/update-keys/`。`mygo-update.key`
  私钥务必妥善保管、不要提交（公钥已写入 `mygo.json` 的
  `updates.publicKey`）：丢失它，已安装的应用将永远收不到更新。
- 把私钥设为 CI secret：

  ```bash
  gh secret set MYGO_UPDATER_PRIVATE_KEY \
    < "$HOME/Library/Application Support/mygo/update-keys/mygo-update.key"
  ```

### 发布

1. 同步改三处：`internal/app/app.go` 的 `Version` 常量、`mygo.json` 的
   `version`，并在 `CHANGELOG.md` 新增 `## <版本> - <日期>` 节（它是更新
   窗口里展示的 release notes；CHANGELOG 存在则必须有对应版本节）
2. 提交后打 tag 并推送：

   ```bash
   git tag v1.1.0 && git push origin v1.1.0
   ```

3. CI（`.github/workflows/release.yml`）自动创建 draft release，构建并上传
   darwin-arm64 与 darwin-amd64 的安装包、签名归档、增量与 manifest，两个
   平台齐全后发布。已安装的应用每日检查 `releases/latest`，也可从托盘右键
   菜单或“关于”页手动检查更新

- 首个版本没有更早的发布可比对，不带增量更新，属正常；之后的构建会为
  最近 3 个版本各生成一个 delta
- 预发布版本请在 GitHub 上勾选 “Set as a pre-release”，或
  `gh release edit <tag> --draft=false --prerelease`：已装应用会跳过它们，
  只有 beta 版应用（`SetPrereleases`）会收到

### 本地验证

```bash
go tool mygo build   # 私钥路径在 mygo.json 的 updates.privateKey，构建即签名
```

产出 `build/restify-<版本>-darwin-arm64.tar.gz` 与
`update-darwin-arm64.json`；`-upload` 另需 `GH_TOKEN`。开发构建
（`mygo dev`、`go run`）不能自更新，更新窗口会说明原因。

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

## 验证

- `go vet ./...` 与 `go tool mygo vet .`
- `go test ./... -race`
- 视图/组件无头断言（`internal/gui`）；PNG 样张二选一：
  - 组件色板：`RESTIFY_GALLERY_DIR=/tmp/gallery go test ./internal/gui/`
  - 全部真实视图：`go run ./tools/preview -out /tmp/preview`
    （`-view` 按名称前缀筛选、`-dark` 只渲染深色、`-measure` 打印面板高度）
- 手动清单：托盘倒计时、面板交互、休息遮罩、喝水弹窗、⌘⇧T/⌘⇧H
  快捷键、深浅色主题、通知权限（需 `mygo build` 的打包版）