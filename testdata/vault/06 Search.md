---
tags: test/search
vessel: dirigible
---
# Search

The box in the header searches this vault. Results are ordered by how well they
match. For [zeppelin](-/search?q=zeppelin) that order must be:

1. [[Zeppelin]], 100% — the note is named after the word
2. [[Airships]], 30% — tagged with it
3. [[History]], 20% — has it in a heading
4. this page, 13% — only mentions the zeppelin, however often: zeppelin, zeppelin, zeppelin, zeppelin, zeppelin

The percentage is a fixed scale, not a comparison with the other results: 100%
is as good as a note named exactly what was searched for. The signals add up. With only `tag:` or `path:`
there is nothing to rank, and no percentage.

## Operators

This page quotes every query below, so it is itself among the results each
time — after the notes that are actually about the words.

- Every word must occur: [zeppelin 1937](-/search?q=zeppelin+1937) finds History, not Zeppelin or Airships.
- A phrase: ["rigid frame"](-/search?q=%22rigid+frame%22) finds Zeppelin first; ["frame rigid"](-/search?q=%22frame+rigid%22) only this page.
- By tag, nested tags included: [tag:test](-/search?q=tag%3Atest) lists the numbered test pages, by title.
- By path: [path:search/](-/search?q=path%3Asearch%2F) lists the three helper notes.
- Combined: [tag:test checkerboard](-/search?q=tag%3Atest+checkerboard) finds page 04 and this one.
- Attachments by file name: [pixel](-/search?q=pixel) finds `pixel.png` first, at 100% — a file's name counts like a note's title — then the notes that embed it.
- Properties: [dirigible](-/search?q=dirigible) finds this page through its `vessel` property, and shows that line. Only values are searched: [vessel](-/search?q=vessel) finds it just because this sentence says the word.
- Not searched: Excalidraw drawings — [versionNonce](-/search?q=versionNonce) finds only this page, never the drawings that contain it.
