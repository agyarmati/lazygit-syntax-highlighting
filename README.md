# lazygit-syntax-highlighting

A fork of [lazygit](https://github.com/jesseduffield/lazygit) with syntax highlighting in staging mode.

## Features

- **Syntax highlighting** in the staging view using [chroma](https://github.com/alecthomas/chroma)
- **Diff backgrounds** (green for additions, red for deletions)
- **Line selection highlight** (full-width gray background in LINE mode)
- **Margin indicator** (`▌`) for HUNK/RANGE selection mode

## Building

```bash
go build -o lazygit-syntax-highlighting .
```

## Screenshot

The staging view now shows syntax-highlighted code with colored diff backgrounds.

---

Based on [jesseduffield/lazygit](https://github.com/jesseduffield/lazygit)
