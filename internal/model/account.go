package model

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type Decimal struct {
	decimal.Decimal
}

func NewDecimal(d decimal.Decimal) Decimal {
	return Decimal{Decimal: d}
}

func (d Decimal) Value() (driver.Value, error) {
	return d.Decimal.String(), nil
}

func (d *Decimal) Scan(value interface{}) error {
	if value == nil {
		d.Decimal = decimal.Zero
		return nil
	}

	switch v := value.(type) {
	case string:
		dec, err := decimal.NewFromString(v)
		if err != nil {
			return fmt.Errorf("failed to parse decimal from string %q: %w", v, err)
		}
		d.Decimal = dec
	case []byte:
		dec, err := decimal.NewFromString(string(v))
		if err != nil {
			return fmt.Errorf("failed to parse decimal from bytes: %w", err)
		}
		d.Decimal = dec
	case float64:
		d.Decimal = decimal.NewFromFloat(v)
	case int64:
		d.Decimal = decimal.NewFromInt(v)
	default:
		return fmt.Errorf("unsupported decimal scan type: %T", value)
	}

	return nil
}

type Account struct {
	ID        int64     `gorm:"primaryKey;autoIncrement:false" json:"account_id"`
	Balance   Decimal   `gorm:"type:numeric(30,10);not null;default:0" json:"balance"`
	Version   int64     `gorm:"not null;default:1" json:"-"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Account) TableName() string {
	return "accounts"
}
