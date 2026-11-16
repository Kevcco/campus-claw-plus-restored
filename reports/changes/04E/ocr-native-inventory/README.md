# 原生依赖清单核对（未收口）

小张，2026-11-16，DI-SC-14/16/25。对本机研究镜像sha256:543c3ae8067ff6d1f3ba32a80a502dc1b2ca13b8a8c6656b1dc4605bf5b8f381使用--pull never、network none、read-only、非特权/128MiB/0.25CPU运行只读清单。实际Tesseract5.5.4、Leptonica1.87.0及链接库版本见runtime.json；40项为镜像内构建时APK清单，不包括源码编译Tesseract或静态PDFium完整组件。

PDFium轮子内实际有37个许可/notice路径，逐文件字节数和SHA已记录，wheel-files.json保留整个轮子文件名。许可包含多平台条目，因此不能把文件名直接认定为该ELF的实际依赖SBOM。初次按大写文件名匹配遗漏了licenses目录内普通文件，已改为对完整路径不区分大小写匹配后重跑；没有把初次0文件当镜像没有许可。

在本轮明确枚举的/opt/pdfium、/usr/local/share、/opt/tessdata、/usr/share/licenses四目录中，37项均来自PDFium轮子，未发现Tesseract/模型独立notice；不能据此断言整镜像任意路径不存在文件。最终生产Dockerfile应从固定源归档/模型源明确保留对应LICENSE/NOTICE与版本来源，并验证安装路径。现研究构建仍缺全编译依赖锁与实际原生SBOM/CVE闭包，不计安全或许可最终通过，不启用生产。

首次从手工转写的错误镜像digest启动退出125，没有执行清单；随后先inspect本机已存在研究tag获取准确不可变ID再--pull never运行成功。两个实际只读探针容器--rm后均退出0，不创建网络/凭据/持久卷，不改正在验收的候选源码。
