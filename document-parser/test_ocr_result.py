import unittest
from ocr_result import HEADER, result_from_tsv

TSV = HEADER + '\n1\t1\t0\t0\t0\t0\t0\t0\t1000\t500\t-1\t\n5\t1\t1\t1\t1\t1\t100\t50\t100\t50\t95\t课程😀\n'


class OCRResult(unittest.TestCase):
    def test_di_sc_17_unicode_offsets_and_original_raster(self):
        result = result_from_tsv([TSV, TSV])
        self.assertEqual(result['body'], '课程😀\n\n课程😀')
        self.assertEqual(result['pages'][0]['words'][0]['bbox'], [.1, .1, .2, .2])
        self.assertEqual(result['pages'][1]['char_start'], 5)
        self.assertEqual(result['status'], 'needs_review')

    def test_di_sc_17_bad_boxes_order_and_nonfinite_are_rejected(self):
        for bad in (TSV.replace('100\t50\t100\t50', '999\t50\t100\t50'),
                    TSV.replace('95\t', 'NaN\t'), TSV.replace('95\t', '101\t'),
                    TSV + TSV.splitlines()[-1] + '\n', HEADER):
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                result_from_tsv([bad])

    def test_di_sc_17_anchor_to_engine_text_without_inventing_chinese_spaces(self):
        tsv = TSV.replace('课程😀', '课程') + '5\t1\t1\t1\t1\t2\t300\t50\t100\t50\t95\t材料\n'
        result = result_from_tsv([tsv], ['课程材料\n'])
        self.assertEqual(result['body'], '课程材料')
        self.assertEqual(result['pages'][0]['words'][1]['char_start'], 2)
        with self.assertRaises(ValueError):
            result_from_tsv([tsv], ['不匹配的正文'])


if __name__ == '__main__':
    unittest.main()
