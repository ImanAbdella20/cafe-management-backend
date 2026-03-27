package orders

import (
	"backend/internal/middleware"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrOrderNotFound = errors.New("order not found")
var ErrOrderStatusRequired = errors.New("order status required")
var ErrOrderStatusInvalid = errors.New("invalid order status")
var ErrOrderTypeRequired = errors.New("order type required")
var ErrOrderBranchRequired = errors.New("order branch is required")
var ErrInvalidQuantity = errors.New("quantity must be greater than zero")
var ErrInvalidMenuItemID = errors.New("menu item id must be greater than zero")
var ErrMenuPriceNotFound = errors.New("active menu price not found")
var ErrUnauthorizedStatusTransition = errors.New("unauthorized status transition")
var ErrInventoryDeductionFailed = errors.New("failed to deduct inventory")

const defaultBranchCode = "BR1"

type InventoryService interface {
	DeductIngredients(ctx context.Context, orderID uuid.UUID) error
}

type noopInventoryService struct{}

func (n noopInventoryService) DeductIngredients(_ context.Context, _ uuid.UUID) error {
	return nil
}

type Service interface {
	CreateOrder(ctx context.Context, cashierID uuid.UUID, branchID uuid.UUID, orderType string) (*Order, error)
	GetOrders(ctx context.Context) ([]Order, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (*OrderWithItems, error)
	AddItem(ctx context.Context, orderID uuid.UUID, menuItemID int, qty int, notes string) error
	UpdateStatus(ctx context.Context, orderID uuid.UUID, status string, userID uuid.UUID) error
	DeleteOrder(ctx context.Context, id uuid.UUID) error
}

type service struct {
	repo             Repository
	inventoryService InventoryService
}

func NewService(repo Repository) Service {
	return &service{repo: repo, inventoryService: noopInventoryService{}}
}

func NewServiceWithInventory(repo Repository, inventoryService InventoryService) Service {
	if inventoryService == nil {
		inventoryService = noopInventoryService{}
	}

	return &service{repo: repo, inventoryService: inventoryService}
}

func (s *service) CreateOrder(ctx context.Context, cashierID uuid.UUID, branchID uuid.UUID, orderType string) (*Order, error) {
	typeValue := strings.TrimSpace(strings.ToLower(orderType))
	if typeValue == "" {
		return nil, ErrOrderTypeRequired
	}
	if branchID == uuid.Nil {
		return nil, ErrOrderBranchRequired
	}

	order := &Order{
		ID:             uuid.New(),
		CashierID:      cashierID,
		BranchID:       branchID,
		OrderType:      typeValue,
		Status:         OrderStatusPending,
		Subtotal:       0,
		TaxAmount:      0,
		DiscountAmount: 0,
		TotalAmount:    0,
	}

	sequenceDate := time.Now().UTC().Format("20060102")
	seq, err := s.repo.NextOrderSequence(ctx, defaultBranchCode, sequenceDate)
	if err != nil {
		return nil, err
	}
	order.OrderNumber = generateOrderNumber(defaultBranchCode, sequenceDate, seq)

	if err := s.repo.CreateOrder(ctx, order); err != nil {
		return nil, err
	}
	_ = s.repo.AddStatusHistory(ctx, order.ID, cashierID, OrderStatusPending)

	return order, nil
}

func (s *service) AddItem(ctx context.Context, orderID uuid.UUID, menuItemID int, qty int, notes string) error {
	if orderID == uuid.Nil {
		return ErrOrderNotFound
	}
	if menuItemID <= 0 {
		return ErrInvalidMenuItemID
	}
	if qty <= 0 {
		return ErrInvalidQuantity
	}

	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		order, err := txRepo.GetOrderByID(ctx, orderID)
		if err != nil {
			return err
		}
		if order == nil {
			return ErrOrderNotFound
		}

		price, err := txRepo.GetCurrentMenuPrice(ctx, menuItemID)
		if err != nil {
			return err
		}
		if price <= 0 {
			return ErrMenuPriceNotFound
		}

		item := &OrderItem{
			ID:         uuid.New(),
			OrderID:    orderID,
			MenuItemID: menuItemID,
			Quantity:   qty,
			UnitPrice:  price,
			TotalPrice: price * float64(qty),
			Notes:      strings.TrimSpace(notes),
		}

		if err := txRepo.AddOrderItem(ctx, item); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalidMenuItemID
			}
			return err
		}

		return nil
	})
}

