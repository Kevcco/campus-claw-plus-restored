# 图片原件候选8约束实测

小张，2026-11-16，DI-SC-02/07/09/16..19/25。准确合同与SQL见docs/contracts/ocr-image-originals；这不是已嵌入的迁移8，不改默认Target6或prepared7。候选只扩格式和原状态，图片OCR仍用独立作业/修订，原v1不写识别正文。

真实MySQL8.4.13在独占1CPU/1GiB、network=none、tmpfs库执行已发布1..7建表SQL，再执行准确单ALTER。三份合成旧原件含NUL/非UTF8字节和SHA、旧正文/状态/页数逐字保持；两种图片needs_ocr/attempts0/一页成功，13个非法状态/NULL/原件长度/原版本/旧PDF和Word规则/跨班OCR复合FK按准确错误码拒绝。本班图片可复用原OCR表，四个原CHECK均保留，无解析作业。

先绿0，在发往隔离库的内存SQL把图片attempts=0改为1，真实合法准入被拒使脚本退出1；活动SQL文件未改。恢复后再次绿0，最终加入冻结SHA预检后再绿0。red绑定早于预检的脚本SHA，green/results绑定最终脚本；SQL字节相同。没有将任何部分当真实OCR引擎或HTTP上传验收。两轮独占资源和临时凭据清理全部成功。foundation、46合同、strict检查通过；默认check保留旧项并追加本探针，新增完整统一gate仍须后续执行。

公开HTTP/默认body修复已在1d68d73提交；该准确提交独立干净工作区的完整gate正在执行，当前check/audit/secret为0，verify尚未终态。不与本候选SQL混称最终全量验收。GitHub账号403/公开404使后续未推送，远端#62和main push CI待核对。
