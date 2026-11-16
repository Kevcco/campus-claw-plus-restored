# 04E 六公开接口与真实页面（小张，2026-11-15）

用户本轮明确要求推进 PDF/Word 真实上传、搜索、下载接口和页面。沿用 #49 已审样板方向，直接实施，无需重复等待方向确认；最终浏览器验收、远端门禁和主干复验仍分别记录。

## 已验收前置

#52 最终20191c4获Kevcco APPROVED5908725476，合入2ece150；#53 最终ce0fd30获APPROVED5908798357，合入2bdb907；#54 最终4ac423a获APPROVED5908869691，合入ae18872；#55 最终6ff53ae获APPROVED5908959859，合入d874b663991730474e2360944ba1587a1e7c1d65。无未解决审查线程，不重复合并或冒称本人批准。

精确main完整CI43675360843成功。artifact14145922538原18文件ZIP SHA256 `f0bd2ef24a5c74f33a0dba06249594aaa1bf68d0b96474c9c53f1a96f583068a` 与GitHub一致；独立下载核验四段0、五精确镜像0 HIGH/CRITICAL、九worker阶段、40文件/195位置/196片段、真实迁移恢复、原队列日志、旧HTTPS/SQL/清理及解包秘密扫描0。详见[主干核验](document-http/main55-proof.json)。这是下载证据核验，不冒称重跑主干或升级持有人库。

## 本批实现（DI-SC-02/07..15）

六个已批准操作原子转current，合计28；原22操作、planned能力、15空配置、精确Target6和五服务保持原义。独立documents领域/存储与HTTP层；复用同一身份服务、DML连接池与TXT/Document每用户每Unix分钟12次额度。整体5秒包含鉴权和慢速读体，新的私有预检500ms，异步解析留在独立worker。

原202先重新授权并查持久快照，新请求预检成功才事务保存原件/job/快照。公开响应校验逐字段拒绝未知/重复/非法UTF8，允许合同明确的两个nullable字段；时间戳固定三位毫秒。读取按当前班级与角色授权，学生只见ready；原件复算SHA并固定安全附件名/no-store/nosniff。关键词使用二进制字面匹配，同一RR快照回查持久正文、同版本页/逻辑块和Unicode码点quote；损坏503，不降级为空结果。

真实React页面提供列表、上传、状态轮询、正文/出处和下载，以及三类可重试失败的显式重试。文件读取只做5字节签名检查，不把PDF按文本解码。写请求/下载不自动重发；未确认受理保留原payload/key供人工确认，离页/切班/退出取消请求并清空文件与旧出处。Word显示逻辑块，扫描PDF如实提示待OCR；无生产samples入口、语义/OCR伪接线。

## 已实际执行

- Node/npm安装与前端setup退出0；初轮完整check退出0（原基础/类型/构建/Go/vet/严格11/11、私有组件/SQL队列门禁均保留）。补充公共激活反例后45合同0；前端33组（新增5）、业务3组0。
- 五组真实TLS/MySQL公开接口测试0：20并发仅一份原件/job/快照（12受理、8按合同429）、共享TXT额度、事务回滚、跨班/角色、RR并发一致性、损坏原件/quote拒绝、精确零毫秒、慢速体5秒504、显式重试三次上限和原202不变。此层解析结果/可用性受控，不冒称真实Python。
- 独立默认Compose实际Go→Python→MySQL公开HTTPS三阶段main/outage/persist通过：2份真实格式、双班五人、逐字持久出处、二进制原件SHA、登录/切班旧令牌失效、学生写403/跨班404、真实停解析器重放/新503无半行、容器重建持久化。原40语料/九worker阶段/旧01/04/05与停库回归全部保留并通过。
- 真实受控移除持久quote逐字保护：新增业务测试红1；逐字恢复源文件后绿0，见[记录](document-http/controlled.json)。未删断言或放宽限流/时间预算。
- Trivy数据库预下载独立完成0；本批干净完整gate/五镜像扫描和最终CI尚待，不用前置main结果代替。

## 保留失败与修正

首轮新的原202校验器直接复用了禁止null的身份strictjson解码器，误拒Document合法nullable字段。改为strictjson检查JSON协议/重复键，再按Document精确字段和nullable规则验证；共享身份解码器不放宽。首单测及首SQL失败原件保留。

首次默认回归因把新五人追加到旧五人seed导致原断言失败。拆成独立Document seed及初始化，原五人断言不改。第二次Docker自动池耗尽：只读清单选择显式空闲子网，不删除其他项目网络。第三次新旧客户端共用重用IP触发原10次登录限流：给新增实际客户端独立测试网络IP，保留真实可信代理和全部原限流，不伪造X-Forwarded-For。对应三份失败JSON保留；第四轮完整默认接线0。

