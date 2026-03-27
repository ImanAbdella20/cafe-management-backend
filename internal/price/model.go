package price

import "time"

type Price struct {
	ID            int       `json:"id"`
	ItemID        int       `json:"item_id"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	EffectiveFrom time.Time `json:"effective_from"`
	IsActive      bool      `json:"is_active"`
}

type CreatePriceRequest struct {
	Amount   float64 `json:"amount" binding:"required"`
	Currency string  `json:"currency"`
}
