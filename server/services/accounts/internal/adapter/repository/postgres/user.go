package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
)

// userDTO is the internal struct for database mapping.
type userDTO struct {
	ID        string    `db:"id"`
	DiscordID string    `db:"discord_id"`
	Name      string    `db:"name"`
	AvatarURL string    `db:"avatar_url"`
	Role      string    `db:"role"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (d *userDTO) toDomain() *model.User {
	return &model.User{
		ID:        d.ID,
		DiscordID: d.DiscordID,
		Name:      d.Name,
		AvatarURL: d.AvatarURL,
		Role:      model.GlobalRole(d.Role), // Cast string to Domain Type
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

func fromDomainUser(u *model.User) *userDTO {
	return &userDTO{
		ID:        u.ID,
		DiscordID: u.DiscordID,
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

const (
	queryCreateUser = `
		INSERT INTO users (id, discord_id, name, avatar_url, role, created_at, updated_at)
		VALUES (:id, :discord_id, :name, :avatar_url, :role, :created_at, :updated_at)
	`
	queryUpdateUser = `
		UPDATE users 
		SET name = :name, avatar_url = :avatar_url, updated_at = :updated_at 
		WHERE id = :id
	`
	queryGetUserByID = `
		SELECT * FROM users WHERE id = $1
	`
	queryGetUserByDiscordID = `
		SELECT * FROM users WHERE discord_id = $1
	`
)

// Create inserts a new user.
func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	dto := fromDomainUser(user)
	// NamedExec is a feature of sqlx that maps struct fields to :params
	_, err := r.db.NamedExecContext(ctx, queryCreateUser, dto)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// Update updates an existing user.
func (r *UserRepository) Update(ctx context.Context, user *model.User) error {
	dto := fromDomainUser(user)
	_, err := r.db.NamedExecContext(ctx, queryUpdateUser, dto)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by ID.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	var dto userDTO
	if err := r.db.GetContext(ctx, &dto, queryGetUserByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return dto.toDomain(), nil
}

// GetByDiscordID retrieves a user by Discord ID.
func (r *UserRepository) GetByDiscordID(ctx context.Context, discordID string) (*model.User, error) {
	var dto userDTO
	if err := r.db.GetContext(ctx, &dto, queryGetUserByDiscordID, discordID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// As per UseCase logic: Return nil, nil if not found
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by discord id: %w", err)
	}
	return dto.toDomain(), nil
}
