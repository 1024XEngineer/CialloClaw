package runengine

import "testing"

func TestEngineUpdateSettingsMarksModelChangesNextTaskEffective(t *testing.T) {
	engine := NewEngine()
	_, updatedKeys, applyMode, needRestart, err := engine.UpdateSettings(map[string]any{
		"models": map[string]any{
			"provider": "openai",
			"model":    "gpt-4.1-mini",
		},
	})
	if err != nil {
		t.Fatalf("model settings update returned error: %v", err)
	}
	if applyMode != "next_task_effective" || needRestart {
		t.Fatalf("expected model settings update to be next_task_effective, got applyMode=%s needRestart=%v updatedKeys=%+v", applyMode, needRestart, updatedKeys)
	}
}

func TestEngineUpdateSettingsKeepsRestartRequiredWhenLanguageAndModelChangeTogether(t *testing.T) {
	engine := NewEngine()
	_, updatedKeys, applyMode, needRestart, err := engine.UpdateSettings(map[string]any{
		"general": map[string]any{"language": "en-US"},
		"models": map[string]any{
			"provider": "openai",
			"model":    "gpt-4.1-mini",
		},
	})
	if err != nil {
		t.Fatalf("combined settings update returned error: %v", err)
	}
	if applyMode != "restart_required" || !needRestart {
		t.Fatalf("expected language change to keep restart_required precedence, got applyMode=%s needRestart=%v updatedKeys=%+v", applyMode, needRestart, updatedKeys)
	}
}
