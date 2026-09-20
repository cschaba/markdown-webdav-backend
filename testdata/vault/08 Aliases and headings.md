---
tags: test/links
---
# Aliases and headings

As in Obsidian: `|` gives a link another text, `#` points it at a heading. A
`#^` points at a block, which [[15 Block references]] shows.

## Alias

- `[[links/Chapters|the chapters]]`: [[links/Chapters|the chapters]]
- `[[links/Chapters#Notes|alias and heading]]`: [[links/Chapters#Notes|alias and heading]]
- `![[dot.png|the red dot]]` — for an image the alias is its alt text: ![[dot.png|the red dot]]

## Heading

- `[[links/Chapters#Notes]]` — shown as "note > heading": [[links/Chapters#Notes]]
- `[[links/Chapters#Maße & Gewichte]]` — umlauts and punctuation: [[links/Chapters#Maße & Gewichte]]
- `[[links/Chapters#überblick]]` — capitals do not matter: [[links/Chapters#überblick]]
- `[[links/Chapters#Bold and code]]` — markup in the heading is not part of its name: [[links/Chapters#Bold and code]]
- `[[links/Chapters#Notes#A section]]` — a path of headings lands on the last: [[links/Chapters#Notes#A section]]
- `[[links/Chapters#No such heading]]` — still the note, but marked: [[links/Chapters#No such heading]]
- `[[Nowhere#Heading]]` — no such note: [[Nowhere#Heading]]

## In this note

- `[[#Alias]]` — shown as the heading alone: [[#Alias]]
- `[[#Markdown links|further down]]`: [[#Markdown links|further down]]
- `[[#Nothing]]`: [[#Nothing]]

## Markdown links

- `[text](links/Chapters.md#Notes)`: [text](links/Chapters.md#Notes)
- `[umlauts](links/Chapters.md#Ma%C3%9Fe%20%26%20Gewichte)`: [umlauts](links/Chapters.md#Ma%C3%9Fe%20%26%20Gewichte)
- `[in this note](#Alias)`: [in this note](#Alias)
