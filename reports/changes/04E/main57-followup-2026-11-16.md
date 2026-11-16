# 04E：#57 合并、精确主干复验与夹具竞态修复

2026-11-16，小张执行。Kevcco 的实际 [APPROVED 5914937960](https://github.com/zwh9616/campus-claw-plus/pull/57#pullrequestreview-5914937960) 绑定 `3fb53ed5363122737622cd5c05f1512e67442ed6`，该 head 完整 CI43801177391 成功且行内反馈已解决。按 expected_head_sha 正常合并 [#57](https://github.com/zwh9616/campus-claw-plus/pull/57)，实际 main 为 `a40b83b3166e3ee2d1d2e753c3b8db40825a6899`，与获批 head 整树一致；没有管理员绕过。

## 精确主干的实际结果

- [主干 CI43804539643](https://github.com/zwh9616/campus-claw-plus/actions/runs/43804539643) 完整 gate 四段退出0，verify 411022ms。artifact14198002496 原始 ZIP SHA256 `3a1f23052037ddaed327b9fab5309018d082971552b22682263154e29ce030b6`，已下载核验且未重打包。
- 原18份证据、全部2422个跟踪 Git blob 指纹匹配精确 main。五镜像 HIGH/CRITICAL 均0，实际运行 ID 与各自扫描匹配；三阶段默认可信 HTTPS、五组真实SQL、九worker阶段/40文档/195位置、真实SIGKILL/数据库恢复、迁移6五DDL中断/隔离恢复5、10组持久队列、16个SQL拒绝探针、原回归及清理全部通过。原 ZIP 解包秘密扫描0。
- 获批浏览器构建的429源码及113原件与 main 匹配；本次没有冒称另一个新的浏览器会话。
- 本机 Windows / Docker / Node v25.2.1：npm ci/setup0。第一次完整 gate 的 check/audit/secret-scan0，verify在原600秒预算退出124，最终离线解析器复核未完成；当轮漏洞库更新约65秒。原失败结果及五个专用审计标签清理原件保留。
- 相同源码、原命令、原600秒预算重跑：check/audit/secret-scan0，verify退出1。前置生产集成、SQL、worker、迁移恢复均过，最后离线解析器的40语料/25负例通过，私有HTTP夹具失败。**本机完整门禁尚未通过，不把CI成功改写成本机成功，也不拼接两个失败批次。**

所有原件见 [main57-followup/manifest.json](main57-followup/manifest.json)，包括两次失败、批准、原CI包/核验、浏览器源码绑定及下面红绿原件。文本仅规范换行和行末空白；ZIP字节未改。

## DI-SC-14：关闭竞态与最小修复

第二次失败是 `test_header_and_body_framing` 的 Content-Length:0 拒绝请求：服务端提前返回并关闭连接，客户端 `shutdown(SHUT_WR)` 偶发得到 Linux `ENOTCONN`，尚未读到响应就退出。不是合法PDF解析失败，也没有依据修改生产拒绝行为。

只修改 `document-parser/test_rpc.py`：仅对半关闭的 ENOTCONN 继续读取响应，其他 OSError 原样抛出。原有 HTTP 状态、完整JSON正文、Cache-Control、非法请求拒绝及后续健康检查全部保留；没有空响应成功兜底、重试、跳过或延长时限。生产代码、Docker镜像内容和公共合同不变。

在原7组真实HTTP测试中增加确定性回归：真实 loopback 请求/响应不替换，只向客户端 shutdown 注入 ENOTCONN，仍要求精确400和 `invalid_request`；EIO/ECONNRESET必须传播失败。新增回归在原 helper 上真实红1，修复后同一固定镜像、network none/read-only/512MiB/1CPU环境7组和40文档绿0，日志安全检查通过。`rpc-red.txt` / `rpc-green.txt` 保留原结果；扫描命令仍要求原7组全部通过。

本修复提交完整门禁与实际远端审查，后续结果由本PR实时checks及交接记录补充；旧#57批准不作为这次修复的批准。A4.3保持未完成，待修复获审、正常合并和精确主干复验后收口。B/OCR、C/语义、持有人部署及整项04E归档均未完成；下一步先处理本修复的验收。
