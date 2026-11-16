# OCR原始词框快照与租约原子提交

2026-11-16，小张。DI-SC-17/18/19，基于上一批真实PDFium/Tesseract开发输出，现有候选版本表上追加source/job两表（共六张候选表），完成原始词框快照、租约和待核对版本的真实MySQL事务组件。没有重跑盲测或冒称新的识别质量验收；生产Target6/28 current/15 active保持，未注册新接口或部署持有人库。

原始OCRResult以MEDIUMBLOB逐字节存储，记录SHA-256及原件摘要，JSON不经MySQL重排。Go严格解码拒绝重复键/大小写别名/null/缺失零值字段/未知字段、伪quote、越界/零面积bbox、非法方向、非有限置信度、像素/正文/输出/总词数超限，使用Unicode码点核对所有词区间。教师校对追加新版页文本，不覆盖原始文字/置信分数/原图框；原始快照只允许本班教师读取，读取时核对快照/原件摘要并重新校验JSON。

持久作业入队先检查可信教师和当前班级；Claim锁定文档与作业，20并发只能一个持有人。每次claim递增epoch/attempts、随机owner，租约60秒、数据库UTC计时，原件字节逐次核对摘要。Heartbeat和Commit都检查owner/epoch/未过期/原件；Commit在同一事务保存原始快照、revision1、head并结束作业。最终UPDATE再次用数据库时间检查lease_until；即使事务已写入source后租约过期，也必须全部回滚。

## 实际证据

九组真实Go/MySQL检查（保留原五组校对/检索），另严格源码/内容单测：

- 重用4页开发PDF的实际TSV生成OCRResult，同份真实PDF原件入库，20并发claim只有一个成功；心跳、原件摘要绑定、原始快照逐字节保存与读回通过。
- 教师校对后原始OCRResult和revision1逐字不变；学生和跨班读取拒绝；修改快照字节但保留旧摘要时拒绝。
- source写入后注入错误，所有原始快照/草稿/head回滚；让租约在事务内部过期也回滚。新epoch接管后旧epoch心跳/提交拒绝。
- 连续三次超时接管后终态failed，不产生任何来源或可发布版本。
- 实际独立Go子进程获得租约后被Process.Kill杀死；未过期时不能接管，随后仅推进自有夹具的租约时间以免等待60秒，新owner/epoch2接管并保存成功。此为真实杀死持有人测试，不声称已经杀过正式生产OCR服务。

去掉最终SQL的lease_until条件时，真实事务内过期检查红1；恢复逐字源码后绿0。red/green原件和实际源码摘要均保留。fixture绑定上一批真实引擎输出、原PDF和生成OCRResult三个SHA，见results.real_ocr_fixture；该过程不挂gold/盲测到识别器，不以保存数据库证明100框IoU达标。

## 剩余接线

数据库job组件尚未由生产worker调度；显式OCR失败分类/重试接口、私有HTTP RPC、PNG/JPEG上传、混合文字层优先、HTTP鉴权/幂等/版本化公开读取及教师校对页待接。迁移7目前为准确候选SQL，尚需结构manifest、dirty/前向repair、Target6备份/恢复6及默认迁移装配验证。本批空库逆向仅证明移除六候选表后原19表保留，不能代替备份恢复。B整体继续未完成。

本轮完整npm run check退出0（最终源码九组真实MySQL、源码严格解码单测及全部原检查），audit两树0。最终数据库run/实际Go源码与候选SQL SHA已核对，凭据和自有容器清理成功。#61前序7fe97a9完整CI43838881811已成功，本次新增来源/租约代码最终head CI另验；本机完整verify本批未重跑。
