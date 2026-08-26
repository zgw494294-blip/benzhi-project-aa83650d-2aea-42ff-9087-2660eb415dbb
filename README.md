# Acoustic Verdict Workbench

Acoustic Verdict Workbench（声学裁决工作台）用于治理野外声学监测数据集的标注质量。它把审校批次范围登记、录音片段批量预检、双人独立多事件标注、确定性分歧匹配、定向返标、复核仲裁、发布前质量检查和不可变清单封存收束为唯一闭环。

浏览器页面、原生 HTML/CSS/JavaScript 与同源 JSON API 均由 Go 服务直接提供，不需要 Node.js 或外部系统。两名标注员提交前只能看到自己的草稿；冻结后不能修改片段与物种范围；封存后所有业务写入都会被状态机拒绝。

## 数据与一致性

每个写请求都携带 `expectedVersion`，并通过 `Idempotency-Key` 持久化去重。单进程写入协调器串行提交命令，本地数据目录包含：

- `snapshots/*.json`：带 `schemaVersion` 的批次快照，通过候选临时文件、`Sync` 和原子 `Rename` 提交。
- `audit.jsonl`：只追加审计日志，记录连续序号、前序摘要和当前 SHA-256 摘要。
- `idempotency.json`：保存写命令的原始响应版本，使重复请求返回相同业务结果。

服务启动时校验审计日志的序号与摘要链，加载全部快照并恢复查询投影。发布清单包含稳定排序的规范事件、片段摘要组成、仲裁轨迹组成和最终 `manifestSHA256`。发布后可以按片段、物种和时间区间分页检索事件，并只读复算 `clipDigest`、`adjudicationDigest` 与 `manifestSHA256`，核验不会改变批次版本。

## 工作台扩展能力

- 草拟批次可在片段区域一次登记 1–200 条元数据。服务会集中检查片段标识、序号、SHA-256 内容摘要和采集边界；错误响应携带行号，任一行失败时整批不写入。
- 标注员可在一个草稿中增删和编辑多条候选事件。草稿按时间稳定排序，只向本人恢复；提交前展示事件数量与总区间并要求确认，提交后锁定当前轮次。
- 复核员退回分歧时必须选择关联标注员并填写理由。工作台向目标标注员展示下一轮待办、原匹配依据和修订上下文，返标提交后仅重算目标片段并保留全部历史轮次。
- 已发布批次提供只读的事件筛选分页、来源片段与仲裁轨迹摘要组成，以及三项摘要复算结果，不提供发布后编辑入口。

## 构建

项目要求 Go 1.22 或更高版本：

```text
go build ./cmd/acoustic-review
```

## 运行

默认仅监听高位回环地址 `127.0.0.1:19081`：

```text
go run ./cmd/acoustic-review
```

显式指定监听地址与数据目录：

```text
go run ./cmd/acoustic-review -addr=127.0.0.1:19082 -data=./var/acoustic-review
```

未传 `-addr` 时，也可通过 `PORT` 提供端口号，服务会绑定 `127.0.0.1:<PORT>`。程序拒绝 `0.0.0.0`、非回环 IP、无效端口和格式错误的地址。启动后打开 `http://127.0.0.1:19081/` 即可使用工作台。

## 测试与自检

运行全部回归测试：

```text
go test ./...
```

运行可自行结束的真实 HTTP 自检：

```text
go run ./cmd/acoustic-review -selfcheck -addr=127.0.0.1:19081
```

自检会创建隔离临时数据目录，启动真实回环监听，通过公开页面和 JSON 接口完成建批、范围冻结、双人冲突标注、仲裁、质量门禁、发布封存与封存后写保护验证，然后关闭服务并清理资源。

## 角色与接口约定

支持 `manager`、`annotator`、`reviewer` 和 `release_manager` 角色。写请求正文提供 `actorId`、`role` 与 `expectedVersion`，请求头提供唯一 `Idempotency-Key`。版本或状态冲突返回 HTTP `409`，越权返回 `403`，对象不存在返回 `404`，字段与领域规则错误返回 `422`。

工作台详情查询使用 `actorId` 和 `role` 查询参数过滤草稿与返标任务。公开 API 从 `GET /api/batches` 和 `POST /api/batches` 开始，批次子资源覆盖 `scope`、`clips`、`clips/bulk`、`freeze`、`draft`、`submit`、`disputes/{disputeID}/resolve`、`quality` 与 `release`。发布明细使用 `GET /api/batches/{batchID}/manifest`，摘要核验使用 `GET /api/batches/{batchID}/manifest/verify`；两者都要求 `release_manager` 或 `reviewer` 身份查询参数。