## 尚待本批收口

真实浏览器四态/两档/键盘/原生200%、干净完整gate、五制品实扫、最终head CI、Kevcco实际审查和正常合入/main复验。当前未提交本批PR，未部署持有人长期环境。OCR和语义检索继续按B/C明确任务推进，A整体未提前验收或归档。

## 中断恢复与长出处修复

2026-11-15小张继续公开接线：b55265e已有真实浏览器两档/767与768/键盘/双班师生/真实上传及逐字下载SHA、扫描和加密终态、长200字符标题/5280码点Word通过；发现第二长片段点击后高亮首行仍在视口下3372px，正修为聚焦高亮并滚动到片段，待同制品复验。原完整gate在会话中断/引擎关闭后无最终结果，不能算通过，原日志与README未暂存事实保留；正在恢复隔离环境重跑。原生200%尚未完成：模拟按键未改变DPR，Windows Firefox电脑控制因URL策略不支持停止，不将CSS缩放当验收。远端PR/最终CI/实际批准待。


2026-11-15小张续验：长2000码点出处已修复，桌面1440/手机360实际点击首行均位于28.9px并聚焦；真实上传、搜索、下载SHA、加载/空/网络失败、扫描待OCR/加密失败、双班师生与离页清空复验通过。早期加载探针延迟DNS异常已废止，以最终浏览器记录为准；临时令牌错误输出已脱敏且真实接口确认401失效。新增五组真实SQL证据缺失/跳过/失败/重复拒绝守卫2组通过。隔离浏览器环境已清理；最终干净完整gate、PR CI和实际批准待。原生200%因Windows浏览器URL策略工具限制仍待验，显式重试按钮另待实浏览器复验；不得标整体A完成或正常合并。


