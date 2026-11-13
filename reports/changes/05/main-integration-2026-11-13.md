# 05 主干集成核对（2026-11-13）

执行人小张/zwh9616（AI辅助）。#40与#41均由小陈/Kevcco真实独立审查并正常合入，本次只核对实际GitHub记录和精确主干原件，不冒称小陈的新批准或本机Docker通过。

## 实际合入与门禁

- #40最终a485b94经真实[APPROVED5877403365](https://github.com/zwh9616/campus-claw-plus/pull/40#pullrequestreview-5877403365)，完整CI43249864667成功后合入main f7eb856；精确main CI43250820540成功。仅HTTP/原子提交批次，独立复测和红绿保留。
- #41最终e59b57a经真实[APPROVED5885799083](https://github.com/zwh9616/campus-claw-plus/pull/41#pullrequestreview-5885799083)，完整PR CI43251835049成功后合入main **d06198de7cbff56b4304b3483aeec32ad9a3e085**。本机实际Git核对：该main树与获批head均为 **0c46acc490690816be0bca0d7bca1d9c0d4c4dc0**。
- 精确main的[完整CI43362518043](https://github.com/zwh9616/campus-claw-plus/actions/runs/43362518043) completed/success。实际下载artifact14008414768，ZIP SHA256 **77fbd1916ffc87246c78bc0222e9f774d459009d648262b4c8e1a37568215d99**与GitHub登记一致；CRC通过，四源报告逐份摘要、源码commit/clean、四镜像HIGH/CRITICAL0、同轮实际运行ID及四标签清理0均核验通过。

完整CI运行原check/audit/secret-scan/verify，无跳过。真实MySQL授权/20并发幂等/事务回滚/截止/丢回执/分页，租约失权/接管/ABA/三次耗尽/取消两种顺序/终态和适配器回滚、真实进程kill恢复/SIGTERM与原01/04、4/5实际制品和一致备份隔离恢复通过。默认可信HTTPS三阶段持久会话/原提交/原202、DB503/恢复/主机入口、30次严格414均保留。仅测试二进制注入执行器，生产仍真实pending。

## 本机尝试与明确限制

今天在精确d06198d的干净隔离worktree执行npm ci与npm run setup均0；npm run gate在Go检查处因Docker engine pipe不存在退出1，audit/secret-scan/verify未运行。常规启动Docker后出现sailor-ingest.sock绑定冲突；实际查看错误窗口，仅正常Quit/重试及停止本次启动的悬挂CLI辅助进程，未使用factory reset、删除卷、改安全/隐私设置或上传诊断。本机完整门禁/本机容器验收**未通过**，不会把主干CI原件说成自己的本机运行。此环境失败与成功CI并列保存，原门禁、阈值、TLS、扫描规则不变。

3.2的精确main集成结果据实际CI及核验原件登记；作者本机重复验收受上述环境阻断，不作为跨机器独立通过。3.3主规约/归档在本分支准备候选，仍须小陈实际复核、当前head CI及正常合入，不提前写主干已归档。真实07B/08、作业页面与持有人生产库升级/长期部署均未执行。

原件：[主干CI ZIP](main-integration/main-ci-runtime.zip)、[逐文件SHA与结论](main-integration/integration-facts.json)、[本机失败原件](main-integration/local-attempt.zip)。之前所有失败/受控红/逐字恢复绿、独立复测原ZIP和真实执行者保留。
