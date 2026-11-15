# 小陈审查04E私有Go客户端（DI-SC-14 / I1、I2）

本轮小陈 / Kevcco（AI辅助）审查小张的既有[PR #52](https://github.com/zwh9616/campus-claw-plus/pull/52)，基于最终来件808b02e09c82bdca76645ffe92b099a88e9104a8。04E实施负责人仍为小张；本轮直接在原分支补充回归与真实审查记录，未重复创建change。

逐行核对固定origin、canonical令牌、禁代理/重定向、500ms预检、1秒连接/32秒响应头/35秒总预算、响应8MiB/正文2MiB、严格UTF-8/重复字段/非空整数、HTTP状态与失败原因映射和完整码点出处覆盖，未发现本组件入库阻断项。原strictjson已拒绝null，本轮没有把已有保护误写成新修复。没有改客户端实现、合同、Compose、配置登记、迁移或公开操作。

新增两组真实httptest套接字回归：PDF与Word的unit/char_start/char_end分别为null共六请求，必须失败且不泄出正文/位置；响应体早断必须worker_interrupted、响应头后读体逾调用方预算必须parse_timeout且及时返回。全包九组Go测试退出0。[真实绿灯](chen-parser-client-review/focused-green.txt)。临时仅将位置strictjson.Decode替换成普通json.Unmarshal，新增char_start:null反例实际失败1；finally逐字恢复client.go后全包绿0，源码SHA256=37b99b388be1ab0d03af1ae4277933c996a7456b2478b70ba4771955102ade53。[受控红](chen-parser-client-review/null-red.txt) / [恢复绿](chen-parser-client-review/null-restored-green.txt) / [还原摘要](chen-parser-client-review/null-red-green.json)。这证明回归能挡住保护退化，不表示原实现有该缺陷。

本人环境Node24.22.1/npm10.5.0/Go1.27.3：npm ci/setup退出0；完整check中基础、合同40组、前端类型/构建、全包Go与strict11/11通过，随后document-parser镜像构建因spawnSync docker ENOENT退出1。容器未启动，后置SQL未运行，未执行本机verify；cleanup中的null/false是Docker命令无法调用，不冒称清理成功。[完整原输出](chen-parser-client-review/local-check.txt) / [未运行原结果](chen-parser-client-review/local-parser-not-run.json)。audit两树0；全历史222与暂存秘密扫描0。四份原未跟踪文本完整保留。[命令退出码](chen-parser-client-review/local-results.json)。

独立下载来件[完整CI43597345884](https://github.com/zwh9616/campus-claw-plus/actions/runs/43597345884)，artifact14114993742，原ZIP SHA256=ef62dc85fccd0bf7d91e65a7961c78cbf852ab34ccaacbfb8a84fa22e85f6a4b，与GitHub digest一致；实际预演b301fe49c80a6e31740b600f392f332ea5f9cdb0的树82b76f132011662e181ef3bdeaccf856748f5900与来件相同。核验四段0、11个实际解析/Go/脚本输入指纹、两轮同不可变镜像Go→HTTP→Python各40文件/195位置、错误鉴权拒绝、真实删除服务后的health=false/worker_interrupted、安全日志及每轮四项资源/秘密清理；原8组内核、7组HTTP、25负例/45关键句、四生产镜像和候选零HIGH/CRITICAL、18组05/五恢复/三HTTPS及SQL16实际拒绝也核验通过。[原ZIP](chen-parser-client-review/incoming52-evidence.zip) / [客户端逐项核对](chen-parser-client-review/incoming52-client-verified.json)。这是下载原件独立核验，不是本人本机Docker复跑。

同时核验#50已经小张真实APPROVED5904277600绑定6422f29273810de9779677cf31d8be9650e5fe99，正常合入mainc1ac4213580df1fcc03520b8dddcfbece2a3344d。精确[main CI43593495925](https://github.com/zwh9616/campus-claw-plus/actions/runs/43593495925)成功，artifact14113482804，原ZIP SHA256=deb1bf85dfdcdbe235fe9a3623b3fb9ce6b826ff2e5f92d12b9b2fa8fd69c78b；四段、原解析/HTTP、05运行、SQL候选与清理逐项核验。[主干原ZIP](chen-parser-client-review/main50-evidence.zip)。

本提交新增测试后必须重跑最终head CI，再实际批准/按保护正常合入并核验精确main；上述来件绿灯不替代本提交最终CI。#53队列、#54迁移6和#55生产接线已提交，仍须逐项审查，#55尤其等待三个组件获审合入。当前生产仍22 current、Target5、13 active空值键和四服务；A1.2b/A1.3b/A2父任务及04E整个A/B/C、真实Document页面未验收。

文本日志按仓库LF保存，两个下载ZIP保持原字节；目录摘要按实际入库文件重算。

2026-11-15实际主干复验：#52精确main2ece150完整CI43669560990成功，artifact14144156151原ZIP与GitHub摘要一致；四段/真实40文件195位置/鉴权/断连/原运行/SQL/清理全部独立核验，解包秘密扫描0。[实际批准与集成](chen-document-queue-review/main52-acceptance.json) / [精确main原ZIP](chen-document-queue-review/main52-evidence.zip)。前文“本提交时运行”为先前快照，现已收口。#53本次只补主干原件，队列代码/回归输入逐字不变，补证head自身CI和实际批准另核。
