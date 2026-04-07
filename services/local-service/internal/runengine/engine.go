package runengine

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) CurrentState() string {
	return "processing"
}

func (e *Engine) CurrentTaskStatus() string {
	return "confirming_intent"
}
