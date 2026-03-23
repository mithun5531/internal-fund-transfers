package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Transaction struct {
	ID                   uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SourceAccountID      int64     `gorm:"not null;index:idx_transactions_source" json:"source_account_id"`
	DestinationAccountID int64     `gorm:"not null;index:idx_transactions_destination" json:"destination_account_id"`
	Amount               Decimal   `gorm:"type:numeric(30,10);not null" json:"amount"`
	CreatedAt            time.Time `gorm:"autoCreateTime;index:idx_transactions_created_at" json:"created_at"`
}

// BeforeCreate assigns a time-ordered UUID v7 for sequential index inserts.
func (t *Transaction) BeforeCreate(_ *gorm.DB) error {
	if t.ID == (uuid.UUID{}) {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		t.ID = id
	}
	return nil
}

func (Transaction) TableName() string {
	return "transactions"
}
