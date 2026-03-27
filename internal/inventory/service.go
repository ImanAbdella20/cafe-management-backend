package inventory

import (
	"backend/internal/middleware"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrItemNameRequired = errors.New("item name is required")
var ErrInvalidLowStockThreshold = errors.New("low stock threshold cannot be negative")
var ErrItemNotFound = errors.New("inventory item not found")
var ErrBranchIDRequired = errors.New("branch id is required")
var ErrMovementTypeRequired = errors.New("movement type is required")
var ErrInvalidStockQuantity = errors.New("quantity cannot be zero")
var ErrInsufficientStock = errors.New("insufficient stock")
var ErrUnauthorizedInventoryAccess = errors.New("unauthorized inventory access")
var ErrForbiddenInventoryBranch = errors.New("forbidden inventory branch access")
var ErrPurchaseRequestNotFound = errors.New("purchase request not found")
var ErrPurchaseRequestNotPending = errors.New("purchase request is not pending")
var ErrInvalidMenuItemID = errors.New("menu item id is required")
var ErrInvalidQuantityPerOrder = errors.New("quantity per order must be greater than zero")

var defaultInventoryBranchID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

type Service interface {
	CreateItem(ctx context.Context, req CreateItemRequest) (*InventoryItem, error)
	GetItems(ctx context.Context) ([]InventoryItem, error)
	GetItemByID(ctx context.Context, id uuid.UUID) (*InventoryItemDetails, error)
	UpdateItem(ctx context.Context, id uuid.UUID, req UpdateItemRequest) (*InventoryItem, error)
	DeleteItem(ctx context.Context, id uuid.UUID) error
	AdjustStock(ctx context.Context, itemID uuid.UUID, req AdjustStockRequest) (*StockAdjustmentResult, error)

	CreatePurchaseRequest(ctx context.Context, req CreatePurchaseRequest) (*InventoryPurchaseRequest, error)
	ListPurchaseRequests(ctx context.Context, branchID *uuid.UUID, status string) ([]InventoryPurchaseRequest, error)
	ApprovePurchaseRequest(ctx context.Context, purchaseRequestID uuid.UUID) (*PurchaseApprovalResult, error)

	UpsertMenuItemIngredient(ctx context.Context, req UpsertMenuItemIngredientRequest) error
	DeductIngredients(ctx context.Context, orderID uuid.UUID) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateItem(ctx context.Context, req CreateItemRequest) (*InventoryItem, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, ErrItemNameRequired
	}
	if req.LowStockThreshold < 0 {
		return nil, ErrInvalidLowStockThreshold
	}
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	unit := strings.TrimSpace(req.Unit)
	if unit == "" {
		unit = "unit"
	}

	item := &InventoryItem{
		ID:                uuid.New(),
		Name:              strings.TrimSpace(req.Name),
		Unit:              unit,
		LowStockThreshold: req.LowStockThreshold,
	}

	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *service) GetItems(ctx context.Context) ([]InventoryItem, error) {
	if err := requireInventoryRole(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *service) GetItemByID(ctx context.Context, id uuid.UUID) (*InventoryItemDetails, error) {
	if id == uuid.Nil {
		return nil, ErrItemNotFound
	}
	if err := requireInventoryRole(ctx); err != nil {
		return nil, err
	}

	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	stocks, err := s.repo.ListStocksByItem(ctx, id)
	if err != nil {
		return nil, err
	}

	authUser, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthorizedInventoryAccess
	}
	if strings.EqualFold(authUser.Role, "manager") {
		filtered, filterErr := filterStocksForManager(stocks, authUser.BranchID)
		if filterErr != nil {
			return nil, filterErr
		}
		stocks = filtered
	}

	return &InventoryItemDetails{Item: *item, Stocks: stocks}, nil
}

func (s *service) UpdateItem(ctx context.Context, id uuid.UUID, req UpdateItemRequest) (*InventoryItem, error) {
	if id == uuid.Nil {
		return nil, ErrItemNotFound
	}
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrItemNameRequired
		}
		item.Name = name
	}

	if req.Unit != nil {
		unit := strings.TrimSpace(*req.Unit)
		if unit == "" {
			unit = "unit"
		}
		item.Unit = unit
	}

	if req.LowStockThreshold != nil {
		if *req.LowStockThreshold < 0 {
			return nil, ErrInvalidLowStockThreshold
		}
		item.LowStockThreshold = *req.LowStockThreshold
	}

	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *service) DeleteItem(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrItemNotFound
	}
	if err := requireAdmin(ctx); err != nil {
		return err
	}

	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item == nil {
		return ErrItemNotFound
	}

	return s.repo.Delete(ctx, id)
}

