# PNG/JPEG 原件兼容组件合同

2026-11-16，小张依据组长统一开发授权冻结本组件。DI-SC-02/07/09/16..19/25。这是后续迁移8的准确候选，不在backend embed/manifest、不改变默认Target6；prepared7、七个OCR HTTP组件及此前真实批准范围保持。执行本目录SQL探针不等于迁移8、恢复、生产接线或页面验收。

## 原件和身份

沿用Document的班级、教师上传、学生已发布只读、原件v1、逐次可信会话、相同Origin、12次/用户分钟及5秒请求总预算。图片原件的扩展名白名单为.png/.jpg/.jpeg，格式归一为png/jpeg，签名必须匹配；每文件1..10MiB，multipart请求上限10551296字节。标题仍1..200码点。完整PNG/JPEG解析、CRC、单帧、20MP、元数据隔离和解码在无外网私有引擎中完成；入口签名通过不能宣称文件有效，失败状态由OCR作业提供。损坏文件仍可由本班教师下载排查。

图片不经过PDF/Word解析worker。一个事务保存不可变原件、SHA、Document记录与原202幂等快照，初始status=needs_ocr、page_count=1、attempts=0、body=NULL；不创建document_parse_jobs。attempts=0准确表示未做文本解析，不借用一次假解析。OCR必须另由教师显式POST ocr-jobs触发，OCR尝试次数、诊断、租约与修订保存在既有OCR表中。重复上传同用户/班级/键/载荷重放原202，即使OCR后来成功也不改快照；异载荷409。PDF/Word仍沿原队列及状态规则。

## 读取与下载

未来激活时原Document元数据格式枚举扩充png/jpeg，图片只允许上面的不可变原状态，不变为Document ready，也不把OCR正文回写原body。教师可读本班所有原件；学生须在同一读事务中核验本班published_revision指向有效review后，才能在列表、详情和原件下载看到该图片/扫描PDF。未发布及跨班同404，列表计数同过滤条件；不能仅凭OCR succeeded或source revision1放行。原Document详情仍body=null/locations=[]，OCR正文和出处只由显式ocr-revisions返回；旧PDF/Word ready路径不变。PNG下载Content-Type=image/png、后缀.png；JPEG为image/jpeg、后缀.jpg，强制attachment和nosniff、no-store，逐次验证原件SHA，不发公网URL。

## 数据库兼容与下一步

准确000008只有一个MySQL原子ALTER TABLE，扩充format并替换ck_document_state；其它四个CHECK、六个Document复合外键及迁移1..7字节均保留。旧状态的有效集合逐字保持在pdf/docx分支；图片只能needs_ocr/0/1页。SQL NULL显式拒绝，不能利用CHECK UNKNOWN绕过。旧Target6/7制品须拒绝8，不能在活跃新库降级。

迁移8正式实施须追加准确SQL和摘要/结构快照、离线Target8迁移/dirty/漂移/单DDL中断前后检查。持有人当前实际Target6，可逐步6→7→8一次前向升级，不要求先部署一轮Target7服务；须保存真实升级前6备份，隔离恢复6并用实际已验收Target6旧制品验证身份/TXT/Document原件与原202，保留升级后8库。内部7检查点另备份/恢复并用明确标为组件的Check7验证，不能冒称已有生产Target7制品。若未来已经实际部署7，再按真实7基线执行同等备份/恢复与旧制品验证。无生产down或force入口；回退仅停写后恢复升级前备份，不保证升级后图片写入随旧库恢复。当前仅冻结up，不发布破坏性down。

激活另须同提交更新Document单源Schema/OpenAPI/语义与前端类型、35 current登记、配置消费者及默认OCRworker、上传/列表/下载SQL过滤和浏览器验收。若先启用PDF OCR七操作而暂不开放图片，可保持Target7；图片准入必须Target8。不得只扩枚举便宣称已经交付。

修复8必须识别单个原子DDL的两个真实崩溃边界：dirty8且documents逐字匹配旧7定义时只执行一次准确ALTER；逐字匹配新8定义时不重放ALTER。两者都须验证其它1..7表/精确历史与8摘要（缺失可显式补记，已有不符拒绝），最终完整8结构一致才记录并清dirty；任何第三种结构、未强制CHECK、错误外键/枚举/表选项均拒绝，不能边学习实际库边修复。此规则待真实Executor/中断/漂移用例实施，不用SQL探针代替。
