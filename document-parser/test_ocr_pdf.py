import math
import unittest
from ocr_pdf import raster_size


class RasterBounds(unittest.TestCase):
    def test_di_sc_19_regular_and_exact_pixel_limit(self):
        self.assertEqual(raster_size(720, 360), (2000, 1000))
        self.assertEqual(raster_size(4000, 5000, 1), (4000, 5000))

    def test_di_sc_19_refuses_bad_size_before_native_allocation(self):
        for values in [(4001, 5000, 1), (0, 1, 1), (-1, 1, 1),
                       (math.inf, 1, 1), (math.nan, 1, 1), (1, 1, 0)]:
            with self.subTest(values=values), self.assertRaises(ValueError):
                raster_size(*values)


if __name__ == '__main__':
    unittest.main()
