# 05 主规约与归档候选核对（2026-11-13）

执行人小张/zwh9616（AI辅助）；原规约/独立终审人小陈/Kevcco。本分支基于精确已合入main d06198de7cbff56b4304b3483aeec32ad9a3e085，准备3.3归档候选，不替代小陈批准或预报主干已归档。

## 已成立前置

#38/#40/#41均已由小陈实际审查并正常合入。#41获批e59b57a与main d06198d整树同为0c46acc490690816be0bca0d7bca1d9c0d4c4dc0；main完整CI43362518043成功，原始artifact14008414768 ZIP摘要与GitHub一致、四段gate/四源报告/四镜像HIGH/CRITICAL0/同轮运行ID及清理已实际核对。见[主干核对报告](main-integration-2026-11-13.md)及完整原件。

今天本机首次gate因Docker未启动退出1，原失败保留；正常启动遇sailor-ingest.sock绑定冲突，未重置Docker、删卷或改变安全/隐私设置。用户随后恢复Docker29.8.0；现正于另一干净d06198d worktree补跑完整gate，本候选不混入该运行输入。未完成前不把本机结果写为成功。

## 具体差异

- 新增assignment-tasks主规约，将已批准delta的八项AT Requirement、二十一个AT-SC Scenario从第一条需求到末尾逐字同步，只把ADDED Requirements改为主规约Requirements并在Purpose链接原设计。
- 完整移动原四个Markdown文件到archive/2026-11-12-assignment-tasks，原来没有.openspec.yaml，不补造。proposal/delta正文保持；design只调整移动后的相对链接；tasks完成已真实通过的3.2，3.3保持未勾。原失败/待测/独立复测均保留为有日期的历史快照，文末追加真实主干结果。
- 九操作OpenAPI的x-spec-change与登记introduced_by、声明校验器固定路径同步指向实际归档位置，不更改字段、错误、权限、配额、幂等/调度规则。结构化去掉这一个路径替换后，与d06198d合同完全相同。
- 运行批次05 spec指向主规约，来源更新为已验收#41/main d06198d；服务集合/配置/网络/镜像/探针及review_status不变，不将10-B全量/C计完成。
- 当前规范/README/共享表据实际main校正22 current/Target5/生产未绑定执行器；给06/07B/08登记可用合同与待验范围，不冒称他们已批准新变更或启动暂停项。唯一历史报告改动仅修一份规约交接报告中的导航链接，结论/原件/原执行者不改。

Go、前端、SQL/摘要、配置/Compose、Dockerfiles与依赖均不改。归档路径变更不开放作业页面、真实批改/成绩或生产mock开关；现有生产Task仍真实pending。

## 已完成与尚待

本候选`npm run check`实际退出0，含合同双路径26组、类型/构建/Go vet与普通测试、样板隔离、批次与严格OpenSpec10/10；逐字八需求/21场景和195个相对链接检查通过。当前主干完整本机gate及本候选暂存/全历史秘密扫描、最终head CI结果按实际完成后追加，不预填。新证据路径与摘要见[结构化候选](archive-candidate-facts-2026-11-13.json)。

下一步小陈独立核对主规约逐字、四文件完整移动、合同只改路径、真实合入/主干原件与当前head CI；按远端保护正常合入后再更新3.3和最终归档状态。06/07B/08及持有人生产升级/回退/长期部署不在本次验收范围。