func (s *service) recalculateTotalsWithRepo(ctx context.Context, repo Repository, orderID uuid.UUID) error {
	subtotal, err := repo.GetOrderSubtotal(ctx, orderID)
	if err != nil {
		return err
	}

	discount := calculateDiscount(subtotal)
	taxable := subtotal - discount
	if taxable < 0 {
		taxable = 0
	}
	tax := taxable * 0.15
	total := taxable + tax

	order, err := repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return ErrOrderNotFound
	}

	order.Subtotal = subtotal
	order.DiscountAmount = discount
	order.TaxAmount = tax
	order.TotalAmount = total

	return repo.UpdateOrder(ctx, order)
}

func (s *service) UpdateStatus(ctx context.Context, orderID uuid.UUID, status string, userID uuid.UUID) error {
	if orderID == uuid.Nil {
		return ErrOrderNotFound
	}

	nextStatus := strings.TrimSpace(strings.ToLower(status))
	if nextStatus == "" {
		return ErrOrderStatusRequired
	}
	if !isValidStatus(nextStatus) {
		return ErrOrderStatusInvalid
	}

	authUser, ok := middleware.UserFromContext(ctx)
	if !ok {
		return ErrUnauthorizedStatusTransition
	}

	role := strings.ToLower(strings.TrimSpace(authUser.Role))

	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		order, err := txRepo.GetOrderByID(ctx, orderID)
		if err != nil {
			return err
		}
		if order == nil {
			return ErrOrderNotFound
		}

		if !isAllowedTransition(order.Status, nextStatus, role) {
			return ErrUnauthorizedStatusTransition
		}

		if err := txRepo.UpdateStatus(ctx, orderID, nextStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrOrderNotFound
			}
			return err
		}

		if order.Status == OrderStatusReady && nextStatus == OrderStatusCompleted {
			if err := s.inventoryService.DeductIngredients(ctx, orderID); err != nil {
				return fmt.Errorf("%w: %v", ErrInventoryDeductionFailed, err)
			}
		}

		if err := txRepo.AddStatusHistory(ctx, orderID, userID, nextStatus); err != nil {
			return err
		}
		return nil
	})
}

func (s *service) GetOrderByID(ctx context.Context, id uuid.UUID) (*OrderWithItems, error) {
	if id == uuid.Nil {
		return nil, ErrOrderNotFound
	}

	order, err := s.repo.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}

	items, err := s.repo.ListOrderItems(ctx, id)
	if err != nil {
		return nil, err
	}

	return &OrderWithItems{Order: *order, Items: items}, nil
}

func (s *service) GetOrders(ctx context.Context) ([]Order, error) {
	return s.repo.ListOrders(ctx)
}

func (s *service) DeleteOrder(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrOrderNotFound
	}

	if err := s.repo.DeleteOrder(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrderNotFound
		}
		return err
	}

	return nil
}

func isValidStatus(status string) bool {
	switch status {
	case OrderStatusPending, OrderStatusPreparing, OrderStatusReady, OrderStatusCompleted, OrderStatusCancelled:
		return true
	default:
		return false
	}
}

func isAllowedTransition(currentStatus, nextStatus, role string) bool {
	current := strings.ToLower(strings.TrimSpace(currentStatus))
	next := strings.ToLower(strings.TrimSpace(nextStatus))
	r := strings.ToLower(strings.TrimSpace(role))

	switch {
	case current == OrderStatusPending && next == OrderStatusPreparing:
		return r == "barista"
	case current == OrderStatusPreparing && next == OrderStatusReady:
		return r == "barista"
	case current == OrderStatusReady && next == OrderStatusCompleted:
		return r == "cashier"
	case current == OrderStatusPending && next == OrderStatusCancelled:
		return r == "manager" || r == "admin"
	default:
		return false
	}
}

func generateOrderNumber(branchCode, sequenceDate string, sequence int) string {
	return fmt.Sprintf("%s-%s-%05d", strings.ToUpper(strings.TrimSpace(branchCode)), sequenceDate, sequence)
}

func calculateDiscount(subtotal float64) float64 {
	if subtotal <= 0 {
		return 0
	}
	return 0
}
