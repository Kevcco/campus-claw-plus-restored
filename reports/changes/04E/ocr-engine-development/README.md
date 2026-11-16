# OCR引擎开发集实测与接口草案

2026-11-16，小张。实际执行Tesseract 5.5.4固定fast中英/方向模型，只输入14张开发PNG/JPEG。识别容器无gold、分组名称或角度提示；50页盲测未识别，100框定位和扫描PDF栅格链路未验。此报告不代表B质量验收或生产接线。

## 实际结果

| 开发正例 | 码点错误/金标准长度 | CER | 单页耗时 |
| --- | --- | --- | --- |
| 中文清晰 | 10/201 | 4.98% | 1186ms |
| 中文90度 | 13/194 | 6.70% | 1147ms |
| 英文清晰 | 0/614 | 0% | 951ms |
| 英文180度 | 2/609 | 0.33% | 889ms |

合并25/1618，micro CER为1.545%。评分沿用#59的NFC/空白规整和Unicode码点编辑距离，不忽略大小写与标点，不拿图片数当分母。10页低质负例均触发暂定开发诊断，4正例未触发；诊断是空文本/失败、平均原始置信分数低于80或低于50的词比例超过20%，尚未校准或冻结。引擎对部分模糊页返回exit0但空文字，原样保留，不计成识别成功。CER大于100%也保留真实插入错误，不截断评分。

14页最长1523ms，容器累计cgroup memory.peak最大171937792字节（约164MiB），不是逐页独立峰值。每个容器1CPU/512MiB无swap、32进程、64MiB tmpfs、非root65532、只读、无外网，真实清理成功；宿主Docker实际20CPU/8182206464字节，不能冒称固定4CPU/8GiB性能验收。单页15秒壁钟预算，原始文本/TSV/时间/内存见engine-raw.json，评分见development-summary.json，14源图SHA对应input-mapping.json及#59语料。

## 引擎和安全范围

真实镜像ID为`sha256:c8588931337197969ade98805f5ec2a1e8d12b445a9dd6fa8670316866dd930c`，源码5.5.4提交`7e548e6eff172f38f97664afe7f7ce40db36c8d6`，模型提交`87416418657359cb625c412a48b6e1d6d41c29bd`；基镜像和三个模型SHA均在候选Dockerfile内校验。来源与许可沿用[已下载候选清单](../ocr-foundation/engine-candidate.json)。构建/运行包库存与实际版本原件已保留，首次Alpine TLS下载失败及重试成功也分别保留。

精确候选镜像导出后用仓库固定Trivy/漏洞库扫描40个OS包，零HIGH/CRITICAL、扫描及清理退出0。**该扫描不覆盖源码编译的Tesseract本体**；构建依赖尚未全部精确锁定。不能据此写完整依赖闭包安全通过或正式选型完成。下一步补编译依赖/SBOM与源码安全核对，并选择扫描PDF栅格器和冻结完整预算。candidate.Dockerfile只是研究复现原件，未加入Compose/生产扫描清单或默认构建。

复现需依照候选清单下载并核对源码和模型，放入独立构建上下文后以candidate.Dockerfile构建，取得实际不可变镜像ID，再运行`node scripts/benchmark-ocr-development.mjs sha256:<实际镜像ID>`。默认只选择development分集；不得把盲测输入用于阈值调整。按[官方构建说明](https://tesseract-ocr.github.io/tessdoc/Compiling.html)与[官方命令参数](https://tesseract-ocr.github.io/tessdoc/Command-Line-Usage.html)执行。

## 机器合同与验证

[独立草案](../../../../docs/contracts/ocr-draft/README.md)具体化OCRResult、教师校对和回执的2020-12 Schema/OpenAPI3.1双路径、码点quote/页序/像素乘积/正面积词框、可信教师/班级及CAS边界。4组检查全过；停用逐字quote保护时红1，逐字恢复绿0。新增代码的完整`npm run check`退出0，详见check.txt；本机完整运行门禁绑定的是先行优化提交fd67834，见[完整原件](../parser-check-concurrency/README.md)，不是本合同最终head的CI。

现有migration6固定v1与FK不能直接递增；草案提出追加不可变revision和发布指针，旧出处不漂移，教师改字不能复用错误词框。准确DDL/迁移恢复、列表上传/搜索/版本读取兼容、RPC/租约预算、质量阈值仍未冻结。草案不注册到current接口，不变更Target6、15个active键或生产服务；B1.2保持未完成。按草案建议全量人工核对也不替代独立低质识别指标。

原始文件及输入代码摘要见manifest.json；代码仅按仓库规则转LF时同时保存实际执行和仓库两种摘要。#59完整CI已经核验，Kevcco尚无实际审查，不合并或虚构批准。此批与#59分开提交依赖PR，后续按最终head检查和实际批准收口。

最终本机audit首轮因npm TLS断连退出1，原件保留；同命令重试两棵依赖树均零漏洞/退出0。全历史251提交、全部暂存及本轮解包证据秘密扫描均0；严格规约11/11、基础入口及转LF后4组合同行为复验0。最终head CI仍待。
