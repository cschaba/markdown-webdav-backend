---
tags: test/print
---
# Printing and PDF export

*Export to PDF* in the footer of a page opens it laid out for paper: no
navigation, no search box, no backlinks, book margins. There the page size and
whether to include the sub pages can be set; the browser's "Save as PDF" makes
the file, and its "headers and footers" option numbers the pages if wanted.
Printing a page directly (Ctrl+P) gives the same layout on the paper chosen in
the dialog.

The **sub pages** of a note are the notes in the folder named like it. This
note has four, in reading order — numbers count as numbers, so 10 comes after 2,
and a folder's own notes come before its subfolders:

1. [[10 Printing/Chapter 1]]
2. [[10 Printing/Chapter 2]]
3. [[10 Printing/Chapter 10]]
4. [[10 Printing/Appendix/Sources]]

## Page breaks

A line of its own saying `\pagebreak` (or `\newpage`) starts a new page. On
screen it is a dashed line. The next paragraph is on page two:

\pagebreak

This paragraph opens the second page.

The spelling Obsidian's own PDF export understands works as well:

<div style="page-break-after: always;"></div>

This paragraph opens the third page. Inside a sentence, \pagebreak is just
text, and in a code block it is an example:

```
\pagebreak
```
