package intercept

import "time"

// InterceptRule represents a traffic interception rule for API/DB storage.
type InterceptRule struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Expression string    `json:"expression"`
	Action     string    `json:"action"` // "block", "drop", "allow"
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// RuleConfig holds rules targeted for specific nodes.
type RuleConfig struct {
	TargetNodes []string        `json:"target_nodes"`
	Rules       []InterceptRule `json:"rules"`
}
