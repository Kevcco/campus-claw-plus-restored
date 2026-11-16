# #58合入及精确主干复验

2026-11-16，小张。#58的最终head `378f6ce0f080cd32949354fb9cc2ff1d90d48139` 已获Kevcco实际批准，并正常合入main `659393b0ecdaa56b622a10fbb8f999fa198fe1ae`，时间与原件摘要见metadata.json。批准head与main整树一致；本报告不替其他账号发表新审查。

## 精确主干CI通过

[main CI43810841732](https://github.com/zwh9616/campus-claw-plus/actions/runs/43810841732)完整成功，verify410441ms，仍在原600000ms预算内。artifact14200457586原ZIP保存在ci-originals.zip，SHA256 `12d5bd3304dd0881f9141a446bf9594d6ffe8d096bca9032d1e8a428b610765b`。实际解包18原件并独立核验2448个跟踪Git blob指纹；四段0，五镜像零HIGH/CRITICAL且与实际运行ID一致，三HTTPS、五真实SQL、九worker、40文件、迁移6/五DDL中断/恢复5及清理通过。详细结论见ci-proof.json；解包秘密扫描退出0。CI是Linux环境结果，不能写成本机通过。

## 本机仍失败，保留门禁

同一main重新安装并预热漏洞库后运行原 `npm run gate`：check199967ms退出0、audit0、secret-scan0；verify600513ms退出124。见local-timeout/gate-results.json和完整gate.txt。日志显示五镜像、默认HTTPS/原件/持久出处、真实中断和重建、迁移6及备份恢复已完成；最后离线解析组件只完成build和核心有界验收，未完成HTTP/客户端/扫描及批次最终汇总。不能把这些局部通过合成为完整通过。

本轮扫描结果目录原有上一提交 `73cdd35` 的旧JSON，不作为本轮通过证据；未产生新的service-batches最终results.json，未伪造补写。check中的source.dirty仍为true（8份历史报告仅工作区行尾/stat差异，git diff无内容变化），不写成本机干净完整验收。生产/测试/门禁源码本轮均未修改。

A4.3继续未完成；#58已合入这一阻塞已解除，剩余是本机原预算完整复验。未加时限、删断言、减少语料或跳过扫描，未停止其他项目容器。当前耗时差异需要分段测量后再作性能修复，不能凭猜测宣布机器资源已被证明是根因。

## 同时交付的可独立准备工作

小张在主干复验等待期间于ignored staging准备OCR语料，复验结束后才复制入跟踪目录；详见[OCR准备](../ocr-foundation/README.md)。仅输入、金标准、评分和选型准备；B识别服务/新HTTP/数据库迁移未实施，A的失败不会被OCR单测掩盖。下一批先完成开发集候选测量与B1.2准确合同，并解决本机完整复验；待A验收和B合同获审合入才apply运行实现。

本机失败原始文本与JSON另存local-timeout-originals.zip；展开文本仅统一LF便于审阅。此次可确定归属的五个审计标签已清理，当前无本轮命名的运行容器；进程被预算终止，未声称完整批次finally已执行。
