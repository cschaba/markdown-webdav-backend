# Roadmap

What is still open, in the order it is planned. It is drawn from the Status
table in the [README](README.md) and the known gaps in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md); when something here is done, it
moves to [CHANGELOG.md](CHANGELOG.md) and its row in the README says "works".
Before 1.0 the order may change; what is out of scope says why.

## Before 1.0: prove it with real clients

The server is tested with `go test`, a headless browser, `curl` and `rclone`,
and GNOME Files has opened a vault. Nothing has been synced from a real
Obsidian yet, and that is the point of it.

- **Sync from Obsidian with Remotely Save** on Linux, macOS, iPhone and iPad:
  first sync of an existing vault, edits on two devices, renames and deletes,
  attachments, conflicts. Write down the client settings that work.
- **Finder and the iOS Files app** as WebDAV clients. GNOME Files on Linux
  works: browsing and opening notes (2026-09-19); writing from it is still to
  be tried.
- **Build and run the Docker image** from the `Dockerfile` and `compose.yaml`,
  and publish it with each release if it proves useful.
- Fix what those turn up. 1.0 is the version that has synced a real vault for a
  while without losing anything.

## Obsidian syntax still missing

- **Callouts** (`> [!note]`, foldable `> [!tip]-`).
- **`==highlight==`** and **`%%comments%%`** - comments must stay out of the
  page, the search and the statistics.
- **Block references**: `^id` at the end of a block, and `[[Note#^id]]` and
  `![[Note#^id]]` pointing at it.

Each gets a page in the test vault, as every feature so far.

## Search

- **Matches highlighted** in the results and on the note opened from them.
- **Results while typing**, without giving up the page that works without
  script.
- **A note's history in the browser**: the git log of a note, and an old
  version to read. The history is already there; only the view is missing.

## Later

- **Bases** (`.base` files): a subset first - filters with and/or/not and
  comparisons on properties and `file.*`, a table view with sort and limit -
  grown from real `.base` files. The plan is in `docs/ARCHITECTURE.md`.
- **Serving Mermaid and the graph library from this server** instead of a CDN,
  as an option, for a server that should make no outside requests.
- **Logseq**: find out whether its Markdown works well enough to say so.

## Not planned

- **Several users, or a login per vault.** One person, one login, by design.
- **Rewriting links when a note is renamed.** Obsidian does that on the
  client; the server never edits notes.
- **Page numbers of our own in the PDF export** - removed by decision: only
  Chromium drew them. The print dialog's page numbers work everywhere.
- **Embeds more than one level deep** - one level by decision, which is also
  what makes loops impossible.
