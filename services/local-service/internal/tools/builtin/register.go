package builtin

import "github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"

func DefaultTools() []tools.Tool {
	return []tools.Tool{
		NewGenerateTextTool(),
		NewReadFileTool(),
		NewWriteFileTool(),
		NewListDirTool(),
		NewExecCommandTool(),
	}
}

func RegisterBuiltinTools(registry *tools.Registry) error {
	for _, tool := range DefaultTools() {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}
