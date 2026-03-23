package dto

type CreateAccountRequest struct {
	AccountID      int64  `json:"account_id" binding:"required,gt=0"`
	InitialBalance string `json:"initial_balance" binding:"required"`
}

type TransferRequest struct {
	SourceAccountID      int64  `json:"source_account_id" binding:"required,gt=0"`
	DestinationAccountID int64  `json:"destination_account_id" binding:"required,gt=0"`
	Amount               string `json:"amount" binding:"required"`
}
