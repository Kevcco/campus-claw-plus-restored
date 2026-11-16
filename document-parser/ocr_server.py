"""Explicit candidate OCR service. Fixed port, canonical token, bounded children."""
import hmac
import json
import os
from pathlib import Path
import re
import selectors
import signal
import socket
import subprocess
import sys
import tempfile
import time
import uuid
from server import ParserServer, Handler, InvalidRequest

MEDIA = {'application/pdf': b'%PDF-', 'image/png': b'\x89PNG\r\n\x1a\n', 'image/jpeg': b'\xff\xd8\xff'}

FAILURES = {'corrupt': 422, 'encrypted': 422, 'unsupported_content': 422,
            'resource_limit': 422, 'ocr_timeout': 504, 'worker_interrupted': 503,
            'language_missing': 503, 'internal_failure': 503}


def failed(reason):
    return dict(status='failed', reason=reason)


def recognize_child(path, connection, timeout):
    child = subprocess.Popen([sys.executable, '-B', str(Path(__file__).with_name('ocr_process.py')), str(path)],
                             stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                             stderr=subprocess.DEVNULL, start_new_session=True)
    output, started = bytearray(), time.monotonic()
    try:
        with selectors.DefaultSelector() as selector:
            selector.register(child.stdout, selectors.EVENT_READ)
            while selector.get_map():
                if time.monotonic() - started >= timeout:
                    return failed('ocr_timeout')
                try:
                    if connection.recv(1, socket.MSG_PEEK | socket.MSG_DONTWAIT) == b'':
                        return failed('worker_interrupted')
                except BlockingIOError:
                    pass
                for key, _ in selector.select(.1):
                    chunk = os.read(key.fd, 65536)
                    if not chunk:
                        selector.unregister(key.fileobj)
                    else:
                        output.extend(chunk)
                        if len(output) > 8_388_608:
                            return failed('resource_limit')
        child.wait(timeout=max(.001, timeout - (time.monotonic() - started)))
        result = json.loads(output)
        if child.returncode == 0 and result.get('status') == 'needs_review':
            return result
        if child.returncode != 0 and result.get('status') == 'failed' and result.get('reason') in FAILURES:
            return result
        return failed('resource_limit' if child.returncode == -signal.SIGKILL else 'internal_failure')
    except subprocess.TimeoutExpired:
        return failed('ocr_timeout')
    finally:
        try:
            os.killpg(child.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        child.wait()
        child.stdout.close()


class OCRHandler(Handler):
    def dispatch(self, started):
        method, target, headers = self.request_data(started)
        self.path = target if target in ('/health', '/internal/ocr') else 'unknown'
        if not hmac.compare_digest(headers.get('authorization', '').encode(), b'Bearer ' + self.server.token):
            return 401, failed('internal_failure')
        if '?' in target or '#' in target:
            raise InvalidRequest()
        if self.path == 'unknown':
            return 404, failed('unsupported_content')
        if method != ('GET' if target == '/health' else 'POST'):
            return 405, failed('unsupported_content')
        if target == '/health':
            if headers.get('content-length', '0') != '0':
                raise InvalidRequest()
            self.reject_buffered_extra()
            return 200, dict(status='ok')
        size = headers.get('content-length', '')
        media = headers.get('content-type')
        if not re.fullmatch(r'[1-9][0-9]{0,7}', size) or int(size) > 10_485_760 or media not in MEDIA:
            raise InvalidRequest()
        if not self.server.parsing.acquire(blocking=False):
            return 503, failed('internal_failure')
        try:
            data, deadline = bytearray(), time.monotonic() + 5
            while len(data) < int(size):
                data.extend(self.recv(min(65536, int(size) - len(data)), deadline))
            self.reject_buffered_extra()
            if not data.startswith(MEDIA[media]):
                return 422, failed('corrupt')
            with tempfile.TemporaryDirectory(prefix='ocr-rpc-') as directory:
                path = Path(directory) / 'source'
                path.write_bytes(data)
                result = self.server.recognize(path, self.request, min(180, max(.001, started + 185 - time.monotonic())))
            if result.get('status') == 'needs_review':
                return 200, result
            return FAILURES.get(result.get('reason'), 503), result
        finally:
            self.server.parsing.release()

    def handle(self):
        started, request_id = time.monotonic(), uuid.uuid4().hex
        self.path, status = 'unknown', 400
        try:
            try:
                status, payload = self.dispatch(started)
            except (InvalidRequest, TimeoutError, ConnectionError):
                status, payload = 400, failed('corrupt')
            except Exception:
                status, payload = 503, failed('internal_failure')
            body = json.dumps(payload, ensure_ascii=False, separators=(',', ':')).encode()
            if len(body) > 8_388_608:
                status, body = 422, b'{"status":"failed","reason":"resource_limit"}'
            self.request.settimeout(2)
            self.request.sendall((f'HTTP/1.1 {status} Response\r\nContent-Type: application/json\r\nContent-Length: {len(body)}\r\nConnection: close\r\nCache-Control: no-store\r\nX-Request-ID: {request_id}\r\n\r\n').encode() + body)
        except OSError:
            pass
        finally:
            print(json.dumps(dict(request_id=request_id, path=self.path, status=status,
                                  duration_ms=round((time.monotonic() - started) * 1000))), flush=True)


class OCRServer(ParserServer):
    def __init__(self, address, token, recognize=recognize_child):
        super().__init__(address, token)
        self.RequestHandlerClass, self.recognize = OCRHandler, recognize


if __name__ == '__main__':
    try:
        import pypdfium2
        for model in ('chi_sim', 'eng', 'osd'):
            if not Path('/opt/tessdata/' + model + '.traineddata').is_file():
                raise ValueError('missing model')
        with OCRServer(('0.0.0.0', 8092), os.environ.get('DOCUMENT_PARSER_TOKEN')) as server:
            server.serve_forever(poll_interval=.2)
    except (ValueError, OSError, ImportError):
        print('{"module":"ocr","message":"Private OCR startup rejected"}', file=sys.stderr)
        sys.exit(1)
