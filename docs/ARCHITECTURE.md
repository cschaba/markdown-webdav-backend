# Architecture

## Shape

```
Obsidian + Remotely Save ──WebDAV──▶ /dav/ ─▶ vault.FS ─▶ files on disk
                                                 │ change
                                                 ├──▶ index   (re-read note / rescan)
                                                 └──▶ gitlog  (debounced commit)
Browser ──GET──▶ /  ─▶ web ─▶ render (goldmark) ◀── index (links, tags, backlinks)
```

One process; per vault one directory, one index, one committer, mounted at
`/<name>/` and `/dav/<name>/`. The files are the only state; the index is rebuilt
from them at start and the git history lives beside them.

## Decisions

**WebDAV, because of iOS.** Obsidian on iPhone and iPad can only open vaults in
its own sandbox or iCloud, so "mount a share" is not an option there. A sync
plugin speaking WebDAV (Remotely Save) works on all four target platforms, and
desktops can mount the same endpoint natively. Git-based sync from the phone was
the alternative; it is fragile on iOS and makes every device handle merges.

**`golang.org/x/net/webdav` rather than a homemade handler.** It implements
locking, `PROPFIND`, `MOVE`/`COPY` and ETags. We only wrap its `FileSystem`
(`internal/vault`) to learn about changes and to hide `.git`. ETags derive from
modification time and size, so they are stable across reads and restarts, which
sync clients rely on.

**Changes are pushed, not watched.** All writes arrive through our own
`FileSystem`, so it reports them on `Close`. No fsnotify, no polling. The cost:
edits made directly on the server's disk are not noticed until restart. If that
becomes a real use, add a rescan trigger rather than a watcher.

**Shell out to `git` instead of go-git.** The binary is the reference
implementation, the vault stays a normal repository for every other tool, and the
code is a page long. The price is `git` in the image. `-c safe.directory=` is
passed on every call because a mounted volume is usually owned by another uid and
git otherwise refuses it.

**Debounced commits with a ceiling.** A sync run is a burst of `PUT`s; committing
each would bury the history. The committer waits for a quiet period
(`-commit-after`) but never longer than `-commit-max-wait` after the first
change, so continuous editing still gets versioned. Pending changes are committed
on shutdown and at start.

**One parser for rendering and indexing.** `render.parse` produces both the AST
to render and the metadata (title, tags, links). The index cannot disagree with
the page about what is a tag, and code blocks are excluded for free.

**Backlinks are computed per request** by resolving every note's links. It keeps
the index free of reverse maps that must be repaired on every rename. It is
linear in the number of links; revisit if a vault makes pages slow.

**Link resolution starts at the linking note.** A bare name is looked for next
to the note first, then vault-wide by Obsidian's rule (base name or trailing
path, shortest path wins). A plain path is tried relative to the note, then
from the root, then as a trailing path. `./`, `../` and a leading `/` say where
to look and are followed strictly: a wrong path shows as missing instead of
landing on a namesake, which would be a wrong link nobody notices. The
vault-wide fallback for bare names stays because Obsidian writes such links
and existing vaults depend on them; when the rule changed, every rendered link
of a real vault was compared before and after, and none moved. Case-insensitive
throughout, `.md` optional; a dot in a name is not treated as an extension
(`[[v1.2 plan]]`), so the note is tried before the literal file. Markdown links
and images use the same resolver; before, their `../` was discarded.

**Several vaults, each under its name.** Test pages in the owner's real notes
were the mistake that prompted this: they end up in the history and on every
synced device. Vaults are fully separate — links never resolve across them, as
in Obsidian — so each is just another `mount` of the same parts, and the only
shared code is the start page and the stylesheet. The name is always in the URL,
even with a single vault: a sync client configured for `/dav/notes/` keeps
working when a second vault is added. `/` redirects while there is only one.

**`nogit` per vault, stated rather than detected.** The test vault sits inside
this project's repository. Letting the server `git init` there would nest a
repository; letting it commit would write into a history it does not own.
Guessing "is this inside another work tree" would also hit a fresh `./vault`
next to the source, so the operator says it.