func (s *service) AdjustStock(ctx context.Context, itemID uuid.UUID, req AdjustStockRequest) (*StockAdjustmentResult, error) {
	if itemID == uuid.Nil {
		return nil, ErrItemNotFound
	}
	if strings.TrimSpace(req.MovementType) == "" {
		return nil, ErrMovementTypeRequired
	}
	if req.Quantity == 0 {
		return nil, ErrInvalidStockQuantity
	}

	resolvedBranchID, err := resolveInventoryBranchID(ctx, req.BranchID)
	if err != nil {
		return nil, err
	}

	item, err := s.repo.GetByID(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	actorID, err := actorIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	movement := InventoryMovement{
		ID:           uuid.New(),
		ItemID:       itemID,
		BranchID:     resolvedBranchID,
		Quantity:     req.Quantity,
		MovementType: strings.TrimSpace(req.MovementType),
		ReferenceID:  req.ReferenceID,
		CreatedBy:    actorID,
	}
	if movement.ReferenceID == uuid.Nil {
		movement.ReferenceID = uuid.New()
	}

	var stock *InventoryStock
	err = s.repo.WithTx(ctx, func(txRepo Repository) error {
		updatedStock, applyErr := applyMovement(ctx, txRepo, movement)
		if applyErr != nil {
			return applyErr
		}
		stock = updatedStock
		return nil
	})
	if err != nil {
		return nil, err
	}

	if stock == nil {
		return nil, errors.New("failed to update stock")
	}

	return &StockAdjustmentResult{Stock: *stock, Movement: movement}, nil
}

func (s *service) CreatePurchaseRequest(ctx context.Context, req CreatePurchaseRequest) (*InventoryPurchaseRequest, error) {
	if req.ItemID == uuid.Nil {
		return nil, ErrItemNotFound
	}
	if req.Quantity <= 0 {
		return nil, ErrInvalidStockQuantity
	}

	resolvedBranchID, err := resolveInventoryBranchID(ctx, req.BranchID)
	if err != nil {
		return nil, err
	}

	item, err := s.repo.GetByID(ctx, req.ItemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	requestedBy, err := actorIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	purchase := &InventoryPurchaseRequest{
		ID:          uuid.New(),
		ItemID:      req.ItemID,
		BranchID:    resolvedBranchID,
		Quantity:    req.Quantity,
		Status:      PurchaseStatusPending,
		RequestedBy: requestedBy,
	}

	if err := s.repo.CreatePurchaseRequest(ctx, purchase); err != nil {
		return nil, err
	}

	return purchase, nil
}

func (s *service) ListPurchaseRequests(ctx context.Context, branchID *uuid.UUID, status string) ([]InventoryPurchaseRequest, error) {
	authUser, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthorizedInventoryAccess
	}

	role := strings.ToLower(strings.TrimSpace(authUser.Role))
	switch role {
	case "admin":
		if branchID != nil {
			if err := requireBranchAccess(ctx, *branchID); err != nil {
				return nil, err
			}
		}
	case "manager":
		if branchID != nil {
			if err := requireBranchAccess(ctx, *branchID); err != nil {
				return nil, err
			}
		}

		managerBranchID, err := uuid.Parse(strings.TrimSpace(authUser.BranchID))
		if err == nil && managerBranchID != uuid.Nil {
			if branchID != nil && *branchID != managerBranchID {
				return nil, ErrForbiddenInventoryBranch
			}
			branchID = &managerBranchID
		}
	default:
		return nil, ErrUnauthorizedInventoryAccess
	}

	return s.repo.ListPurchaseRequests(ctx, branchID, status)
}

func (s *service) ApprovePurchaseRequest(ctx context.Context, purchaseRequestID uuid.UUID) (*PurchaseApprovalResult, error) {
	if purchaseRequestID == uuid.Nil {
		return nil, ErrPurchaseRequestNotFound
	}

	approvedBy, err := actorIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var result PurchaseApprovalResult
	err = s.repo.WithTx(ctx, func(txRepo Repository) error {
		purchase, getErr := txRepo.GetPurchaseRequestByIDForUpdate(ctx, purchaseRequestID)
		if getErr != nil {
			return getErr
		}
		if purchase == nil {
			return ErrPurchaseRequestNotFound
		}
		if purchase.Status != PurchaseStatusPending {
			return ErrPurchaseRequestNotPending
		}

		if authErr := requireBranchAccess(ctx, purchase.BranchID); authErr != nil {
			return authErr
		}

		movement := InventoryMovement{
			ID:           uuid.New(),
			ItemID:       purchase.ItemID,
			BranchID:     purchase.BranchID,
			Quantity:     purchase.Quantity,
			MovementType: "purchase_approved",
			ReferenceID:  purchase.ID,
			CreatedBy:    approvedBy,
		}

		stock, applyErr := applyMovement(ctx, txRepo, movement)
		if applyErr != nil {
			return applyErr
		}

		now := time.Now().UTC()
		if setErr := txRepo.SetPurchaseApproved(ctx, purchase.ID, approvedBy, now); setErr != nil {
			return setErr
		}

		purchase.Status = PurchaseStatusApproved
		purchase.ApprovedBy = approvedBy
		purchase.ApprovedAt = &now

		result = PurchaseApprovalResult{
			PurchaseRequest: *purchase,
			Stock:           *stock,
			Movement:        movement,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *service) UpsertMenuItemIngredient(ctx context.Context, req UpsertMenuItemIngredientRequest) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	if req.MenuItemID <= 0 {
		return ErrInvalidMenuItemID
	}
	if req.InventoryItemID == uuid.Nil {
		return ErrItemNotFound
	}
	if req.QuantityPerOrder <= 0 {
		return ErrInvalidQuantityPerOrder
	}

	item, err := s.repo.GetByID(ctx, req.InventoryItemID)
	if err != nil {
		return err
	}
	if item == nil {
		return ErrItemNotFound
	}

	return s.repo.UpsertMenuItemIngredient(ctx, MenuItemIngredient{
		MenuItemID:       req.MenuItemID,
		InventoryItemID:  req.InventoryItemID,
		QuantityPerOrder: req.QuantityPerOrder,
	})
}

func (s *service) DeductIngredients(ctx context.Context, orderID uuid.UUID) error {
	if orderID == uuid.Nil {
		return errors.New("order id is required")
	}

	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		requirements, err := txRepo.GetOrderRequirements(ctx, orderID)
		if err != nil {
			return err
		}

		for _, req := range requirements {
			movement := InventoryMovement{
				ID:           uuid.New(),
				ItemID:       req.ItemID,
				BranchID:     req.BranchID,
				Quantity:     -req.Quantity,
				MovementType: "order_deduction",
				ReferenceID:  orderID,
				CreatedBy:    systemActorID(orderID),
			}

			if _, err := applyMovement(ctx, txRepo, movement); err != nil {
				return err
			}
		}
		return nil
	})
}

func applyMovement(ctx context.Context, repo Repository, movement InventoryMovement) (*InventoryStock, error) {
	if err := repo.InsertMovement(ctx, &movement); err != nil {
		return nil, err
	}

	stock, err := repo.GetStockForUpdate(ctx, movement.ItemID, movement.BranchID)
	if err != nil {
		return nil, err
	}

	if stock == nil {
		if movement.Quantity < 0 {
			return nil, ErrInsufficientStock
		}

		created := &InventoryStock{
			ID:       uuid.New(),
			ItemID:   movement.ItemID,
			BranchID: movement.BranchID,
			Quantity: movement.Quantity,
		}
		if err := repo.CreateStock(ctx, created); err != nil {
			if isInventoryConstraintViolation(err) {
				return nil, ErrInsufficientStock
			}
			return nil, err
		}
		return created, nil
	}

	newQty := stock.Quantity + movement.Quantity
	if newQty < 0 {
		return nil, ErrInsufficientStock
	}

	updatedAt, err := repo.UpdateStockQuantity(ctx, stock.ID, newQty)
	if err != nil {
		if isInventoryConstraintViolation(err) {
			return nil, ErrInsufficientStock
		}
		return nil, err
	}
	stock.Quantity = newQty
	stock.UpdatedAt = updatedAt

	return stock, nil
}

func actorIDFromContext(ctx context.Context) (uuid.UUID, error) {
	authUser, ok := middleware.UserFromContext(ctx)
	if !ok {
		return uuid.Nil, ErrUnauthorizedInventoryAccess
	}
	role := strings.ToLower(strings.TrimSpace(authUser.Role))
	if role != "admin" && role != "manager" {
		return uuid.Nil, ErrUnauthorizedInventoryAccess
	}

	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("user:%d", authUser.UserID))), nil
}

