package patch

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// SyntaxHighlighter provides syntax highlighting for code using chroma
type SyntaxHighlighter struct {
	lexer chroma.Lexer
	style *chroma.Style
}

// NewSyntaxHighlighter creates a new syntax highlighter for the given filename
func NewSyntaxHighlighter(filename string) *SyntaxHighlighter {
	// Get lexer based on filename
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Use Dracula style (matches user's delta config)
	style := styles.Get("dracula")
	if style == nil {
		style = styles.Fallback
	}

	return &SyntaxHighlighter{
		lexer: lexer,
		style: style,
	}
}

// DiffBackground represents the background color for diff lines
type DiffBackground int

const (
	NoDiffBackground DiffBackground = iota
	AdditionBackground
	DeletionBackground
	SelectedLineBackground // Subtle background for line-by-line selection
)

// HighlightLineWithBackground highlights a line and applies a diff background color
func (h *SyntaxHighlighter) HighlightLineWithBackground(code string, bg DiffBackground) string {
	if h.lexer == nil || code == "" {
		return h.applyBackgroundOnly(code, bg)
	}

	iterator, err := h.lexer.Tokenise(nil, code)
	if err != nil {
		return h.applyBackgroundOnly(code, bg)
	}

	var buf strings.Builder
	tokens := iterator.Tokens()

	for _, token := range tokens {
		tokenStyle := h.style.Get(token.Type)
		text := token.Value

		// Skip newlines in output
		text = strings.TrimSuffix(text, "\n")
		if text == "" {
			continue
		}

		// Build ANSI escape sequence with both foreground and background
		buf.WriteString(h.formatToken(text, tokenStyle, bg))
	}

	return buf.String()
}

// formatToken applies syntax foreground color and diff background color
func (h *SyntaxHighlighter) formatToken(text string, tokenStyle chroma.StyleEntry, bg DiffBackground) string {
	var codes []string

	// Apply foreground color from syntax highlighting
	if tokenStyle.Colour.IsSet() {
		r, g, b := tokenStyle.Colour.Red(), tokenStyle.Colour.Green(), tokenStyle.Colour.Blue()
		codes = append(codes, fmt.Sprintf("38;2;%d;%d;%d", r, g, b))
	}

	// Apply background color from diff
	switch bg {
	case AdditionBackground:
		// Subtle green background (#004d24 = RGB 0, 77, 36)
		codes = append(codes, "48;2;0;77;36")
	case DeletionBackground:
		// Subtle red background (#4d0018 = RGB 77, 0, 24)
		codes = append(codes, "48;2;77;0;24")
	case SelectedLineBackground:
		// Subtle gray background for line selection (like vim cursorline)
		codes = append(codes, "48;2;60;60;60")
	}

	// Apply text decorations
	if tokenStyle.Bold == chroma.Yes {
		codes = append(codes, "1")
	}
	if tokenStyle.Italic == chroma.Yes {
		codes = append(codes, "3")
	}
	if tokenStyle.Underline == chroma.Yes {
		codes = append(codes, "4")
	}

	if len(codes) == 0 {
		return text
	}

	return fmt.Sprintf("\x1b[%sm%s\x1b[0m", strings.Join(codes, ";"), text)
}

// applyBackgroundOnly applies just the diff background without syntax highlighting
func (h *SyntaxHighlighter) applyBackgroundOnly(text string, bg DiffBackground) string {
	var bgCode string
	switch bg {
	case AdditionBackground:
		bgCode = "\x1b[48;2;0;77;36m"
	case DeletionBackground:
		bgCode = "\x1b[48;2;77;0;24m"
	case SelectedLineBackground:
		bgCode = "\x1b[48;2;60;60;60m"
	default:
		return text
	}
	return bgCode + text + "\x1b[0m"
}

// HighlightLine highlights a single line without diff background (for backwards compat)
func (h *SyntaxHighlighter) HighlightLine(code string) string {
	return h.HighlightLineWithBackground(code, NoDiffBackground)
}

// ExtractFilenameFromHeader extracts the filename from a patch header
// Header format: "diff --git a/path/to/file b/path/to/file"
// or "+++ b/path/to/file"
func ExtractFilenameFromHeader(header []string) string {
	for _, line := range header {
		// Try +++ line first (most reliable)
		if strings.HasPrefix(line, "+++ ") {
			// Handle "+++ b/filename" or "+++ /dev/null"
			path := strings.TrimPrefix(line, "+++ ")
			if path == "/dev/null" {
				continue
			}
			// Remove the "b/" prefix if present
			path = strings.TrimPrefix(path, "b/")
			return filepath.Base(path)
		}
	}

	for _, line := range header {
		// Fall back to diff --git line
		if strings.HasPrefix(line, "diff --git ") {
			// Format: "diff --git a/path b/path"
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				// Get the b/path part
				path := parts[len(parts)-1]
				path = strings.TrimPrefix(path, "b/")
				return filepath.Base(path)
			}
		}
	}

	return ""
}