**One function names a heading.** A link to a heading works only if link and
heading arrive at the same id. goldmark's ids skip every multi-byte character
("Frühstück" became `frhstck`), and the link side had its own approximation of
them that kept the umlauts, so in a German vault such links led nowhere. Now
`render.Slug` is given to the parser for the headings and applied to the
fragment of every link, and it keeps letters of any script. The index records
each note's heading ids, which is what lets a link to a heading that is not
there be marked instead of failing silently. Block references (`#^id`) lead to
the note: rendered pages carry no block ids yet.

**An embedded note is rendered as itself, then placed.** `![[Note]]` runs the
same renderer on the other note with that note's path, so everything inside it
— links, images, tags — means what it means on its own page; only then is the
HTML put where the wikilink stood. Rendering it "as part of the host" would
have resolved its links from the host's folder, which the link rules make
wrong. A wikilink is an inline node inside a paragraph, and a note's content
cannot be inside a paragraph, so the paragraph is cut in two around it. A
section embed needs no code of its own: a heading with what is below it is
exactly the section that foldable headings already build. Embedding is one
level deep by decision; that keeps pages predictable and rules out loops
without tracking a chain. Embedded headings lose their ids so the host's
anchors stay unambiguous.

**The browser makes the PDF.** The export is a page laid out for paper with
CSS paged media, and "Save as PDF" in the print dialog does the rest. Making
PDFs on the server would mean a headless browser in the image, a few hundred
megabytes for a button; this way it also works from a phone. The price is that
the browser decides what paper can do, and that ruled out page numbers of our
own: they were built with CSS page-margin boxes, including a settable first
page number and a script that left the number off a single page, verified in
Chromium by reading the PDFs back — and then removed, because only Chromium
draws margin boxes. In any other browser the numbers did not appear and the
setting did nothing, which is worse than not offering it. The print dialog's
own headers and footers number pages everywhere. Should numbers of our own
matter one day, the server has to make the PDF. Slides are a separate task.

**A note and a folder may share a name, not an address.** A note's URL drops
its `.md`, so `Daily.md` and `Daily/` both wanted `/Daily`, and the folder won:
every link to such a note showed a folder listing. Found when the PDF check
printed a listing instead of a note. The note now has the URL, because links
lead to notes; folder URLs end in a slash.

**Keyboard support moves the real focus.** A Vim-like "cursor" could have been a
highlight of our own; instead `j`/`k` on a list and `]]`/`[[` in a note move
the browser's focus, to links and to the `<summary>` that folds a heading's
section. That costs nothing and gives the rest for free: Enter opens or folds,
the focus ring shows the place, a screen reader announces it, and the keys
compose with Tab instead of fighting it. For links inside running text there is
no natural order to walk, so `f` labels them, as Vimium does. Single-key
shortcuts are a known accessibility hazard (dictation types them, screen
readers have their own), hence the switch WCAG 2.1 asks for, reachable from
the footer. The list of keys lives once, in the script; the help is made from
it.

**Private, single user.** One basic-auth login in front of everything. This is
what makes it acceptable to skip per-note publishing rules. Even so, raw HTML is
off and attachments are sandboxed, because notes and files get pasted in from
anywhere and the origin carries the owner's credentials.

**Excalidraw drawings are shown through the plugin's own export.** The Obsidian
plugin can write `Name.excalidraw.svg` beside every drawing, produced by
Excalidraw itself. Showing that file is exact (fonts, embedded images, LaTeX,
dark variant), needs no script in the page, and is a page of code. The cost is
one plugin setting, and that a drawing never opened since shows a notice
instead of a picture. Rendering scenes ourselves was examined first:

- `@excalidraw/utils@0.1.5` (current) renders correctly but is a 19.6 MB script
  plus a 23 MB font — per page with a drawing, on a phone. See its file list:
  `curl -s 'https://data.jsdelivr.com/v1/packages/npm/@excalidraw/utils@0.1.5?structure=flat'`.
- `@excalidraw/utils@0.1.2` (2021) is 1.5 MB and runs, but draws a current scene
  wrong: no rounded corners, serif instead of Excalifont, clipped text. Verified
  by rendering a current-format scene with it in headless Chromium.
- A renderer of our own (rough.js) would approximate the look forever and is
  the "homemade version" this project avoids.

The fixture pictures in `testdata/vault/drawings` were made once with the
current library from the scene in `Sketch.excalidraw.md`, so they are what the
plugin would export.

