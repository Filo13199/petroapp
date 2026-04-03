package models

import (
	"time"

	"github.com/go-playground/validator/v10"
)

type Event struct {
	EventId   string  `json:"event_id" validate:"required"`
	Status    string  `json:"status" validate:"required"`
	StationId string  `json:"station_id" validate:"required"`
	Amount    float64 `json:"amount" validate:"min=0"`
	CreatedAt string  `json:"created_at" validate:"required,isodate"`
}

type TransferRequest struct {
	Events []Event `json:"events"`
}

type TransferResponse struct {
	Inserted   int `json:"inserted"`
	Duplicates int `json:"duplicates"`
	Invalid    int `json:"invalid"`
}

type StationSummary struct {
	StationId           string  `json:"station_id"`
	TotalApprovedAmount float64 `json:"total_approved_amount"`
	EventsCount         int     `json:"events_count"`
}

func ValidateDate(fl validator.FieldLevel) bool {
	_, err := time.Parse("2006-01-02T15:04:05Z", fl.Field().String())
	return err == nil
}