2026-11-15公开实现已提交[PR #57](https://github.com/zwh9616/campus-claw-plus/pull/57)草稿（代码5cac6db）。完整CI43693601789通过；与head整树相同的预演3df6f43原artifact14153148265，ZIP SHA256=8693942463a1aa4c4051291b5cd95cbaca59ac3a0b5e62e416251d3f8f0a45bf，18原件/全部跟踪指纹独立核验，四段0、verify508258ms、五镜像零HIGH/CRITICAL、三公开HTTPS阶段、五真实SQL组、九worker/40语料/迁移恢复/原回归/全部清理及秘密扫描0。本机同代码check/audit/secret0，verify最后离线组件重复复验超600秒退出124，保留失败而不放宽时限；已清理本轮被中断的私有服务/网络/测试凭据和五审计标签。桌面/手机真实功能与长出处修复已复验；原生200%与显式重试按钮实浏览器检查、实际批准和合入仍待。保持实现与自测，不把CI等同完整页面验收。当前补证提交仅文档/证据/状态，最终head CI另核。


2026-11-15小张承接#57的Kevcco实际CHANGES_REQUESTED（a05d258）：已在原分支补上传/原202重放和retry的Location，先用原真实handler探针复现红1，修复后httpserver全包绿0；在真实TLS/MySQL和默认HTTPS调用器增加所有Document 202头断言。原生200%与显式重试浏览器实测正在同版本补齐；最终完整门禁/真实批准/main复验待，不先标完成。


2026-11-15小张续验261d94e：Location缺陷已修复并推送，真实handler红1→全包绿0；修复版默认可信HTTPS main（含实际上传/原202重放Location断言）通过，完整CI跟进中。隔离Firefox真实登录/资料页已打开；Computer Use再次明确返回“Computer Use has been stopped for this turn because browser URL policy enforcement is not yet supported for the current Windows browser”，已停止电脑输入。原生200%仍未验，显式重试按钮实测尚未执行；需要人工在该隔离浏览器调到200%后继续实际指标/截图/交互验收，不用CSS或设备模拟替代，不冒用旧CI或旧批准。维持#57草稿与CHANGES_REQUESTED，不请求最终复审/不合并。


2026-11-15晚小张在真实默认HTTPS环境补#57重试实测：上传202后真实SIGKILL私有解析器（不改SQL），失败1→键盘Enter只发一次retry202/正确Location→真实ready2及逐字原件SHA通过；另份资料两次显式retry202/实际GET轮询至失败3，按钮隐藏/耗尽提示通过。耗尽页矛盾文案已真实浏览器红→绿修复，并重建frontend sha256:40eae59a240f6f9b148a4e829641be6b0faa264b5d8a71dc03ecc24a9600bd19复跑成功2/耗尽3/扫描needs_ocr/加密失败，实际Enter只一次写、GET轮询、202 Location、原件SHA均通过；本机类型/33前端/构建/strict11退出0。证据见reports/changes/04E/document-http/retry-browser/manifest.json。原生200%新窗口仍实测DPR1.5/innerWidth1281（Windows150%，页面100%），viewport=null/CSSzoom1，不冒称200%通过。9a327ac完整CI43698110827已独立核验18原件/跟踪指纹/三HTTPS/五SQL/五镜像及清理0，verify402973ms；本提交最终CI/200%/实际复审/合并仍待。


2026-11-15晚补证：64ea99a代码最终完整CI43714084399成功，预演095d282与head整树一致；artifact14162150910原ZIP SHA256 fe3ea94890179a9c4d3d3576a12c1398a634883982d1feca17a0921ff91860de、18原件和全部跟踪指纹独立核验，四段0、verify508019ms、五镜像零HIGH/CRITICAL、三公开HTTPS/五真实SQL/九worker/迁移恢复/原回归及全部清理通过，解包秘密扫描0。本机64ea99a完整check/audit/secret-scan亦全部0；429源码/30浏览器原件与Git blob一致。Location行内线程已由小张凭实证解决，Kevcco仍CHANGES_REQUESTED、未获新批准。新原生Nightly窗口多次实测仍DPR1.5/innerWidth1281/viewport=null/CSSzoom1（页面100%），尚未收到新窗口调整完成回复，200%未验证。保持#57草稿，不请求最终复审或合并；本提交仅补证与进度，新的文档head CI另核。代码原件见document-http/retry-final-ci/及retry-final-local/，200%通过后再请求实际复审、满足门禁后正常合并/main复验。


2026-11-15晚#57原生200%已实际补齐：隔离Firefox经Mozilla tabs.setZoom设置原生2倍，只读站点zoom记录=2，DPR1.5→3、宽1281→641，viewport=null/CSSzoom1/visualViewport.scale1；随后重启移除临时扩展和调试器，最终真实页面继续保持200%。修复长出处在正文DOM提交前定位的时序竞态（useLayoutEffect按busy/data/citation提交后定位）及列表Tab焦点未完整入视口；受控恢复旧逻辑红1、恢复修复绿0。最终14页面态/14个完整可见Tab停靠点/200字符标题/5280码点正文末尾/12次长出处/6份下载SHA、原生200%真实Enter重试15→ready2与16→耗尽3/实际轮询全部通过，另100%下1440/360出处复验0；真实隔离资源清理0。最终类型/33前端/audit0，完整check与本提交最终CI/实际复审/正常合并及main复验待。本次证据见reports/changes/04E/document-http/native200-final/manifest.json，04E仍全部小张负责，不提前勾A父项/B/C或持有人部署。


2026-11-15晚#57交审补证：代码ee7e285原生200%/最终键盘及重试实测完成，已转ready并实际请求Kevcco复审。该代码完整CI43725610304成功；预演eec9a608与head整树一致，artifact14166756905原ZIP SHA256 c9670aa6311cf4d0b05fcf0b99f1f383f0fbce784e16687f44922c755dff38a7、18原件及全跟踪Git blob指纹已独立核验，四段0、verify518477ms、五镜像零HIGH/CRITICAL、三公开HTTPS/五真实SQL/九worker/迁移恢复/原回归与清理均通过，解包秘密扫描0。ee7e285本机完整check0、429源码/68浏览器原件均与Git匹配。最新仍无Kevcco实际APPROVED，故未合并；待真实批准并确认最终head门禁后正常合并/main复验。本提交只补证与进度，代码保持ee7e285逐字一致，其自身CI仍按正常门禁跟进；不提前勾A/B/C或持有人部署。证据见reports/changes/04E/document-http/native200-ci/。

2026-11-16小张处理#57最新复审：补767/768实测时实际发现768–1000px状态与文件大小重叠，已修正共享列表选择器排除document-status，真实重叠断言红1→绿0；360/767/768/1440四档52截图、48次Tab、长200码点标题/5280码点正文末尾/逐字出处通过，另1000/1001边界无重叠。修复构建重新完成原生200%（站点zoom2/DPR3/viewport=null，临时扩展移除后验收）、14页面态、14次Tab、12次长出处及真实文档7失败1→Enter一次202→ready2、文档8两次202→耗尽3、扫描/加密不重试，五份下载SHA一致。本机完整check/audit0，实际JS/CSS与同次dist逐字匹配，隔离运行清理0；两处证据末尾空行按意见清理并同步原清单摘要。A3.1b浏览器验收完成，最终head CI/实际批准/main复验仍由A4.3收口，不提前勾A父项或持有人部署。证据见reports/changes/04E/document-http/breakpoint-final/。
