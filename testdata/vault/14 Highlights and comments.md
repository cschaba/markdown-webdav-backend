---
tags: test/highlights
---
# Highlights and comments

Two more pieces of Obsidian's Markdown. Back to [[00 Index]].

## Highlight

`==text==` marks a passage: this is ==highlighted==, and so is ==a run with
**bold**, `code` and a [[01 Formatting|link]] in it==.

A lone `=` or `==` is nothing: 2 == 2, and a = b.

## Comment

`%%…%%` hides a note to yourself. Nothing of it reaches the page, the word
count, the tags, the links or the search.

Inline, on one line: before %%a comment holding #hiddentag and [[05 Excalidraw]]%% after.

%%
A comment on lines of its own, opened and closed by `%%` alone on a line.
It holds a word that stands nowhere else in this vault: onlyinacomment.

# It swallows headings, lists and everything else
- including this
%%

A comment is only a comment where a comment can be: in `%%code%%` and in
a fenced block it is text.

```
%%not a comment%%
```

What must hold: the Tags page has `test/highlights` and nothing from the
comments; `05 Excalidraw` does not list this page under "Backlinks", although
a comment names it; and the search finds neither of the two words that only a
comment holds — the tag above, and the long one in the block comment.
