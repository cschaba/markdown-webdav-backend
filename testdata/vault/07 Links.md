---
tags: test/links
---
# Where a link leads

What `[[a link]]` means depends on where it stands. Three notes are called
*Twin* — in the vault root, in `links/` and in `links/deep/` — and two images
`dot.png`: red in `links/`, blue in `links/deep/`. The links are written in
[[links/Here]], which stands in `links/`; from here, in the root:

- `[[Twin]]` is the one next to this page: [[Twin]]
- `[[links/Twin]]` follows the path: [[links/Twin]]
- `![[dot.png]]` has no namesake next to this page, so the vault is searched, and the shorter path wins — red: ![[dot.png]]

| Written | Means |
|---|---|
| `[[Name]]` | the note of that name next to the linking note; if there is none, anywhere in the vault, the shortest path winning |
| `[[sub/Name]]` | that path from the linking note's folder; if there is none, from the vault root; if there is none, any file whose path ends like that |
| `[[./Name]]`, `[[../Name]]` | that path from the linking note's folder, and nothing else |
| `[[/folder/Name]]` | that path from the vault root, and nothing else |

The same holds for `![[images]]`, for `[Markdown](links.md)` and for
`![Markdown](images.png)`. Capitals never matter, and `.md` may be left out.
