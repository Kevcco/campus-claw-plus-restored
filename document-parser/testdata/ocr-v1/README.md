# OCR 输入与金标准 v1

对应04E B1.1、DI-SC-16/17。这里冻结的是输入、金标准与评分口径，**没有运行OCR，没有识别质量达标结论**。不进入解析服务镜像，不开放图片上传。

| 分集 | 页数 | 用途 |
| --- | ---: | --- |
| blind / clear_zh | 20 | 清晰中文，与英文合计40页计算micro-CER<=5% |
| blind / clear_en | 20 | 清晰英文；保留大小写、标点及难字 |
| blind / rotated | 10 | 中英各5页；90/180/270度各4/3/3页，micro-CER<=10% |
| development-negative | 10 | 模糊低对比度5页、遮挡5页；正式执行时至少9页needs_review/失败 |
| development-positive | 4 | 独立开发/选型正例，含方向变化，不计入50页或质量分母 |

每页有原始PNG/JPEG、源文本gold、文件SHA/尺寸/来源、PDF文件与真实页号。三份PDF只包含扫描图像，不含可直接提取的文字层。50页盲测各两个词段，共100个bbox；坐标为**OCR方向矫正前的原始输入图像**左上原点归一化xyxy，旋转样本已把标注映射到实际输入方向。它不是矫正后的画布坐标，也不是旋转样本生成前的正向画布坐标。词段以Unicode码点半开区间关联源正文。

正文由本项目自制，不含真实学生资料或第三方课本。字体为NotoSansSC，SIL Open Font License 1.1，许可原文见licenses；字体文件不重复入库，来源、精确版本与SHA见manifest.authoring。当前覆盖短篇印刷体、三种字重、中英段落、简单文本列与整页旋转；不据此宣称手写、公式结构化、复杂多栏或真实相机透视已验收。

## 冻结与评分纪律

- corpus-lock.json固定所有输入/PDF/manifest/许可字节；生成器SHA也在manifest中。运行前先执行`npm run check:ocr-corpus`。
- 识别程序只取得待识别字节，不得读取gold、角度标签、分集文件名或bbox。评测器在识别完成后独立读取金标准。
- B1.2选型与阈值只能使用development目录；50页盲测在语料版本获审及实现配置冻结后统一执行，不用它挑引擎、调参数、筛选失败页。
- 评分仅NFC+Unicode White_Space折叠+去首尾空白，保留大小写、标点与难字。CER按码点Levenshtein距离计算，分组使用总编辑数/总金标准码点数，同时报告逐页结果；不得拿页均值替代micro分母。
- bbox以IoU>=0.5计命中，100段中至少95段命中，页码全部正确。OCR产物不得自行提交“金标准ID”作为定位正确证据；匹配/字符对齐规则在B1.2独立评测器合同中冻结。
- 当前检查器验证输入与评分基元，不计算真实OCR通过率。后续新增或修订样本须新版本与原因记录，不覆盖v1失败原件。

## 可复现生成

作者环境：Pillow13.0.0、ReportLab4.4.9，精确字体SHA见manifest。生成器只用于准备输入，CI和生产不安装这两个作者依赖。

```text
python scripts/generate-ocr-fixtures.py --font <verified-NotoSansSC.ttf> --license <OFL.txt> --out <new-output-directory>
node scripts/check-ocr-corpus.mjs <new-output-directory>
```

所有64页已解码检查，PDF64页用独立Poppler渲染，与相应输入对照；检查与可视页见reports/changes/04E/ocr-foundation。生成器自带清晰源文字不是OCR输出，不能将gold回填到识别结果。
