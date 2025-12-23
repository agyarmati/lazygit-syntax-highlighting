package context

import (
	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/gui/patch_exploring"
	"github.com/jesseduffield/lazygit/pkg/gui/types"
	deadlock "github.com/sasha-s/go-deadlock"
)

type PatchExplorerContext struct {
	*SimpleContext
	*SearchTrait

	state                  *patch_exploring.State
	viewTrait              *ViewTrait
	getIncludedLineIndices func() []int
	c                      *ContextCommon
	mutex                  deadlock.Mutex
	isFocused              bool
}

var (
	_ types.IPatchExplorerContext = (*PatchExplorerContext)(nil)
	_ types.ISearchableContext    = (*PatchExplorerContext)(nil)
)

func NewPatchExplorerContext(
	view *gocui.View,
	windowName string,
	key types.ContextKey,

	getIncludedLineIndices func() []int,

	c *ContextCommon,
) *PatchExplorerContext {
	ctx := &PatchExplorerContext{
		state:                  nil,
		viewTrait:              NewViewTrait(view),
		c:                      c,
		getIncludedLineIndices: getIncludedLineIndices,
		SimpleContext: NewSimpleContext(NewBaseContext(NewBaseContextOpts{
			View:                       view,
			WindowName:                 windowName,
			Key:                        key,
			Kind:                       types.MAIN_CONTEXT,
			Focusable:                  true,
			HighlightOnFocus:           true, // Keep true for proper focus handling
			NeedsRerenderOnWidthChange: types.NEEDS_RERENDER_ON_WIDTH_CHANGE_WHEN_WIDTH_CHANGES,
		})),
		SearchTrait: NewSearchTrait(c),
	}

	ctx.GetView().SetOnSelectItem(ctx.SearchTrait.onSelectItemWrapper(
		func(selectedLineIdx int) error {
			ctx.GetMutex().Lock()
			defer ctx.GetMutex().Unlock()
			ctx.NavigateTo(selectedLineIdx)
			return nil
		}),
	)

	ctx.SetHandleRenderFunc(ctx.OnViewWidthChanged)

	return ctx
}

func (self *PatchExplorerContext) IsPatchExplorerContext() {}

// HandleFocus overrides SimpleContext.HandleFocus to manage gocui's visual highlight
// In LINE mode: enable Highlight for full-width background
// In HUNK/RANGE mode: disable Highlight, use margin indicator instead
func (self *PatchExplorerContext) HandleFocus(opts types.OnFocusOpts) {
	self.isFocused = true
	// Call parent's HandleFocus for all the normal focus handling
	self.SimpleContext.HandleFocus(opts)
	// Update highlight based on current selection mode
	self.updateHighlight()
}

// HandleFocusLost overrides SimpleContext.HandleFocusLost to track focus state
func (self *PatchExplorerContext) HandleFocusLost(opts types.OnFocusLostOpts) {
	self.isFocused = false
	self.GetView().Highlight = false
	self.SimpleContext.HandleFocusLost(opts)
}

// updateHighlight sets gocui's Highlight based on selection mode
// LINE mode uses gocui's native highlight for full-width background
// HUNK/RANGE mode uses margin indicator instead
func (self *PatchExplorerContext) updateHighlight() {
	if self.GetState() == nil {
		self.GetView().Highlight = false
		return
	}
	// In LINE mode, enable gocui's highlight for full-width selection
	// In HUNK/RANGE mode, disable it (we use margin indicator instead)
	self.GetView().Highlight = self.isFocused && self.GetState().SelectingLine()
}

func (self *PatchExplorerContext) GetState() *patch_exploring.State {
	return self.state
}

func (self *PatchExplorerContext) SetState(state *patch_exploring.State) {
	self.state = state
}

func (self *PatchExplorerContext) GetViewTrait() types.IViewTrait {
	return self.viewTrait
}

func (self *PatchExplorerContext) GetIncludedLineIndices() []int {
	return self.getIncludedLineIndices()
}

func (self *PatchExplorerContext) RenderAndFocus() {
	self.setContent()

	self.FocusSelection()
	self.c.Render()
}

func (self *PatchExplorerContext) Render() {
	self.setContent()

	self.c.Render()
}

func (self *PatchExplorerContext) setContent() {
	self.GetView().SetContent(self.GetContentToRender())
}

func (self *PatchExplorerContext) FocusSelection() {
	view := self.GetView()
	state := self.GetState()
	bufferHeight := view.InnerHeight()
	_, origin := view.Origin()
	numLines := view.ViewLinesHeight()

	newOriginY := state.CalculateOrigin(origin, bufferHeight, numLines)

	view.SetOriginY(newOriginY)

	startIdx, endIdx := state.SelectedViewRange()
	// As far as the view is concerned, we are always selecting a range
	view.SetRangeSelectStart(startIdx)
	view.SetCursorY(endIdx - newOriginY)
}

func (self *PatchExplorerContext) GetContentToRender() string {
	if self.GetState() == nil {
		return ""
	}

	// Update gocui highlight mode (LINE mode uses native highlight, HUNK/RANGE use margin indicator)
	self.updateHighlight()

	// Pass view width for full-line backgrounds
	viewWidth := self.GetView().InnerWidth()

	// Only show selection indicator when this view is focused
	return self.GetState().RenderForLineIndices(self.GetIncludedLineIndices(), self.isFocused, viewWidth)
}

func (self *PatchExplorerContext) NavigateTo(selectedLineIdx int) {
	self.GetState().SetLineSelectMode()
	self.GetState().SelectLine(selectedLineIdx)

	self.RenderAndFocus()
}

func (self *PatchExplorerContext) GetMutex() *deadlock.Mutex {
	return &self.mutex
}

func (self *PatchExplorerContext) ModelSearchResults(searchStr string, caseSensitive bool) []gocui.SearchPosition {
	return nil
}

func (self *PatchExplorerContext) OnViewWidthChanged() {
	if state := self.GetState(); state != nil {
		state.OnViewWidthChanged(self.GetView())
		self.setContent()
		self.RenderAndFocus()
	}
}
