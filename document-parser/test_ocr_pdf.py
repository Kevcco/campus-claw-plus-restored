import math
from pathlib import Path
import unittest
from types import SimpleNamespace
from unittest.mock import patch
from ocr_pdf import one_page, raster_size


class RasterBounds(unittest.TestCase):
    def test_di_sc_19_regular_and_exact_pixel_limit(self):
        self.assertEqual(raster_size(720, 360), (2000, 1000))
        self.assertEqual(raster_size(4000, 5000, 1), (4000, 5000))

    def test_di_sc_19_refuses_bad_size_before_native_allocation(self):
        for values in [(4001, 5000, 1), (0, 1, 1), (-1, 1, 1),
                       (math.inf, 1, 1), (math.nan, 1, 1), (1, 1, 0)]:
            with self.subTest(values=values), self.assertRaises(ValueError):
                raster_size(*values)

    def test_di_sc_19_page_elapsed_includes_raster_before_ocr(self):
        clock = [100.0]
        original_read = Path.read_text

        def render(_path, _index, destination):
            clock[0] += 7
            destination.write_bytes(b'controlled raster')
            return 100, 50

        def engine(args, **_kwargs):
            clock[0] += 2
            prefix = Path(args[2])
            prefix.with_suffix('.txt').write_text('controlled text', encoding='utf8')
            prefix.with_suffix('.tsv').write_text('controlled tsv', encoding='utf8')
            return SimpleNamespace(returncode=0)

        def read(path, *args, **kwargs):
            if str(path).replace('\\', '/') == '/sys/fs/cgroup/memory.peak':
                return '64'
            return original_read(path, *args, **kwargs)

        with patch('ocr_pdf.time.monotonic', side_effect=lambda: clock[0]), \
                patch('ocr_pdf.render_page', side_effect=render), \
                patch('ocr_pdf.subprocess.run', side_effect=engine), \
                patch.object(Path, 'read_text', read):
            result = one_page('controlled.pdf', 0)
        self.assertEqual(result['elapsed_ms'], 9000)


if __name__ == '__main__':
    unittest.main()
