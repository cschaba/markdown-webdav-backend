#!/usr/bin/env bash
# Runs every check that needs a real browser, each in a fresh browser profile:
# builds the server, serves the test vault, starts headless Chromium and hands
# both to the tools/check-*.mjs. CI runs this; so can anyone before a commit
# that touches a script, a template or the policy.
#
#   tools/check-browser.sh                 # all of them
#   tools/check-browser.sh keyboard slides # some: tools/check-<name>.mjs
#   CHROME=/path/to/chrome tools/check-browser.sh
#
# Needs go, node (22 or newer: WebSocket), a Chromium or Chrome, and the
# network, since check-security loads the scripts from the CDN. A missing tool
# is an error, not a skip: a check that did not run has not passed.
set -euo pipefail
cd "$(dirname "$0")/.."

checks=("$@")
[[ ${#checks[@]} -gt 0 ]] || checks=(keyboard slides export-settings security)

chrome=${CHROME:-}
if [[ -z $chrome ]]; then
    for candidate in chromium chromium-browser google-chrome google-chrome-stable; do
        if command -v "$candidate" >/dev/null; then chrome=$candidate; break; fi
    done
fi
[[ -n $chrome ]] || { echo "missing: a Chromium or Chrome (set CHROME=) - the browser checks cannot run without it" >&2; exit 2; }
for tool in go node curl; do
    command -v "$tool" >/dev/null || { echo "missing: $tool - the browser checks cannot run without it" >&2; exit 2; }
done
for name in "${checks[@]}"; do
    [[ -f tools/check-$name.mjs ]] || { echo "no such check: tools/check-$name.mjs" >&2; exit 2; }
done

work=$(mktemp -d)
server='' browser=''
# stop ends a process and waits for it, but not forever: a browser that is
# still starting may not answer TERM, and a bare wait for it hung a release
# for half an hour.
stop() {
    kill "$1" 2>/dev/null || return 0
    for _ in $(seq 1 50); do kill -0 "$1" 2>/dev/null || break; sleep 0.1; done
    kill -KILL "$1" 2>/dev/null || true
    wait "$1" 2>/dev/null || true
}
cleanup() {
    [[ -z $browser ]] || stop "$browser"
    [[ -z $server ]] || kill "$server" 2>/dev/null || true
    sleep 0.5 # the browser's children are still writing their profile
    rm -rf "$work" 2>/dev/null || true
}
trap cleanup EXIT

port=18086 debug=9333
go build -o "$work/server" .
"$work/server" -no-auth -listen "127.0.0.1:$port" -vault test=./testdata/vault,nogit >"$work/server.log" 2>&1 &
server=$!
for _ in $(seq 1 50); do curl -s -o /dev/null "http://127.0.0.1:$port/" && break; sleep 0.1; done
curl -s -o /dev/null "http://127.0.0.1:$port/" || { echo "the server did not start:" >&2; cat "$work/server.log" >&2; exit 2; }

failed=()
for name in "${checks[@]}"; do
    echo "=== $name"
    # --no-sandbox: a CI runner's container has no user namespaces to sandbox
    # with; what is loaded is our own test vault.
    "$chrome" --headless=new --no-sandbox --remote-debugging-port=$debug --user-data-dir="$work/profile-$name" about:blank >"$work/browser-$name.log" 2>&1 &
    browser=$!
    node "tools/check-$name.mjs" "$debug" "http://127.0.0.1:$port" "$work" || failed+=("$name")
    stop "$browser"
    browser=''
done

if [[ ${#failed[@]} -gt 0 ]]; then
    echo; echo "failed: ${failed[*]}"; exit 1
fi
echo; echo "all browser checks passed: ${checks[*]}"
