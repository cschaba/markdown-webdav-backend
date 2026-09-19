---
title: Feature Test Index
tags: [test, test/index]
status: draft
related: "[[01 Formatting]]"
---
# Pages

Every page of this vault, and what it is there to show. A test fails when a
page cannot be reached from here by following links.

| Page | Shows |
|---|---|
| [[01 Formatting]] | tables, task lists, footnotes, quotes, foldable headings, raw HTML stays inert |
| [[02 Code and Diagrams]] | syntax highlighting, Mermaid |
| [[03 Nested]] | links and backlinks across folders, image and file attachments |
| [[04 Missing attachments]] | links and embeds whose file does not exist, image sizes |
| [[05 Excalidraw]] | drawings shown through the plugin's exported picture, light and dark, and the ones without a picture |
| [[06 Search]] | the search box: ranking, phrases, `tag:`, `path:` and the task operators; brings three helper notes in `search/` |
| [[v1.2 plan]] | a dot in a note name, broken front matter |
| this page | front matter, tags, every way to write a link |

Not a page of the vault: the **graph view**, under *Graph* in the header. For
this vault it must show this page as the hub, the two drawings in a second
colour, `v1.2 plan` in grey (its front matter is broken, so it has no tags), and
every missing file of page 04 as a hollow ring.

# Links

- By name: [[01 Formatting]]
- Case-insensitive: [[01 formatting]]
- With label: [[02 Code and Diagrams|code & diagrams]]
- By partial path: [[sub/03 Nested]]
- To a heading: [[01 Formatting#Tables]]
- Dotted name (not an extension): [[v1.2 plan]]
- Unresolved (must show as plain text): [[Does Not Exist]]
- Markdown link, resolved by name: [nested note](03%20Nested.md)
- External: [Obsidian](https://obsidian.md)

Inline tags: #test/inline and #Überprüfung

Expect under "Backlinks": 01 Formatting, 03 Nested.
