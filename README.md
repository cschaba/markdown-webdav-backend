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
| Search, history view in the browser | not started |
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
