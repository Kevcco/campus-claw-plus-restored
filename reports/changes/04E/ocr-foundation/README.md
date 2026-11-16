# OCR B1.1实际产物与B1.2选型准备

执行人：小张。2026-11-16。本轮在#58精确main验收运行期间准备语料与候选依赖；不修改验收中的跟踪源码。主干本机验收最终超时124，按失败记录；本轮仅准备B输入/评分，不启动B运行apply。

## 已完成的输入准备

- 50页盲测=20中文+20英文+10旋转，另10低质开发负例与4独立开发正例，三份扫描PDF，共64页。
- 100个词段bbox、源文字/码点区间、方向/尺寸、分集/来源/许可、原件SHA和冻结生成器齐全。全部64图像解码、100框内有实际墨迹；PDF读取确认64页均无文字层。
- Poppler逐页实际渲染64页并与对应输入比较，灰度绝对差均值最大值见pdf-render-qa.json，阈值8/255。输入8张联系页已逐张查看；重新渲染的PDF联系页1/6/8已实看，覆盖中文、旋转与低质/开发样本，没有裁切、错向或空白替代。其余页有逐页渲染和图像差值记录，不冒称另一个审查者逐页阅读。
- 5组输入/评分检查覆盖样本缺失、跨分集重复、字节漂移、阈值放宽、错误坐标、错误正文区间、PDF页绑定、CER分母与Unicode、IoU边界；真实受控破坏/恢复记录另附。没有把评分工具单测称为OCR质量测试。

## B1.2选型候选与准确边界

选型优先评估 **Tesseract5.5.4 + tessdata_fast eng/chi_sim/osd**。依据是本地CPU方案、可固定中英模型与TSV位置输出；这属于工程候选，**尚未以本项目开发语料证明质量/时延**。5.5.4官方发行包含安全修复，已核对发布tag映射的精确提交与GitHub签名状态；源码归档、三个语言文件和许可证均已实际下载并逐件SHA，见engine-candidate.json。源码和模型均Apache-2.0，模型二进制未放进生产镜像。

官方依据：

- [Tesseract5.5.4发行说明](https://github.com/tesseract-ocr/tesseract/releases/tag/5.5.4)：选择具体修复版，不能用未扫描的旧发行替代。
- [官方语言数据说明](https://tesseract-ocr.github.io/tessdoc/Data-Files.html)：fast为整数LSTM，要求OEM1；速度/精度取舍须在独立开发集测量。
- [CLI及TSV输出](https://tesseract-ocr.github.io/tessdoc/Command-Line-Usage.html)：多语言顺序会影响结果，必须连同参数一起冻结，TSV提供字框和置信值。
- [输入格式限制](https://github.com/tesseract-ocr/tessdoc/blob/main/InputFormats.md)：Tesseract不能直接读PDF；PDF栅格器的精确版本、许可、无脚本/外链、资源预算与漏洞扫描必须另行冻结。
- [PaddleOCR官方CPU结果](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-OCRv5/PP-OCRv5.en.md)：公开测试环境和资源开销不能直接当本项目512MiB/1CPU通过证据；作为备选，不提前引入模型与依赖。

以上来源支持能力与选型约束，不支持本项目OCR已达到任何质量阈值。**未做**：生产候选镜像构建/扫描、开发集真实识别与时延、最终栅格器及依赖闭包、公共/私有机器Schema和准确追加迁移。B1.2保持未完成。

## 下一批可直接执行的工作

1. 只挂载4页开发正例与10页低质负例，测Tesseract5.5.4精确候选的中英/旋转、TSV位置、单页时间和内存；识别容器不接触blind/gold目录，记录原始输出。失败保留，不改50页盲测。
2. 固定栅格器和依赖/语言包SHA，制作无外网/只读/nonroot隔离候选，先完成漏洞扫描与真实预算测试。
3. 按本change的B1.2冻结新版本正文/页/词框、needs_review和教师核对写入、旧出处版本语义、资源预算、失败/中断/租约、配置与追加迁移；补单源双路径正反例，获真实批准合入后才接线。
4. 把OCR内核、持久化、教师核对页面按依赖合成可验收批次；仅有合同/语料时不显示可用OCR入口。B完成之后才进入C语义检索。

## 本提交验证

完整npm run check退出0（含Go、真实私有解析40文档/7组HTTP、真实MySQL候选约束与strict11），audit两树零漏洞，历史/暂存秘密扫描0。最终生成器显式LF，并以图像内容生成PDF对象标识，两个不同目录重建的全部输入/PDF/锁一致；64页最终PDF重新渲染，与此前已实看的8张联系页逐字节一致。最终check:ocr-corpus再次0，见validation.json及各日志。主干本机完整verify超时124仍保留，不借本批检查关闭A4.3；PR CI与实际批准另跟进。
