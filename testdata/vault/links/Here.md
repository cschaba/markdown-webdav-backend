# Links, seen from `links/`

This page stands in `links/`. Back to [[07 Links]].

## A name

- `[[Twin]]` — the one next to this page, although the root has one too, with the shorter path: [[Twin]]
- `[[00 Index]]` — none next to this page, so it is found in the vault: [[00 Index]]
- `![[dot.png]]` — the red one next to this page: ![[dot.png]]

## A path

- `[[deep/Twin]]` — relative to this page: [[deep/Twin]]
- `[[sub/03 Nested]]` — not below this page, so taken from the vault root: [[sub/03 Nested]]
- `![[deep/dot.png]]` — the blue one: ![[deep/dot.png]]

## `./` and `../`

- `[[./Twin]]`: [[./Twin]]
- `[[../Twin]]` — the one in the root: [[../Twin]]
- `![[./deep/dot.png|16]]`, with a size: ![[./deep/dot.png|16]]
- `[[./00 Index]]` — exists, but not here. A path that says where to look is followed, and nothing else: [[./00 Index]]
- `[[../../Twin]]` — above the vault: [[../../Twin]]

## From the vault root

- `[[/Twin]]`: [[/Twin]]
- `[[/links/deep/Twin]]`: [[/links/deep/Twin]]
- `![[/links/deep/dot.png]]` — blue: ![[/links/deep/dot.png]]
- `[[/deep/Twin]]` — there is no `deep` in the root: [[/deep/Twin]]

## Markdown links and images

- `[relative](deep/Twin.md)`: [relative](deep/Twin.md)
- `[up](../Twin.md)`: [up](../Twin.md)
- `[from the root](/links/Twin.md)`: [from the root](/links/Twin.md)
- `[by name](03%20Nested.md)` — not next to this page, found in the vault: [by name](03%20Nested.md)
- `![blue](deep/dot.png)`: ![blue](deep/dot.png)
- `![gone](../dot.png)` — no `dot.png` in the root: ![gone](../dot.png)
