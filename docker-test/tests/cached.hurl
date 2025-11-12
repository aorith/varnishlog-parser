GET http://127.0.0.1:8001/item
Host: varnishlog.iou.re
X-Forwarded-For: 1.2.3.4
HTTP 200


GET http://127.0.0.1:8001/item
Host: varnishlog.iou.re
X-Forwarded-For: 1.2.3.4
Cached: 1

[Options]
delay: 100ms
