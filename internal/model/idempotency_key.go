package model

import "time"

type IdempotencyKey struct {
	Key          string    `gorm:"primaryKey;size:64" json:"key"`
	StatusCode   int       `gorm:"not null" json:"status_code"`
	ResponseBody string    `gorm:"type:jsonb" json:"response_body"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	ExpiresAt    time.Time `gorm:"not null;index:idx_idempotency_keys_expires" json:"expires_at"`
}

func (IdempotencyKey) TableName() string {
	return "idempotency_keys"
}
