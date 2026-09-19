---
tags: test/embeds
---
# Embedded notes

`![[Note]]` shows another note in place, the way `![[image.png]]` shows an
image. The embedded note keeps its own links; its front matter is not shown.

## A whole note

![[embeds/Recipe]]

From this page, in the vault root, `[[Twin]]` is another note: [[Twin]].

## One section

`![[links/Chapters#Notes]]` shows that heading and what is below it, up to the
next heading of the same level:

![[links/Chapters#Notes]]

## Between text

Text before ![[embeds/Twin]] and text after: the paragraph is cut in two around
the embed.

- In a list item: ![[embeds/Twin]]

## A diagram in an embedded note

This page has no diagram of its own, and still loads the script for this one:

![[embeds/Flow]]

## One level deep

`Menu` embeds `Recipe`. Embedded here, that inner embed is a link:

![[embeds/Menu]]

## What stays a link

- a heading the note does not have: ![[links/Chapters#No such heading]]
- a note embedding itself: ![[09 Embedded notes]]
- a note that does not exist is marked missing: ![[No such note]]
- inside emphasis there is no room for a block: *![[embeds/Twin]]*
- and `[[embeds/Recipe]]` without the `!` is a link, as ever: [[embeds/Recipe]]
