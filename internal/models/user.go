package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Role represents user roles
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// User represents a user in the system
type User struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Email       string             `json:"email" bson:"email"`
	Password    string             `json:"-" bson:"password"` // Hidden in JSON responses
	FirstName   string             `json:"first_name" bson:"first_name"`
	LastName    string             `json:"last_name" bson:"last_name"`
	Role        Role               `json:"role" bson:"role"`
	Permissions []string           `json:"permissions" bson:"permissions"`
	IsActive    bool               `json:"is_active" bson:"is_active"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" bson:"updated_at"`
	LastLoginAt *time.Time         `json:"last_login_at,omitempty" bson:"last_login_at,omitempty"`
}

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required,min=2,max=50"`
	LastName  string `json:"last_name" validate:"required,min=2,max=50"`
	Role      Role   `json:"role" validate:"required,oneof=admin user"`
}

// UpdateUserRequest represents the request to update user information
type UpdateUserRequest struct {
	FirstName   string   `json:"first_name,omitempty" validate:"omitempty,min=2,max=50"`
	LastName    string   `json:"last_name,omitempty" validate:"omitempty,min=2,max=50"`
	Role        Role     `json:"role,omitempty" validate:"omitempty,oneof=admin user"`
	Permissions []string `json:"permissions,omitempty"`
	IsActive    *bool    `json:"is_active,omitempty"`
}

// LoginRequest represents the login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Token     string    `json:"token"`
	User      UserInfo  `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UserInfo represents public user information
type UserInfo struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	Role        Role       `json:"role"`
	Permissions []string   `json:"permissions"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// ChangePasswordRequest represents the change password request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// ToUserInfo converts User to UserInfo (public information)
func (u *User) ToUserInfo() UserInfo {
	return UserInfo{
		ID:          u.ID.Hex(),
		Email:       u.Email,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		Role:        u.Role,
		Permissions: u.Permissions,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt,
		LastLoginAt: u.LastLoginAt,
	}
}

// IsAdmin returns true if user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// HasPermission checks if user has a specific permission
func (u *User) HasPermission(permission string) bool {
	for _, p := range u.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// GetFullName returns the full name of the user
func (u *User) GetFullName() string {
	return u.FirstName + " " + u.LastName
}

// BeforeCreate sets timestamps and default values before creating
func (u *User) BeforeCreate() {
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	u.IsActive = true

	// Set default permissions based on role
	if u.Role == RoleAdmin {
		u.Permissions = []string{
			"workflows:create",
			"workflows:read",
			"workflows:update",
			"workflows:delete",
			"users:create",
			"users:read",
			"users:update",
			"users:delete",
			"logs:read",
		}
	} else {
		u.Permissions = []string{
			"workflows:read",
			"workflows:create",
			"logs:read",
		}
	}
}

// BeforeUpdate sets updated timestamp before updating
func (u *User) BeforeUpdate() {
	u.UpdatedAt = time.Now()
}

// Validate validates user data
func (req *CreateUserRequest) Validate() error {
	// Add custom validation logic here if needed
	// The struct tags handle basic validation
	return nil
}

// Validate validates update request
func (req *UpdateUserRequest) Validate() error {
	// Add custom validation logic here if needed
	return nil
}
