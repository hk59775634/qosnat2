# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 WARP 启用长时间卡在连接中；状态区显示 License Key 并支持应用 WARP+；加固损坏 netns pin 清理。

## 新增

- WARP 页状态区显示已保存的 License Key（明文便于核对）
- 「应用 WARP+」接口与按钮（连接中应用 License，等同 warp-go --update --license）

## 优化

- 缩短 WARP 连接稳定检测与 CLI 重试时间，目标十余秒内完成启用
- 看门狗在连接任务失败或运行后 45 秒内不再抢连，避免与 UI 并发拆 netns
- WARP 连接任务 90 秒超时；服务重启清理残留 running 状态；UI 轮询 2 分钟超时提示

## 修复

- 修复损坏的 `/run/netns/qosnat2-warp` pin 导致 `ip netns add: File exists` 无法启用
- 移除 Connect 失败时与 RecoverQuick 叠加的反复 scrub，避免 netns 删除/重建死循环
- 策略应用失败不再拆掉已连通的 WARP 隧道；健康检查不再因 veth 瞬时抖动长时间重试
- 系统设置版本切换确认框在提交后正确关闭

## 删除

- （无）

## 其他

- （无）
