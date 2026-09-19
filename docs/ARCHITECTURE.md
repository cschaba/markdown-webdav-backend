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

**Link resolution follows Obsidian:** by base name or trailing path,
case-insensitive, `.md` optional, shortest path wins. A dot in a name is not
treated as an extension (`[[v1.2 plan]]`), so the note is tried before the
literal file.

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
