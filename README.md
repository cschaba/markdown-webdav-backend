# Editable Markdown & WebDAV Server

One small self-hosted server for vaults of Markdown notes:

- **Sync and edit** the notes from Obsidian on laptop, phone and tablet over WebDAV.
- **Read** them in any browser, rendered the way Obsidian shows them.
- **Every change is versioned** in a git repository inside the vault.

Plain files, one Go binary, no database. Private by design: a single login
protects both the WebDAV endpoint and the rendered site.

## Status

An early MVP. What is listed as working has been tested with `go test` and with
`curl` against a running server. **It has not yet been tested against a real
Obsidian client**, and the Docker image has not been built yet — see
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for what that leaves open.

| Goal | State |
|---|---|
| Render Markdown to HTML | works |
| Create and edit from Obsidian on Linux, macOS, iPhone, iPad | WebDAV server works; client setup untested |
| Attachments in pages | works (`![[image.png]]`, `![[image.png\|200]]` sizes, `![](image.png)`, other files as links) |
| Links and attachments whose file is missing | shown as a "missing" marker instead of a dead link or broken image |
| Versioning via git | works (debounced commits) |
| Front matter, tags, wikilinks, backlinks | works |
| Foldable headings, expanded by default | works, without JavaScript |
| Several vaults, switchable in the browser | works |
| Graph view of notes and links, coloured by tag | works, loads the force-graph library from a CDN |
| Mermaid diagrams | works, rendered in the browser; loads mermaid.js from a CDN |
| Syntax highlighting | works |
| Excalidraw drawings | works through the plugin's exported picture, see below |
| Obsidian "Bases" (`.base` files) | not started |
| Note embeds, `![[Note]]` and `![[Note#Heading]]` | works, one level deep |
| Keyboard: Vim-style keys, link hints, help, can be switched off | works |
| Printing and PDF export in book format | works; page numbers come from the print dialog |
| Slide show, and the PDF export's slide format | works |
| Callouts, block references (`#^id`), `==highlight==`, `%%comments%%` | not started |
| Search | works: words, phrases, `tag:`, `path:` and Obsidian's task operators, ranked by how well a note matches |
| Search history | works: the last 10 searches, in a cookie, with a way to clear them |
| Live results while typing, highlighted matches, history view in the browser | not started |
| Logseq | untested |

What the first day of building this cost in time and tokens is recorded in
[docs/EFFORT-2026-09-19.md](docs/EFFORT-2026-09-19.md).

## Run it

Needs Go and git.

```sh
go build
MDWEBDAV_PASSWORD='choose one' ./markdown-webdav-backend -vault notes=./vault
```

- Rendered site: <http://localhost:8080/notes/>
- WebDAV endpoint for sync clients: `http://localhost:8080/dav/notes/`
- Login name: `vault` (change with `-user` or `MDWEBDAV_USER`)

### Several vaults

Repeat `-vault`. Each vault is a directory with a name; the name is where it
lives: `/<name>/` in the browser, `/dav/<name>/` for sync. Pages show a switcher
once there is more than one vault, and `/` lists them.

```sh
./markdown-webdav-backend -vault notes=/srv/notes -vault work=/srv/work
```

`-vault` takes `[name=]directory[,nogit]`. Without a name the directory's own
name is used. `,nogit` turns versioning off for that vault, for files something
else already versions. Names are letters, digits, `-` and `_`; `dav` is taken.

### The test vault

`testdata/vault` has one page per feature: every kind of link, tags, folding,
code, diagrams, attachments, broken front matter. `go test` renders it and checks
the result. To look at it in a browser, next to your own notes:

```sh
./markdown-webdav-backend -vault notes=./vault -vault test=./testdata/vault,nogit
```

`nogit` matters here: the test vault is part of this repository, and the server
must not create a second repository inside it.

With Docker, see [compose.yaml](compose.yaml); it reads the password from a
secret file instead of the environment.

Basic auth sends the password with every request. Anywhere but localhost, put a
TLS-terminating reverse proxy in front — and without one, listen on
`127.0.0.1:8080` rather than the default, which is every interface.

### Security

One password is all that stands in front of the notes, so choose a long random
one. What the server does around it:

- **Guessing is slow.** After 10 wrong passwords within a minute an address is
  refused for five minutes (`429`), the right password included. Behind a
  reverse proxy the address is the last entry of `X-Forwarded-For`, which the
  proxy wrote; it is believed only from a peer on this machine or a private
  network.
- **Nothing synced in runs as a page.** Raw HTML in notes is off. Attachments
  that can carry script are served sandboxed, and so is everything WebDAV
  answers to a browser. Pages carry a Content-Security-Policy that allows our
  own scripts and two from a CDN (Mermaid, the graph library), each pinned to
  one version with a hash the browser checks; no inline script.