func requireInventoryRole(ctx context.Context) error {
	authUser, ok := middleware.UserFromContext(ctx)
	if !ok {
		return ErrUnauthorizedInventoryAccess
	}
	role := strings.ToLower(strings.TrimSpace(authUser.Role))
	if role != "admin" && role != "manager" {
		return ErrUnauthorizedInventoryAccess
	}
	return nil
}

func requireAdmin(ctx context.Context) error {
	authUser, ok := middleware.UserFromContext(ctx)
	if !ok {
		return ErrUnauthorizedInventoryAccess
	}
	if !strings.EqualFold(strings.TrimSpace(authUser.Role), "admin") {
		return ErrUnauthorizedInventoryAccess
	}
	return nil
}

func requireBranchAccess(ctx context.Context, targetBranch uuid.UUID) error {
	authUser, ok := middleware.UserFromContext(ctx)
	if !ok {
		return ErrUnauthorizedInventoryAccess
	}

	role := strings.ToLower(strings.TrimSpace(authUser.Role))
	if role == "admin" {
		return nil
	}
	if role != "manager" {
		return ErrUnauthorizedInventoryAccess
	}

	managerBranchRaw := strings.TrimSpace(authUser.BranchID)
	if managerBranchRaw == "" {
		return nil
	}

	managerBranch, err := uuid.Parse(managerBranchRaw)
	if err != nil {
		return nil
	}
	if managerBranch != targetBranch {
		return ErrForbiddenInventoryBranch
	}
	return nil
}

