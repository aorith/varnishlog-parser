POST http://127.0.0.1:8001/test1
Host: varnishlog.iou.re
[Form]
user: myself
password: 1234
HTTP 200

GET http://127.0.0.1:8001/test2
Host: varnishlog.iou.re
X-Same-Session: 1
