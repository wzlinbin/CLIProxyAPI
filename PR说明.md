## PR: 使用统计 SQLite 持久化 & 运营计费页面

**分支**: `sqlite` → `main`

---

### Summary

- 新增 SQLite 插件，将使用统计数据持久化到本地数据库，重启后数据不丢失
- 新增运营计费页面（ops-billing.html），提供可视化的成本分摊和计费展示
- 增强 API Key 维度的统计功能，支持按 key 查询使用量
- 添加 SQLite 数据库 schema 自动迁移机制，兼容旧版本数据库升级
- 优化配置文件路径解析逻辑，支持可执行文件所在目录作为默认路径

### Changes

**核心功能**
- `internal/usage/sqlite_plugin.go` — 新增 SQLite 持久化插件，支持 WAL 模式、自动清理（90天/25万行）、schema 自动迁移
- `internal/usage/sqlite_plugin_test.go` — 完整的单元测试覆盖
- `internal/usage/logger_plugin.go` — 增强日志记录，支持 API Key 字段
- `sdk/cliproxy/service.go` — 集成 SQLite 插件到服务生命周期（启动时加载、关闭时释放）

**管理面板**
- `internal/managementasset/updater.go` — 重构静态资源管理，新增 ops-billing.html 支持、优化目录解析
- `internal/api/server.go` — 新增 ops-billing 路由

**编译与部署**
- `compile_linux.sh` / `compile_linux.bat` — Linux 交叉编译脚本（含版本注入 ldflags）
- `cmd/server/path_resolution.go` — 提取配置路径解析为独立模块
- `cmd/server/path_resolution_test.go` — 路径解析测试

**依赖**
- `go.mod` / `go.sum` — 新增 `modernc.org/sqlite`（纯 Go SQLite，无需 CGO）

### Test Plan

- [ ] 启动服务，确认日志中出现 `usage sqlite persistence enabled`
- [ ] 发送 API 请求后，检查 `state/usage-statistics.sqlite` 文件生成
- [ ] 重启服务，确认统计数据从 SQLite 恢复
- [ ] 在旧版本数据库上升级，确认自动迁移日志 `migrated schema — added column`
- [ ] 访问管理面板 → 运营计费菜单，确认页面正常渲染
- [ ] `go test ./internal/usage/... ./cmd/server/...` 通过
