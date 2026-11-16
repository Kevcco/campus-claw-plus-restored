import binascii
from pathlib import Path
import struct
import unittest
from types import SimpleNamespace
from unittest.mock import patch

from ocr_image import admitted_image, recognize_image, PNG


def chunk(kind, data):
    return struct.pack('>I', len(data)) + kind + data + struct.pack('>I', binascii.crc32(kind + data))


def png(width=100, height=50, extra=b''):
    return PNG + chunk(b'IHDR', struct.pack('>IIBBBBB', width, height, 8, 2, 0, 0, 0)) + extra + chunk(b'IDAT', b'header-only-test') + chunk(b'IEND', b'')


def jpeg(width=100, height=50, extra=b''):
    return b'\xff\xd8' + extra + b'\xff\xc0\x00\x0b\x08' + struct.pack('>HH', height, width) + b'\x01\x01\x11\x00' + b'\xff\xda\x00\x08\x01\x01\x00\x00\x3f\x00\x01\xff\x00\x01\xff\xd0\x02\xff\xd9'


class ImageAdmission(unittest.TestCase):
    def test_di_sc_16_real_development_headers_and_metadata_not_pixels(self):
        root = Path(__file__).parent / 'testdata/ocr-v1/development-positive'
        for path in root.iterdir():
            pixels, (w, h) = admitted_image(path.read_bytes())
            self.assertEqual((w, h), (2339, 1654) if path.suffix == '.jpg' else (1654, 2339))
            self.assertTrue(pixels)
        self.assertEqual(admitted_image(png(extra=chunk(b'eXIf', b'orientation is not geometry')))[0], png())
        self.assertEqual(admitted_image(jpeg(extra=b'\xff\xe1\x00\x06Exif'))[0], jpeg())

    def test_di_sc_19_pixel_limit_before_any_native_decoder(self):
        for factory in (png, jpeg):
            self.assertEqual(admitted_image(factory(4000, 5000))[1], (4000, 5000))
            for w, h in ((4001, 5000), (0, 50), (1, 0)):
                with self.subTest(format=factory.__name__, w=w), self.assertRaises(ValueError):
                    admitted_image(factory(w, h))

    def test_di_sc_19_closed_signature_structure_crc_and_single_frame(self):
        for data in (b'', b'GIF89a', png()[:-1], png() + b'extra', jpeg()[:-1], jpeg() + jpeg(),
                     png(extra=chunk(b'acTL', b'animation')), png(extra=chunk(b'ABCD', b'unknown critical')),
                     png()[:30] + b'X' + png()[31:], jpeg()[:6] + b'\x10' + jpeg()[7:]):
            with self.subTest(size=len(data)), self.assertRaises(ValueError):
                admitted_image(data)

    def test_di_sc_19_native_failures_keep_timeout_resource_and_internal_distinct(self):
        for code, stdout, reason in [(-9, b'', 'resource_limit'), (1, b'ocr_timeout\n', 'ocr_timeout'), (1, b'internal_failure\n', 'internal_failure'), (1, b'untrusted stack trace', 'internal_failure'), (1, b'image_corrupt\n', 'image_corrupt')]:
            with patch('ocr_image.subprocess.run', return_value=SimpleNamespace(returncode=code, stdout=stdout)):
                with self.assertRaisesRegex(ValueError, '^' + reason + '$'):
                    recognize_image('opaque-test-path')


if __name__ == '__main__':
    unittest.main()
