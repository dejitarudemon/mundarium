package locations

import (
	"context"
	"errors"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"gorm.io/gorm"
)

var (
	// ErrConnectionSameEndpoints is returned when PutConnection would set FromID == ToID.
	// CreateConnection allows self-links (e.g. magical corridors back to the entry).
	ErrConnectionSameEndpoints = errors.New("connection between the same locations")
)

// Connection is not a physical place; it represents a directed link between two locations.
// It records an explicit canonical transition (secret tunnel, portal, special passage),
// not ordinary adjacency implied by the location hierarchy.
type Connection struct {
	ID repositories.ID `gorm:"primaryKey" json:"id"`

	FromID repositories.ID `gorm:"not null;index" json:"from_id"`
	From   *Location       `gorm:"foreignKey:FromID;references:ID;constraint:OnDelete:RESTRICT" json:"from,omitempty"`

	ToID repositories.ID `gorm:"not null;index" json:"to_id"`
	To   *Location       `gorm:"foreignKey:ToID;references:ID;constraint:OnDelete:RESTRICT" json:"to,omitempty"`

	Bidirectional bool `gorm:"not null;default:false" json:"bidirectional"`

	State   repositories.JSON `gorm:"type:jsonb" json:"state"`
	Details repositories.JSON `gorm:"type:jsonb" json:"details"`

	Description string `gorm:"type:text" json:"description"`

	ExpiresAt *int64
}

// CreateConnection inserts a new connection between two locations.
// Endpoints must exist (FK). Self-links (FromID == ToID) are allowed.
// The repository does not judge whether the link is narratively meaningful.
func (r *LocationRepository) CreateConnection(ctx context.Context, connection *Connection) error {
	if err := r.db.WithContext(ctx).Create(connection).Error; err != nil {
		r.error("failed to create connection", "type", "LocationRepository", "func", "CreateConnection", "data", connection, "error", err)
		return err
	}
	r.info("connection is created", "type", "LocationRepository", "func", "CreateConnection", "data", connection)
	return nil
}

// GetConnection returns the connection with the given id.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetConnection(ctx context.Context, id repositories.ID) (*Connection, error) {
	var connection Connection

	if err := r.db.WithContext(ctx).First(&connection, "id = ?", id).Error; err != nil {
		r.error("failed to get connection", "type", "LocationRepository", "func", "GetConnection", "id", id, "error", err)
		return nil, err
	}

	r.debug("got connection", "type", "LocationRepository", "func", "GetConnection", "data", &connection)
	return &connection, nil
}

// FindConnections returns connections matching filter. Nil/empty fields are ignored.
// With an empty filter, all connections are returned ordered by id.
//
// LocationID, when set, matches rows where the location is either endpoint and
// ignores FromID/ToID/EitherDirection/IncludeBidirectionalReverse.
//
// When both FromID and ToID are set:
//   - default: exact directed pair FromID -> ToID
//   - EitherDirection: also include ToID -> FromID
//   - IncludeBidirectionalReverse (without EitherDirection): also include
//     ToID -> FromID rows that are marked bidirectional
func (r *LocationRepository) FindConnections(ctx context.Context, filter FindConnectionsFilter) ([]Connection, error) {
	connections := make([]Connection, 0)

	query := r.db.WithContext(ctx).Model(&Connection{})

	switch {
	case filter.LocationID != nil:
		query = query.Where("from_id = ? OR to_id = ?", *filter.LocationID, *filter.LocationID)

	case filter.FromID != nil && filter.ToID != nil:
		fromID, toID := *filter.FromID, *filter.ToID
		switch {
		case filter.EitherDirection:
			query = query.Where(
				"(from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)",
				fromID, toID, toID, fromID,
			)
		case filter.IncludeBidirectionalReverse:
			query = query.Where(
				"(from_id = ? AND to_id = ?) OR (bidirectional = TRUE AND from_id = ? AND to_id = ?)",
				fromID, toID, toID, fromID,
			)
		default:
			query = query.Where("from_id = ? AND to_id = ?", fromID, toID)
		}

	default:
		if filter.FromID != nil {
			query = query.Where("from_id = ?", *filter.FromID)
		}
		if filter.ToID != nil {
			query = query.Where("to_id = ?", *filter.ToID)
		}
	}

	if err := query.Order("id").Find(&connections).Error; err != nil {
		r.error("failed to find connections", "type", "LocationRepository", "func", "FindConnections", "filter", filter, "error", err)
		return nil, err
	}

	r.debug("found connections", "type", "LocationRepository", "func", "FindConnections", "count", len(connections))
	return connections, nil
}

// FindInderectConnections returns connections directed from fromID to toID.
// Kept for callers that need the exact directed pair; prefer FindConnections for new code.
func (r *LocationRepository) FindInderectConnections(ctx context.Context, fromID, toID repositories.ID) ([]Connection, error) {
	return r.FindConnections(ctx, FindConnectionsFilter{
		FromID: &fromID,
		ToID:   &toID,
	})
}

// PutConnection updates mutable fields of an existing connection identified by connection.ID.
// Returns ErrConnectionSameEndpoints if FromID and ToID refer to the same location.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PutConnection(ctx context.Context, connection *Connection) error {
	if connection.FromID == connection.ToID {
		r.error("failed to put connection", "type", "LocationRepository", "func", "PutConnection", "id", connection.ID, "error", ErrConnectionSameEndpoints)
		return ErrConnectionSameEndpoints
	}

	result := r.db.WithContext(ctx).
		Model(&Connection{}).
		Where("id = ?", connection.ID).
		Select(
			"FromID",
			"ToID",
			"Bidirectional",
			"State",
			"Details",
			"Description",
			"ExpiresAt",
		).
		Updates(connection)

	if result.Error != nil {
		r.error("failed to put connection", "type", "LocationRepository", "func", "PutConnection", "id", connection.ID, "error", result.Error)
		return result.Error
	}

	if result.RowsAffected == 0 {
		r.error("failed to put connection", "type", "LocationRepository", "func", "PutConnection", "id", connection.ID, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("connection is updated", "type", "LocationRepository", "func", "PutConnection", "data", connection)
	return nil
}

// DeleteConnection deletes the connection with the given id.
// Returns gorm.ErrRecordNotFound if no row was deleted.
func (r *LocationRepository) DeleteConnection(ctx context.Context, id repositories.ID) error {
	result := r.db.WithContext(ctx).Delete(&Connection{}, "id = ?", id)
	if result.Error != nil {
		r.error("failed to delete connection", "type", "LocationRepository", "func", "DeleteConnection", "id", id, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.error("failed to delete connection", "type", "LocationRepository", "func", "DeleteConnection", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}
	r.info("connection is deleted", "type", "LocationRepository", "func", "DeleteConnection", "id", id)
	return nil
}
