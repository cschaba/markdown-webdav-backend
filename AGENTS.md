# AGENTS.md

A Go server exposing directories of Markdown notes ("vaults") over WebDAV for
Obsidian sync, as a rendered read-only website, with every change committed to
git. Single user, private, no database. Goals and their state: `README.md`.
Design decisions and the reasons: `docs/ARCHITECTURE.md` — read it before
changing how the pieces fit.

Generic cross-project lessons: `~/AI-Memory/README.md`.

## Layout

| Path | Owns |
|---|---|
| `main.go` | flags, auth, and `mount`: per vault, a WebDAV change updates the index and arms the committer |
| `vaults.go` | parsing of `-vault [name=]dir[,nogit]` |
| `testdata/vault` | the test vault: one page per feature, rendered by `internal/web/fixture_test.go` |
| `internal/vault` | `webdav.FileSystem` wrapper: reports changes, hides `.git` |
| `internal/gitlog` | debounced `git commit`, shells out to `git` |
| `internal/render` | goldmark setup; HTML **and** metadata extraction from one parse |
| `internal/index` | in-memory notes, tags, backlinks, Obsidian link resolution |
| `internal/web` | the rendered site, templates and CSS (embedded) |

## Commands

```sh
go build ./... && go vet ./... && go test ./...
gofmt -l .                       # must print nothing

# look at the test vault in a browser: http://127.0.0.1:18080/test/
MDWEBDAV_PASSWORD=secret go run . -listen 127.0.0.1:18080 -vault test=./testdata/vault,nogit

# a throwaway vault for anything that writes (sync, commits)
MDWEBDAV_PASSWORD=secret go run . -listen 127.0.0.1:18080 -vault tmp=/tmp/testvault -commit-after 1s
curl -u vault:secret -T note.md http://127.0.0.1:18080/dav/tmp/note.md
curl -u vault:secret -X PROPFIND -H 'Depth: 1' http://127.0.0.1:18080/dav/tmp/
```

## Conventions that bind

- **Look for a goldmark extension before writing a parser.** Everything
  Obsidian-flavoured so far is an existing extension (`go.abhg.dev/goldmark/*`).
  Check its real API with `go doc`, not from memory.
- **Never find tags or links in a note body with a regex.** Walk the AST in
  `render.parse`; that is what keeps `#tag` inside code blocks from becoming a
  tag. Front matter values are the one exception: they are plain YAML strings.
- **Everything that should produce a backlink goes into `Meta.Links`**
  (wikilinks, wikilinks in front matter, relative Markdown links and images).
  Backlinks are derived from that one list.
- **A link is resolved from the note it stands in**: `Index.Resolve(from,
  target)`, rules in its comment and in the README. Every consumer passes the
  linking note — rendering, backlinks, the graph, drawing exports. A new one
  that passes `""` silently resolves from the vault root and will be right for
  every test whose notes lie in the root.
- **Paths in `index` and `vault` are vault-relative, slash-separated, no leading
  slash.** URLs into a vault are built only through `Index.URL` / `Index.TagURL`,
  which add the vault's prefix (`/<name>`). A hand-built `"/" + path` works in a
  test with an empty prefix and is broken in the real server.
- **A new feature gets a page (or a line on one) in `testdata/vault`**, a row
  in the Pages table of its `00 Index.md`, and an assertion in
  `internal/web/fixture_test.go`. A test fails for a page the index omits. Never put test pages into a real
  vault. Always serve the test vault with `,nogit`: it lies inside this
  repository, and a second `.git` in there would break both.
- **The web view reads files only through `os.Root`** (`web.Handler.root`), which
  is what stops `..` and symlinks from leaving the vault. Do not add `os.Open`.
- **Anything starting with a dot is invisible to the web view and the index;**
  `.git` is additionally invisible to WebDAV. Keep both when adding routes.
- Raw HTML in notes stays disabled, and attachments are served with
  `Content-Security-Policy: sandbox`: synced content must not script the origin
  that holds the owner's login.
- No database and no cache files. State that cannot be rebuilt from the vault
  does not exist.

## Rendering extras

- Links into the vault (`internal/render/links.go`): the library parses
  wikilinks, we render them. `render.parse` resolves every wikilink, Markdown
  link and image — there, not in the node renderer, because only `parse` knows
  which note it is rendering; the result travels on the node. what has no file behind it is replaced by a `missing` node
  and shows as a dashed "missing" marker, never as a dead link or a
  broken-image icon. A missing target still goes into `Meta.Links`.
