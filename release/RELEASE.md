# qosnat2 发布策略

## 版本号

- 格式：`YYYYMMDD` + 当日 2 位序号（01–99），如 `2026053111`
- 清单：[`releases/qosnat2-versions.json`](../releases/qosnat2-versions.json)（保留最新 5 条）
- GitHub Release tag：`v` + 版本号

## 发布前（必做）

1. **完成开发与自测**（`go test`、`npm run build` 等）
2. **编写更新说明**
   ```bash
   ./scripts/release-notes.sh draft    # 可选：从 git 提交生成待分类草稿
   # 编辑 release/pending-release-notes.md
   ./scripts/release-notes.sh validate
   ```
3. **按栏目梳理变动**（勿留模板占位文字）：
   - **概要**：一句话，写入版本清单 `summary`
   - **新增** / **优化** / **修复** / **删除** / **其他**：至少一栏有一条有效说明
4. **与功能代码同一 commit 提交** `release/pending-release-notes.md`

## 触发发布

| 变更类型 | 触发方式 |
|----------|----------|
| 含 `internal/`、`cmd/`、`scripts/`（非仅 md）等 | 推送 `main` 自动触发 [release.yml](../.github/workflows/release.yml) |
| 仅 `web/` 或文档 | 手动：`gh workflow run release.yml`（仍需已提交 pending 说明） |

CI 步骤：

1. 校验 `release/pending-release-notes.md`
2. 分配版本号、构建资产
3. 用更新说明作为 **GitHub Release 正文**（不再使用自动生成的 commit 列表）
4. 归档至 `releases/notes/<版本>.md`，写入 manifest `summary`
5. 重置 pending 模板并 commit manifest + 归档说明

## 发布后

- GitHub Release 页与 [`releases/notes/`](../releases/notes/) 可查阅完整说明
- Web **系统 → 常规 → 版本管理**：选择版本可查看概要与更新说明
- 仅重传二进制（不改版本号）：[`republish-release.yml`](../.github/workflows/republish-release.yml) 或 `./push.sh <tag>`

## 命令速查

```bash
./scripts/release-notes.sh draft
./scripts/release-notes.sh validate
./scripts/release-notes.sh finalize 2026053112   # 一般由 CI 调用
./scripts/release-version.sh qosnat2 latest
gh workflow run release.yml                      # 纯前端发布
```
