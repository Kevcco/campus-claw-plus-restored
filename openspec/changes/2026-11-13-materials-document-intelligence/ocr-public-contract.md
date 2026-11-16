# OCR 版本与校对公共组件合同

2026-11-16，小张按组长最新统一开发授权冻结此组件范围，DI-SC-17/18/19/25。准确机器入口为 docs/contracts/ocr-public/openapi.json、OCRPublic.schema.json 和 profile.json；不将本冻结等同于整个B1.2完成、原生安全/质量验收或生产激活。当前默认仍Target6/28 current/15空active。

## 版本、权限与兼容

原件始终original_version=1；现有Document v1 URL、字节、正文与原202保持。OCR revision是独立轴：revision1是不可覆盖的原始识别稿，教师确认追加revision2及以后。显式读取 /documents/{id}/ocr-revisions/{revision}，旧修订永久按原页/码点读回；学生只读kind=review的修订，检索只用当前published_revision。校对页定位到新正文页/码点，不冒用旧词框；原词框与置信分数只在教师ocr-source读取，始终原编码像素坐标。

新增七个明确组件操作：GET ocr状态、GET ocr-source、GET ocr-revisions/{revision}、POST ocr-reviews、POST ocr-jobs、POST ocr-retry，以及GET /api/v1/ocr-search。路由只由显式OCRHandler装配；本批不在默认server登记，默认current接口保持。后续生产批次须原子登记/激活Target7、配置消费者、服务和页面；图片原件公开上传/下载与统一列表仍另需完整兼容合同，不能由本七操作推定已实现。

每次以现有身份服务Me重新读取真实会话/当前班级。学生写入与原始识别源一律403；跨班与不存在404相同。状态对学生只返回已有发布的资料，未核对识别稿隐藏。教师可看当前班级未发布状态；所有写入遵循现有Origin规则、空body或严格JSON和共享每用户12次/分钟配额，业务总预算5秒含身份、读body与SQL。

## 请求和回执

POST ocr-jobs无body，按文档/班级自然幂等；初次或重复都202/accepted，重复不自动重跑失败或已完成作业。失败只允许教师对超时/中断/内部失败显式POST ocr-retry，attempts<3才排回queued；并发重试只有一次成功，重复409。两者202 Location指向ocr状态，校对201 Location指向不可变修订。

POST ocr-reviews沿用唯一OCRReview Schema，要求Idempotency-Key、base_revision、acknowledged=true、连续完整页文本。用户/班级/文档/键作用域、规范结构序列化摘要、事务CAS及原201字节重放保持；每次重放先重新授权。不修改教师文本的Unicode/空白，保存原输入码点；识别原稿沿用引擎NFC。解码后正文总2MiB含页间两个换行，HTTP JSON最多12MiB+64KiB，覆盖ASCII全量Unicode转义及100页开销；非法/未知/重复字段或状态为400/409，HTTP过大413，拒绝半发布。

GET读取与两种作业动作必须空body，无额外query。检索只接受q/mode=keyword/top_k；q去首尾空白后1..2000码点，top_k默认4/最大10，与现有资料查询长度兼容。持久片段4096码点、相邻重叠1999，保证任何合法2000码点查询在某片内；出处输出逐字quote、页、全正文码点及显式修订URL，不查询旧发布片段或其他班级。仅加大新候选片段布局，无已部署OCR索引需迁移；production Target6没有OCR表。

所有已识别路由返回no-store、服务端request_id，失败沿用现有Error单源。405带Allow且空body。未认证401、越权403、不存在404、状态409、配额429/Retry-After、SQL不可用503、5秒超时504均不写入或发布部分结果。识别180秒工作永不在HTTP请求或SQL事务内执行。
