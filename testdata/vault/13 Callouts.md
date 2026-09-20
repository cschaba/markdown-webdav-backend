---
tags: test/callouts
---
# Callouts

A block quote whose first line is `> [!type]` is a callout, as in Obsidian.
Back to [[00 Index]].

## The types

> [!note]
> No title given, so the type is the title.

> [!tip] A title of its own
> The title takes the rest of the line, and may hold markup and links:
> see [[01 Formatting]].

> [!warning] Several blocks
> A callout holds whatever a note holds.
>
> - a list
> - `code`
>
> | and | a table |
> |---|---|
> | 1 | 2 |

> [!success] Green
> success, check and done share a colour, an icon and a meaning.

> [!danger] Red
> danger, error, failure and bug are the red ones.

> [!example] Purple
> The last colour of the set.

> [!quote] Grey
> quote and cite have no colour of their own.

## Folding

`-` after the type closes the callout, `+` opens it. Both are `<details>`, so
they fold without JavaScript, and printing opens them.

> [!tip]- Folded, closed
> Only visible once it is opened.

> [!tip]+ Folded, open
> Open to begin with, and can be closed.

## Aliases and unknown types

> [!tldr] An alias
> `tldr` is Obsidian's name for `abstract` and gets its icon.

> [!my-own] A type nobody knows
> Still a callout, with the icon of a note — as in Obsidian.

## Not a callout

> A plain block quote stays a block quote.

> `[!note]` in code, not at the start of the line, is no callout either.
