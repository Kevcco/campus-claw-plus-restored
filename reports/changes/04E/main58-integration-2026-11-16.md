# 04E：#58合入后的精确主干复验

2026-11-16，小陈 / Kevcco（AI辅助）核对。#58最终head `378f6ce0f080cd32949354fb9cc2ff1d90d48139` 获本人[APPROVED 5915431379](https://github.com/zwh9616/campus-claw-plus/pull/58#pullrequestreview-5915431379)，完整PR CI43809266164成功后正常合入main `659393b0ecdaa56b622a10fbb8f999fa198fe1ae`。获批head与合入树相同，主干保护仍生效；[实际合并记录](main58-integration/merge-facts.json)逐项绑定批准、head、树及CI。

[精确main完整CI43810841732](https://github.com/zwh9616/campus-claw-plus/actions/runs/43810841732)成功：check/audit/secret-scan/verify均退出0，verify为410441ms，无未执行步骤。原artifact14200457586的ZIP SHA256为`12d5bd3304dd0881f9141a446bf9594d6ffe8d096bca9032d1e8a428b610765b`，实际下载重算与GitHub一致，未重打包；2448个跟踪Git blob内容指纹与精确main一致，各来源报告摘要、时间及镜像扫描/运行ID相匹配。

本次逐项核对原件：五生产镜像HIGH/CRITICAL均0及五项清理通过；Document三阶段默认可信HTTPS、五组真实SQL、双班五人、原字节与持久出处、解析器停机和容器重建证据通过。九worker阶段覆盖40文档/195位置/196片段、实际SIGKILL和数据库恢复；测试租约显式推进，未冒称等待生产租约。迁移6五DDL中断、精确结构检查、前向修复、旧5备份/隔离恢复和真实旧制品矩阵通过。10组持久队列与16个SQL拒绝探针、原01/04/05回归保留。check/scan各有40份清晰语料、25类负例、7组私有HTTP、3组实际健康及真实Go客户端断联证据。完整判定见[独立核验结果](main58-integration/ci-verified.json)。

验收包解包后Gitleaks8.30.1扫描退出0、无秘密命中；[扫描输出](main58-integration/artifact-secret-scan.txt)、原ZIP、GitHub元数据和本次核验JSON按[清单](main58-integration/manifest.json)保存。文本仅规范换行，ZIP原字节不改。这里记录下载CI原件的独立核验，没有新执行本机Docker/数据库/浏览器会话，也没有持有人部署；此前#57精确PR head的独立Docker运行按其原记录保留。

#58只修复测试客户端半关闭ENOTCONN竞态，不改变生产行为/合同。04E全部实施及阶段收口仍由小张负责；当前A1.2/A1.3父项、A2.5、A3.1/A3.2、A4.2/A4.3仍未勾选，须对照实际证据逐项核对后再记录A完成、交接B。不据这次主干成功启动OCR/语义或归档整个04E。#56收口文档的当前head检查及小张实际审查另核，未代其批准。

本次仅补文档/原件，运行、测试、配置及合同输入与main659393b一致。本机采用Node24.22.1/npm10.5.0执行`check:foundation`、`check:contracts`（45项）、`check:spec`（strict11/11）、`audit`（根/前端两树无漏洞）均退出0；全历史249提交及暂存内容秘密扫描均退出0、无命中。命令原输出随本目录清单保存。本提交未另跑本机完整Docker门禁，当前文档PR的最终head完整CI按远端结果跟进。
