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

## History

Each of the links on this page, once followed, shows up under *Recent searches*
on the search page, and in the list that drops down from the search box when it
is clicked, or its magnifier is — newest on top, ten at most, no duplicates,
scrolling once there are more than six. ["frame rigid"](-/search?q=%22frame+rigid%22) and
[versionNonce](-/search?q=versionNonce) find only this page, but they find
something, so they are kept; [xyzzy-nothing](-/search?q=xyzzy-nothing) would be
too, for the same reason. Type something that finds nothing to see it left out.
*Clear history* forgets them all; *Clear search* only empties the box.

## Tasks

Obsidian's three operators. The tasks of this page:

- [ ] inflate the zeppelin
- [x] moor the zeppelin
- [ ] check the ballast

[task-todo:""](-/search?q=task-todo%3A%22%22) lists the notes with open tasks,
this one first with two, then 01 Formatting with one, and shows the tasks
rather than a percentage. [task-done:zeppelin](-/search?q=task-done%3Azeppelin)
finds only the mooring, [task:""](-/search?q=task%3A%22%22) all five tasks of
the vault, [task-todo:ballast tag:test](-/search?q=task-todo%3Aballast+tag%3Atest)
combines. A task written as an example in a code block does not count:

```md
- [ ] an example, not a task
```

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
