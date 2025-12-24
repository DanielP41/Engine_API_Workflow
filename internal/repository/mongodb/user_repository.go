package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// userRepository implements the UserRepository interface
type userRepository struct {
	collection *mongo.Collection
}

// NewUserRepository creates a new user repository instance
func NewUserRepository(db *mongo.Database) repository.UserRepository {
	collection := db.Collection("users")

	// Create indexes
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Unique index on email
		emailIndex := mongo.IndexModel{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		}

		// Index on role for faster queries
		roleIndex := mongo.IndexModel{
			Keys: bson.D{{Key: "role", Value: 1}},
		}

		// Index on is_active for filtering
		activeIndex := mongo.IndexModel{
			Keys: bson.D{{Key: "is_active", Value: 1}},
		}

		// Compound index for search operations
		searchIndex := mongo.IndexModel{
			Keys: bson.D{
				{Key: "first_name", Value: "text"},
				{Key: "last_name", Value: "text"},
				{Key: "email", Value: "text"},
			},
		}

		indexes := []mongo.IndexModel{emailIndex, roleIndex, activeIndex, searchIndex}

		if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
			// Log error but don't fail application startup
			fmt.Printf("Warning: Failed to create user indexes: %v\n", err)
		}
	}()

	return &userRepository{
		collection: collection,
	}
}

// Create creates a new user - CORREGIDO: retorna (*models.User, error)
func (r *userRepository) Create(ctx context.Context, user *models.User) (*models.User, error) {
	if user == nil {
		return nil, errors.New("user cannot be nil")
	}

	// Note: Password should be hashed by the caller/service layer before being passed here

	// Set timestamps and defaults
	user.BeforeCreate()

	result, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, errors.New("user with this email already exists")
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	user.ID = result.InsertedID.(primitive.ObjectID)
	return user, nil // CAMBIO: retornar el usuario creado
}

// GetByID retrieves a user by ID
func (r *userRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var user models.User

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return &user, nil
}

// GetByEmail retrieves a user by email
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if email == "" {
		return nil, errors.New("email cannot be empty")
	}

	var user models.User

	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}

// Update updates a user
func (r *userRepository) Update(ctx context.Context, id primitive.ObjectID, update *models.UpdateUserRequest) error {
	if update == nil {
		return errors.New("update request cannot be nil")
	}

	// Build update document
	updateDoc := bson.M{
		"updated_at": time.Now(),
	}

	if update.FirstName != "" {
		updateDoc["first_name"] = update.FirstName
	}
	if update.LastName != "" {
		updateDoc["last_name"] = update.LastName
	}
	if update.Role != "" {
		updateDoc["role"] = update.Role
	}
	if update.Permissions != nil {
		updateDoc["permissions"] = update.Permissions
	}
	if update.IsActive != nil {
		updateDoc["is_active"] = *update.IsActive
	}

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": updateDoc},
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	if result.MatchedCount == 0 {
		return errors.New("user not found")
	}

	return nil
}

// Delete deletes a user (soft delete by setting is_active to false)
func (r *userRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"is_active":  false,
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.MatchedCount == 0 {
		return errors.New("user not found")
	}

	return nil
}

// List retrieves a paginated list of users
func (r *userRepository) List(ctx context.Context, page, pageSize int) (*models.UserListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	// Calculate skip
	skip := (page - 1) * pageSize

	// Build filter (only active users)
	filter := bson.M{"is_active": true}

	// Get total count
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// Build find options
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	// Execute query
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer cursor.Close(ctx)

	// Decode results
	var users []models.User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("failed to decode users: %w", err)
	}

	// Calculate total pages
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.UserListResponse{
		Users:      users,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// Search searches users by name or email
func (r *userRepository) Search(ctx context.Context, query string, page, pageSize int) (*models.UserListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	// Calculate skip
	skip := (page - 1) * pageSize

	// Build search filter
	filter := bson.M{
		"is_active": true,
		"$or": []bson.M{
			{"first_name": bson.M{"$regex": query, "$options": "i"}},
			{"last_name": bson.M{"$regex": query, "$options": "i"}},
			{"email": bson.M{"$regex": query, "$options": "i"}},
		},
	}

	// Get total count
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// Build find options
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	// Execute query
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer cursor.Close(ctx)

	// Decode results
	var users []models.User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("failed to decode users: %w", err)
	}

	// Calculate total pages
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.UserListResponse{
		Users:      users,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// ListByRole retrieves users by role
func (r *userRepository) ListByRole(ctx context.Context, role models.Role, page, pageSize int) (*models.UserListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	// Calculate skip
	skip := (page - 1) * pageSize

	// Build filter
	filter := bson.M{
		"is_active": true,
		"role":      role,
	}

	// Get total count
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// Build find options
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	// Execute query
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list users by role: %w", err)
	}
	defer cursor.Close(ctx)

	// Decode results
	var users []models.User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("failed to decode users: %w", err)
	}

	// Calculate total pages
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.UserListResponse{
		Users:      users,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// UpdateLastLogin updates the last login timestamp
func (r *userRepository) UpdateLastLogin(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"last_login_at": now,
			"updated_at":    now,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	if result.MatchedCount == 0 {
		return errors.New("user not found")
	}

	return nil
}

// UpdatePassword updates the user's password
func (r *userRepository) UpdatePassword(ctx context.Context, id primitive.ObjectID, hashedPassword string) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"password":   hashedPassword,
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	if result.MatchedCount == 0 {
		return errors.New("user not found")
	}

	return nil
}

// SetActiveStatus sets the active status of a user
func (r *userRepository) SetActiveStatus(ctx context.Context, id primitive.ObjectID, isActive bool) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"is_active":  isActive,
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to set active status: %w", err)
	}

	if result.MatchedCount == 0 {
		return errors.New("user not found")
	}

	return nil
}

// Count returns the total number of active users
func (r *userRepository) Count(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"is_active": true})
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// CountByRole returns the number of users by role
func (r *userRepository) CountByRole(ctx context.Context, role models.Role) (int64, error) {
	filter := bson.M{
		"is_active": true,
		"role":      role,
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count users by role: %w", err)
	}
	return count, nil
}

// EmailExists checks if an email already exists
func (r *userRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"email": email})
	if err != nil {
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}
	return count > 0, nil
}

// EmailExistsExcludeID checks if an email exists excluding a specific user ID
func (r *userRepository) EmailExistsExcludeID(ctx context.Context, email string, excludeID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"email": email,
		"_id":   bson.M{"$ne": excludeID},
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}
	return count > 0, nil
}

// AGREGADO: Métodos de compatibilidad para auth.go

// GetByIDString método de compatibilidad
func (r *userRepository) GetByIDString(ctx context.Context, id string) (*models.User, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid user ID format")
	}
	return r.GetByID(ctx, objectID)
}

// UpdateLastLoginString método de compatibilidad
func (r *userRepository) UpdateLastLoginString(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid user ID format")
	}
	return r.UpdateLastLogin(ctx, objectID)
}

// CountUsers cuenta el total de usuarios
func (r *userRepository) CountUsers(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}
