package user

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidInput = errors.New("invalid input")
var ErrNotFound = errors.New("user not found")
var ErrEmailAlreadyExists = errors.New("email already exists")

var roleSet = map[string]struct{}{
	"admin":   {},
	"manager": {},
	"cashier": {},
	"barista": {},
	"staff":   {},
}

var passwordPattern = regexp.MustCompile(`[A-Za-z]`)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateUser(ctx context.Context, u *User) error {
	if u == nil {
		return ErrInvalidInput
	}

	u.Name = strings.TrimSpace(u.Name)
	u.Email = strings.TrimSpace(strings.ToLower(u.Email))
	u.Role = strings.TrimSpace(strings.ToLower(u.Role))
	u.BranchID = strings.TrimSpace(strings.ToLower(u.BranchID))
	u.Password = strings.TrimSpace(u.Password)

	if u.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if !isValidEmail(u.Email) {
		return fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if !isStrongPassword(u.Password) {
		return fmt.Errorf("%w: password must be at least 8 characters and include letters and numbers", ErrInvalidInput)
	}

	if u.Role == "" {
		u.Role = "staff"
	}
	if !isValidRole(u.Role) {
		return fmt.Errorf("%w: invalid role", ErrInvalidInput)
	}
	if u.BranchID != "" {
		if _, err := uuid.Parse(u.BranchID); err != nil {
			return fmt.Errorf("%w: invalid branch_id", ErrInvalidInput)
		}
	}

	existing, err := s.repo.GetByEmail(ctx, u.Email)
	if err != nil {
		return err
	}
	if existing != nil {
		return ErrEmailAlreadyExists
	}
	u.IsActive = true

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	u.Password = string(hashedPassword)

	if err := s.repo.Create(ctx, u); err != nil {
		if isUniqueEmailViolation(err) {
			return ErrEmailAlreadyExists
		}
		return err
	}

	return nil
}

func (s *Service) ListUsers(ctx context.Context, page, limit int, role string) ([]User, int64, error) {
	role = strings.TrimSpace(strings.ToLower(role))
	if role != "" && !isValidRole(role) {
		return nil, 0, fmt.Errorf("%w: invalid role", ErrInvalidInput)
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	users, total, err := s.repo.ListUsers(ctx, page, limit, role)
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (s *Service) GetUserByID(ctx context.Context, id uint) (*User, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	return user, nil
}

func (s *Service) UpdateStatus(ctx context.Context, id uint, status bool) error {
	if id == 0 {
		return ErrInvalidInput
	}

	err := s.repo.UpdateStatus(ctx, id, status)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func (s *Service) UpdateRole(ctx context.Context, id uint, role string) error {
	role = strings.TrimSpace(strings.ToLower(role))
	if id == 0 {
		return fmt.Errorf("%w: id is required", ErrInvalidInput)
	}
	if !isValidRole(role) {
		return fmt.Errorf("%w: invalid role", ErrInvalidInput)
	}

	err := s.repo.UpdateRole(ctx, id, role)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func (s *Service) AssignShift(ctx context.Context, shift *Shift) error {
	if shift == nil {
		return ErrInvalidInput
	}

	shift.ShiftDate = strings.TrimSpace(shift.ShiftDate)
	shift.StartTime = strings.TrimSpace(shift.StartTime)
	shift.EndTime = strings.TrimSpace(shift.EndTime)

	if shift.UserID == 0 || shift.ShiftDate == "" || shift.StartTime == "" || shift.EndTime == "" {
		return ErrInvalidInput
	}

	user, err := s.repo.GetByID(ctx, shift.UserID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}

	err = s.repo.AssignShift(ctx, shift)
	if isForeignKeyUserViolation(err) {
		return ErrNotFound
	}
	return err
}

func (s *Service) ListShiftsByUserID(ctx context.Context, userID uint) ([]Shift, error) {
	if userID == 0 {
		return nil, ErrInvalidInput
	}
	_, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListShiftsByUserID(ctx, userID)
}

func (s *Service) ListShiftsByDate(ctx context.Context, shiftDate string) ([]Shift, error) {
	shiftDate = strings.TrimSpace(shiftDate)
	if shiftDate == "" {
		return nil, fmt.Errorf("%w: shift_date is required", ErrInvalidInput)
	}
	return s.repo.ListShiftsByDate(ctx, shiftDate)
}

func Roles() []string {
	return []string{"admin", "manager", "cashier", "barista", "staff"}
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func isStrongPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	hasLetter := passwordPattern.MatchString(password)
	hasDigit := strings.ContainsAny(password, "0123456789")
	return hasLetter && hasDigit
}

func isValidRole(role string) bool {
	_, ok := roleSet[role]
	return ok
}

func isUniqueEmailViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && strings.Contains(strings.ToLower(pgErr.Message), "email")
	}
	return false
}

func isForeignKeyUserViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503" && strings.Contains(strings.ToLower(pgErr.Message), "user")
	}
	return false
}
