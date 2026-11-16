FROM python@sha256:5862d971d1f0ae26b258cefa4e893556b1d3aba81bd2ff30888704ad378bab67 AS build-dependencies
RUN apk add --no-cache build-base cmake leptonica-dev
FROM build-dependencies AS build
WORKDIR /build
COPY tesseract-5.5.4.tar.gz .
RUN echo '445603ab1d0a63df552e193a600d9d5c4021aff36e144f3091af149e5efb381a  tesseract-5.5.4.tar.gz' | sha256sum -c - \
    && mkdir source && tar -xzf tesseract-5.5.4.tar.gz --strip-components=1 -C source \
    && cmake -S source -B obj -DCMAKE_BUILD_TYPE=Release -DBUILD_TRAINING_TOOLS=OFF -DBUILD_TESTS=OFF -DOPENMP_BUILD=OFF -DDISABLE_ARCHIVE=ON -DDISABLE_CURL=ON \
    && cmake --build obj --parallel 2 && cmake --install obj
FROM python@sha256:5862d971d1f0ae26b258cefa4e893556b1d3aba81bd2ff30888704ad378bab67
RUN apk add --no-cache leptonica=1.87.0-r0 libstdc++=15.2.0-r5 \
    && apk --no-network info -v > /opt/apk-packages.txt \
    && rm -rf /usr/local/lib/python3.14/ensurepip /usr/local/lib/python3.14/site-packages/pip /usr/local/lib/python3.14/site-packages/pip-*.dist-info /usr/local/bin/pip*
COPY --from=build /usr/local/bin/tesseract /usr/local/bin/tesseract
COPY --from=build /usr/local/lib/libtesseract* /usr/local/lib/
COPY --from=build /usr/local/share/tessdata/configs /opt/tessdata/configs
COPY eng.traineddata chi_sim.traineddata osd.traineddata /opt/tessdata/
RUN echo '7d4322bd2a7749724879683fc3912cb542f19906c83bcc1a52132556427170b2  /opt/tessdata/eng.traineddata' | sha256sum -c - \
    && echo 'a5fcb6f0db1e1d6d8522f39db4e848f05984669172e584e8d76b6b3141e1f730  /opt/tessdata/chi_sim.traineddata' | sha256sum -c - \
    && echo '9cf5d576fcc47564f11265841e5ca839001e7e6f38ff7f7aacf46d15a96b00ff  /opt/tessdata/osd.traineddata' | sha256sum -c -
ENV TESSDATA_PREFIX=/opt/tessdata OMP_THREAD_LIMIT=1 PYTHONDONTWRITEBYTECODE=1
USER 65532:65532
ENTRYPOINT ["tesseract"]
