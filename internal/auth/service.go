package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"backend/internal/user"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type tokenClaims struct {
	UserID   uint   `json:"user_id"`
	Role     string `json:"role"`
	BranchID string `json:"branch_id"`
	jwt.RegisteredClaims
}

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already exists")
	ErrInvalidInput       = errors.New("invalid input")
)

type Service struct {
	repo      Repository
	jwtSecret []byte
	tokenTTL  time.Duration
}

func NewService(repo Repository, jwtSecret string, tokenTTL time.Duration) *Service {
	if tokenTTL <= 0 {
		tokenTTL = 24 * time.Hour
	}
	if strings.TrimSpace(jwtSecret) == "" {
		jwtSecret = "dev-secret-key"
	}

	return &Service{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		tokenTTL:  tokenTTL,
	}
}

func (s *Service) Register(ctx context.Context, name, email, password string) (*AuthResponse, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(strings.ToLower(email))
	password = strings.TrimSpace(password)
	if email == "" || len(password) < 8 {
		return nil, ErrInvalidInput
	}

	existing, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	newUser := &user.User{
		Name:     name,
		Email:    email,
		Password: string(hash),
		Role:     "staff",
		IsActive: true,
	}
	if err := s.repo.Create(ctx, newUser); err != nil {
		return nil, err
	}

	token, err := s.generateToken(*newUser)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{Token: token, User: *newUser}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return nil, ErrInvalidInput
	}

	foundUser, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if foundUser == nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(foundUser.Password), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.generateToken(*foundUser)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{Token: token, User: *foundUser}, nil
}

func (s *Service) generateToken(user user.User) (string, error) {
	role := strings.TrimSpace(user.Role)
	if role == "" {
		role = "staff"
	}

	expiresAt := time.Now().Add(s.tokenTTL)
	claims := tokenClaims{
		UserID:   user.ID,
		Role:     role,
		BranchID: strings.TrimSpace(strings.ToLower(user.BranchID)),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
