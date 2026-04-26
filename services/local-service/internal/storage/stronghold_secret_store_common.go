package storage

type strongholdFilePayload struct {
	Backend string                  `json:"backend"`
	Records map[string]SecretRecord `json:"records"`
}
