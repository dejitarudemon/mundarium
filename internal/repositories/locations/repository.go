// Package locations provides data-access helpers for location domain models.
package locations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	"github.com/dejitarudemon/mundarium/internal/repositories"

	"gorm.io/gorm"
)

var (
	// ErrLocationWouldCreateCycle is returned when a parent update would form a cycle.
	ErrLocationWouldCreateCycle = errors.New("location would create a cycle")
)

// LocationRepository persists and queries Location, LocationSubtype, and Connection records.
type LocationRepository struct {
	db       *gorm.DB
	maxDepth int

	logger logger.Logger
}

// NewLocationRepository creates a LocationRepository backed by db.
// maxDepth is the maximum relative depth accepted by FindLocations (inclusive).
// logger may be nil. Returns an error if db is nil or maxDepth is negative.
func NewLocationRepository(db *gorm.DB, maxDepth int, logger logger.Logger) (*LocationRepository, error) {
	if db == nil {
		return nil, errors.New("expected *gorm.DB, got nil")
	}
	if maxDepth < 0 {
		return nil, errors.New("max depth must be >= 0")
	}

	return &LocationRepository{db: db, maxDepth: maxDepth, logger: logger}, nil
}

// MaxDepth returns the maximum relative depth accepted by FindLocations.
func (r *LocationRepository) MaxDepth() int {
	return r.maxDepth
}

// CreateLocation inserts a new location and returns the persisted record
// (including the generated ID and any DB defaults).
// Hierarchy validity (including cycles) is not checked here; use
// PatchLocationParent or PutLocation when reparenting an existing location.
func (r *LocationRepository) CreateLocation(ctx context.Context, location *Location) (*Location, error) {
	if err := r.db.WithContext(ctx).Create(location).Error; err != nil {
		r.error("failed to create location", "type", "LocationRepository", "func", "CreateLocation", "data", &location, "error", err)
		return nil, err
	}

	r.info("location is created", "type", "LocationRepository", "func", "CreateLocation", "data", &location)

	return location, nil
}

// GetLocation returns the location with the given id, preloading its Subtype.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetLocation(ctx context.Context, id repositories.ID) (*Location, error) {
	var location Location

	if err := r.db.
		WithContext(ctx).
		Where("id = ?", id).
		Preload("Subtype").
		First(&location).Error; err != nil {
		r.error("failed to get location", "type", "LocationRepository", "func", "GetLocation", "id", id, "error", err)
		return nil, err
	}

	r.debug("got location", "type", "LocationRepository", "func", "GetLocation", "data", &location)

	return &location, nil
}

// FindChildLocations returns descendants of id grouped by depth.
//
// Depth semantics:
//   - 0 or less: empty map
//   - 1: direct children only (map key 1)
//   - 2 or more: children, grandchildren, … up to the given depth (keys 1..depth)
//
// If id does not exist, gorm.ErrRecordNotFound is returned.
// Each level is loaded with one query ordered by id. Already seen IDs are
// skipped to break parent_id cycles. When loadSubtypes is true, each level
// preloads Subtype.
func (r *LocationRepository) FindChildLocations(ctx context.Context, id repositories.ID, depth int, loadSubtypes bool) (map[int][]Location, error) {
	result := make(map[int][]Location)
	if depth <= 0 {
		r.error("failed to find location", "type", "LocationRepository", "func", "FindChildLocations", "id", id, "error", fmt.Sprintf("invalid depth %v", depth))
		return result, nil
	}

	var exists int64
	if err := r.db.WithContext(ctx).Model(&Location{}).Where("id = ?", id).Count(&exists).Error; err != nil {
		r.error("failed to find location", "type", "LocationRepository", "func", "FindChildLocations", "id", id, "error", err)
		return nil, err
	}
	if exists == 0 {
		r.error("failed to find location", "type", "LocationRepository", "func", "FindChildLocations", "id", id, "error", gorm.ErrRecordNotFound)
		return nil, gorm.ErrRecordNotFound
	}

	seen := map[repositories.ID]struct{}{id: {}}
	parentIDs := []repositories.ID{id}

	for d := 1; d <= depth; d++ {
		if len(parentIDs) == 0 {
			break
		}

		var found []Location
		q := r.db.WithContext(ctx).
			Where("parent_id IN ?", parentIDs).
			Order("id")

		if loadSubtypes {
			q = q.Preload("Subtype")
		}

		if err := q.Find(&found).Error; err != nil {
			r.error("failed to find location children", "type", "LocationRepository", "func", "FindChildLocations", "id", id, "level", d, "depth", depth, "error", err)
			return nil, err
		}

		parentIDs = parentIDs[:0]
		level := make([]Location, 0, len(found))
		for _, loc := range found {
			if _, ok := seen[loc.ID]; ok {
				continue
			}
			seen[loc.ID] = struct{}{}
			level = append(level, loc)
			parentIDs = append(parentIDs, loc.ID)
		}

		if len(level) == 0 {
			break
		}
		result[d] = level
	}

	r.debug("found location", "type", "LocationRepository", "func", "FindChildLocations", "data", result)
	return result, nil
}

