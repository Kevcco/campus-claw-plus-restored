# #61 修复复审与候选边界核对

小陈 / Kevcco（AI辅助），2026-11-16。审查基线 `d540bdc74ed8d3f621c1b286fd0c1fef896fb157`，关联 DI-SC-17/18/19/25；前次 CHANGES_REQUESTED 5917719926 保留为历史，本轮不将 #60 批准当作 #61 批准。

## 两条实际反馈

1. 512码点窗口每313码点推进，保留199码点重叠；独立检查300000个起点/长度组合，覆盖最长200码点查询。新增真实MySQL中文、非BMP、最大查询长度、旧修订出处及班级/发布过滤用例；已核对作者修复前两条失败、修复后13组完整日志和10个源文件的精确Git blob摘要。
2. 幂等唯一键及回查同时加入 document_id。候选与嵌入迁移7 SQL逐字相等；双文档同键20并发、同文档原回执重放及异载荷冲突用例保留。此结论依据代码、测试与原件核验，本轮未另起独立MySQL会话。

## 本人实际执行

支持的Node24.22.1 / npm10.5.0下，隔离工作树 npm ci --ignore-scripts --no-fund 退出0（97包、零漏洞），46项合同检查和13项OCR相关检查退出0；Go1.27.3三个相关包的随机顺序测试退出0；Python3.12.7六项栅格/TSV/私有HTTP边界测试退出0。后者使用受控识别器，只证明HTTP边界，不声称本轮启动Tesseract。foundation和严格规约11/11通过。

分别临时破坏私有客户端非法200结果拒绝、恢复后enum精确等价比较，两项均红1→恢复绿0；活动源码逐字恢复。原始输出见 [local-checks-originals.zip](chen-ocr-followup/local-checks-originals.zip)。没有新的本机完整Docker gate、OCR引擎、数据库或浏览器运行结论。

## 原件独立核验

精确PR head完整CI43861270879成功，artifact14224341190的ZIP SHA-256为 `1022f4bce036482d93a1a38d6209905a3ddbf9071b23cf15e11beae9220b63af`，与GitHub元数据一致。2800个跟踪Git blob重新汇总，与实际预演制品源码指纹一致；check/audit/secret-scan/verify四段全0，五镜像及同ID运行、原HTTPS/SQL/worker/迁移6恢复、40份PDF/Word及25反例、私有RPC/客户端/健康和清理均通过原验证器，解包秘密扫描0。

另核对prepared7的四组真实迁移、中断/结构漂移、五阶段独立恢复6/7原件及8个源码摘要；核对真实OCR RPC四页437词框、逐字出处、无gold/盲测挂载、清理与对应源码。按最终接口正文独立重算开发CER为35/1618=2.16%；这不是50页盲测或100框IoU验收。详见 [author-source-verified.json](chen-ocr-followup/author-source-verified.json) 与 [ci-verified.json](chen-ocr-followup/ci-verified.json)。

## 本轮直接修正及交付边界

本会话用户明确提供的工程约定只授权04E统一小张负责，保留真实远端批准、公共合同先合入再apply及其他暂停安排。原候选扩展到整个项目、取消审批和恢复暂停项的文案不作为已生效授权；本提交将六份当前文档及非04E进度行对齐现行约定，保留实现和全部测试原件。

生产仍Target6、28 current、15个空active键；没有新公开OCR接口、默认服务或持有人部署。prepared7可供隔离验收，不授权持有人升级。A余项、B1.2准确公共兼容/配置及安全闭包、PNG/JPEG/混合页、教师页面、正式质量和C语义仍待。后续生产apply必须先满足各自批准及前置。

本提交相对上述head仅修正文档/进度和添加复审证据，最终head完整CI、实际远端review与正常合入/main复验仍需实时核对；本报告不预填批准或合并成功。

提交前文档复验：foundation、严格规约11/11、264提交全历史和暂存秘密扫描均退出0；生产/测试/脚本/机器合同与d540bdc逐字保持。原始输出见 [final-document-checks.zip](chen-ocr-followup/final-document-checks.zip)。
