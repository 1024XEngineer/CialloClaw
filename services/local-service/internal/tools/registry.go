package tools

type Registry struct {
	toolNames []string
}

func NewRegistry() *Registry {
	return &Registry{toolNames: []string{"read_file", "search_web", "run_command"}}
}

func (r *Registry) Names() []string {
	return append([]string(nil), r.toolNames...)
}
