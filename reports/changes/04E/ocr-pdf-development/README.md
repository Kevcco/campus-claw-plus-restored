# 扫描PDF真实开发链路

2026-11-16，小张。DI-SC-16/17/19：固定pypdfium2 5.15.2 musllinux x86_64轮子SHA，实际PDFium版本159.0.8149.0。以已有Tesseract不可变候选为基，研究镜像ID见manifest；未加入生产Compose或激活接口。直接来源、许可声明与SHA见dependency.json；轮子中PDFium及第三方许可完整闭包还须核对。

两份仅图像开发PDF共14页，容器只挂固定input.pdf和执行器，gold与blind.pdf均未挂入。每页200DPI、无表单初始化/不绘制注释、不添加旋转；栅格输出原始RGB PPM交Tesseract chi_sim+eng。主进程先核对10MiB/100页/20MP，再为每页启动子进程；15秒包含渲染、OCR及序列化，180秒整任务，2MiB文字及8MiB累计输出上限，失败不返回半成品成功。每页及时关闭页面/位图并清理临时目录。

| 最终实测 | 结果 |
| --- | --- |
| 4页开发正例 | 35/1618码点错误，micro CER 2.163% |
| 10页低质开发负例 | 暂定诊断全部触发，未自动ready |
| 最长单页（含渲染） | 3090ms |
| 最长整份开发PDF | 19944ms（10页） |
| 容器累计峰值 | 206745600字节，约197MiB |
| 清理 | 成功 |

1CPU/512MiB无swap、32进程、64MiB tmpfs、非root、只读、无网络。测的是开发集，不计50盲测/100金框定位或固定4CPU宿主性能验收。CER对引擎原txt评分；TSV重建正文另逐字校验quote，不挪用txt的分数。该组件尚不实施文字层优先混合页、持久队列/教师校对或数据库版本迁移。

正常/无效尺寸/精确20MP边界两个单测加入默认check；关闭像素保护后红1，恢复绿0。实际PDFium原生创建101页/超像素PDF，断言在启动OCR前拒绝；空/伪签名/损坏/10MiB超限也拒绝，原生反例两组通过。首次错误使用页对象上下文管理器已修为显式finally关闭，失败原件保留。新的完整check结果见后续同提交记录。

精确镜像40个OS包Trivy HIGH/CRITICAL为0，原始报告保留；**不能覆盖源码编译Tesseract或轮子内PDFium的全部原生漏洞**。本批不将此扫描记成完整安全闭包。固定轮子避免运行时下载，最终生产选型仍需原生依赖/许可/漏洞审计、恶意PDF和OOM/kill恢复实测。

复现：先按上一批候选记录构建并核对Tesseract基镜像，再从dependency.json下载并核对轮子，将其命名pdfium.whl和candidate.Dockerfile放入研究上下文构建；运行`node scripts/benchmark-ocr-pdf.mjs sha256:<实际镜像ID>`。候选Dockerfile的本机base标签必须先核对manifest.base_image_id，不能把该标签当远端不可变供应链锁。默认生产依赖和Target6保持。

API依据：[官方渲染参数与位图接口](https://pypdfium2.readthedocs.io/en/stable/python_api.html)、[官方包及PDFium许可入口](https://pypi.org/project/pypdfium2/)。

本批完整npm run check退出0（新增2个栅格边界单测与全部旧检查），audit两树0，原生反例最终精确错误类别断言2组0；本机完整verify未重跑，最终head CI/main复验待。#61前序137d933完整CI43835404493已成功，本次新增PDF代码不能沿用旧head检查。
