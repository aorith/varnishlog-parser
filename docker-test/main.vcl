vcl 4.0;

import std;
import dynamic;

backend backend1 {
    .host = "192.168.50.11";
    .port = "80";
}

backend whoami {
    .host = "192.168.50.12";
    .port = "80";
}

sub vcl_init {
    new d = dynamic.director(port = 80, ttl = 1s);
}

sub vcl_recv {
    std.log("start custom recv");

    if (req.url ~ "^(/delay)") {
        set req.http.httpbin = "1";
    } else {
        set req.backend_hint = whoami;
        set req.http.whoami = "1";
    }

    if (req.restarts == 0 && req.url ~ "^/rt") {
        set req.http.x-do-restart = "yes";
        return(restart);
    }

    if (req.url ~ "^/rbt") {
        set req.http.x-do-retry = "yes";
    }

    if (req.url ~ "^(/ec[0-9]|/nested-esi)") {
        set req.http.x-do-esi = "1";
        set req.backend_hint = backend1;
    }
    if (req.url ~ "^/esi") {
        set req.backend_hint = backend1;
    }

    std.log("end custom recv");
}

sub vcl_backend_fetch {
    if (bereq.http.httpbin == "1") {
        set bereq.http.host = "httpbin.org";
        set bereq.backend = d.backend(bereq.http.host).resolve();
    }
}

sub vcl_backend_response {
    if (bereq.http.x-do-esi == "1") {
        set beresp.do_esi = true;
    }

    if (bereq.http.whoami == "1") {
        set beresp.http.Cache-Control = "max-age=5";
        set beresp.ttl = 5s;
    }

    if (bereq.retries == 0 && bereq.http.x-do-retry == "yes") {
        return(retry);
    }
}