// PutLocation updates mutable fields of an existing location identified by location.ID.
//
// Before writing, checkCircle walks ancestors of location.ParentID (no transaction)
// and rejects the update if that parent link would create a cycle.
// A nil ParentID means the location becomes a root.
//
// Returns ErrLocationWouldCreateCycle if the new parent would form a cycle.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PutLocation(ctx context.Context, location *Location) error {
	cycle, err := r.checkCircle(ctx, location.ID, location.ParentID)
	if err != nil {
		r.error("failed to put location", "type", "LocationRepository", "func", "PutLocation", "id", location.ID, "error", err)
		return err
	}
	if cycle {
		r.error("failed to put location", "type", "LocationRepository", "func", "PutLocation", "id", location.ID, "error", ErrLocationWouldCreateCycle)
		return ErrLocationWouldCreateCycle
	}

	result := r.db.WithContext(ctx).
		Model(&Location{}).
		Where("id = ?", location.ID).
		Select(
			"ParentID",
			"Type",
			"SubtypeID",
			"Name",
			"Description",
			"Details",
		).
		Updates(location)

	if result.Error != nil {
		r.error("failed to put location", "type", "LocationRepository", "func", "PutLocation", "id", location.ID, "error", result.Error)
		return result.Error
	}

	if result.RowsAffected == 0 {
		r.error("failed to put location", "type", "LocationRepository", "func", "PutLocation", "id", location.ID, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("location is updated", "type", "LocationRepository", "func", "PutLocation", "data", location)
	return nil
}

// PatchLocationParent sets the parent of the location identified by id to parentID.
//
// Before writing, checkCircle walks ancestors of parentID (no transaction)
// and rejects the change if that parent link would create a cycle.
// A nil parentID means the location becomes a root.
//
// Returns ErrLocationWouldCreateCycle if the new parent would form a cycle.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationParent(ctx context.Context, id repositories.ID, parentID *repositories.ID) error {
	cycle, err := r.checkCircle(ctx, id, parentID)
	if err != nil {
		r.error("failed to patch location parent", "type", "LocationRepository", "func", "PatchLocationParent", "id", id, "error", err)
		return err
	}
	if cycle {
		r.error("failed to patch location parent", "type", "LocationRepository", "func", "PatchLocationParent", "id", id, "error", ErrLocationWouldCreateCycle)
		return ErrLocationWouldCreateCycle
	}

	query := r.db.WithContext(ctx).Model(&Location{}).Where("id = ?", id).Update("parent_id", parentID)

	if err := query.Error; err != nil {
		r.error("failed to patch location parent", "type", "LocationRepository", "func", "PatchLocationParent", "id", id, "error", err)
		return err
	}

	if query.RowsAffected == 0 {
		r.error("failed to patch location parent", "type", "LocationRepository", "func", "PatchLocationParent", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("location parent is patched", "type", "LocationRepository", "func", "PatchLocationParent", "id", id, "parent_id", parentID)
	return nil
}

// PatchLocationName sets the name of the location identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationName(ctx context.Context, id repositories.ID, name string) error {
	query := r.db.WithContext(ctx).Model(&Location{}).Where("id = ?", id).Update("name", name)

	if err := query.Error; err != nil {
		r.error("failed to patch location name", "type", "LocationRepository", "func", "PatchLocationName", "id", id, "error", err)
		return err
	}

	if query.RowsAffected == 0 {
		r.error("failed to patch location name", "type", "LocationRepository", "func", "PatchLocationName", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("location name is patched", "type", "LocationRepository", "func", "PatchLocationName", "id", id, "name", name)
	return nil
}

// PatchLocationDetails sets the details of the location identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationDetails(ctx context.Context, id repositories.ID, details repositories.JSON) error {
	query := r.db.WithContext(ctx).Model(&Location{}).Where("id = ?", id).Update("details", details)

	if err := query.Error; err != nil {
		r.error("failed to patch location details", "type", "LocationRepository", "func", "PatchLocationDetails", "id", id, "error", err)
		return err
	}

	if query.RowsAffected == 0 {
		r.error("failed to patch location details", "type", "LocationRepository", "func", "PatchLocationDetails", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("location details are patched", "type", "LocationRepository", "func", "PatchLocationDetails", "id", id)
	return nil
}

// PatchLocationDescription sets the description of the location identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PatchLocationDescription(ctx context.Context, id repositories.ID, description string) error {
	query := r.db.WithContext(ctx).Model(&Location{}).Where("id = ?", id).Update("description", description)

	if err := query.Error; err != nil {
		r.error("failed to patch location description", "type", "LocationRepository", "func", "PatchLocationDescription", "id", id, "error", err)
		return err
	}

	if query.RowsAffected == 0 {
		r.error("failed to patch location description", "type", "LocationRepository", "func", "PatchLocationDescription", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("location description is patched", "type", "LocationRepository", "func", "PatchLocationDescription", "id", id)
	return nil
}

// CreateSubtype inserts a new location subtype.
func (r *LocationRepository) CreateSubtype(ctx context.Context, subtype *LocationSubtype) error {
	if err := r.db.WithContext(ctx).Create(subtype).Error; err != nil {
		r.error("failed to create subtype", "type", "LocationRepository", "func", "CreateSubtype", "data", subtype, "error", err)
		return err
	}
	r.info("subtype is created", "type", "LocationRepository", "func", "CreateSubtype", "data", subtype)
	return nil
}

// GetSubtype returns the location subtype with the given id.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetSubtype(ctx context.Context, id repositories.ID) (*LocationSubtype, error) {
	var subtype LocationSubtype

	if err := r.db.
		WithContext(ctx).
		First(&subtype, "id = ?", id).Error; err != nil {
		r.error("failed to get subtype", "type", "LocationRepository", "func", "GetSubtype", "id", id, "error", err)
		return nil, err
	}

	r.debug("got subtype", "type", "LocationRepository", "func", "GetSubtype", "data", &subtype)
	return &subtype, nil
}

// FindSubtypes returns subtypes matching any of the given keys and/or location types.
// Empty keys or types means that filter is not applied. If both are empty, all subtypes are returned.
func (r *LocationRepository) FindSubtypes(ctx context.Context, keys []string, types []LocationType) ([]LocationSubtype, error) {
	var subtypes []LocationSubtype

	query := r.db.WithContext(ctx).Model(LocationSubtype{})

	if len(keys) > 0 {
		query = query.Where("key IN ?", keys)
	}
	if len(types) > 0 {
		query = query.Where("type IN ?", types)
	}

	if err := query.Find(&subtypes).Error; err != nil {
		r.error("failed to find subtypes", "type", "LocationRepository", "func", "FindSubtypes", "keys", keys, "types", types, "error", err)
		return nil, err
	}

	r.debug("found subtypes", "type", "LocationRepository", "func", "FindSubtypes", "count", len(subtypes))
	return subtypes, nil
}

// CreateConnection inserts a new connection between two locations.
// The repository does not validate whether the link is logically meaningful;
// that is left to the caller.
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

// FindInderectConnections returns connections directed from fromID to toID.
func (r *LocationRepository) FindInderectConnections(ctx context.Context, fromID, toID repositories.ID) ([]Connection, error) {
	connections := make([]Connection, 0)

	if err := r.db.WithContext(ctx).Find(&connections, "from_id = ? AND to_id = ?", fromID, toID).Error; err != nil {
		r.error("failed to find connections", "type", "LocationRepository", "func", "FindInderectConnections", "from_id", fromID, "to_id", toID, "error", err)
		return nil, err
	}

	r.debug("found connections", "type", "LocationRepository", "func", "FindInderectConnections", "from_id", fromID, "to_id", toID, "count", len(connections))
	return connections, nil
}

// PutConnection updates mutable fields of an existing connection identified by connection.ID.
// Returns an error if FromID and ToID refer to the same location.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *LocationRepository) PutConnection(ctx context.Context, connection *Connection) error {
	if connection.FromID == connection.ToID {
		err := errors.New("connection between the same locations")
		r.error("failed to put connection", "type", "LocationRepository", "func", "PutConnection", "id", connection.ID, "error", err)
		return err
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
func (r *LocationRepository) DeleteConnection(ctx context.Context, id repositories.ID) error {
	if err := r.db.WithContext(ctx).Delete(Connection{}, "id = ?", id).Error; err != nil {
		r.error("failed to delete connection", "type", "LocationRepository", "func", "DeleteConnection", "id", id, "error", err)
		return err
	}
	r.info("connection is deleted", "type", "LocationRepository", "func", "DeleteConnection", "id", id)
	return nil
}

// checkCircle reports whether setting id's parent to newParent would create a cycle.
//
// It reads the current ancestor chain of newParent and does not write.
// Returns true if a cycle would be created, false otherwise.
// A nil newParent means becoming a root and is never a cycle.
// A nil parent_id on an ancestor ends the walk as a tree root.
func (r *LocationRepository) checkCircle(ctx context.Context, id repositories.ID, newParent *repositories.ID) (bool, error) {
	if newParent == nil {
		return false, nil
	}
	if *newParent == 0 {
		return false, errors.New("parent id is zero")
	}
	if *newParent == id {
		return true, nil
	}

	next := *newParent
	seen := map[repositories.ID]struct{}{
		id:   {},
		next: {},
	}

	for ctx.Err() == nil {
		var parentID *repositories.ID
		if err := r.db.WithContext(ctx).
			Model(&Location{}).
			Select("parent_id").
			Where("id = ?", next).
			Scan(&parentID).Error; err != nil {
			r.error("failed to check location cycle", "type", "LocationRepository", "func", "checkCircle", "id", id, "next", next, "error", err)
			return false, err
		}

		// root: parent is null
		if parentID == nil {
			return false, nil
		}

		next = *parentID
		if next == id {
			return true, nil
		}

		if _, ok := seen[next]; ok {
			return true, nil
		}

		seen[next] = struct{}{}
	}

	return false, ctx.Err()
}
