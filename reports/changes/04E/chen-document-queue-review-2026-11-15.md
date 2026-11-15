# 小陈审查04E持久队列（DI-SC-07至10 / DI-03、I3）

本轮小陈 / Kevcco（AI辅助）在小张原[PR #53](https://github.com/zwh9616/campus-claw-plus/pull/53)审查34defc12fe910d8a9245245486900f31520c0d79；全部04E实施仍小张负责。没有另建同一change，没有把队列组件判为生产上传或A2父项完成。

核对store.go/worker.go与真实MySQL用例：授权身份由未来调用层供给；(user,class,key)二进制作用域与长度前缀payload摘要；原件/job/原202事务和不确定提交后原key查回；数据库UTC时间、owner/epoch/60秒租约、20秒心跳、1秒轮询/单worker/三次上限；原文SHA复核、全文码点覆盖、2000/200片段与结果原子事务、最终live-lease CAS；三种失败显式retry及取消不发布。5秒SQL与事务外35秒解析预算分列。未发现组件入库阻断项；真实解析回调、授权HTTP/幂等预检顺序与生产网络/恢复仍需后续接线验收。

新增TestReviewerUploadBoundaries：14类非法输入，包括无身份、超signed BIGINT班级、key长短/Unicode/路径、空白/控制/无效UTF-8/超码点标题、空/超10MiB原件和开放格式；200个非BMP码点标题、64字节key、精确10MiB、PDF/Word合法边界通过。新增TestReviewerDigestPreservesBoundaries：字段拼接歧义、格式、原字节差异必须改变摘要，相同字节必须相同。本人本机连同原结果边界共3组Go测试0，不含build-tag MySQL用例。[真实测试](chen-document-queue-review/local-go-green.txt)。临时仅移除digest的字段长度写入，实际触发field concatenation collision而红1；finally逐字恢复store.go（SHA256=263478076f3281fbb45066c4fe93a36614212a988fefa59d33e625cad8335995）后3组绿0。[受控红](chen-document-queue-review/digest-red.txt) / [恢复绿](chen-document-queue-review/digest-restored-green.txt) / [摘要](chen-document-queue-review/digest-red-green.json)。原实现已经正确区分长度，不误记为修复既有缺陷。

本人独立下载来件[完整CI43595089231](https://github.com/zwh9616/campus-claw-plus/actions/runs/43595089231)，artifact14114280810原ZIP SHA256=1bc08d1cbf0330b228889f63b39d12d7e7f4e5bb203122d4816f9302ad4ee5bc与GitHub一致。合并预演4f08c3383047840251efb4ef0925d946801530e2和来件整树bcc1e292738baaeed01b15e82b2067df4459904c一致。逐字核对4个队列源文件及queue-test.log摘要，10组真实MySQL全执行无skip，涵盖20并发、三受理/四发布边界回滚、三次耗尽/ABA/错误owner与班级、进程Kill/持久接管、最后CAS时租约过期和80ms SQL取消。四段0、原18组05/五恢复/三HTTPS、原内核/私有HTTP、四生产镜像+候选零HIGH/CRITICAL、SQL16实际拒绝和所有清理核验通过；解包秘密扫描0。[原ZIP](chen-document-queue-review/incoming53-evidence.zip) / [队列逐项核验](chen-document-queue-review/incoming53-queue-verified.json)。这是下载原件核验，不是本人本机数据库复跑。

本机Node24.22.1/npm10.5.0/Go1.27.3：合入客户端前后各跑完整check，基础/40合同/类型/构建/全包Go/strict11/11通过，解析镜像步骤均因spawnSync docker ENOENT退出1；后置SQL与本机verify未运行。audit两树0，历史/暂存秘密扫描0，失败原件保留，不声称本机Docker或MySQL通过。[合并后原输出](chen-document-queue-review/merged-check.txt) / [退出码](chen-document-queue-review/merged-results.json)。文本日志按仓库LF保存，下载ZIP保持原字节，目录摘要按入库字节重算。

已实际收口#52组件：本人APPROVED5908725476绑定最终20191c4157be936c948fc0faa58aaa3ec853836d；[最终CI43668637038](https://github.com/zwh9616/campus-claw-plus/actions/runs/43668637038)成功，artifact14143274631原ZIP SHA256=ff83d6a82086bbed74049846e444a3864fe1fe60394446af0ed4859937bf2bf0，与GitHub一致，11实际输入/两轮Go→Python各40文件195位置/鉴权/真实服务消失及全部原运行/SQL/清理已核验。[最终原ZIP](chen-document-queue-review/final52-evidence.zip)。正常合入main2ece150b39d794acc849fe8812624bf5ed1ecb86，本提交时精确main CI43669560990仍运行，未预填通过。main显示protected=true；普通写账号读取保护管理端点HTTP404，未声称已枚举全部保护参数，合并通过正常API并绑定最终SHA，不使用管理员绕过。

本轮在原#53同步该已获审主干，逐列合并共享表/tasks，保留小张队列、客户端和小陈审查所有历史与原件；只新增reviewer_test.go及本轮证据，原4个队列实现/测试文件、SQL和脚本逐字保留。新增测试提交的最终CI、实际批准/正常合入与main复验仍待实际执行。当前仍22 current/Target5/13 active空键/四服务；#54迁移6、#55生产接线、授权Document HTTP/页面及A/B/C未获本次批准，不勾父项。
