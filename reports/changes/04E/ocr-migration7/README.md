# OCR 迁移7、中断修复与隔离恢复

2026-11-16，小张，DI-SC-18/19/25。此批把候选六表接入真实嵌入式迁移执行器，显式 Up(7)/Check(7)/Repair(7) 可执行。生产 Target 仍为6，identity-admin 默认目标未改，启动不执行 DDL；未升级持有人库，也未启用公开 OCR 接口或页面。

## 实际验证

- 固定 MySQL 8.4.13 独占临时容器；空库到7、带旧资料的6→7和重复执行通过。SQL与结构快照绑定准确 SHA；旧1—6 SQL未改，未知结构和迁移历史不接受。
- 在第1至第6条 DDL 后逐一中断，均保留 dirty7。错误正文排序规则阻断 repair；恢复准确结构后前向 repair/repeat 通过，旧 TXT、PDF 原件和原202重放逐字保持。
- 错误摘要列类型、停用 CHECK、不可见索引、COMPACT 行格式与伪造历史摘要均阻断 Check/Repair/Up。关闭 SHOW CREATE 精确比较的受控破坏为红1，恢复活动源码后全套绿0，见 red-green.json 和对应原始输出。
- 逆向入口只存在于 ocrmigration 构建标签，限定明确确认的测试库名及六表全部为空。非空和错误确认拒绝；空库逆向仍保留 dirty7，不能假装回到干净6。正式回退采用独立恢复库，不删除已有 OCR 数据。
- 真实 mysqldump 源6及源7，分别恢复到两个独立数据库。恢复6通过 Check6/拒绝 Check7；恢复7通过 Check7/拒绝 Check6。原 TXT、PDF、真实队列 API 生成的原202，以及7的原始 OCRResult 字节、revision2、原校对回执均核验保持。完成后重查源7未变。

夹具使用显式可信测试 Principal，不声称新登录会话；TXT 和202由现有仓储/队列 API 生成并读取。恢复用 OCRResult 来自此前真实 RPC 的开发集输出，本批不重复冒称 OCR 引擎质量实测。备份 SQL 和临时凭据仅在忽略目录，完成后删除；报告保留字节数和 SHA，不提交数据库内容。

## 恢复发现并修复的问题

首次真实恢复6被结构守卫误拒，失败原件完整保留：first/second-failure、diagnosed-restored6-tests 和 diagnosed-restore6-structure。MySQL 导出恢复给 enum 列补写 CHARACTER SET utf8mb4，而该列已有 utf8mb4_0900_bin，数据库含义相同。比较器仅归一化这个精确组合；错误字符集、其他排序规则、列序、类型、默认值、索引、FK、CHECK及执行状态仍严格匹配。例外同步部署规范，单元正反例和真实恢复共同验证，不从待检查库学习“正确”结构。

## 证据与边界

results.json 和 migration/prepare6/upgrade7/restored6/restored7/source7-tests.txt 对应最终默认检查的真实运行；green-* 是受控破坏恢复后的另一次完整运行，before-red-* 是更早成功，不能混用各次备份摘要。restore_ms 是两次恢复及验证与源库复查的合计用时，不是生产恢复 SLA。

准确代码、SQL、结构快照摘要在 results.json；最终完整门禁与最终证据摘要见本目录补充文件。默认 npm run check 新增 check:ocr-migration，保留原全部检查和600秒预算。没有宣称 TLS 新旧服务进程切换矩阵、生产 Target7 激活、OCR 公开 API/校对页、完整盲测或语义检索已经验收。

本批最终完整check退出0，修订后的13组MySQL、4组迁移和5阶段备份恢复通过；同代码真实RPC重跑仍4页437词框、CER35/1618=2.16%，Schema/取消释放/清理0。完整gate在check通过后因npm审计TLS连接中断退出1，原件保留；原命令重试audit两树0。secret-scan和干净提交完整gate/本机verify待，不把前序CI冒称本head。#61前序e924fd1完整CI43852860415成功，本批未获新批准。

提交前补记：audit原命令重试两树0，260历史提交及暂存内容秘密扫描0；首次扫描误命中原有公开SQL摘要的diff上下文，已保留旧manifest格式、仅追加prepared7，未加任何扫描豁免。该格式调整只改变manifest及合同绑定摘要，干净提交完整gate仍待。
