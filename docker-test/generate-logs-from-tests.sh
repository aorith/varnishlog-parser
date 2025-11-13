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
    docker compose exec -d "$container" sh -c "timeout 12 varnishlog ${query[*]} > /tmp/output.txt 2>&1"

    until docker compose exec varnish varnishadm ping; do
        sleep 0.1
    done
    sleep 0.5
    for _ in $(seq 1 "$runs"); do
        (
            hurl "$hurlfile" >/dev/null
        ) &
    done
    wait
    sleep 12

    docker compose cp "${container}:/tmp/output.txt" "$output"
}

mkdir -p output

TESTS=(
    "1 ./tests/simple-post.hurl   ./output/simple-post.txt   varnish        -g session"
    "1 ./tests/req-restart.hurl   ./output/req-restart.txt   varnish        -g session"
    "1 ./tests/backend-retry.hurl ./output/backend-retry.txt varnish        -g session"
    "1 ./tests/cached.hurl        ./output/cached.txt        varnish        -g vxid -q 'VCL_call eq \"HIT\"'"
    "1 ./tests/esi1.hurl          ./output/esi-1.txt         varnish        -g session"
    "1 ./tests/esi1.hurl          ./output/esi-synth.txt     varnishbackend -g session"
    "3 ./tests/streaming-hit.hurl ./output/streaming-hit.txt varnish        -g session"
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