- **No way out of the vault.** Files are opened through `os.Root`, by the web
  view, the index and WebDAV alike: `..` and symbolic links that lead out of
  the vault are not followed. `.git` cannot be reached over WebDAV, however it
  is spelled.
- Pages cannot be framed by another site, and a link out of a note does not
  tell the other site where it came from.

| Flag | Environment | Default | |
|---|---|---|---|
| `-listen` | `MDWEBDAV_LISTEN` | `:8080` | address to listen on |
| `-vault` | `MDWEBDAV_VAULT` | `./vault` | vault to serve, see above; repeat the flag, or separate with `;` in the variable |
| `-user` | `MDWEBDAV_USER` | `vault` | login name |
| | `MDWEBDAV_PASSWORD` | | password |
| | `MDWEBDAV_PASSWORD_FILE` | | file holding the password; wins over the above |
| `-commit-after` | | `30s` | commit once a vault was unchanged for this long |
| `-commit-max-wait` | | `5m` | commit at the latest this long after the first change |
| `-no-auth` | | off | no login; only behind something else that authenticates |

## Connecting Obsidian

Obsidian cannot open a WebDAV folder directly, least of all on iOS. The intended
route is the community plugin **Remotely Save**, which syncs a local vault with a
WebDAV server on every platform: point it at `https://your-host/dav/<vault>/`
with the login above. On a desktop the endpoint can also simply be mounted (Finder, GNOME
Files, `rclone`, `davfs2`).

## Keyboard

Every page can be used from the keyboard, with keys that Vim and Vimium users
know. Press `?` for the list; it is also behind *Keyboard shortcuts* at the foot
of every page.

| Key | |
|---|---|
| `/` | focus the search field; there, the arrow keys walk the recent searches |
| `?` | the help |
| `Esc` | leave a field, close the help, cancel a key sequence |
| `j` / `k` | scroll down and up — on a list (a folder, search results, tags): to the next and previous item |
| `d` / `u` | half a page down and up |
| `gg` / `G` | to the top and bottom — on a list: the first and last item |
| `Enter` | open what has the focus |
| `f` / `F` | follow a link: letters appear on every link in view, type them; `F` opens a new tab |
| `]]` / `[[` | to the next and previous heading of a note |
| `J` / `K` | the same, for keyboards where `[` and `]` need AltGr |
| `za` | fold or unfold the section at the heading that has the focus |
| `zM` / `zR` | fold all, unfold all |
| `H` / `L` | back and forward in the browser's history |
| `-` | up, to the folder the page is in |
| `gh` / `gt` / `gr` / `gp` / `gs` | go home, to the tags, to the graph, to the PDF export, to the slide show |
| `Tab` | the browser's own way from link to link |

**Focus is the browser's own.** Moving through a list or to a heading moves the
real focus, shown by a ring, so *Enter* opens it and a screen reader announces
it. A heading is focused through the control that folds its section.

**In the help** the scrolling keys (`j` `k` `d` `u` `gg` `G`) scroll the help,
and every other character does nothing. The same holds in the help of the slide
show.

**A character that is no shortcut does nothing**, on any page and in the slide
show. It is not left to the browser either, which in Firefox would open its own
find bar on `/`, on `'` or, where "search for text when you start typing" is
set, on any letter. Space, the keys with Ctrl, Alt or Meta, and whatever is
typed into a field stay the browser's; with the shortcuts switched off, every
key does.

**Accessibility.** Single-key shortcuts can clash with dictation and with a
screen reader's own keys, so they can be switched off in the help (WCAG 2.1,
2.1.4); the choice is remembered, and the help stays reachable from the footer.
They never act while a field is being typed in, nor with Ctrl, Alt or Meta. The
first Tab on a page reaches a *Skip to content* link; the help is a modal dialog
that keeps the focus and gives it back; scrolling is not animated where the
system asks for reduced motion. None of it needs a mouse, and without
JavaScript the pages work as before, by Tab and Enter.

## Links

`[[Wikilinks]]`, `![[embedded images]]`, `[Markdown](links.md)` and
`![Markdown](images.png)` all find their target by the same rules. What a link
means depends on the folder of the note it stands in:

| Written | Means |
|---|---|
| `[[Name]]` | the note of that name next to the linking note; if there is none, anywhere in the vault, the shortest path winning |
| `[[sub/Name]]` | that path from the linking note's folder; if there is none, from the vault root; if there is none, any file whose path ends like that |
| `[[./Name]]`, `[[../Name]]` | that path from the linking note's folder, and nothing else |
| `[[/folder/Name]]` | that path from the vault root, and nothing else |

Capitals never matter, and `.md` may be left out. As in Obsidian:

