"""Bounded experimental PDF raster/OCR component. No HTTP or database wiring."""
import json
import math
from pathlib import Path
import subprocess
import sys
import tempfile
import time


def raster_size(width, height, scale=200 / 72):
    if not all(math.isfinite(v) and v > 0 for v in (width, height, scale)):
        raise ValueError('invalid_page_size')
    w, h = math.ceil(width * scale), math.ceil(height * scale)
    if w * h > 20_000_000:
        raise ValueError('page_pixel_limit')
    return w, h


def render_page(path, index, destination):
    import pypdfium2 as pdfium
    import pypdfium2.raw as raw
    with pdfium.PdfDocument(path) as document:
        page = document[index]
        try:
            w, h = raster_size(*page.get_size())
            # No form initialization, JavaScript engine or annotation rendering.
            bitmap = page.render(scale=200 / 72, may_draw_forms=False,
                                 force_bitmap_format=raw.FPDFBitmap_BGR,
                                 rev_byteorder=True, draw_annots=False)
            try:
                if (bitmap.width, bitmap.height) != (w, h):
                    raise ValueError('raster_geometry_mismatch')
                with open(destination, 'wb') as output:
                    output.write(f'P6\n{w} {h}\n255\n'.encode('ascii'))
                    for row in range(h):
                        output.write(bytes(bitmap.buffer[row * bitmap.stride:row * bitmap.stride + w * 3]))
            finally:
                bitmap.close()
        finally:
            page.close()
    return w, h


def one_page(path, index):
    with tempfile.TemporaryDirectory(prefix='ocr-page-') as temporary:
        image = Path(temporary) / 'page.ppm'
        width, height = render_page(path, index, image)
        return recognize_raster(image, width, height, index)


def recognize_raster(image, width, height, index):
    started = time.monotonic()
    with tempfile.TemporaryDirectory(prefix='ocr-text-') as temporary:
        prefix = Path(temporary) / 'result'
        child = subprocess.run(['tesseract', str(image), str(prefix), '--oem', '1',
                                '--psm', '1', '-l', 'chi_sim+eng', '--dpi', '200', 'txt', 'tsv'],
                               capture_output=True, timeout=15)
        if child.returncode == -9:
            raise ValueError('resource_limit')
        if child.returncode != 0:
            raise ValueError('ocr_engine_failure')
        if prefix.with_suffix('.txt').stat().st_size > 2_097_152 or prefix.with_suffix('.tsv').stat().st_size > 8_388_608:
            raise ValueError('output_limit')
        text = prefix.with_suffix('.txt').read_text(encoding='utf8')
        tsv = prefix.with_suffix('.tsv').read_text(encoding='utf8')
        return dict(page=index + 1, width=width, height=height, text=text, tsv=tsv,
                    elapsed_ms=round((time.monotonic() - started) * 1000),
                    peak_bytes=int(Path('/sys/fs/cgroup/memory.peak').read_text()))


def recognize_pdf(path):
    import pypdfium2 as pdfium
    started = time.monotonic()
    data = Path(path).read_bytes()
    if not 1 <= len(data) <= 10_485_760 or not data.startswith(b'%PDF-'):
        raise ValueError('input_limit_or_signature')
    with pdfium.PdfDocument(data) as document:
        pages = len(document)
        if not 1 <= pages <= 100:
            raise ValueError('page_count_limit')
        # Check every page before allocating a raster or starting OCR.
        for page in document:
            try:
                raster_size(*page.get_size())
            finally:
                page.close()
    results = []
    output_bytes = 0
    for index in range(pages):
        remaining = 180 - (time.monotonic() - started)
        if remaining <= 0:
            raise ValueError('task_timeout')
        # Includes native raster work, OCR and output serialization in page budget.
        child = subprocess.run([sys.executable, '-B', __file__, '--page', str(path), str(index)],
                               capture_output=True, timeout=min(15, remaining))
        if child.returncode != 0:
            raise ValueError('page_worker_failure')
        output_bytes += len(child.stdout)
        if output_bytes > 8_388_608:
            raise ValueError('response_limit')
        results.append(json.loads(child.stdout))
    return dict(pages=results, elapsed_ms=round((time.monotonic() - started) * 1000),
                pypdfium2=str(pdfium.PYPDFIUM_INFO), pdfium=str(pdfium.PDFIUM_INFO))


if __name__ == '__main__':
    if sys.argv[1] == '--page':
        print(json.dumps(one_page(sys.argv[2], int(sys.argv[3])), ensure_ascii=False))
    else:
        print(json.dumps(recognize_pdf(sys.argv[1]), ensure_ascii=False))
