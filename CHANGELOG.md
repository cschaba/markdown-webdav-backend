# Changelog

What changed for someone who runs the server. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), the numbers
[Semantic Versioning](https://semver.org/). Before 1.0 a minor version may
change flags, URLs or behaviour.

## [Unreleased]

### Added

- The version at the foot of every page, and an About page (`/<vault>/-/about`)
  with a link to the source on GitHub and the vault's statistics: notes,
  drawings, attachments, folders, tags, links and missing links, size and last
  change. The keyboard shortcuts help links to it too.

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

[Unreleased]: https://github.com/cschaba/markdown-webdav-backend/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/cschaba/markdown-webdav-backend/releases/tag/v0.1.0
