#!/usr/bin/env bash
# Makes the README's slideshow, docs/screenshots/tour.png: an animated PNG of
# the test vault, one captioned picture every few seconds. Run it after a
# change that shows, and look at the result before committing it.
#
#   tools/screenshots.sh
#
# Needs go, node, a Chromium or Chrome (set CHROME=), ffmpeg and ImageMagick,
# and the network: the diagrams and the graph load their scripts from the CDN.
set -euo pipefail
cd "$(dirname "$0")/.."

chrome=${CHROME:-}
if [[ -z $chrome ]]; then
    for candidate in chromium chromium-browser google-chrome google-chrome-stable; do
        if command -v "$candidate" >/dev/null; then chrome=$candidate; break; fi
    done
fi
[[ -n $chrome ]] || { echo "missing: a Chromium or Chrome (set CHROME=)" >&2; exit 2; }
for tool in go node curl ffmpeg magick; do
    command -v "$tool" >/dev/null || { echo "missing: $tool" >&2; exit 2; }
done

work=$(mktemp -d)
server='' browser=''
stop() {
    kill "$1" 2>/dev/null || return 0
    for _ in $(seq 1 50); do kill -0 "$1" 2>/dev/null || break; sleep 0.1; done
    kill -KILL "$1" 2>/dev/null || true
    wait "$1" 2>/dev/null || true
}
cleanup() {
    [[ -z $browser ]] || stop "$browser"
    [[ -z $server ]] || stop "$server"
    rm -rf "$work" 2>/dev/null || true
}
trap cleanup EXIT

port=18088 debug=9335
go build -o "$work/server" .
"$work/server" -no-auth -listen "127.0.0.1:$port" -vault test=./testdata/vault,nogit >"$work/server.log" 2>&1 &
server=$!
for _ in $(seq 1 50); do curl -s -o /dev/null "http://127.0.0.1:$port/" && break; sleep 0.1; done
"$chrome" --headless=new --no-sandbox --hide-scrollbars --remote-debugging-port=$debug \
    --user-data-dir="$work/profile" about:blank >"$work/browser.log" 2>&1 &
browser=$!
mkdir "$work/shots" "$work/frames"
node tools/screenshots.mjs "$debug" "http://127.0.0.1:$port" "$work/shots"

# A caption bar under each picture: a slideshow without one leaves the reader
# guessing what a frame shows.
for shot in "$work"/shots/*.png; do
    name=$(basename "$shot" .png)
    magick "$shot" \
        \( -size 1200x56 xc:'#1f2328' -font Liberation-Sans -pointsize 22 -fill white \
           -gravity center -annotate +0+0 "@$work/shots/$name.txt" \) \
        -append PNG24:"$work/frames/$name.png"
done
mkdir -p docs/screenshots
# One palette of 256 colours for all frames - APNG allows only one - made from
# all of them: screenshots are flat colour and text, and a full-colour
# animation would weigh several megabytes. Four seconds a picture, looping.
ffmpeg -loglevel error -y -i "$work/frames/%02d.png" \
    -vf "palettegen=max_colors=256:stats_mode=full:reserve_transparent=0" "$work/palette.png"
ffmpeg -loglevel error -y -framerate 1/4 -i "$work/frames/%02d.png" -i "$work/palette.png" \
    -lavfi "paletteuse=dither=none" -plays 0 -f apng docs/screenshots/tour.png
ls -l docs/screenshots/tour.png
