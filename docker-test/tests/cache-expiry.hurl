GET http://127.0.0.1:8001/msp
Host: varnishlog.iou.re
X-Forwarded-For: 1.2.3.4
whoami: 1
HTTP 200

GET http://127.0.0.1:8001/msp
Host: varnishlog.iou.re
X-Forwarded-For: 1.2.3.4
whoami: 1
[Options]
delay: 5s
HTTP 200

GET http://127.0.0.1:8001/msp
Host: varnishlog.iou.re
X-Forwarded-For: 1.2.3.4
whoami: 1
[Options]
delay: 2s
HTTP 200
