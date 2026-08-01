#!/usr/bin/env bash
set -e

run_test() {
    local runs="$1"
    local hurlfile="$2"
    local output="$3"
    local container="$4"
    shift 4
    local query=("$@")

    docker compose up -d --force-recreate
    until docker compose exec varnish varnishadm ping; do
        sleep 0.1
    done

    docker compose exec -d "$container" sh -c "timeout 20 varnishlog ${query[*]} > /tmp/output.txt 2>&1"
    sleep 1

    for _ in $(seq 1 "$runs"); do
        (
            hurl "$hurlfile" >/dev/null
        ) &

        if [[ "$hurlfile" =~ "streaming" ]]; then
            sleep 0.5
        fi
    done
    wait

    sleep 2

    docker compose cp "${container}:/tmp/output.txt" "$output"
}

mkdir -p output

TESTS=(
    "1 ./tests/cached.hurl        ./output/cached.txt        varnish        -g vxid -q 'VCL_call eq \"HIT\"'"
    "1 ./tests/simple-post.hurl   ./output/simple-post.txt   varnish        -g session"
    "1 ./tests/req-restart.hurl   ./output/req-restart.txt   varnish        -g session"
    "1 ./tests/backend-retry.hurl ./output/backend-retry.txt varnish        -g session"
    "1 ./tests/esi1.hurl          ./output/esi-1.txt         varnish        -g session"
    "1 ./tests/esi1.hurl          ./output/esi-synth.txt     varnishbackend -g session"
    "3 ./tests/streaming-hit.hurl ./output/streaming-hit.txt varnish        -g session"
    # "-g raw" counterparts, used as vsl.Parser test fixtures
    "1 ./tests/simple-post.hurl   ./output/simple-post_g_raw.txt   varnish        -g raw"
    "1 ./tests/req-restart.hurl   ./output/req-restart_g_raw.txt   varnish        -g raw"
    "1 ./tests/backend-retry.hurl ./output/backend-retry_g_raw.txt varnish        -g raw"
    "1 ./tests/esi1.hurl          ./output/esi-1_g_raw.txt         varnish        -g raw"
    "1 ./tests/esi1.hurl          ./output/esi-synth_g_raw.txt     varnishbackend -g raw"
    "3 ./tests/streaming-hit.hurl ./output/streaming-hit_g_raw.txt varnish        -g raw"
    # caches a few distinct objects, to catch non-transactional (VXID 0)
    # ExpKill events from the expiry thread, not just CLI ping/pong
    "1 ./tests/cache-expiry.hurl  ./output/cache-expiry_g_raw.txt  varnish        -g raw"
    # with verbose mode ("-v"), one capture per grouping mode
    "1 ./tests/req-restart.hurl ./output/req-restart_verbose_g_session.txt varnish -v -g session"
    "1 ./tests/req-restart.hurl ./output/req-restart_verbose_g_request.txt varnish -v -g request"
    "1 ./tests/req-restart.hurl ./output/req-restart_verbose_g_vxid.txt    varnish -v -g vxid"
    "1 ./tests/req-restart.hurl ./output/req-restart_verbose_g_raw.txt     varnish -v -g raw"
)

PS3="Test to run: "
select test in "${TESTS[@]}"; do
    if [[ -z "${test:-}" ]]; then
        echo "Invalid option."
        exit 1
    fi

    read -r -a args <<<"$test"
    echo "Running: run_test ${args[*]}"
    run_test "${args[@]}"
    exit $?
done
