# Editor & IDE Setup

Configure your editor to recognize `.nguyen` files as Go for syntax highlighting, autocompletion, and formatting.

## VS Code

The project includes `.vscode/settings.json` with pre-configured settings. If you're starting fresh:

### Automatic (Project Config)

Clone the repo and open in VS Code — settings apply automatically.

### Manual Setup

1. Install the [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.Go)
2. Open Settings (JSON) and add:

```json
{
  "files.associations": {
    "*.nguyen": "go"
  },
  "emmet.includeLanguages": {
    "go": "html"
  }
}
```

This gives you:
- Go syntax highlighting in `.nguyen` files
- Emmet HTML expansion in the template section
- gopls autocompletion for Go code in frontmatter

## JetBrains (GoLand / IntelliJ IDEA)

The project includes `.idea/filetypes.xml`. If you need to configure manually:

1. Go to **Settings → Editor → File Types**
2. Find **Go** in the list
3. Click **+** under "Registered Patterns"
4. Add `*.nguyen`
5. Click **OK**

Or add to `.idea/filetypes.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project version="4">
  <component name="FileTypeManager" version="18">
    <extensionMap>
      <mapping ext="nguyen" type="Go" />
    </extensionMap>
  </component>
</project>
```

## Zed

The project includes `.zed/settings.json`. Manual setup:

1. Open **Settings** (`Cmd+,` / `Ctrl+,`)
2. Add to your project settings:

```json
{
  "file_types": {
    "Go": ["*.nguyen"]
  }
}
```

## Vim / Neovim

Copy the included `ftdetect/nguyen.vim`:

```bash
# Neovim
mkdir -p ~/.config/nvim/ftdetect
cp ftdetect/nguyen.vim ~/.config/nvim/ftdetect/

# Vim
mkdir -p ~/.vim/ftdetect
cp ftdetect/nguyen.vim ~/.vim/ftdetect/
```

Contents of `ftdetect/nguyen.vim`:

```vim
au BufRead,BufNewFile *.nguyen set filetype=go
```

For additional HTML support in the template section, add to your config:

```vim
" Enable HTML snippets in .nguyen files
autocmd FileType go if expand('%:e') == 'nguyen' | setlocal omnifunc=htmlcomplete#CompleteTags | endif
```

## Helix

Add to `~/.config/helix/languages.toml`:

```toml
[[language]]
name = "go"
file-types = ["go", "nguyen"]
```

## Sublime Text

1. Open any `.nguyen` file
2. Go to **View → Syntax → Open all with current extension as... → Go**

Or create `Nguyen.sublime-settings` in your Packages/User directory:

```json
{
  "extensions": ["nguyen"]
}
```

## EditorConfig

The project includes `.editorconfig` for consistent formatting across all editors:

```ini
[*.nguyen]
indent_style = space
indent_size = 4
```

Any editor with EditorConfig support (most modern editors) will apply these rules automatically.

## Language Server (gopls)

The `.nguyen` frontmatter is valid Go code. gopls provides:
- Autocompletion for imports and functions
- Type checking
- Go to definition
- Hover documentation

Note: gopls won't understand the HTML template section or `export const` syntax (which is parsed by the Nguyen.go compiler, not the Go compiler). This is expected — the template section is handled by the framework at build time.

## Recommended Extensions

### VS Code
- **Go** (`golang.go`) — Go language support
- **Tailwind CSS IntelliSense** (`bradlc.vscode-tailwindcss`) — CSS class autocompletion

### JetBrains
- **Go** plugin (built-in with GoLand)
- **Tailwind CSS** plugin

## Troubleshooting

### "Unknown file type" warnings
Make sure the file association is set. Restart the editor after changing settings.

### gopls errors in template section
This is normal. gopls only understands the Go frontmatter between `---` delimiters. The HTML template below is processed by the Nguyen.go compiler.

### No autocompletion for `core.*` functions
Ensure `go mod tidy` has been run and the `nguyen.go/pkg/core` package is available. Note that `pkg/core` uses build tag `//go:build js && wasm`, so gopls needs WASM build flags:

```json
{
  "gopls": {
    "build.buildFlags": ["-tags=js,wasm"],
    "build.env": {
      "GOOS": "js",
      "GOARCH": "wasm"
    }
  }
}
```
