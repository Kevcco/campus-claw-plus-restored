# 私有OCR RPC与真实worker链路

2026-11-16，小张。DI-SC-17/18/19：新增独立私有OCR服务、固定origin的Go适配器和顺序worker。当前在独占internal网络运行研究镜像及候选六表数据库；生产Compose/迁移Target6/28 current接口和15 active键保持。此批交付的是实测互通组件，正式公开校对接口/页面、迁移7恢复与生产装配仍待。

## 最终隔离实测

服务固定`http://document-ocr:8092/internal/ocr`，二进制PDF输入；原A解析器35秒入口保持。候选OCR使用180秒识别任务、185秒工作预算、2秒响应发送、190秒客户端总预算；输入/响应有界，1CPU/512MiB无swap、32进程、只读、64MiB临时盘，无发布宿主端口。只接受canonical bearer，禁代理/重定向/用户URL；并发1，额外解析请求明确503。机器profile见[private-rpc.json](../../../../docs/contracts/ocr-draft/private-rpc.json)。复用既有令牌格式仅限隔离进程，正式消费者/密钥装配尚未激活。

最终实际OCR容器只挂六个运行文件，启动后用真实文件系统断言没有corpus/gold/blind/fixtures；Go验收器与宿主评估工具的gold不会进入识别服务。4页真实开发扫描PDF通过私有HTTP重新识别，顺序worker完成claim/heartbeat/lease提交，437个原图词框及正文入库。教师调用校对组件发布revision2后，同班学生关键词检索得到逐字出处，跨班结果为0，原始快照逐字不变。以上是服务/数据库组件互通，未声称真实登录浏览器或公开页面已经交付。

最终接口正文沿用Tesseract原TXT（NFC），将TSV词按顺序定位到该正文，再保存码点区间，避免中文多插空格；找不到逐字词段时拒绝。宿主对实际返回/持久化的OCRResult跑2020-12 Schema和开发CER，而不是借用另一份txt分数。4页正例实际35/1618码点错误，micro CER2.163%；50页盲测和100框IoU没有运行。

取消测试先观察服务busy，再取消真实私有请求，识别子进程组回收、并发名额释放和下一请求可用均通过。服务日志严格只有request_id/path/status/duration_ms，没有token、原件或正文。最终原件见real-results.json、real-pipeline-tests.txt、real-server-logs.txt和actual-source.json；整个pipeline的所有容器/专属网络/临时凭据均已清理。

## worker和失败

worker每20秒续租，收到取消/丢失owner即取消HTTP，不发布晚到结果；SQL停机等错误仍返回非零，由后续默认装配监督恢复。识别失败保存闭集reason并终态failed，不自动耗尽三次；本班教师只能对超时/中断/内部失败显式重试，attempts累计最多3次。新SQL同步queued/processing/succeeded/failed状态、epoch/attempt计数和reason约束。

默认check保留原全部检查，真实MySQL由9组增至11组，新增失败分类/显式重试/取消和丢失租约测试。Python六组覆盖栅格、正文/词框与HTTP鉴权/忙碌/闭合输入；Go客户端覆盖固定origin、合法令牌、错误200/状态码错配、重定向和真实短超时。HTTP200缺合法OCRResult保护在同源隔离副本中受控破坏红1，恢复绿0；活动源码未在完整检查期间被修改。

## 留存的失败与限制

first-real与pre-isolation记录是早期功能运行，当时代码挂载范围包含语料目录，**不计最终隔离验收通过**。现已改成六个文件的白名单目录并在容器内实断言排除gold/盲测，最终独立重跑通过；旧记录保留，不追改成当时已隔离。早期TSV拼正文没有沿用TXT，最终已修正并对真实接口正文重测。

当前实现PDF RPC；PNG/JPEG、文字层优先混合页、完整native许可/漏洞闭包、迁移7结构/dirty/前向修复/Target6备份恢复，以及公开版本API/校对页仍须继续。默认CI自动跑Python/Go/SQL测试；实际OCR RPC需指定本机研究镜像ID运行，尚未加入干净机器默认构建。复现：先按ocr-pdf-development构建并核对研究镜像，再运行`node scripts/check-ocr-revisions.mjs sha256:<实际镜像ID>`，原件独立写入output/ocr-rpc-pipeline，不会被普通check覆盖。

本轮最终完整check退出0、audit两树0；修正识别容器挂载隔离和接口正文后，六文件代码白名单/无gold无盲测的实际容器断言、两组真实RPC（含取消）、11组真实MySQL及最终接口Schema/CER全部通过。最终响应4页/437词框，35/1618字符错误、CER2.16%；原件/源码摘要、凭据与专属资源清理均核对。私有RPC原件使用独立输出目录，避免普通check覆盖。#61前序debc839完整CI43841089715通过，本批最终head CI待；本机verify未重跑。浏览器管理入口实查为未登录，未更改保护规则。