| Written | Shows | Leads to |
|---|---|---|
| `[[Name\|alias]]` | alias | the note |
| `[[Name#Heading]]` | Name > Heading | that heading of the note |
| `[[Name#Heading\|alias]]` | alias | that heading |
| `[[#Heading]]` | Heading | a heading of the same note |
| `[[Name#Chapter#Section]]` | Name > Chapter > Section | the last heading named |
| `[[Name#^block]]` | Name > ^block | the note; block ids have no place in the page yet |
| `![[image.png\|200]]`, `![[image.png\|alt text]]` | the image | 200 pixels wide, or with that alt text |

Headings are matched whatever their capitals, umlauts or punctuation
(`[[Name#Maße & Gewichte]]`); of two headings with the same name a link lands on
the first. A link to a heading the note does not have still leads to the note,
with a dashed underline and a tooltip saying so. `[Markdown](Name.md#Heading)`
links behave the same.
A path that says where to look is followed strictly: if nothing is there, the
link is marked missing rather than quietly pointed at a namesake elsewhere. A
bare name does fall back to the whole vault, which is how Obsidian writes links
and what existing notes rely on.

## Embedded notes

`![[Note]]` shows another note in place, the way `![[image.png]]` shows an
image; `![[Note#Heading]]` shows that heading and what is below it, up to the
next heading of the same level. The embed appears in a box under the note's
title, which links to the note. Its front matter is not shown.

- Links and images inside the embedded note are resolved from *its* folder, so
  they lead where they lead on its own page.
- Embedding goes **one level deep**: a note embed inside an embedded note is a
  link, with a tooltip saying so. On that note's own page it is an embed again.
- Headings inside an embed have no anchors, so they cannot collide with the
  page's own; a link to "a heading of this note" inside the embed leads to the
  embedded note's page.
- Diagrams in an embedded note are drawn; the page loads the script once.
- It stays a link when the heading does not exist, when a note embeds itself,
  and where a block has no place (inside emphasis, a table cell, a link text).
  A note that does not exist is marked missing, like any broken link.

An embed counts as a link: it shows up in the backlinks and the graph.

## Slides

A note is a deck of slides if it separates them the way Obsidian's Slides plugin
does: a line of `---` with a blank line before and after it. (Without the blank
line before, `---` makes the line above it a heading; inside a quote or a list
it is a rule. Front matter is no slide.)

*Slide show* in the footer of such a note, or `gs`, opens it: one slide at a
time, filling the window. A slide is laid out at a fixed 16:9 size and scaled as
a whole, so it looks the same in every window and on paper; a slide with too
much on it is made smaller until it fits, not cut off.

| | |
|---|---|
| Next slide | `→` `↓` `PgDn` `Space` `Enter`, `l` `j`, a click on the right half, a swipe, the `›` button |
| Previous slide | `←` `↑` `PgUp` `Backspace` `Shift+Space`, `h` `k`, a click on the left half, a swipe, the `‹` button |
| First and last | `Home` and `End`, `gg` and `G` |
| Full screen | `f`, or the button |
| Leave | `q` or `Esc`, or *Exit* |
| The keys | `?` |

The slide number is in the address (`…#3`), so reloading, the Back button and a
copied link keep the place. Each slide is announced as "Slide 3 of 8", the
slides that are not showing are out of reach of Tab and of a screen reader, and
the letter keys follow the switch in the keyboard help. Without JavaScript the
slides are a page to scroll through. Embedded notes, images, code, tables and
Mermaid diagrams work on slides as they do in a note.

*PDF* in the slide show's bar, or **Format: Slides** on the export page, prints
one slide a page — 16:9, 4:3 or A4 landscape — with sub pages if asked, every
note's slides after the other.

## Printing and PDF export

*Export to PDF* in the footer of a note or folder opens it laid out for paper:
no navigation, search box, tags or backlinks; portrait, book margins, a serif
text face. **Print / Save as PDF** there opens the browser's print dialog —
choose "Save as PDF". The server makes no PDFs itself, so nothing needs
installing, and it works on a phone too.

| Option | |
|---|---|
| Format | Book, or Slides: one slide a page, see above |
| Page size | A4, A5, B5, Letter, Legal, each with margins that suit it; for slides 16:9, 4:3, A4 landscape |
| Include sub pages | see below |

A setting applies as soon as it is chosen; there is no button to press. Both
are remembered for the next export (in a cookie, so per browser and vault).

**Sub pages** of a note are the notes in the folder named like it (`Daily.md`
and `Daily/`, to any depth); for a folder, the notes of the folders below it.
They follow in reading order — numbers count as numbers, so *Chapter 10* comes
after *Chapter 2*, and a folder's own notes come before its subfolders — each on
a new page, after a contents page. A note that does not open with a heading is
given its title.

