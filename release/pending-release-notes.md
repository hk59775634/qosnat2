# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

WARP License Key 明文核对、删除 Key 停止错误重连，修复看门狗反复连接

## 新增

- `DELETE /api/v1/network/warp/license` 删除已保存 License Key 并断开 WARP
- WARP 页「删除 License Key」按钮，防止错误 Key 触发反复自动重连

## 优化

- License Key 在 status API 与输入框中明文显示，便于核对是否填错
- 断开 WARP 且无 License 时清除 warp-cli 注册，便于下次以普通 WARP 重新连接

## 修复

- License Key 无效导致连接失败时自动关闭 `warp_enabled`，看门狗不再每 20 秒用错误 Key 重试

## 删除

- （无）

## 其他

- OpenAPI 补充 WARP license DELETE 与 status 返回 `warp_license_key` 字段
