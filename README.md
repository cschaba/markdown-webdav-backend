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
| Callouts, note embeds (`![[Note]]`), `==highlight==`, `%%comments%%` | not started |
| Search | works: words, phrases, `tag:`, `path:` and Obsidian's task operators, ranked by how well a note matches |
| Search history | works: the last 10 searches, in a cookie, with a way to clear them |
| Live results while typing, highlighted matches, history view in the browser | not started |
| Logseq | untested |

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
TLS-terminating reverse proxy in front.

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

Capitals never matter, and `.md` may be left out. `[[Name|label]]` sets the
text, `[[Name#Heading]]` jumps to a heading, `![[image.png|200]]` sets a width.
A path that says where to look is followed strictly: if nothing is there, the
link is marked missing rather than quietly pointed at a namesake elsewhere. A
bare name does fall back to the whole vault, which is how Obsidian writes links
and what existing notes rely on.

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
