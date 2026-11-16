# OCR 公共版本组件 v1

小张于2026-11-16按本会话最新授权冻结本七操作组件的准确合同。入口为 openapi.json、OCRPublic.schema.json、profile.json；contract-lock.json绑定唯一共享Schema及本地白名单路径，不从远端取类型。

已实现显式OCRHandler和真实身份/HTTPS/MySQL测试；默认server尚未安装，仍28 current、Target6、15空active。不可把“组件冻结”当成生产公开可调用，也不表示B1.2全部完成。原件v1、原202和旧Document接口不被该合同重写。

状态与修订、教师原识别源/校对、开始/重试、2000码点检索的具体权限、错误、配额、预算和回执见[执行依据](../../../openspec/changes/2026-11-13-materials-document-intelligence/ocr-public-contract.md)。原识别词框只属于原稿，校对修订使用页/码点。学生只读已发布资料及历史review修订；每次重新验证会话/当前班级，不能借原回执跨班。

待完成：图片原件公开上传/下载与统一列表兼容、生产迁移/配置消费者/服务及worker接线、校对页面、混合页、native安全闭包与正式质量。完整实测与限制见[本轮证据](../../../reports/changes/04E/ocr-public/README.md)。
