---
tags: test/links
---
# Block references

`^id` names one block, so that a link can point at it instead of at a whole
note. Back to [[00 Index]].

## Blocks with a name

The id is written at the end of the block's last line. ^claim

- A list item can carry one. ^item
- The one above, not this one.

> A quote, named through the paragraph inside it. ^quoted

A block whose last line cannot hold the id — a table, a code block, a diagram —
is named on the line after it.

| a | b |
|---|---|
| 1 | 2 |

^table

```go
func named() {}
```

^code

## Links to them

- `[[#^claim]]` — in this note: [[#^claim]]
- `[[#^table]]` — a table: [[#^table]]
- `[[#^code]]` — a code block, which has its anchor in front of it: [[#^code]]
- `[[links/Chapters#^para1]]` — in another note: [[links/Chapters#^para1]]
- `[[links/Chapters#^para1|the paragraph]]` — with an alias: [[links/Chapters#^para1|the paragraph]]
- `[[#^CLAIM]]` — the id is not case-sensitive: [[#^CLAIM]]
- `[[#^nosuchblock]]` — no such block: still the note, but marked: [[#^nosuchblock]]

## Embedded blocks

`![[Note#^id]]` shows the block itself, not the whole note:

![[links/Chapters#^para1]]

A named list item keeps its list:

![[links/Chapters#^bullet]]

## Not an id

`2^10`, `a^b`, `^bad_id` and a bare `^` stay text: 2^10, a^b, ^bad_id and ^ .
