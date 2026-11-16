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
            def request(path, auth=True, body=None):
                client = http.client.HTTPConnection(*server.server_address, timeout=2)
                headers = {'Authorization': 'Bearer '+token} if auth else {}
                if body is not None:
                    headers['Content-Type'] = 'application/pdf'
                client.request('POST' if body is not None else 'GET', path, body=body, headers=headers)
                response = client.getresponse()
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
            finally:
                server.shutdown()
                thread.join()


if __name__ == '__main__':
    unittest.main()
