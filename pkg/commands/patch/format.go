package patch

import (
	"strings"

	"github.com/jesseduffield/generics/set"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/theme"
	"github.com/samber/lo"
)

type patchPresenter struct {
	patch *Patch
	// if true, all following fields are ignored
	plain bool

	// line indices for tagged lines (e.g. lines added to a custom patch)
	incLineIndices *set.Set[int]

	// syntax highlighter for code content
	highlighter *SyntaxHighlighter

	// selection range for margin indicator (patch line indices, -1 means no selection)
	selectedStartIdx int
	selectedEndIdx   int
	// true if in line select mode (use background highlight instead of margin indicator)
	lineSelectMode bool
	// view width for full-line backgrounds (0 means no padding)
	viewWidth int
}

// formats the patch as a plain string
func formatPlain(patch *Patch) string {
	presenter := &patchPresenter{
		patch:          patch,
		plain:          true,
		incLineIndices: set.New[int](),
	}
	return presenter.format()
}

func formatRangePlain(patch *Patch, startIdx int, endIdx int) string {
	lines := patch.Lines()[startIdx : endIdx+1]
	return strings.Join(
		lo.Map(lines, func(line *PatchLine, _ int) string {
			return line.Content + "\n"
		}),
		"",
	)
}

type FormatViewOpts struct {
	// line indices for tagged lines (e.g. lines added to a custom patch)
	IncLineIndices *set.Set[int]
	// selection range for margin indicator (patch line indices, -1 means no selection)
	SelectedStartIdx int
	SelectedEndIdx   int
	// true if in line select mode (use background highlight instead of margin indicator)
	LineSelectMode bool
	// view width for full-line backgrounds (0 means no padding)
	ViewWidth int
}

// formats the patch for rendering within a view, meaning it's coloured and
// highlights selected items
func formatView(patch *Patch, opts FormatViewOpts) string {
	includedLineIndices := opts.IncLineIndices
	if includedLineIndices == nil {
		includedLineIndices = set.New[int]()
	}

	// Extract filename from header for syntax highlighting
	filename := ExtractFilenameFromHeader(patch.header)
	var highlighter *SyntaxHighlighter
	if filename != "" {
		highlighter = NewSyntaxHighlighter(filename)
	}

	presenter := &patchPresenter{
		patch:            patch,
		plain:            false,
		incLineIndices:   includedLineIndices,
		highlighter:      highlighter,
		selectedStartIdx: opts.SelectedStartIdx,
		selectedEndIdx:   opts.SelectedEndIdx,
		lineSelectMode:   opts.LineSelectMode,
		viewWidth:        opts.ViewWidth,
	}
	return presenter.format()
}

// isLineSelected returns true if the given line index is within the selection range
func (self *patchPresenter) isLineSelected(lineIdx int) bool {
	if self.selectedStartIdx < 0 || self.selectedEndIdx < 0 {
		return false
	}
	return lineIdx >= self.selectedStartIdx && lineIdx <= self.selectedEndIdx
}

// selectionIndicator returns the margin indicator for selected lines (hunk/range mode only)
func (self *patchPresenter) selectionIndicator(lineIdx int) string {
	// Don't add indicator for plain patches (used for git apply)
	if self.plain {
		return ""
	}
	// In line select mode, we use background highlight instead of margin indicator
	if self.lineSelectMode {
		return ""
	}
	if self.isLineSelected(lineIdx) {
		// Cyan vertical bar as selection indicator
		return style.FgCyan.Sprint("▌")
	}
	// Space to maintain alignment when not selected
	return " "
}

func (self *patchPresenter) format() string {
	// if we have no changes in our patch (i.e. no additions or deletions) then
	// the patch is effectively empty and we can return an empty string
	if !self.patch.ContainsChanges() {
		return ""
	}

	stringBuilder := &strings.Builder{}
	lineIdx := 0
	appendLine := func(line string) {
		// Prepend selection indicator
		indicator := self.selectionIndicator(lineIdx)
		_, _ = stringBuilder.WriteString(indicator + line + "\n")

		lineIdx++
	}

	for _, line := range self.patch.header {
		// always passing false for 'included' here because header lines are not part of the patch
		appendLine(self.formatLineAux(line, theme.DefaultTextColor.SetBold(), false, lineIdx))
	}

	for _, hunk := range self.patch.hunks {
		appendLine(
			self.formatLineAux(
				hunk.formatHeaderStart(),
				style.FgCyan,
				false,
				lineIdx,
			) +
				// we're splitting the line into two parts: the diff header and the context
				// We explicitly pass 'included' as false for both because these are not part
				// of the actual patch
				self.formatLineAux(
					hunk.headerContext,
					theme.DefaultTextColor,
					false,
					lineIdx,
				),
		)

		for _, line := range hunk.bodyLines {
			style := self.patchLineStyle(line)
			if line.IsChange() {
				appendLine(self.formatLine(line.Content, style, lineIdx, line.Kind))
			} else {
				appendLine(self.formatLineWithKind(line.Content, style, false, line.Kind, lineIdx))
			}
		}
	}

	return stringBuilder.String()
}

func (self *patchPresenter) patchLineStyle(patchLine *PatchLine) style.TextStyle {
	switch patchLine.Kind {
	case ADDITION:
		return style.FgGreen
	case DELETION:
		return style.FgRed
	default:
		return theme.DefaultTextColor
	}
}

func (self *patchPresenter) formatLine(str string, textStyle style.TextStyle, index int, lineKind PatchLineKind) string {
	included := self.incLineIndices.Includes(index)

	return self.formatLineWithKind(str, textStyle, included, lineKind, index)
}

// 'selected' means you've got it highlighted with your cursor
// 'included' means the line has been included in the patch (only applicable when
// building a patch)
// lineKind is used to determine the background color for diff lines
func (self *patchPresenter) formatLineAux(str string, textStyle style.TextStyle, included bool, lineIdx int) string {
	return self.formatLineWithKind(str, textStyle, included, CONTEXT, lineIdx)
}

func (self *patchPresenter) formatLineWithKind(str string, textStyle style.TextStyle, included bool, lineKind PatchLineKind, lineIdx int) string {
	if self.plain {
		return str
	}

	firstCharStyle := textStyle
	if included {
		firstCharStyle = firstCharStyle.MergeStyle(style.BgGreen)
	}

	var result string
	var bg DiffBackground

	if len(str) < 2 {
		result = firstCharStyle.Sprint(str)
		bg = NoDiffBackground
	} else {
		// Apply syntax highlighting to the code content (after the +/- prefix)
		codeContent := str[1:]

		// Determine background color based on line kind
		switch lineKind {
		case ADDITION:
			bg = AdditionBackground
		case DELETION:
			bg = DeletionBackground
		default:
			bg = NoDiffBackground
		}

		if self.highlighter != nil && codeContent != "" {
			highlightedCode := self.highlighter.HighlightLineWithBackground(codeContent, bg)
			result = firstCharStyle.Sprint(str[:1]) + highlightedCode
		} else {
			result = firstCharStyle.Sprint(str[:1]) + textStyle.Sprint(str[1:])
		}
	}

	// Add padding to extend background to full view width
	result = self.padToWidth(result, bg)

	return result
}

// padToWidth is disabled - width calculation doesn't reliably match gocui's wrapping
func (self *patchPresenter) padToWidth(line string, bg DiffBackground) string {
	return line
}
