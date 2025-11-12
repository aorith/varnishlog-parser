vcl 4.0;

import std;

# I am the backend
backend default none;

sub vcl_recv {
    set req.backend_hint = default;

    if (req.url ~ "^/ec1") {
        return(synth(700, "Level 1 ESI"));
    }
    if (req.url ~ "^/ec2") {
        return(synth(701, "Level 1 and 2 ESI"));
    }
    if (req.url ~ "^/nested-esi") {
        return(synth(707, "ESI 1"));
    }
    if (req.url ~ "^/esi1") {
        return(synth(708, "ESI 1"));
    }
    if (req.url ~ "^/esi2") {
        return(synth(709, "ESI 1"));
    }

    return(synth(710, "OK"));
}

sub vcl_synth {
    if (resp.status == 710) {
        set resp.status = 200;
        set resp.http.Content-Type = "text/plain";
        set resp.http.X-Default-Response = "yes";
        set resp.body = {"Default Response"};
        return(deliver);
    }

    # Level 1 ESI
    if (resp.status == 700) {
        set resp.status = 200;
        set resp.http.Content-Type = "text/html; charset=utf-8";
        set resp.body = {"<html><body><p>This HTML includes an ESI</p><esi:include src="/esi1"/></body></html>"};
        return(deliver);
    }

    # Level 1 and 2 ESI
    if (resp.status == 701) {
        set resp.status = 200;
        set resp.http.Content-Type = "text/html; charset=utf-8";
        set resp.body = {"<html><body><p>Main</p><esi:include src="/nested-esi"/></body></html>"};
        return(deliver);
    }

    # /nested-esi
    if (resp.status == 707) {
        set resp.status = 200;
        set resp.http.Content-Type = "text/html; charset=utf-8";
        set resp.body = {"<p>ESI response which includes another ESI.</p><esi:include src="/esi2"/>"};
        return(deliver);
    }

    # /esi1
    if (resp.status == 708) {
        set resp.status = 200;
        set resp.http.Content-Type = "text/html; charset=utf-8";
        set resp.body = {"<p>This is included content via ESI.</p>"};
        return(deliver);
    }

    # /esi2
    if (resp.status == 709) {
        set resp.status = 200;
        set resp.http.Content-Type = "text/html; charset=utf-8";
        set resp.body = {"<p>This is also an ESI response.</p>"};
        return(deliver);
    }
}
