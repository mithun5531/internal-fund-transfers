package repository

import (
	"gorm.io/gorm/clause"
)

func forUpdate() clause.Locking {
	return clause.Locking{Strength: "UPDATE"}
}
