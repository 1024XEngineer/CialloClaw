package execution

import (
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
)

func workspaceDocumentContent(title, outputText string) string {
	trimmed := strings.TrimSpace(outputText)
	if trimmed == "" {
		trimmed = presentation.Text(presentation.MessageDocumentEmpty, nil)
	}
	if strings.HasPrefix(trimmed, "#") {
		return trimmed + "\n"
	}
	return fmt.Sprintf("# %s\n\n%s\n", firstNonEmpty(strings.TrimSpace(title), presentation.Text(presentation.MessageDocumentDefaultTitle, nil)), trimmed)
}

func previewTextForOutput(outputText, deliveryType string) string {
	preview := truncateText(normalizeWhitespace(outputText), deliveryPreviewMaxLength)
	if preview == "" {
		preview = presentation.Text(presentation.MessagePreviewGenerated, nil)
	}
	if deliveryType == "workspace_document" {
		return presentation.Text(presentation.MessagePreviewWorkspaceGenerated, map[string]string{"preview": preview})
	}
	return preview
}

func previewTextForDeliveryType(deliveryType string) string {
	return presentation.DeliveryPreviewText(deliveryType)
}

func truncateBubbleText(outputText string) string {
	trimmed := strings.TrimSpace(outputText)
	if trimmed == "" {
		return presentation.Text(presentation.MessageBubbleGenerated, nil)
	}
	return truncateText(trimmed, bubbleTextMaxLength)
}

func effectiveDeliveryType(request Request, outputText string) string {
	deliveryType := firstNonEmpty(request.DeliveryType, "workspace_document")
	if shouldPromoteAgentLoopWorkspaceDocument(request, deliveryType, outputText) {
		return "workspace_document"
	}
	return deliveryType
}

func shouldPromoteAgentLoopWorkspaceDocument(request Request, deliveryType, outputText string) bool {
	if strings.TrimSpace(deliveryType) != "bubble" {
		return false
	}
	if effectiveIntentName(request.Intent) != defaultAgentLoopIntentName {
		return false
	}
	if boolValue(request.BudgetDowngrade, "applied") {
		return false
	}
	return len(strings.TrimSpace(outputText)) > bubbleTextMaxLength
}
