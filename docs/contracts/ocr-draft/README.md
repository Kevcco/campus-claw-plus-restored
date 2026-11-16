# 04E B1.2 OCR接口合同草案（未冻结、不可接线）

本轮小张在A完整复验及#59真实批准等待期间准备B合同。`openapi.json`是独立草案，不并入当前公开/私有OpenAPI或endpoint-register；28 current、Target6、15 active键保持。此目录通过检查也不表示B1.2全部完成。

## 已具体化的边界

- 输入候选为原始PNG/JPEG/扫描PDF，1..10MiB、最多100页、每页20MP；正文2MiB UTF-8、响应8MiB、词段总数100000，单页15秒、作业180秒。CPU1、512MiB无swap、32进程、64MiB tmpfs、并发1、非root/只读/无外网；不能用文件名、URL或gold调用识别器。
- 私有结果单源`OCRResult`含正文码点区间、连续页号、原始图像尺寸、方向矫正角度及词框。所有bbox映射回矫正前原始图像左上归一化xyxy，不能用矫正后坐标混评分；quote必须逐字等于body区间。错误没有body/半结果，明确语言包缺失、超时、中断和资源失败。
- 引擎仍为Tesseract5.5.4/固定fast模型候选，实际选型以开发实测与安全扫描为准。`engine_confidence`只是引擎原始分数，不是准确率或校准概率；暂定mean<80或低于50的词占比>20%仅供开发诊断，**不是冻结的生产阈值**。
- 本草案提议OCR原始结果一律`needs_review`，教师明确核对才发布可检索版本；低质另标记，不能靠“一律需核对”声称质量分类达标。清晰/旋转CER及负例识别诊断仍单独测量。是否采用全量人工核对须在最终冻结时审定。
- 校对候选`POST /api/v1/documents/{id}/ocr-reviews`使用可信教师/当前班级、Origin/CSRF规则沿用现有写入、共享12次/用户分钟配额与5秒SQL预算。body只含base_revision、acknowledged和完整连续页文本，拒绝客户端role/class/owner。跨班与不存在404、学生403、旧base_revision409；无写入副作用。
- 同一用户/当前班/文档/幂等键提交相同规范载荷只生成一个新revision和原201快照，重放前重新授权；异载荷409。事务内CAS草稿版本、追加不可变正文/页记录/关键词索引、发布指针和原201；任一失败全回滚。识别过程不在SQL事务内，不将旧202改成201。
- 教师改正文不能复用与新文字不相符的OCR词框。新版本保存新正文与页定位；有改动页的词级定位需重新标注或显式不可用，原识别版本及其原框留存。旧出处按document/revision/page/range读原版本，不跟随最新发布指针漂移。

## 兼容、数据库与准确冻结前必须完成的工作

1. 现有000006将documents.version和相关FK锚定为1，当前Document Schema也为const1。**禁止直接覆盖这个v1正文或递增该父键**；既有GET详情、下载、原202及旧引用保持。候选方向是追加独立revision/页词框/校对幂等/ocr job表，并保留原件父资源为固定锚点；最新发布指针与不可变旧版本分离。
2. 原GET详情URL未含版本；新版本读取必须有显式revision路径，旧URL仍能回查v1，不把原URL默认为最新正文。新资料列表/上传PNG-JPEG/搜索与当前六接口的兼容投影需要同一批准确OpenAPI和Schema；当前草案**尚未定义完这些公开操作**，不得据两条草案path提前实现。
3. 追加迁移编号7只是预留方向，准确DDL/摘要/复合FK/结构校验、Target6备份、五表旧数据回填策略、dirty中断/前向修复/隔离恢复6仍待独占MySQL验证。不得改旧六份SQL、提前更改Target或运行数据库。
4. A私有RPC现有35秒不能承载180秒OCR；最终须冻结准确origin/path、输入读预算、185秒左右总RPC上界、认证/健康、持久job的owner/epoch/租约心跳和最多三次规则。无外网模型下载、共享解析并发与A新上传公平性、解析器停机旧ready可读都须有真实反例。目前不新增active配置或新服务。
5. PDF栅格器精确版本/轮子/许可/外链与脚本禁用、单页尺寸预检、完整依赖包闭包/漏洞扫描、开发集方向与质量阈值仍待；必须与最终引擎选择一并冻结。后续14页开发PDF的PDFium栅格/OCR链路已实测，4正例CER2.16%，见ocr-pdf-development报告；完整依赖/安全闭包与生产混合页策略仍待。

## 可执行检查与范围

