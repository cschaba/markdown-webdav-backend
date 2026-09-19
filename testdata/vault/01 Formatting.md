---
tags: test
---
Text before the first heading stays outside every fold.

# Level 1

Back to [[00 Index]].

## Tables

| Feature | State |
|---|---|
| Table | ok |
| ~~strike~~ | ok |

## Task list

- [x] done
- [ ] open

### Level 3 inside Level 2

Collapsing "Task list" must hide this. Collapsing "Level 1" hides everything below it.

## Footnote and quote

A claim.[^1]

> A block quote.

[^1]: The footnote text.

## Safety

<script>alert("raw HTML must NOT run")</script>

`#notatag` in code must not appear on the Tags page.
