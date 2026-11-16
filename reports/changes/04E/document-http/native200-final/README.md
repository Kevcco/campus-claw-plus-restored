# DI-SC-10/13：原生200%与键盘实际验收

小张执行，2026-11-15；同一真实默认HTTPS、Go、Python、MySQL及生产前端构建。源码与原件摘要、实际前端镜像、已加载JS摘要见manifest.json。所有账号和文件均为隔离测试夹具，未调用持有人环境。

## 原生缩放如何确认

使用Mozilla官方WebExtension `browser.tabs.setZoom(tab.id, 2)`，仅允许当前隔离campus.test站点；临时安装方式与Mozilla web-ext相同。只读Firefox站点数据库记录`browser.content.full-zoom=2`；原生窗口viewport=null，DPR由Windows150%对应的1.5变为3，CSS视口1281变641，CSS zoom与visualViewport.scale均为1。临时扩展和仅127.0.0.1调试器随后随浏览器重启移除，最终测试仍保持原生200%。没有CSS缩放、deviceScaleFactor或viewport模拟代替原生验收。

官方依据：[setZoom](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/tabs/setZoom)、[临时扩展](https://extensionworkshop.com/documentation/develop/temporary-installation-in-firefox/)、[Mozilla安装客户端](https://github.com/mozilla/web-ext/blob/master/src/firefox/remote.js)。截图是实际浏览器页面捕获，不冒称Windows浏览器外框截图或另一成员的新会话。

## 最终通过项

- 原生200%下14页面状态无横向溢出：列表、空结果、上传、校验错误、长Word、搜索、出处、PDF成功、扫描待OCR、加密失败、次数耗尽、网络错误/恢复和加载。请求延迟使用真实route.continue，故障使用显式route.abort，没有伪造业务成功响应。
- 14次连续Tab的焦点元素完整处于视口；200码点标题、5280码点Word、Control+End到正文最后字符。12次独立打开列表后键盘进入长出处，逐字匹配且高亮首行28.53px、焦点正确。
- 文档15真实SIGKILL解析器后失败1，恢复解析器，Enter只一次retry202且Location正确，真实GET轮询至ready2；文档16两次显式重试/实际轮询至失败3，按钮隐藏。没有SQL状态修改。最终前端源码运行时再次执行这些操作。
- 六份实际下载与输入逐字SHA一致。扫描与加密终态来自真实解析器；不可重试终态见native200截图和前轮真实终态记录。
- 单独恢复原生100%后，以1440和360两种CSS视口重跑最终出处修复，分别高亮首行28.87px、准确焦点且无横向溢出。这两项明确是响应式视口测试，不计为原生200%。
- 最终类型检查、33前端用例、npm两树audit通过；最新完整check/CI另跟进。真实浏览器服务、测试数据库/网络/私有凭据清理退出0，见runtime.json。历史完整verify超时记录仍保留。

## 实测修复与失败保留

原代码在请求完成后安排requestAnimationFrame，可能在正文节点提交前执行而无法定位；实际出现高亮首行1928px、焦点BODY。移除缩放扩展后仍复现。改为useLayoutEffect依赖busy/data/id/citation，在对应正文DOM提交后聚焦与滚动。受控恢复原已审代码再次红1，恢复修复绿0；native-controlled.json绑定的是此焦点修复的中间候选源码，之后列表焦点滚动修正的最终源码另由manifest及最终12次回归绑定。

200%下原生Tab对输入框边框或多行长标题仅保证部分可见；列表增加onFocusCapture的nearest滚动，最终14个停靠点保持原有完整可见断言通过。没有放宽断言。

工具限制与排查：原生缩放下Playwright鼠标坐标发生偏移，使用真实Tab/Enter/Control+End，不使用强制DOM点击。重建期间浏览器缓存曾继续加载旧JS，所以中间effect候选的失败不作为新版本结果；最终以真实route.continue关闭缓存，核对实际JS指纹后复验。早期Control+End探针读取发生在浏览器平滑滚动结束前，最终等实际滚动抵达末尾再断言。一次夹具并发导航/临时脚本文件冲突导致测试未到目标状态；已改串行和独立临时文件，未计通过。最终成功/耗尽用15/16独立文档重做，原失败与探针文件保留。

04E仍全部小张负责。此目录说明作者的真实执行，不代替Kevcco最终批准、远端门禁、正常合并或精确main复验。

本机附加清理限制：浏览器已关闭，临时扩展/调试器退出；测试数据库及服务销毁后这些会话已不可使用。自动审批拒绝删除忽略目录中的测试浏览器profile，进一步仅删除三个Cookie数据库文件的较小范围操作也返回“blocked by policy”，未提供更具体原因。停止这些删除操作，不换工具绕过；残留浏览器缓存和失效会话文件仅在output/material-real/native-api-55f3b56d01aa，不入Git。runtime.json的清理0指Compose资源及运行夹具，不包含此附加profile删除。
