package inventory

import (
	"time"

	"github.com/google/uuid"
)

type InventoryItem struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	Unit              string    `json:"unit"`
	LowStockThreshold float64   `json:"low_stock_threshold"`
	CreatedAt         time.Time `json:"created_at"`
}

type InventoryStock struct {
	ID        uuid.UUID `json:"id"`
	ItemID    uuid.UUID `json:"item_id"`
	BranchID  uuid.UUID `json:"branch_id"`
	Quantity  float64   `json:"quantity"`
	UpdatedAt time.Time `json:"updated_at"`
}

type InventoryMovement struct {
	ID           uuid.UUID `json:"id"`
	ItemID       uuid.UUID `json:"item_id"`
	BranchID     uuid.UUID `json:"branch_id"`
	Quantity     float64   `json:"quantity"`
	MovementType string    `json:"movement_type"`
	ReferenceID  uuid.UUID `json:"reference_id"`
	CreatedBy    uuid.UUID `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

const (
	PurchaseStatusPending  = "pending"
	PurchaseStatusApproved = "approved"
	PurchaseStatusRejected = "rejected"
)

type InventoryPurchaseRequest struct {
	ID          uuid.UUID  `json:"id"`
	ItemID      uuid.UUID  `json:"item_id"`
	BranchID    uuid.UUID  `json:"branch_id"`
	Quantity    float64    `json:"quantity"`
	Status      string     `json:"status"`
	RequestedBy uuid.UUID  `json:"requested_by"`
	ApprovedBy  uuid.UUID  `json:"approved_by"`
	RequestedAt time.Time  `json:"requested_at"`
	ApprovedAt  *time.Time `json:"approved_at"`
}

type MenuItemIngredient struct {
	MenuItemID       int       `json:"menu_item_id"`
	InventoryItemID  uuid.UUID `json:"inventory_item_id"`
	QuantityPerOrder float64   `json:"quantity_per_order"`
}

type InventoryItemDetails struct {
	Item   InventoryItem    `json:"item"`
	Stocks []InventoryStock `json:"stocks"`
}

type StockAdjustmentResult struct {
	Stock    InventoryStock    `json:"stock"`
	Movement InventoryMovement `json:"movement"`
}

type PurchaseApprovalResult struct {
	PurchaseRequest InventoryPurchaseRequest `json:"purchase_request"`
	Stock           InventoryStock           `json:"stock"`
	Movement        InventoryMovement        `json:"movement"`
}

type CreateItemRequest struct {
	Name              string  `json:"name" binding:"required"`
	Unit              string  `json:"unit"`
	LowStockThreshold float64 `json:"low_stock_threshold"`
}

type UpdateItemRequest struct {
	Name              *string  `json:"name"`
	Unit              *string  `json:"unit"`
	LowStockThreshold *float64 `json:"low_stock_threshold"`
}

type AdjustStockRequest struct {
	BranchID     uuid.UUID `json:"branch_id"`
	Quantity     float64   `json:"quantity" binding:"required"`
	MovementType string    `json:"movement_type" binding:"required"`
	ReferenceID  uuid.UUID `json:"reference_id"`
}

type CreatePurchaseRequest struct {
	ItemID   uuid.UUID `json:"item_id" binding:"required"`
	BranchID uuid.UUID `json:"branch_id"`
	Quantity float64   `json:"quantity" binding:"required"`
}

type ApprovePurchaseRequest struct {
	Note string `json:"note"`
}

type UpsertMenuItemIngredientRequest struct {
	MenuItemID       int       `json:"menu_item_id" binding:"required"`
	InventoryItemID  uuid.UUID `json:"inventory_item_id" binding:"required"`
	QuantityPerOrder float64   `json:"quantity_per_order" binding:"required"`
}
