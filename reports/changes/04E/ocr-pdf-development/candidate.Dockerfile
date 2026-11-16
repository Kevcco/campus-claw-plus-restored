FROM campus-ocr-dev-20261116:research
USER 0
COPY pdfium.whl /tmp/pdfium.whl
RUN echo 'ef452a7178744196d4ff97029f46b56a13817346fc7ee0473c75643512276b3a  /tmp/pdfium.whl' | sha256sum -c - && python -m zipfile -e /tmp/pdfium.whl /opt/pdfium && rm /tmp/pdfium.whl
ENV PYTHONPATH=/opt/pdfium
USER 65532:65532
ENTRYPOINT ["python"]
