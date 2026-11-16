"""Bounded PNG/JPEG admission before the existing Leptonica decoder runs.

Keep encoded pixels, palette and transparency, but remove unrelated metadata.
EXIF is not a geometry instruction: coordinates refer to the encoded raster.
"""
import binascii
from pathlib import Path
import struct
import subprocess
import sys

from ocr_pdf import raster_size, recognize_raster

PNG = b'\x89PNG\r\n\x1a\n'


def png_pixels(data):
    offset, chunks, dimensions, ended, seen_data = 8, [PNG], None, False, False
    while offset < len(data):
        if offset + 12 > len(data):
            raise ValueError('image_corrupt')
        size = int.from_bytes(data[offset:offset + 4], 'big')
        kind = data[offset + 4:offset + 8]
        if not all(65 <= c <= 90 or 97 <= c <= 122 for c in kind) or kind[2] & 32:
            raise ValueError('image_corrupt')
        end = offset + 12 + size
        if end > len(data) or binascii.crc32(data[offset + 4:end - 4]) != int.from_bytes(data[end - 4:end], 'big'):
            raise ValueError('image_corrupt')
        payload = data[offset + 8:end - 4]
        if dimensions is None:
            if kind != b'IHDR' or size != 13:
                raise ValueError('image_corrupt')
            w, h, depth, colour, compression, filtering, interlace = struct.unpack('>IIBBBBB', payload)
            allowed = {0: (1, 2, 4, 8, 16), 2: (8, 16), 3: (1, 2, 4, 8), 4: (8, 16), 6: (8, 16)}
            if depth not in allowed.get(colour, ()) or compression or filtering or interlace not in (0, 1):
                raise ValueError('image_unsupported')
            dimensions = raster_size(w, h, 1)
        elif kind == b'IHDR':
            raise ValueError('image_corrupt')
        if kind in (b'acTL', b'fcTL', b'fdAT'):
            raise ValueError('image_unsupported')
        if kind == b'IDAT':
            seen_data = True
        elif kind == b'IEND':
            if size or not seen_data or end != len(data):
                raise ValueError('image_corrupt')
            ended = True
        elif kind not in (b'IHDR', b'PLTE', b'tRNS') and not kind[0] & 32:
            raise ValueError('image_unsupported')
        if kind in (b'IHDR', b'PLTE', b'tRNS', b'IDAT', b'IEND'):
            chunks.append(data[offset:end])
        offset = end
        if ended:
            break
    if not ended or dimensions is None:
        raise ValueError('image_corrupt')
    return b''.join(chunks), dimensions


def jpeg_pixels(data):
    offset, kept, dimensions, scanned, ended = 2, [data[:2]], None, False, False
    while offset < len(data):
        start = offset
        if data[offset] != 255:
            raise ValueError('image_corrupt')
        while offset < len(data) and data[offset] == 255:
            offset += 1
        if offset >= len(data):
            raise ValueError('image_corrupt')
        marker = data[offset]
        offset += 1
        if marker == 0xD9:
            if not scanned or dimensions is None or offset != len(data):
                raise ValueError('image_corrupt')
            kept.append(data[start:offset])
            ended = True
            break
        if marker in (0, 0xD8) or 0xD0 <= marker <= 0xD7 or offset + 2 > len(data):
            raise ValueError('image_corrupt')
        size = int.from_bytes(data[offset:offset + 2], 'big')
        end = offset + size
        if size < 2 or end > len(data):
            raise ValueError('image_corrupt')
        if marker in (0xC0, 0xC1, 0xC2):
            if dimensions is not None or size < 8 or data[offset + 2] != 8:
                raise ValueError('image_unsupported')
            h, w = struct.unpack('>HH', data[offset + 3:offset + 7])
            components = data[offset + 7]
            if components not in (1, 3, 4) or size != 8 + components * 3:
                raise ValueError('image_unsupported')
            dimensions = raster_size(w, h, 1)
        elif 0xC0 <= marker <= 0xCF and marker not in (0xC4, 0xC8, 0xCC):
            raise ValueError('image_unsupported')
        # Drop EXIF/XMP/ICC/comments, not APP14's CMYK colour interpretation.
        if marker not in (*range(0xE1, 0xEE), 0xEF, 0xFE):
            kept.append(data[start:end])
        offset = end
        if marker == 0xDA:
            if dimensions is None:
                raise ValueError('image_corrupt')
            scanned = True
            start = offset
            while offset < len(data):
                if data[offset] != 255:
                    offset += 1
                    continue
                next_byte = offset + 1
                while next_byte < len(data) and data[next_byte] == 255:
                    next_byte += 1
                if next_byte >= len(data):
                    raise ValueError('image_corrupt')
                if data[next_byte] == 0 or 0xD0 <= data[next_byte] <= 0xD7:
                    offset = next_byte + 1
                else:
                    break
            kept.append(data[start:offset])
    if not ended:
        raise ValueError('image_corrupt')
    return b''.join(kept), dimensions


def admitted_image(data):
    if not 1 <= len(data) <= 10_485_760:
        raise ValueError('input_limit_or_signature')
    if data.startswith(PNG):
        return png_pixels(data)
    if data.startswith(b'\xff\xd8\xff'):
        return jpeg_pixels(data)
    raise ValueError('image_corrupt')


def one_image(path):
    import tempfile
    pixels, (width, height) = admitted_image(Path(path).read_bytes())
    with tempfile.TemporaryDirectory(prefix='ocr-image-') as directory:
        image = Path(directory) / 'raster'
        image.write_bytes(pixels)
        return recognize_raster(image, width, height, 0)


def recognize_image(path):
    child = subprocess.run([sys.executable, '-B', __file__, str(path)], capture_output=True, timeout=15)
    if child.returncode != 0:
        if child.returncode == -9:
            raise ValueError('resource_limit')
        # Only closed failure labels cross the process boundary.
        reason = child.stdout.decode('ascii', errors='replace').strip()
        if reason not in ('page_pixel_limit', 'image_corrupt', 'image_unsupported', 'output_limit', 'resource_limit', 'internal_failure', 'ocr_timeout'):
            reason = 'internal_failure'
        raise ValueError(reason)
    import json
    if len(child.stdout) > 8_388_608:
        raise ValueError('response_limit')
    return dict(pages=[json.loads(child.stdout)])


if __name__ == '__main__':
    import json
    try:
        print(json.dumps(one_image(sys.argv[1]), ensure_ascii=False))
    except ValueError as error:
        print(str(error) if str(error) in ('page_pixel_limit', 'image_corrupt', 'image_unsupported', 'output_limit', 'resource_limit') else 'image_corrupt')
        sys.exit(1)
    except subprocess.TimeoutExpired:
        print('ocr_timeout')
        sys.exit(1)
    except MemoryError:
        print('resource_limit')
        sys.exit(1)
    except Exception:
        print('internal_failure')
        sys.exit(1)
