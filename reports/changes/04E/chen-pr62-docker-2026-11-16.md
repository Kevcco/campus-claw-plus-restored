# #62 独立 Docker / OCR / MySQL 复验

小陈 / Kevcco（AI辅助），2026-11-16，DI-SC-16/17/19/25。

## 结论与版本

原PR直接修正的 `134b2b9059abf26ebb85f78c948e0f4dddd7e9e6` 已在独立 Docker 测试环境实际复跑：`npm ci`、`npm run setup` 退出0；完整 `npm run gate` 的check、audit、secret-scan、verify均退出0，无跳过。check150633ms，audit4148ms，secret-scan1896ms，verify441357ms；原600秒verify上限保持。Node22.23.4、npm10.9.9、Docker28.3.3、Compose2.39.1，strict-ssl=true，未关闭TLS校验。没有修改系统默认Node或其他项目。

原ZIP SHA-256为 `8742c010745dcc23b0dfc035c3d2740bb03ec194ecb9c741630184a71d6f85e0`，58份原件摘要逐项一致；实测Git树2856个跟踪blob与报告指纹逐项核对。五生产制品同轮扫描/真实运行ID、HTTPS、身份与资料/作业事务、持久worker、迁移6/实际备份恢复5、PDF/Word40正例/25负例、私有HTTP/探针和清理均通过。解包秘密扫描0。精确原件与独立核验JSON见[清单](chen-pr62-docker/manifest.json)。

## 修正后的真实 OCR 与数据库

重新按已登记源码/模型/轮子摘要构建隔离研究镜像，实际OCR镜像ID `sha256:5495f576331e1bb033a1a88b1bd4aea5f7ea6c2576c9a99e40242eae8e11be07`。14个开发PNG/JPEG经真实Go→HTTP→Tesseract识别：4正例25/1618，micro CER1.545%；中文23/395（5.82%），英文2/1223（0.16%）。10负例诊断全部命中，两个元数据变体逐字不变，超像素/损坏PNG/JPEG三类实际拒绝通过。独立核对Schema、原图尺寸、码点quote及正面积有界框；连同PDF共核对1205个词段，不冒称正式100框IoU验收。

原4页PDF→私有RPC→worker→MySQL→教师revision2→同班学生逐字出处检索实际通过，437词框、35/1618（2.16%）；跨班为空，原始快照保持。取消时真实观察busy、请求取消、子进程回收和服务复用。13组真实MySQL授权/并发幂等/CAS/回滚/租约与恢复测试通过。识别器只挂载7份运行代码，无gold、fixtures或盲测输入。

prepared7的4组真实迁移测试及prepare6、upgrade7、restored6、restored7、source7五阶段真实dump/隔离恢复通过。原6源与7源分开，dirty/严格前向repair/结构和历史漂移拒绝保留；27份相关源码/SQL摘要与实测Git blob一致。生产仍Target6，不因测试成功激活迁移7。

## 失败保留及工具准备

首次研究依赖下载不完整而退出1，完整原件保留；按同URL、版本、字节数及SHA重试成功。首轮gate的check/audit通过，扫描器官方包下载连接超时，secret-scan退出1、verify未运行。随后从既有官方8.30.1发布归档核验SHA `551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb` 并提取同一二进制，扫描规则未修改。

下一轮前三段通过，verify在GHCR漏洞库下载的原3分钟预算内超时，尚未扫描镜像，失败原件保留。仓库已有prepare:scan-db预下载曾发起ECR下载；发现仍有效的既有缓存后，只停止本次下载容器，并只读复制源DB、核对完整SHA `dcbd479f6de2e82a0d1588afe354c12456dcacd4b8f6128e2046ae0623a67f87` 与源前后稳定性。原metadata逐字保持，NextUpdate为2026-11-17T01:11:02Z；未改时间或使用过期缓存。原prepare命令重新执行退出0后，再原样跑完整gate并通过。上述失败、受控停止、准备及最终成功原件均在ZIP内；未跳过扫描、放宽超时或改写失败为通过。

## 清理、远端与剩余范围

测试专属容器/网络/临时凭据清理通过；额外研究镜像标签已删除。临时SSH授权已移除，旧密钥登录被拒绝，本机临时私钥及公钥文件已删除，其余授权保持。没有持有人数据库升级或长期部署。

Git读取仍main909eefd，仓库数字ID和精确main CI43865539691的REST读取仍404，原因未确定；浏览器读取另因自动检查无法可靠确认当前URL而停止，未操作页面。没有把旧PR绿灯当主干通过，也未伪造review、批准或合并。#62后续报告提交只补文档和原件，运行源码保持实测134b2b9；最终head远端CI/实际批准及合入后main复验仍待核验。

本轮只是隔离图片候选复审与真实复跑，生产28 current/15空active/Target6保持。A余项、B1.2完整公共冻结/正式50页与100框/native闭包/混合页/公开校对与生产接线、C及其他暂停模块均不因此完成或启动。共享进度表对应04E行同提交更新。

入库复核：本次仅修改文档和证据，运行源码、测试、配置及机器合同与实测134b2b9保持；九份文档313个本地链接、23份作者原件及9份本轮入库文件的Git blob摘要逐项核对，非04E进度行和表格列数保持。foundation、严格规约11/11、全历史与暂存秘密扫描均退出0，diff空白检查通过。一次误用不存在的spec:validate命令退出1后，按仓库登记的check:spec执行并通过；没有改动或跳过检查。
