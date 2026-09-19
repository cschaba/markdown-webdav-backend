#!/usr/bin/env bash
# Prints the test vault's export pages to real PDFs with headless Chromium and
# reads them back. What paper looks like cannot be told from the HTML: whether
# the page size took, where the pages break, and that nothing of the site's
# furniture got printed - nor a page number of ours: those were removed, the
# CSS for them works in Chromium only.
#
#   go build && tools/check-pdf-export.sh            # needs chromium, pdfinfo, pdftotext
#   tools/check-pdf-export.sh /tmp/keep              # keep the PDFs there to look at
set -euo pipefail
cd "$(dirname "$0")/.."

for tool in chromium pdfinfo pdftotext curl; do
    command -v "$tool" >/dev/null || { echo "missing: $tool - this check cannot run without it" >&2; exit 2; }
done
[[ -x ./markdown-webdav-backend ]] || { echo "build first: go build" >&2; exit 2; }

out=${1:-$(mktemp -d)}
mkdir -p "$out"
port=18097
base="http://127.0.0.1:$port/test"
./markdown-webdav-backend -no-auth -listen "127.0.0.1:$port" -vault test=./testdata/vault,nogit >"$out/server.log" 2>&1 &
server=$!
trap 'kill $server 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do curl -s -o /dev/null "http://127.0.0.1:$port/" && break; sleep 0.1; done

failures=0
fail() { echo "FAIL  $*"; failures=$((failures + 1)); }
pass() { echo "ok    $*"; }

# pdf <name> <url>: print to $out/<name>.pdf
pdf() {
    timeout 90 chromium --headless=new --no-sandbox --no-pdf-header-footer --virtual-time-budget=8000 \
        --print-to-pdf="$out/$1.pdf" "$2" >/dev/null 2>&1 || true
    [[ -s "$out/$1.pdf" ]] || { fail "$1: no PDF written"; return 1; }
}
pages() { pdfinfo "$out/$1.pdf" | awk '/^Pages:/ {print $2}'; }
size() { pdfinfo "$out/$1.pdf" | awk '/^Page size:/ {printf "%.0fx%.0f", $3, $5}'; }
# the text of one page, on one line
page() { pdftotext -f "$2" -l "$2" -layout "$out/$1.pdf" - | tr '\n' ' ' | sed 's/  */ /g'; }
# the last line of a page; a bare number there would be a page number
foot() { pdftotext -f "$2" -l "$2" "$out/$1.pdf" - | grep -v '^\s*$' | tail -1 | tr -d ' \f'; }
unnumbered() { # unnumbered <name>: no page of the PDF ends in a bare number
    local n; n=$(pages "$1")
    for p in $(seq 1 "$n"); do
        if [[ "$(foot "$1" "$p")" =~ ^[0-9]+$ ]]; then fail "$1: page $p ends in a bare number, '$(foot "$1" "$p")'"; return; fi
    done
    pass "$1: no page numbers of ours on its $n page(s)"
}
expect() { # expect <what> <actual> <wanted>
    if [[ "$2" == "$3" ]]; then pass "$1: $2"; else fail "$1: got '$2', want '$3'"; fi
}
contains() { if grep -qF -- "$3" <<<"$2"; then pass "$1"; else fail "$1: lacks '$3' in: ${2:0:160}"; fi; }
lacks() { if grep -qF -- "$3" <<<"$2"; then fail "$1: contains '$3'"; else pass "$1"; fi; }

note="10%20Printing.md"

echo "--- one note, A4: three pages through its two page breaks"
pdf note-a4 "$base/-/export?path=$note" && {
    expect "page size" "$(size note-a4)" "595x842"
    expect "pages" "$(pages note-a4)" "3"
    contains "page 2 opens after \\pagebreak" "$(page note-a4 2)" "This paragraph opens the second page"
    contains "page 3 opens after the div" "$(page note-a4 3)" "This paragraph opens the third page"
    unnumbered note-a4
    all=$(pdftotext "$out/note-a4.pdf" -)
    # (the page's own text says "Export to PDF" and "Save as PDF": not those)
    for chrome in "Search" "Backlinks" "Back" "Page size" "First page number" "Include sub pages" "Apply" "Graph" "#test/print" "page break"; do
        lacks "no site furniture: $chrome" "$all" "$chrome"
    done
}

echo "--- page size"
pdf note-a5 "$base/-/export?path=$note&size=A5" && expect "A5" "$(size note-a5)" "420x595"
pdf note-letter "$base/-/export?path=$note&size=letter" && expect "Letter" "$(size note-letter)" "612x792"
pdf note-bogus "$base/-/export?path=$note&size=A0&start=41" && {
    expect "an unknown size falls back to A4" "$(size note-bogus)" "595x842"
    unnumbered note-bogus # a leftover "start" does nothing
}

echo "--- sub pages: contents, then every note on a new page, in reading order"
pdf book "$base/-/export?path=$note&sub=1" && {
    n=$(pages book)
    if ((n >= 9)); then pass "pages: $n"; else fail "pages: $n, want at least 9 (contents, 3 for the note, chapters 1, 2 over several, 10, sources)"; fi
    contents=$(page book 1)
    contains "contents page" "$contents" "Chapter 10"
    # headings on a line of their own, after the contents page
    # (pdftotext puts a form feed before the first line of a page)
    order=$(pdftotext -f 2 "$out/book.pdf" - | tr -d '\f' | { grep -E '^(Chapter 1|Chapter 2|Chapter 10|Sources)$' || true; } | tr '\n' ',')
    expect "reading order" "$order" "Chapter 1,Chapter 2,Chapter 10,Sources,"
    unnumbered book
    contains "a note without a heading gets its title" "$(page book "$n")" "Sources"
}
pdf no-sub "$base/-/export?path=$note" && expect "without the switch, the note alone" "$(pages no-sub)" "3"

echo "--- a folder"
pdf folder "$base/-/export?path=10%20Printing" && {
    lacks "its own notes only" "$(pdftotext "$out/folder.pdf" -)" "A note in a subfolder"
}
pdf folder-sub "$base/-/export?path=10%20Printing&sub=1" &&
    contains "with its subfolders when asked" "$(pdftotext "$out/folder-sub.pdf" -)" "A note in a subfolder"

echo "--- an ordinary page, printed as it is (Ctrl+P)"
pdf plain "$base/10%20Printing" && {
    all=$(pdftotext "$out/plain.pdf" -)
    contains "its text" "$all" "Printing and PDF export"
    for chrome in "Search" "Backlinks" "Graph" "Tags" "#test/print"; do lacks "no site furniture: $chrome" "$all" "$chrome"; done
    # its text says "Export to PDF" once; the footer link would be the second
    expect "the footer link is not printed" "$(grep -o "Export to PDF" <<<"$all" | wc -l)" "1"
    expect "page breaks work here too" "$(pages plain)" "3"
    unnumbered plain
}

echo
if ((failures)); then echo "$failures check(s) failed; PDFs in $out"; exit 1; fi
echo "all checks passed; PDFs in $out"