**A page break** is a line of its own saying `\pagebreak`
or `\newpage`, or the element Obsidian's own PDF export understands,
`<div style="page-break-after: always;"></div>`. On screen it is a dashed line.

Printing a page directly (Ctrl+P) gives the same layout, on the paper chosen in
the dialog. Headings stay with their text; tables, code, images and embeds are
not cut in two where that can be avoided; folded sections are printed unfolded.

**Page numbers** are left to the print dialog: its "headers and footers"
option numbers the pages in every browser. Numbers of our own, with a settable
first page number, were built and removed again — the CSS for them (page-margin
boxes) is drawn by Chromium only, so elsewhere the setting silently did nothing.

A note and a folder of the same name share a name but not an address: the note
is `/vault/Daily`, the folder `/vault/Daily/`.

## Search

The box in the header searches the current vault.

| Query | Finds |
|---|---|
| `garden plan` | notes containing both words, in title, tags, properties, headings or text |
| `"garden plan"` | the exact phrase |
| `tag:project` | only notes with that tag, or one nested below it |
| `path:daily/` | only notes whose folder or file name contains that text |
| `task-todo:""` | notes with open tasks (`- [ ]`), the one with most of them first |
| `task-todo:dentist` | notes with an open task that mentions the word |
| `task-done:""`, `task:""` | the same for completed tasks, and for tasks of either kind |
| `tag:project roof` | operators and words combine |

Capitals do not matter. Results are ordered by how well they match: a note named
after the word comes first, then one tagged with it, then one with it in a
heading, then notes that mention it — more mentions rank higher, up to a cap, so
a long note cannot outrank a title. Each result shows the first lines that
matched, and a percentage for how well it matches. That is a fixed scale, not a
comparison with the other results: 100% is as good as a note named exactly what
you searched for. On its own a word of the title gives around 70%, a tag 30%, a
property 25%, a heading 20%, mentions 5–13%; they add up, so a note with the word in its title,
as a tag and in its text reaches 100% as well. A search whose best hit shows 5% found nothing that is really about
your words. Attachments are found by file name, which counts like a note's title.
Of the properties (front matter) the values are searched — an author, a
description, a date as written — but not their names, which are the same in
every note. Excalidraw drawings are not searched; a drawing is found through the
notes that embed it.

The task operators are Obsidian's own, so a query works in both places. Only
`[ ]` is open; any other mark (`[x]`, `[-]`, `[/]`) counts as completed. A task
search lists the tasks it found and their number instead of a percentage; tasks
in block quotes count, tasks in code blocks do not.

The last ten searches that found something are remembered. Click into the
search box, or on its magnifier, and they drop down below it, latest on top;
the list scrolls when it is longer than fits. They are also listed on the search
page, where *Clear history* forgets them. They are kept in a cookie, so per
browser and per vault — the phone does not know what the laptop searched for —
and nothing is stored on the server. *Clear search* on a result list empties
the current search.

## Graph view

`/<vault>/-/graph`, or *Graph* in the header: every note as a dot, every link as
a line, as in Obsidian. Hover a note to see its neighbours, click to open it,
scroll to zoom, drag to move. The panel filters what is shown — tags,
attachments, notes that are linked but do not exist yet (hollow rings), orphans
— and searches by name.

Notes are coloured by their most common top-level tags, up to eight groups:
four colours as circles, then the same four as diamonds. That is deliberate. In
a graph any two groups can end up side by side, and four is how many colours
stay tellable apart for everyone, including with a colour vision deficiency;
the shape carries the rest. Selecting a group in the legend singles it out.

## Excalidraw

The server shows a drawing through the picture the Obsidian Excalidraw plugin
exports next to it; it does not draw scenes itself. In Obsidian, under
*Settings → Excalidraw → Embedding Excalidraw into your Notes and Exporting*:

- turn on **Auto-export SVG** (PNG works too), and
- optionally **Export both dark- and light-themed image**, so drawings follow
  the browser's dark mode. With a single export, a drawing keeps the background
  it was exported with.

The exports (`Name.excalidraw.svg`, or `.light.svg` / `.dark.svg`) sync like any
other attachment. `![[Name.excalidraw]]` then shows the picture, `|300` scales
it, and the drawing has a page of its own with backlinks. A drawing that was
never exported shows a notice naming this setting rather than a broken image.
Drawings that existed before the setting was turned on are exported the next
time they are opened in Obsidian.

## Versioning

Each vault directory is an ordinary git repository, created on first start. Use
any git tool on it. Changes are committed once the vault has been quiet for
`-commit-after`, so one sync run becomes one commit rather than one per file.
`.git` is not reachable over WebDAV or the website. A `.gitignore` written on
first start keeps client litter (`.DS_Store`, `._*`, `.trash/`, Obsidian
workspace state) out of the history; edit it freely.

If a commit fails, every rendered page shows a warning until one succeeds.
