package main

import "errors"

type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type LayoutRequest struct {
	WorkArea           Rect  `json:"workArea"`
	ChatVisible        bool  `json:"chatVisible"`
	ChatBounds         *Rect `json:"chatBounds"`
	OrbBounds          *Rect `json:"orbBounds"`
	SummaryBounds      Rect  `json:"summaryBounds"`
	CaptionsBounds     Rect  `json:"captionsBounds"`
	SummaryMinWidth    int   `json:"summaryMinWidth"`
	SummaryMinHeight   int   `json:"summaryMinHeight"`
	CaptionsMinWidth   int   `json:"captionsMinWidth"`
	CaptionsMinHeight  int   `json:"captionsMinHeight"`
	Gap                int   `json:"gap"`
	Margin             int   `json:"margin"`
}

type LayoutResponse struct {
	Summary  Rect `json:"summary"`
	Captions Rect `json:"captions"`
}

func computeOverlayLayout(request LayoutRequest) (LayoutResponse, error) {
	if request.WorkArea.Width <= 0 || request.WorkArea.Height <= 0 {
		return LayoutResponse{}, errors.New("invalid work area")
	}

	margin := maxInt(request.Margin, 24)
	gap := maxInt(request.Gap, 16)

	summaryMinWidth := maxInt(request.SummaryMinWidth, 320)
	summaryMinHeight := maxInt(request.SummaryMinHeight, 220)
	captionsMinWidth := maxInt(request.CaptionsMinWidth, 420)
	captionsMinHeight := maxInt(request.CaptionsMinHeight, 260)

	summaryWidth := clampInt(request.SummaryBounds.Width, summaryMinWidth, request.WorkArea.Width-(margin*2))
	captionsWidth := clampInt(request.CaptionsBounds.Width, captionsMinWidth, request.WorkArea.Width-(margin*2))
	summaryHeight := clampInt(request.SummaryBounds.Height, summaryMinHeight, request.WorkArea.Height-(margin*2))
	captionsHeight := clampInt(request.CaptionsBounds.Height, captionsMinHeight, request.WorkArea.Height-(margin*2))

	side := pickOverlaySide(request, margin, maxInt(summaryWidth, captionsWidth))
	availableWidth := request.WorkArea.Width - (margin * 2)
	if request.ChatVisible && request.ChatBounds != nil {
		if side == "left" {
			availableWidth = maxInt(request.ChatBounds.X-request.WorkArea.X-(margin*2), maxInt(summaryMinWidth, captionsMinWidth))
		} else {
			chatRight := request.ChatBounds.X + request.ChatBounds.Width
			availableWidth = maxInt((request.WorkArea.X+request.WorkArea.Width)-chatRight-(margin*2), maxInt(summaryMinWidth, captionsMinWidth))
		}
	}

	summaryWidth = clampInt(summaryWidth, summaryMinWidth, maxInt(summaryMinWidth, availableWidth))
	captionsWidth = clampInt(captionsWidth, captionsMinWidth, maxInt(captionsMinWidth, availableWidth))

	availableHeight := request.WorkArea.Height - (margin * 2)
	if summaryHeight+gap+captionsHeight > availableHeight {
		overflow := summaryHeight + gap + captionsHeight - availableHeight
		reduceCaptions := minInt(overflow, maxInt(captionsHeight-captionsMinHeight, 0))
		captionsHeight -= reduceCaptions
		overflow -= reduceCaptions
		if overflow > 0 {
			reduceSummary := minInt(overflow, maxInt(summaryHeight-summaryMinHeight, 0))
			summaryHeight -= reduceSummary
		}
	}

	summaryY := request.WorkArea.Y + margin
	captionsY := summaryY + summaryHeight + gap
	maxCaptionsHeight := (request.WorkArea.Y + request.WorkArea.Height) - margin - captionsY
	captionsHeight = clampInt(captionsHeight, captionsMinHeight, maxInt(captionsMinHeight, maxCaptionsHeight))

	summaryX := request.WorkArea.X + margin
	captionsX := request.WorkArea.X + margin
	if side == "right" {
		summaryX = request.WorkArea.X + request.WorkArea.Width - margin - summaryWidth
		captionsX = request.WorkArea.X + request.WorkArea.Width - margin - captionsWidth
	}

	return LayoutResponse{
		Summary: clampRect(Rect{
			X:      summaryX,
			Y:      summaryY,
			Width:  summaryWidth,
			Height: summaryHeight,
		}, request.WorkArea),
		Captions: clampRect(Rect{
			X:      captionsX,
			Y:      captionsY,
			Width:  captionsWidth,
			Height: captionsHeight,
		}, request.WorkArea),
	}, nil
}

func pickOverlaySide(request LayoutRequest, margin int, requiredWidth int) string {
	if request.ChatVisible && request.ChatBounds != nil {
		leftSpace := request.ChatBounds.X - request.WorkArea.X - (margin * 2)
		rightSpace := (request.WorkArea.X + request.WorkArea.Width) - (request.ChatBounds.X + request.ChatBounds.Width) - (margin * 2)
		if leftSpace >= requiredWidth && leftSpace >= rightSpace {
			return "left"
		}
		if rightSpace >= requiredWidth {
			return "right"
		}
		if leftSpace >= rightSpace {
			return "left"
		}
		return "right"
	}

	if request.OrbBounds != nil {
		orbCenter := request.OrbBounds.X + (request.OrbBounds.Width / 2)
		workAreaCenter := request.WorkArea.X + (request.WorkArea.Width / 2)
		if orbCenter >= workAreaCenter {
			return "left"
		}
		return "right"
	}

	return "right"
}

func clampRect(bounds Rect, workArea Rect) Rect {
	width := minInt(bounds.Width, workArea.Width-24)
	height := minInt(bounds.Height, workArea.Height-24)
	x := clampInt(bounds.X, workArea.X, workArea.X+workArea.Width-width)
	y := clampInt(bounds.Y, workArea.Y, workArea.Y+workArea.Height-height)
	return Rect{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	}
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func clampInt(value int, minValue int, maxValue int) int {
	if maxValue < minValue {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
