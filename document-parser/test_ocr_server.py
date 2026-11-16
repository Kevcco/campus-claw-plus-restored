import base64
import http.client
import threading
import unittest
from ocr_server import OCRServer


class OCRHTTP(unittest.TestCase):
    def test_di_sc_19_auth_health_busy_and_closed_input(self):
        token = base64.urlsafe_b64encode(bytes(32)).decode().rstrip('=')
        calls = []
        def recognize(path, connection, timeout):
            calls.append(path.read_bytes())
            return dict(status='failed', reason='unsupported_content')
        with OCRServer(('127.0.0.1', 0), token, recognize) as server:
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            def request(path, auth=True, body=None, media='application/pdf'):
                client = http.client.HTTPConnection(*server.server_address, timeout=2)
                headers = {'Authorization': 'Bearer '+token} if auth else {}
                if body is not None:
                    headers['Content-Type'] = media
                client.request('POST' if body is not None else 'GET', path, body=body, headers=headers)
                response = client.getresponse()
                self.assertEqual(response.getheader('Cache-Control'), 'no-store')
                self.assertRegex(response.getheader('X-Request-ID'), r'^[a-f0-9]{32}$')
                self.assertEqual(response.getheader('Content-Type'), 'application/json')
                status, data = response.status, response.read()
                client.close()
                return status, data
            try:
                self.assertEqual(request('/health', False)[0], 401)
                self.assertEqual(request('/health'), (200, b'{"status":"ok"}'))
                self.assertEqual(request('/unknown')[0], 404)
                self.assertEqual(request('/internal/ocr?url=x', body=b'%PDF-test')[0], 400)
                server.parsing.acquire()
                try:
                    self.assertEqual(request('/internal/ocr', body=b'%PDF-test')[0], 503)
                finally:
                    server.parsing.release()
                self.assertEqual(request('/internal/ocr', body=b'%PDF-test')[0], 422)
                self.assertEqual(calls, [b'%PDF-test'])
                for media, signature in [('image/png', b'\x89PNG\r\n\x1a\n'), ('image/jpeg', b'\xff\xd8\xff')]:
                    self.assertEqual(request('/internal/ocr', body=signature + b'test', media=media)[0], 422)
                    self.assertEqual(calls[-1], signature + b'test')
                    count = len(calls)
                    self.assertEqual(request('/internal/ocr', body=b'%PDF-wrong-media', media=media)[0], 422)
                    self.assertEqual(len(calls), count)
                self.assertEqual(request('/internal/ocr', body=b'GIF89a', media='image/gif')[0], 400)
                server.recognize = lambda *_: dict(status='needs_review', body='x' * 8_388_609)
                self.assertEqual(request('/internal/ocr', body=b'%PDF-test'), (422, b'{"status":"failed","reason":"resource_limit"}'))
            finally:
                server.shutdown()
                thread.join()


if __name__ == '__main__':
    unittest.main()
