package orders

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	OrderStatusPending   = "pending"
	OrderStatusPreparing = "preparing"
	OrderStatusReady     = "ready"
	OrderStatusCompleted = "completed"
	OrderStatusCancelled = "cancelled"
)

type Order struct {
	ID             uuid.UUID `json:"id"`
	OrderNumber    string    `json:"order_number"`
	CashierID      uuid.UUID `json:"cashier_id"`
	BranchID       uuid.UUID `json:"branch_id"`
	OrderType      string    `json:"order_type"`
	Status         string    `json:"status"`
	Subtotal       float64   `json:"subtotal"`
	TaxAmount      float64   `json:"tax_amount"`
	DiscountAmount float64   `json:"discount_amount"`
	TotalAmount    float64   `json:"total_amount"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type OrderItem struct {
	ID         uuid.UUID `json:"id"`
	OrderID    uuid.UUID `json:"order_id"`
	MenuItemID int       `json:"menu_item_id"`
	Quantity   int       `json:"quantity"`
	UnitPrice  float64   `json:"unit_price"`
	TotalPrice float64   `json:"total_price"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateOrderRequest struct {
	OrderType string `json:"order_type"`
}

type AddOrderItemRequest struct {
	MenuItemID int    `json:"menu_item_id" binding:"required"`
	Qty        int    `json:"qty" binding:"required"`
	Notes      string `json:"notes"`
}

func (r *AddOrderItemRequest) UnmarshalJSON(data []byte) error {
	type rawAddOrderItemRequest struct {
		MenuItemID json.RawMessage `json:"menu_item_id"`
		Qty        json.RawMessage `json:"qty"`
		Notes      string          `json:"notes"`
	}

	var raw rawAddOrderItemRequest
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	menuItemID, err := parseFlexibleInt(raw.MenuItemID, "menu_item_id")
	if err != nil {
		return err
	}

	qty, err := parseFlexibleInt(raw.Qty, "qty")
	if err != nil {
		return err
	}

	r.MenuItemID = menuItemID
	r.Qty = qty
	r.Notes = raw.Notes

	return nil
}

func parseFlexibleInt(value json.RawMessage, fieldName string) (int, error) {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" || trimmed == "null" {
		return 0, nil
	}

	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		unquoted, err := strconv.Unquote(trimmed)
		if err != nil {
			return 0, fmt.Errorf("invalid %s", fieldName)
		}
		unquoted = strings.TrimSpace(unquoted)
		if unquoted == "" {
			return 0, nil
		}

		parsed, err := strconv.Atoi(unquoted)
		if err != nil {
			return 0, fmt.Errorf("invalid %s", fieldName)
		}
		return parsed, nil
	}

	var parsed int
	if err := json.Unmarshal(value, &parsed); err != nil {
		return 0, fmt.Errorf("invalid %s", fieldName)
	}

	return parsed, nil
}

type UpdateOrderStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type StatusHistory struct {
	ID        uuid.UUID `json:"id"`
	OrderID   uuid.UUID `json:"order_id"`
	Status    string    `json:"status"`
	ChangedBy uuid.UUID `json:"changed_by"`
	CreatedAt time.Time `json:"created_at"`
}

type OrderWithItems struct {
	Order
	Items []OrderItem `json:"items"`
}
