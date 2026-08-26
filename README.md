# PhonemeReleaseDesk

PhonemeReleaseDesk 是面向方言语音资料团队的音素标注发布资格工作台。它把语料规范、录音片段范围、双人独立标注、确定性规则检查、冲突裁定、定向返修、复核封存和发布凭据核验放在一个可审计的本地流程中。浏览器页面和同源 JSON API 均由 Go 服务直接提供，不需要 Node.js 构建链或外部系统。

## 业务流程

批次依次经过 `draft`、`frozen`、`annotating`、`checking`、`adjudicating`、`candidate`、`repair` 和 `sealed` 状态。冻结后不能增删片段；两名标注员只能查看和修改自己的标注；复核退回只解锁命中的标注或裁定项；封存后所有业务写入都会被拒绝。

每个写请求携带 `expectedVersion` 进行乐观并发控制，并建议携带唯一的 `Idempotency-Key` 请求头。数据保存在 `-data` 指定目录：`events.jsonl` 是带递增序号和 SHA-256 哈希链的事件账本，`snapshots/` 保存原子更新的批次快照，`idempotency.json` 保存幂等结果。启动时服务会校验账本并重建投影。

工作台现已支持片段批量预检与原子登记、双标任务负载预览与批量分配、检查运行历史及范围感知对比、冲突队列筛选与原子批量裁定。定向返修以任务状态跟踪命中目标、重检运行和前后差异；已封存凭据可按批次或凭据检索，分页查看稳定排序的规范化区间，并分别核验摘要、片段数和区间数。所有预览、历史、差异和凭据查询均为只读操作。

## 构建与测试

项目要求 Go 1.22 或更高版本。

```text
go build ./cmd/server
go test ./...
```

## 运行

默认只监听高位回环地址 `127.0.0.1:19081`：

```text
go run ./cmd/server -data=./var/data
```

也可以显式配置回环地址：

```text
go run ./cmd/server -addr=127.0.0.1:19082 -data=./var/data
```

未传 `-addr` 时，可以通过 `PORT` 指定端口，服务会绑定 `127.0.0.1:<PORT>`。出于安全考虑，程序拒绝 `0.0.0.0`、非回环 IP、无效端口和格式错误的监听地址。启动后访问 `http://127.0.0.1:19081/` 即可使用工作台。

## 有界自检

以下命令启动真实回环 HTTP 服务，通过与浏览器相同的 API 完成建批、冻结、双标冲突、检查、裁定、定向返修、重检、封存和摘要核验，然后主动关闭并返回退出码：

```text
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081 -data=./var/selfcheck
```

## 角色和 API 约定

页面请求体会明确提交 `actorId` 和 `role`。受支持角色包括 `manager`、`annotator`、`adjudicator` 和 `reviewer`。API 错误以统一 JSON 返回；版本冲突使用 HTTP `409`，越权使用 `403`，对象不存在使用 `404`，字段或业务规则问题使用 `422`。健康检查位于 `GET /api/health`，凭据可通过 `GET /api/credentials/{id}/verify` 独立核验。

批量写接口为 `segments/bulk`、`assignments/bulk` 和 `decisions/bulk`，确认请求必须携带 `Idempotency-Key`；对应预检或预览接口为 `segments/preflight` 和 `assignments/preview`。检查历史与对比位于 `checks/history`、`checks/compare`，返修任务位于 `repairs/tasks`。`GET /api/credentials` 提供凭据明细分页，`GET /api/credentials/verify` 提供按 `credentialId`、`batchId` 的分项核验。
