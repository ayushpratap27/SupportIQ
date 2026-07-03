package services

import (
	"errors"
	"net/http"

	"github.com/ayush/supportiq/internal/config"
	"github.com/ayush/supportiq/internal/dto"
	jwtpkg "github.com/ayush/supportiq/internal/jwt"
	"github.com/ayush/supportiq/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Sentinel errors used by handlers to determine response codes.
var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
)

// AuthService contains all business logic for authentication.
type AuthService struct {
	db  *gorm.DB
	cfg *config.Config
}

// NewAuthService constructs an AuthService via dependency injection.
func NewAuthService(db *gorm.DB, cfg *config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg}
}

// Register validates uniqueness, hashes the password, persists the user,
// and returns a token pair.
func (s *AuthService) Register(req *dto.RegisterRequest) (*dto.AuthResponse, int, error) {
	var existing models.User
	if err := s.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		return nil, http.StatusConflict, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	user := models.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         models.RoleSupportAgent,
		IsActive:     true,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return s.buildAuthResponse(&user)
}

// Login verifies credentials and returns a token pair.
func (s *AuthService) Login(req *dto.LoginRequest) (*dto.AuthResponse, int, error) {
	var user models.User
	if err := s.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Return the same error whether email is wrong or password is wrong
		// to prevent user enumeration.
		return nil, http.StatusUnauthorized, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, http.StatusUnauthorized, ErrInvalidCredentials
	}

	return s.buildAuthResponse(&user)
}

// GetUserByID fetches a user record by primary key and returns a safe DTO.
func (s *AuthService) GetUserByID(id uint) (*dto.UserResponse, int, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, http.StatusNotFound, ErrUserNotFound
	}
	resp := toUserResponse(&user)
	return &resp, http.StatusOK, nil
}

// buildAuthResponse generates a JWT pair and assembles the AuthResponse DTO.
func (s *AuthService) buildAuthResponse(user *models.User) (*dto.AuthResponse, int, error) {
	pair, err := jwtpkg.GenerateTokenPair(
		user.ID,
		user.Email,
		string(user.Role),
		s.cfg.JWTAccessSecret,
		s.cfg.JWTRefreshSecret,
	)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return &dto.AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User:         toUserResponse(user),
	}, http.StatusOK, nil
}

// toUserResponse maps a User model to the safe public DTO.
func toUserResponse(u *models.User) dto.UserResponse {
	return dto.UserResponse{
		ID:       u.ID,
		Name:     u.Name,
		Email:    u.Email,
		Role:     string(u.Role),
		IsActive: u.IsActive,
	}
}
