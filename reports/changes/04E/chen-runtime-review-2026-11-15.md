# 小陈审查04E私有解析与worker接线，2026-11-15

实际审查：小陈 / Kevcco（AI辅助）。实施责任仍为小张 / zwh9616。原PR #55来源分支继续处理，不新增同一实施变更，不伪造小张或小葛批准。

本轮将已获审main ae18872c0f29b2c021b73ba6e4c3be548b161094合入原候选4a7fc662153fa6bf1ec5dbe350797a3904d0f6f1。README/共享表/tasks/SQL探针范围四处冲突逐块保留双方历史；保留#52六个null坐标、截断/读体超时回归以及#53上传边界/摘要歧义回归，保留队列原始queue-test.log生成与CI上传。原接线实现、配置/合同/Compose、资源/网络限制、worker联调程序与来件逐字保持；SQL探针说明采用main准确范围。最终合并提交自身完整CI与批准另核，不能使用来件绿灯代替。

审查DI-SC-01/03..07/10/14：默认server只装配runtimeapp，共享原TLS/DML池，固定私有origin/canonical32字节token；配置无缺省、无回显。worker取消先于关闭连接池，事务外RPC、单并发/数据库UTC租约/心跳和故障后监督重启；旧身份/TXT/05和公开每请求MySQL健康不依赖parser可用。两个既审键原子激活为15个空active样例；parser仅独占internal网络，无宿主端口/DB或模型秘密/外网，nonroot/read-only/512MiB无swap/1CPU/32PID/64MiB tmpfs/500ms认证环回探针。六个Document公开操作仍frozen，未称用户可上传PDF/Word。

审查DI-SC-15：承接准确Target6/原十SQL不可变/manifest/严格结构与dirty拒绝/显式前向repair/测试独占空库down；未增加自动DDL、生产force/down或假回退。#52本人APPROVED5908725476→main2ece150/精确CI43669560990；#53本人APPROVED5908798357→main2bdb907/精确CI43671233011；#54本人APPROVED5908869691绑定4ac423a后正常合入上述main，精确CI43673282647 success。主干原artifact14145621037的ZIP SHA256=14cc26a030075d141c000f86f5d95555226b86840e95ada8c905e2374a1a4bd0与GitHub一致，完整四段/真实迁移4组、五中断、13漂移子项及八阶段5备份→失败6保留→另库恢复5/旧会话与原pending/202、10组队列和原日志/两轮Go-Python40文件195位置/原回归/镜像/清理与全跟踪指纹独立核验通过，解包秘密扫描0。[实际主干收口](chen-runtime-review/main54-acceptance.json)。这是原件独立核验，不是本人Docker复跑。

本人本地：npm ci/setup均0；完整check退出1，在parser镜像构建缺少docker，此前基础/合同/前端类型构建/所有Go包/strict11/11通过；audit两树与secret-scan为0。尝试原生Python运行test_health.py退出1，因为该套件明确要求容器/app路径，导入即失败，未执行探针测试；不能写成探针通过。真实MySQL与npm run verify本地未运行。[命令原件](chen-runtime-review/local-results.json)及原失败输出均保留，无删除断言或放宽门禁。

来件精确head完整CI43611861247 success，下载artifact14120669029原18文件ZIP SHA256=129fe4e7f2dd0eecac656bca2d5077e670756c9e813b8cc6ad2d6bcf0e6bcdbd，与GitHub一致。1886个跟踪Git blob字节指纹重算匹配预演树；五生产制品零HIGH/CRITICAL且运行ID与扫描一致；check/scan两轮40文件、195出处、25负例、7组真实私有HTTP、40文件Go互通/真实消失、3组认证probe/坏启动/总超时和全部隔离/清理已核验。迁移四组/五中断/八恢复、原18组05/五恢复/三原HTTPS全部保留；parser实际停机时额外第四轮原HTTPS/TXT/pending通过。[独立运行核对](chen-runtime-review/incoming55-runtime-verified.json)。

九个真实worker阶段按顺序执行且无skip：40份语料经默认Go→Python→SQL持久195位置/196片段，gold正文/页块/Unicode quote/原字节SHA/原202一致；实际parser停止后失败worker_interrupted、显式retry第2次成功；停worker后入队、重建后第1次成功；实际SIGKILL处理中worker后保留租约，用隔离库显式推进到期，再以epoch2/第2次成功；真实DB停启后监督worker处理新文件成功。强制测试租约到期不冒充等满生产60秒，内部夹具入队不冒充HTTP授权。

来件ZIP未包含queue-test.log，虽有10组摘要及log SHA，不能仅凭摘要称已逐字核验该原日志；本轮保留获审main生成/上传原日志的代码，最终CI必须实际提供并验证，且必须执行本轮保留的全部回归。来件解包秘密扫描0。最终head完整CI/实际批准/正常合入与精确main复验待收口；持有人库尚未升级，按离线迁移交接，不推定生产部署。

本次只审私有parser/worker接线。授权六HTTP、PDF/Word真实页面、用户样板方向、处理中SIGTERM/OOM等剩余A验收、OCR/语义和A/B/C整体完成均另验，不勾父项或整体归档。原始ZIP保留字节，其他原件SHA见evidence-manifest.json。

本轮暂存扫描首次退出1：摘要清单以日志文件名为JSON键，两个SHA256被generic-api-key规则误判；已逐项核实为文件摘要，改为path/sha256记录数组，不加豁免或放宽规则，重新执行原扫描。首次失败原件保留manifest-scan-first.txt，最终结果绑定提交前扫描。