- Note embeds (`internal/render/embed.go`): `![[Note]]` renders the other note
  with the same renderer, *from the other note's path*, and puts the HTML into
  the host's tree in place of the wikilink. **One level deep, by the owner's
  decision** — do not add recursion; it is also what makes loops impossible.
  The embedded tree is changed before rendering (`prepareEmbedded`): heading
  ids and the Mermaid script come off, and the host adds the script once.
  Anything new that a rendered note brings with an id or a script needs the
  same treatment.
- Heading anchors (`internal/render/headings.go`): `Slug` is the *only* place
  that turns a heading into an id. The parser gets it for the headings, links
  apply it to their fragment. goldmark's default ids drop every non-ASCII
  character, and a second function "approximating" them is how
  `[[Note#Überblick]]` ended up pointing nowhere. `Meta.Headings` lets a link be
  checked against the note it points into.
- Foldable headings (`internal/render/fold.go`) are an AST transformer that
  nests top-level blocks into `<details open>` sections. Anything another
  extension appends to the document for the whole note (footnotes, the Mermaid
  script) must be kept out of the last section there.

- Search (`internal/index/search.go`) reads the note files on every query; there
  is no search index to keep in step. The score constants are an order (title,
  tag, property value, heading, path, text), pinned by `TestSearchRanking`. Anything that makes
  the scan slower shows at once in the timings in `docs/ARCHITECTURE.md`:
  re-measure the same way after touching it, do not estimate.
- Task search (`internal/index/tasks.go`) uses Obsidian's operators and its
  rule that only `[ ]` is open. Keep it compatible: the point is that a query
  typed in one place works in the other.
- Search history (`internal/web/history.go`) is a cookie the server reads and
  writes; there is no script and no server-side state. The dropdown under the
  search box opens through CSS `:focus-within`; the magnifier is the field's
  `<label>`, so a click on it focuses the field. Both choices are for Safari,
  which focuses neither a clicked link nor a clicked button — read the
  comments in `style.css` before "simplifying" them. What only shows on focus
  cannot be screenshotted or tested from the HTML:
  `tools/check-search-dropdown.mjs` drives a real browser (usage in the file). Whatever comes back in
  that cookie is untrusted input. Anything that changes state is a POST.
- Graph view: data from `Index.Graph` (`internal/index/graph.go`), drawn by
  `internal/web/static/graph.js` with the force-graph library. **The four group
  colours are validated, not chosen**: they are the largest subset of the
  palette that passes an all-pairs colour-vision check on both page
  backgrounds. More groups reuse them with a second shape. Do not add a colour
  by eye. Judge any change to the graph from a screenshot, at three sizes: the
  test vault, a generated vault of several hundred notes, and phone width.
- Excalidraw (`internal/index/drawing.go`): drawings are shown through the
  picture the Obsidian plugin exports beside them. The server never decodes or
  draws a scene; read the dead ends in `docs/ARCHITECTURE.md` before changing
  that. `Index.ResolveEmbed` decides what any `![[embed]]` shows.
- **Look at rendered pages, not only at their HTML.** Chromium is enough:
  ```sh
  ./markdown-webdav-backend -no-auth -listen 127.0.0.1:18082 -vault test=./testdata/vault,nogit &
  chromium --headless=new --window-size=820,1500 --virtual-time-budget=8000 \
    --screenshot=/tmp/page.png 'http://127.0.0.1:18082/test/05%20Excalidraw'
  # dark mode: add --force-dark-mode --blink-settings=preferredColorScheme=0
  ```

## Before you believe a test

- **Files edited on disk are not noticed until restart** — that includes
  `testdata/vault`. The index only hears about changes made over WebDAV. The
  page itself shows the new text (it is read per request), but links, tags and
  backlinks are stale. `go test` always sees the current files.
- **curl is not Obsidian.** The WebDAV layer passing curl checks says nothing
  about Remotely Save, Finder or iOS. Claims about client compatibility need the
  client.
- Templates, CSS and the highlight stylesheet are embedded or built at start:
  rebuild and restart before looking at the browser.
- The committer's timers are real time. Its tests sleep; a loaded machine can make
  them flaky, which is not a logic bug.
- `git config --get user.name` succeeds from your global config on a dev machine
  and fails in a container; the fallback identity only shows up there.
- A test page that demonstrates search queries contains those queries, so it
  finds itself. `06 Search.md` says so; do not "fix" the search for it.
- A wikilink that renders as plain text means "did not resolve", not "extension
  broken".
