vcl 4.0;

import std;

backend varnishb {
    .host = "192.168.50.11";
    .port = "80";
}

sub vcl_recv {
    set req.backend_hint = varnishb;
    set req.http.A = "3";
}

sub vcl_backend_response {
    set beresp.http.Cache-Control = "no-cache, no-store, private";
    set beresp.ttl = 0s;
}
