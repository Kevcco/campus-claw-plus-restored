import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch
import pypdfium2 as pdfium
sys.path.insert(0, '/tests')
from ocr_pdf import recognize_pdf

class NativeFailures(unittest.TestCase):
    def test_corrupt_and_oversized_input(self):
        with tempfile.TemporaryDirectory() as d:
            p=Path(d)/'input.pdf'
            for data in (b'',b'not-pdf',b'%PDF-'+b'0'*10485760):
                p.write_bytes(data)
                with self.assertRaisesRegex(ValueError, 'input_limit_or_signature'): recognize_pdf(p)
            p.write_bytes(b'%PDF-corrupt')
            with self.assertRaises(pdfium.PdfiumError): recognize_pdf(p)

    def test_page_count_and_pixels_prevent_ocr(self):
        with tempfile.TemporaryDirectory() as d:
            for name,count,width,height in [('pages',101,20,20),('pixels',1,3000,3000)]:
                p=Path(d)/(name+'.pdf')
                doc=pdfium.PdfDocument.new()
                for _ in range(count):doc.new_page(width,height).close()
                doc.save(p);doc.close()
                with patch('ocr_pdf.subprocess.run') as child:
                    with self.assertRaisesRegex(ValueError, 'page_count_limit' if name == 'pages' else 'page_pixel_limit'):recognize_pdf(p)
                    child.assert_not_called()

if __name__=='__main__':unittest.main()
