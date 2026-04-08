// 该文件负责记忆层接入与检索后端声明。
package memory

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) RetrievalBackend() string {
	return "sqlite_fts5+sqlite_vec"
}