func filterStocksForManager(stocks []InventoryStock, managerBranchID string) ([]InventoryStock, error) {
	if strings.TrimSpace(managerBranchID) == "" {
		return stocks, nil
	}
	branchUUID, err := uuid.Parse(strings.TrimSpace(managerBranchID))
	if err != nil {
		return stocks, nil
	}

	filtered := make([]InventoryStock, 0, len(stocks))
	for _, stock := range stocks {
		if stock.BranchID == branchUUID {
			filtered = append(filtered, stock)
		}
	}
	return filtered, nil
}

func systemActorID(orderID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("order-deduction:"+orderID.String()))
}

func resolveInventoryBranchID(ctx context.Context, requestedBranchID uuid.UUID) (uuid.UUID, error) {
	if requestedBranchID != uuid.Nil {
		if err := requireBranchAccess(ctx, requestedBranchID); err != nil {
			return uuid.Nil, err
		}
		return requestedBranchID, nil
	}

	authUser, ok := middleware.UserFromContext(ctx)
	if !ok {
		return uuid.Nil, ErrUnauthorizedInventoryAccess
	}

	authBranchRaw := strings.TrimSpace(authUser.BranchID)
	if authBranchRaw == "" {
		return defaultInventoryBranchID, nil
	}

	authBranchID, err := uuid.Parse(authBranchRaw)
	if err != nil || authBranchID == uuid.Nil {
		return defaultInventoryBranchID, nil
	}

	if err := requireBranchAccess(ctx, authBranchID); err != nil {
		return uuid.Nil, err
	}

	return authBranchID, nil
}
