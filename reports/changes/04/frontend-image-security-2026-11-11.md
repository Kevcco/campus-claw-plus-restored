# 04 合同 PR 的前端镜像安全修复

2026-11-11，小陈（Kevcco，AI辅助）在原 PR [#25](https://github.com/zwh9616/campus-claw-plus/pull/25) 修复交付门禁；对应 MR-SC-01..10 的运行前置与既有 IDF-SC-17 镜像安全回归。源提交为 `5d794b9`，集成基线为 `031c13f`。

## 实际失败与修复

当前提交的[完整 CI 42970555936](https://github.com/zwh9616/campus-claw-plus/actions/runs/42970555936) 在 `verify` 失败。下载并核对该次上传的真实扫描制品：frontend 的 Alpine 3.24.4 中 `libxslt 1.1.43-r3` 命中 HIGH 级别 CVE-2026-87636、CVE-2026-9947，报告均标注修复版本 `1.1.43-r4`。MySQL、backend、admin 镜像扫描成功；check、audit、secret-scan 成功。原失败保留，不以旧提交的绿色 CI 替代。

本提交在既有、摘要固定的官方 Nginx 1.30.6 运行镜像上，使用签名 APK 仓库将 `libxslt` 精确升级到 `1.1.43-r4`。不改变 Nginx 版本、配置、前端产物或业务合同。修复的包可获得性、实际安装版本与四镜像 HIGH/CRITICAL 为零均需由新提交的真实构建和扫描确认；不增加漏洞忽略项，不降低扫描级别，不删除失败断言。

## 已执行与待验证

- 本机 Windows，Node24.22.1、npm10.5.0、Go1.27.3。
- `npm ci`、`npm run setup`、完整 `npm run check` 均退出0，严格规约8/8；Go vet/test、现有身份与前端回归和合同16组均通过。
- `npm run audit` 退出0，根目录及前端均0漏洞。
- `npm run secret-scan` 全历史及暂存内容均退出0，未发现秘密；`git diff --check`通过。
- `npm run verify` 因本机没有可用 Docker 命令退出1（数据库准备进程ENOENT）；容器构建/镜像实扫与真实 MySQL/HTTPS 集成未在本机执行，完整运行验证以本提交 CI 的实际结果为准。
- 本次只修镜像交付前置，不创建000004 SQL，不启用资料路由。tasks1.2/1.3的既有审定保留，不能说小张已批准本次镜像修复。

## 下一步

新提交完整 CI 通过后核对实际镜像包版本和漏洞结果，向小张请求当前 head 复审。必须满足远端强制审批/检查后才能正常合入 #25；合入后从最新 main 开始04的2.1—2.6，再交小张终审。共享表同提交同步，原四份未跟踪文本保留。