`node --test scripts/ocr-draft-contract.test.mjs`对相同正反例跑Schema2020-12单源和OpenAPI3.1解引用两条路径，另验像素乘积、连续页码、逐字区间、正面积bbox和教师/班级/CAS语义。它是合同检查，**不能代替真实权限、事务、OCR准确率或恢复测试**。B1.2保持未完成，候选参数与未决项必须完成审查后才转frozen。

## 2026-11-16版本事务候选实测

本目录000007_ocr_revisions.up/down.sql已在独占MySQL8.4.13真实执行，追加revision/head/idempotency/chunk四表，不改变documents.version=1。Go ocrreview组件的CAS、20并发幂等、跨班与学生限制、事务回滚和仅当前发布索引已通过五组真实测试及班级过滤红1→绿0。此为准确候选SQL，不是runtime已启用迁移7；完整词框来源及owner-epoch绑定后续已实现并经九组真实MySQL验证；结构manifest/备份/dirty修复/恢复及公开接口兼容仍待。表名前缀不自动授予公开调用能力。

## 原始词框来源与作业候选

本轮追加document_ocr_sources和document_ocr_jobs，准确候选SQL共六表。来源保存原始OCRResult字节/SHA和原件摘要，校对不会覆盖词框；提交严格解码并绑定owner/epoch/数据库租约，最终SQL再次核对到期时间，来源/草稿/head/作业同事务。独占MySQL九组和最终租约保护红1→绿0通过，包含实际杀死持有人与接管。仍需私有RPC/默认worker、显式失败分类重试、结构manifest/Target6备份恢复及公开接口接线。机器候选见source-jobs.json，不改变runtime activation。

## 私有RPC与worker候选已互通

准确隔离组件profile见private-rpc.json。独立服务http://document-ocr:8092/internal/ocr，当前实现PDF，PNG/JPEG仍计划；190秒客户端总预算独立于原A解析器35秒。沿用既有令牌格式在隔离进程传入DOCUMENT_PARSER_TOKEN，不代表该键已登记新生产消费者；生产服务/密钥消费者须随正式接线原子更新。实际4页PDF→私有RPC→顺序worker→11组MySQL候选→教师校对→学生检索及请求取消释放进程已通过。显式失败原因保存，只有超时/中断/内部失败可在attempts<3时教师重试，不自动消耗三次。候选六表DDL已同步状态约束与摘要；结构manifest/备份恢复/公开接口及最终质量仍待。

## 迁移7真实嵌入式执行和恢复已验证

六表SQL已按SHA加入migrations目录，执行器显式Up(7)/Check(7)/Repair(7)和测试专用空库down可用；生产Target6和identity-admin默认目标保持，不会启动自动DDL。固定结构快照仅从已知SHA候选SQL首次生成并提交，校验器不从待检查库学习。旧合同登记12个当前SQL+2个prepared SQL，准确SHA/快照路径白名单仍强校验。实际6→7/六DDL中断/严格结构/真实dump恢复6和7全部通过，原件见ocr-migration7报告。公开接口与服务消费者/Target7的原子正式激活待下一批。

## PNG/JPEG组件和私有OpenAPI已具体实测

2026-11-16图片OCR推进：新增PNG/JPEG单帧、签名/CRC/尺寸/20MP及元数据隔离准入，复用原Tesseract而不加解码依赖。实际14开发图片经Go/私有HTTP重新识别，4正例25/1618错误、CER1.55%，10低质负例全部低质或失败；2个元数据变体结果逐字不变，超像素/损坏PNG/JPEG三种真实拒绝正确。旧4页PDF/worker/校对检索/取消及13组MySQL保持通过。识别容器7代码文件、无gold/盲测，全部专属资源清理；私有OpenAPI准确补齐，公开草案移除旧私有path。像素守卫隔离红1→绿0、默认Python增至10组/私有合同2组，最终完整check0；audit两次TLS网络失败后原命令两树0。默认地址池并发耗尽的失败原件保留，改用已有显式空闲internal子网分配器后复验0。前序d540bdc完整CI43861270879成功；本批最终CI/本机完整verify未重跑，公开图片上传/版本API/校对页/混合页/native安全闭包和正式盲测仍待，不勾B整体。

当前private-openapi.json是准确私有组件HTTP合同，private-rpc.json为机器资源profile；public openapi.json只保留校对候选，不再包含旧私有path。两者仍未激活生产/公开路由；后续需完成公开版本读取、图片原件兼容投影、配置消费者与Target7准确冻结，再原子生产接线。
