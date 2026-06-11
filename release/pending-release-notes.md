# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

新增 SNMP（snmpd）配置 UI 与 API，支持 SNMPv2c 只读 community 与源网段 ACL。

## 新增

- `state.json` 字段 `snmp`：端口、community、sysLocation/Contact/Name、允许查询网段
- API：`GET/PUT /api/v1/snmp`、apply、service、install
- 系统菜单「SNMP」配置页：安装 snmpd、保存并应用、配置预览
- 生成并托管 `/etc/qosnat2/snmpd.conf`，应用时写入 `/etc/snmp/snmpd.conf`（首次备份原配置）

## 优化

- `install-deps.sh` 可选包增加 snmpd

## 修复

- （无）

## 删除

- （无）

## 其他

- OpenAPI 补充 SNMPState 与 /api/v1/snmp 路径文档
