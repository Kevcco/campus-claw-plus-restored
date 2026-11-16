"""Strict Tesseract TSV -> OCRResult with original-raster word coordinates."""
import math
import unicodedata

HEADER = 'level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext'


def result_from_tsv(pages, texts=None):
    if not 1 <= len(pages) <= 100:
        raise ValueError('page_count_limit')
    body, output, scores = '', [], []
    if texts is not None and len(texts) != len(pages):
        raise ValueError('invalid_tsv')
    for page_index, tsv in enumerate(pages):
        lines = tsv.splitlines()
        if not lines or lines.pop(0) != HEADER or len(tsv.encode()) > 8_388_608:
            raise ValueError('invalid_tsv')
        width = height = None
        words, previous, previous_line = [], None, None
        if output:
            body += '\n\n'
        page_start = len(body)
        cursor = page_start
        if texts is not None:
            page_text = unicodedata.normalize('NFC', texts[page_index]).strip()
            if not page_text or any(unicodedata.category(c) in ('Cc', 'Cs') and c not in '\n\t' for c in page_text):
                raise ValueError('empty_ocr_page')
            body += page_text
            if len(body.encode()) > 2_097_152:
                raise ValueError('output_limit')
        for line in lines:
            if not line:
                continue
            fields = line.split('\t')
            if len(fields) != 12:
                raise ValueError('invalid_tsv')
            numbers = fields[:10]
            if any(not n.isascii() or not n.isdecimal() or (len(n) > 1 and n[0] == '0') for n in numbers):
                raise ValueError('invalid_tsv')
            level, page, block, paragraph, row, word, x, y, w, h = map(int, numbers)
            if not 1 <= level <= 5 or page != 1:
                raise ValueError('invalid_tsv')
            if level == 1:
                if width is not None or x or y or w < 1 or h < 1 or w * h > 20_000_000:
                    raise ValueError('invalid_tsv')
                width, height = w, h
                continue
            if width is None:
                raise ValueError('invalid_tsv')
            if level != 5:
                continue
            if min(block, paragraph, row, word, w, h) < 1 or x + w > width or y + h > height:
                raise ValueError('invalid_tsv')
            confidence = float(fields[10])
            if not math.isfinite(confidence) or not 0 <= confidence <= 100:
                raise ValueError('invalid_tsv')
            text = unicodedata.normalize('NFC', fields[11]).strip()
            if any(unicodedata.category(c) in ('Cc', 'Cs') for c in fields[11]):
                raise ValueError('invalid_tsv')
            if not text:
                continue
            key = block, paragraph, row, word
            if len(text) > 2000 or (previous is not None and key <= previous):
                raise ValueError('invalid_tsv')
            line_key = key[:3]
            if texts is not None:
                start = body.find(text, cursor)
                if start < cursor:
                    raise ValueError('word_quote_mismatch')
                end = start + len(text)
                cursor = end
            else:
                if words:
                    body += ' ' if line_key == previous_line else '\n'
                start = len(body)
                body += text
                end = len(body)
            words.append(dict(text=text, char_start=start, char_end=end,
                              bbox=[x / width, y / height, (x + w) / width, (y + h) / height],
                              engine_confidence=confidence))
            scores.append(confidence)
            previous, previous_line = key, line_key
            if len(words) > 20000 or len(scores) > 100000 or (texts is None and len(body.encode()) > 2_097_152):
                raise ValueError('output_limit')
        if width is None or not words:
            raise ValueError('empty_ocr_page')
        output.append(dict(page=len(output) + 1, width=width, height=height,
                           rotation_applied=0, source='ocr', char_start=page_start,
                           char_end=len(body), words=words))
    attention = sum(scores) / len(scores) < 80 or sum(s < 50 for s in scores) / len(scores) > .2
    return dict(status='needs_review', engine='tesseract-5.5.4-fast-8741641', body=body,
                quality='low_quality' if attention else 'acceptable_candidate', pages=output)
