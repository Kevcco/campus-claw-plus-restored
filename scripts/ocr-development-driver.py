"""Runs only opaque development images. Gold and blind inputs are never mounted."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

inputs = sorted(Path('/inputs').iterdir())
assert len(inputs) == 14 and all(p.suffix in ('.png', '.jpg') for p in inputs)
assert not Path('/inputs/manifest.json').exists()
rows = []
for image in inputs:
    started = time.monotonic()
    prefix = '/tmp/' + image.stem
    command = ['tesseract', str(image), prefix, '--oem', '1', '--psm', '1',
               '-l', 'chi_sim+eng', '--dpi', '200', 'txt', 'tsv']
    try:
        child = subprocess.run(command, capture_output=True, timeout=15)
        code = child.returncode
        error = child.stderr.decode('utf8', errors='replace')
    except subprocess.TimeoutExpired:
        code, error = 124, 'page wall clock exceeded 15 seconds'
    text = Path(prefix + '.txt').read_text(encoding='utf8') if code == 0 and Path(prefix + '.txt').exists() else None
    tsv = Path(prefix + '.tsv').read_text(encoding='utf8') if code == 0 and Path(prefix + '.tsv').exists() else None
    peak = int(Path('/sys/fs/cgroup/memory.peak').read_text())
    rows.append({'file': image.name, 'sha256': hashlib.sha256(image.read_bytes()).hexdigest(),
                 'exit_code': code, 'elapsed_ms': round((time.monotonic()-started)*1000),
                 'container_peak_bytes_so_far': peak, 'text': text, 'tsv': tsv, 'stderr': error})
    for suffix in ('.txt', '.tsv'):
        Path(prefix + suffix).unlink(missing_ok=True)
print(json.dumps({'engine_version': subprocess.run(['tesseract', '--version'], capture_output=True, text=True).stdout,
                  'parameters': ['--oem', '1', '--psm', '1', '-l', 'chi_sim+eng', '--dpi', '200'],
                  'uid': os.getuid(), 'rows': rows, 'blind_inputs': 0,
                  'tmp_output_files_after': len(list(Path('/tmp').glob('page-*')))}, ensure_ascii=False))