**Search scans the files on every query.** No inverted index, no copy of the
text in memory: nothing that can go stale, and text edited on disk is found at
once. A search library (Bleve, SQLite FTS) would be a database beside the plain
files; search in the browser would ship the whole vault to a phone. The scan
runs on all cores and works on each note's whole text at once.

Measured on generated vaults of ~5 KB notes, 10 runs after 2 warm-up, 8 cores,
files in the page cache, whole request including rendering the page:

| Vault | One word | Two words / phrase | No match | `tag:` + word | One letter (everything matches) |
|---|---|---|---|---|---|
| 1,000 notes, 5 MB | 25 ms | 25 ms | 23 ms | 3 ms | 44 ms |
| 5,000 notes, 26 MB | 48 ms | 46 ms | 43 ms | 3 ms | 66 ms |

The first version was estimated at "20–50 ms for 2,000 notes" and measured at
100 ms per 1,000 notes, and 10 s for a one-letter query on the large vault. The
cost was not reading the files: it was lowercasing line by line, a whole-word
check that converted strings to runes for every occurrence, and building
snippets for every match instead of the hundred shown. If a vault outgrows
this, keep each note's lowercased text in the index; only `search.go` changes.

The percentage shown with a result is the score on a fixed scale, 100% being
what a note named exactly the search term scores (per term, so two words are
not held to a higher standard than one). Signals add up and the number is
capped, so several notes can show 100%; they are still ordered by the uncapped
score. First written up as "100% means named exactly that", which the first
search in a real vault disproved: three notes with the word in title, tag and
text showed 100%. Making the best hit 100% was the alternative; it
would call the top result of a hopeless search a perfect match.

**Task search copies Obsidian's operators** (`task:`, `task-todo:`,
`task-done:`) instead of inventing syntax, including its rule that any mark but
a space means "not open". Tasks are found by reading the lines, like the rest
of the search; nothing about tasks is kept in the index. A pure task query has
no match quality to speak of, so it shows a count and orders by it.

**The search history is a cookie.** Without a database the server has nowhere
to keep it, and should not grow a place for the sake of eight strings. The
server reads and writes the cookie itself (HttpOnly, no script), scoped to the
vault's path. The price: it is per browser. Only searches that found something
are kept, so a typo does not push a useful entry out. Clearing is a POST,
because a link would be followed by a prefetching browser.

The list under the search box is our own, not a `<datalist>`. That was the
first version: the browser filters it by what is typed, shows it only in some
situations, and draws it differently everywhere. The replacement is a list
inside the form, opened by `:focus-within`, still without script.

Ranking is by where a term occurs, not how often: title, tag, property, heading,
path, text, with text hits capped. Property values come from the front matter
the index has parsed anyway; keys are not searched, since "created" or "source"
would find every note. It needs no corpus statistics, and it is
explainable to the person wondering why a note came first.

**The graph view is drawn by a library, in the browser.** Layout, zoom, pan and
drag come from `force-graph` (canvas, MIT, 57 KB compressed, version pinned in
`graph.html`); a force simulation is not something to write by hand. It is the
second script loaded from jsDelivr, after Mermaid, and only on the graph page.
The server sends the whole graph — notes, linked attachments, missing notes,
tags — and the page filters, so toggling a filter needs no request. Two things
the library does are replaced: its zoom-to-fit (ours leaves out the strip under
the panel, ignores unlinked notes, and stops zooming in on a tiny vault), and
unlimited repulsion (which flings unlinked notes far away).

**Mermaid renders in the browser.** Server-side rendering needs a headless
browser in the image. The page currently loads mermaid.js from jsDelivr, which
is the one request leaving the server; embedding the script is the fix if that
matters.

## Known gaps

- Not yet exercised with Remotely Save, Finder, or any iOS client.
- The Docker image has not been built.
- One login for all vaults. Per-vault logins would be the first step towards
  multiple users, which is out of scope.
- A renamed note leaves links in other notes unresolved, exactly as the files
  say. Obsidian rewrites them on rename; the server never edits notes.

## Planned: Bases

`.base` files are YAML describing filters, formulas and views over note
properties — all of which the index already holds. The work is the expression
language inside filters and formulas (`file.hasTag("x")`, arithmetic, dates).
Plan: start with a subset (and/or/not, comparisons on properties and `file.*`,
table view with sort and limit), render a `.base` like a note, and grow from
real `.base` files in the vault rather than from the full specification.
