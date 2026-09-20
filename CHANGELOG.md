# Changelog

What changed for someone who runs the server. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), the numbers
[Semantic Versioning](https://semver.org/). Before 1.0 a minor version may
change flags, URLs or behaviour.

## [Unreleased]

## [0.3.0] - 2026-09-20

The Obsidian syntax that was still missing.

### Added

- **Callouts**: `> [!note]`, with a title of your own, all of Obsidian's types
  and its aliases, and folding with `> [!tip]-` and `> [!tip]+` - as
  `<details>`, so it needs no JavaScript and printing opens it. A type nobody
  knows is still a callout, as in Obsidian. The markup is Obsidian's, so a
  stylesheet written for Obsidian fits.
- **`==highlight==`**.
- **`%%comments%%`**, inline and as a block: they reach neither the page, the
  word count, the tags, the links, the backlinks, the graph nor the search.
  `%%` inside a code span or a fenced block stays text.
- **Block references**: `^id` at the end of a block, or on the line after it
  for a table, a list or a code block. `[[Note#^id]]` leads to that block and
  marks it on arrival, `![[Note#^id]]` shows the block in place of the whole
  note, and a link to a block the note does not have says so, as one to a
  missing heading does.
- Three pages in the test vault for them.

### Changed

- `[[Note#^id]]` used to lead to the note, since pages carried no block ids.

## [0.2.0] - 2026-09-19

What a vault holds, at a glance, and a WebDAV address a file manager can open.

### Added

- The version at the foot of every page, and an About page (`/<vault>/-/about`)
  with a link to the source on GitHub and the vault's statistics: notes,
  drawings, attachments, folders, tags, links and missing links, size and last
  change. The keyboard shortcuts help links to it too.
- A note's statistics - words, characters, reading time, headings, links,
  backlinks, tasks, size, last change - shown and hidden with `i` or
  *Statistics* at the foot of the note.
- `/dav/` lists the vaults as folders, read-only, instead of answering 404: a
  file manager pointed at the server's WebDAV address now finds them.

## [0.1.0] - 2026-09-19

The first numbered version, and the first public one.

### Added

- WebDAV endpoint per vault for syncing from Obsidian; every change is committed
  to a git repository inside the vault, debounced.
- The vaults as a read-only website: Obsidian-flavoured Markdown with wikilinks,
  embeds, tags, front matter, backlinks, attachments, Mermaid diagrams, syntax
  highlighting, Excalidraw exports and foldable headings.
- Several vaults behind one login, a graph view, tag pages and a search with
  phrases, `tag:`, `path:` and task operators, with a history of recent searches.
- Vim-style keyboard support with link hints and a help, which can be switched
  off; a key that is no shortcut is kept from the browser's find bar.
- PDF export in book format, and a note as a slide show.
- One login with throttled password guessing, a Content-Security-Policy, pinned
  and hashed CDN scripts, sandboxed attachments, and file access confined to the
  vault through `os.Root`.
- `-version`, release archives for Linux and macOS, and CI.

[Unreleased]: https://github.com/cschaba/markdown-webdav-backend/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/cschaba/markdown-webdav-backend/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/cschaba/markdown-webdav-backend/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cschaba/markdown-webdav-backend/releases/tag/v0.1.0
