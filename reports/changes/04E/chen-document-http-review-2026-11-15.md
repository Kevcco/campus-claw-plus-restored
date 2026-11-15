# 小陈对 #57 公开Document接线的审查，2026-11-15

审查账号小陈 / Kevcco（AI辅助）。04E实施、修正、浏览器验证和收口仍由小张 / zwh9616负责，本记录不转派实施责任，不记实际批准或合并。

核对代码5cac6dbd314ac0c253d7bbe9de02f6b590f3e4b5及随后仅补证的74ecfe3bdef74ca4fecf731db4dffb5f9abaee25；最新main仍为已验收d874b663991730474e2360944ba1587a1e7c1d65。代码CI43693601789成功，artifact14153148265原ZIP SHA256=8693942463a1aa4c4051291b5cd95cbaca59ac3a0b5e62e416251d3f8f0a45bf，与GitHub及作者已提交的[原件ZIP](document-http/final-gates/code-ci-originals.zip)逐字一致。2154个跟踪Git blob指纹与CI预演来源一致；不能据此声称74ecfe3或本审查提交自身CI已完成。[独立下载核验](chen-document-http-review/ci-verified.json)。

## 必须处理的三项

1. **DI-SC-07/10，202响应与已公布头部不一致。** OpenAPI中POST /api/v1/documents及POST /api/v1/documents/{id}/retry的202均声明Location为/api/v1/documents/{id}，但httpserver/documents.go的235及259行直接写202，没有设置该头。本人临时以真实DocumentsHandler和内存身份/仓储替身复现，两条路径均202但Location为空，探针退出1；[探针源码](chen-document-http-review/location-probe.go.txt)与[实际结果](chen-document-http-review/location-probe.txt)保留，临时测试已删除，原包测试随后通过。请小张使首次受理、原202重放及显式重试均返回相同受保护详情路径，补真实HTTP/HTTPS响应头断言；不能只修文档样例或只测JSON。若要改已公布合同，先按公共合同修订流程明确。
2. **DI-SC-13，原生200%仍未验。** 最终浏览器manifest明确NOT VERIFIED，不能拿360px、CSS缩放或旧样板200%代替当前真实页面。补同版本真实浏览器缩放/键盘/长内容可达证据及实际截图，再更新任务。
3. **DI-SC-10/13，真实页面显式重试仍未验。** 五组SQL和九worker阶段证明后台边界，不能证明页面按钮真实请求、受理提示/轮询和重试上限隐藏等交互。按原规约在实际可重试失败资料上执行并留请求/状态/截图；不可重试、needs_ocr、耗尽情况同时核对。

以上处理前保持draft，不提交APPROVE、不合并，不把A3或整个A标完成。下一步由小张在原#57修正和补证，交当前最终head复核；完整CI及main复验分别记录。

## 已核验的实际结果与限制

下载的CI四段check/audit/secret-scan/verify均0，verify508258ms；五个生产镜像零HIGH/CRITICAL并与实际运行ID一致、批次五项清理通过。五组真实TLS/MySQL公开HTTP、main/outage/persist三阶段默认HTTPS、两格式/双班五人/逐字原件和持久码点出处、真实parser停止及容器重建保留；九worker阶段40文件/195位置/196片段、实际SIGKILL与数据库恢复，真实迁移6/五DDL中断/恢复5、原01/04/05回归均核验。16SQL拒绝、10组队列原queue-test.log及五源码摘要、两轮40语料/25负例/7组私有HTTP/Go客户端真实失联/3组健康和清理仍通过。解包原件gitleaks0。这些为下载CI原件独立核验，未冒称本人本机数据库/容器复跑。

浏览器manifest的367源码摘要与获审候选逐一一致、42原件摘要通过；本人实看1440/360列表、360上传及长出处修复四张已有截图。它们不替代上述两项缺失浏览器操作，也不声称新会话。

本人使用Node24.22.1/npm10.5.0/Go1.27.3；npm ci/setup0，33组前端与19组合同比对/证据守卫0；完整check在check:document-parser因缺少Docker命令退出1，此前基础/合同/批次守卫/身份接线/前端类型与构建/全部Go包/strict11/11通过；后置真实SQL与本机npm run verify未运行。[本机原始结果](chen-document-http-review/local-results.json)与[完整check](chen-document-http-review/check.txt)。本机audit两树0、审查文档基础/strict11/11均0。秘密扫描首轮因Windows PowerShell归档模块不可用退出1；核对官方工具包SHA后用可用PowerShell解包，同一脚本重跑全历史237提交及暂存内容均0，未改扫描器或规则。失败和恢复原件均保留。作者本机verify124超时及清理照原证据保留，不修改阈值，不以旧main或本轮CI冒称其本机verify已通过。

入库文本日志仅规范LF和行尾空白，结果与退出码不变；原ZIP字节及GitHub摘要保持。

本次只新增真实审查记录和同步共享表/任务，不修改实现、配置、合同、旧断言或已验收历史。远端本次最终head检查与实际批准另核；持有人数据库未部署升级，OCR/语义、整个A/B/C和10-B全量/C仍未放行。
