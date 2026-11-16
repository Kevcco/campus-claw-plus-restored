import json
from pathlib import Path
import subprocess
import sys
from ocr_pdf import recognize_pdf
from ocr_result import result_from_tsv

if __name__ == '__main__':
    try:
        pages = recognize_pdf(sys.argv[1])['pages']
        result = result_from_tsv([p['tsv'] for p in pages], [p['text'] for p in pages])
        if len(json.dumps(result, ensure_ascii=False).encode()) > 8_388_608:
            raise ValueError('output_limit')
        code = 0
    except subprocess.TimeoutExpired:
        result, code = dict(status='failed', reason='ocr_timeout'), 1
    except ValueError as error:
        reason = 'resource_limit' if str(error) in ('output_limit', 'response_limit', 'page_pixel_limit', 'page_count_limit') else 'unsupported_content'
        result, code = dict(status='failed', reason=reason), 1
    except Exception:
        result, code = dict(status='failed', reason='corrupt'), 1
    print(json.dumps(result, ensure_ascii=False, separators=(',', ':')))
    sys.exit(code)
