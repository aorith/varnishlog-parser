# This test might need some retries to generate a streaming-hit
# it must be executed in parallel with a cold varnish cache

GET http://127.0.0.1:8001/drip?duration=5&numbytes=1024
Host: httpbin.org
Accept: application/octet-stream
