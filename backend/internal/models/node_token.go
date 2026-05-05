package models

type NodeToken struct {
	ID          string `json:"id"`
	Token       string `json:"token"`       // hashed display: first 4 + "..." + last 4
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   int64  `json:"created_at"`
	LastUsedAt  *int64 `json:"last_used_at"`
}
