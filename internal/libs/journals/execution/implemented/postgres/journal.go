// Package postgres implements Journal on PostgreSQL via GORM.
//
// Groups are implicit: requests share a group_id, and no separate group row is created.
// An in-memory emergency set records request ids whose Fail could not be persisted;
// Find returns ErrAlreadyFailed for those ids. Close / DeleteAllExpired clear the set.
package postgres

import (
	"context"
	"errors"
	"time"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"gorm.io/gorm"
)

var (
	_ journal.Journal = (*PostgresJournal)(nil)
)

// PostgresJournal is a Journal backed by PostgreSQL.
type PostgresJournal struct {
	connection *postgres.Connection
	emergency  *emergency
}

// NewPostgresJournal creates a PostgresJournal with a built-in emergency set.
// connection must be non-nil.
func NewPostgresJournal(connection *postgres.Connection) (*PostgresJournal, error) {
	if connection == nil {
		return nil, errors.New("connection is nil")
	}
	return &PostgresJournal{
		connection: connection,
		emergency:  newEmergency(),
	}, nil
}

func (j *PostgresJournal) db(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return j.connection.DB().WithContext(ctx)
}

// Register inserts a running request. Groups are not materialised separately.
// Returns AlreadyRegistered if the (id, group_id) pair exists, or ErrGroupIsClosed
// if the group was closed (any request in the group already has ExpiresAt set).
func (j *PostgresJournal) Register(ctx context.Context, requestID, groupRequestID journal.ID, tool string, argsHash []byte) (journal.CompleteFunc, journal.FailFunc, error) {
	err := j.connection.WithTransaction(ctx, func(tx *gorm.DB) error {
		var existing journal.Request
		err := tx.First(&existing, "id = ? AND group_id = ?", requestID, groupRequestID).Error
		if err == nil {
			return journal.NewAlreadyRegistered(existing)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return journal.NewInternalErr(err)
		}

		closed, err := j.groupClosed(tx, groupRequestID)
		if err != nil {
			return err
		}
		if closed {
			return journal.ErrGroupIsClosed
		}

		req := journal.Request{
			ID:        requestID,
			GroupID:   groupRequestID,
			CreatedAt: time.Now().UTC(),
			Tool:      tool,
			ArgsHash:  argsHash,
			Status:    journal.Status_Running,
		}
		if err := tx.Create(&req).Error; err != nil {
			return journal.NewInternalErr(err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return j.Complete, j.Fail, nil
}

// Complete marks a running request completed and stores result.
func (j *PostgresJournal) Complete(ctx context.Context, requestID, groupRequestID journal.ID, result journal.Result, tx *gorm.DB) error {
	db := j.db(ctx, tx)

	var request journal.Request
	if err := db.First(&request, "id = ? AND group_id = ?", requestID, groupRequestID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return journal.NewNotFound(requestID, groupRequestID)
		}
		return journal.NewInternalErr(err)
	}

	if request.Status != journal.Status_Running {
		return journal.NewStatusConflict(requestID, groupRequestID, journal.Status_Completed, request.Status)
	}

	request.Status = journal.Status_Completed
	request.Result = result
	if err := db.Save(&request).Error; err != nil {
		return journal.NewInternalErr(err)
	}
	return nil
}

// Fail marks a running request failed and stores cause.
// If Postgres cannot load (other than not-found) or save the failure, the fail is
// recorded in the emergency journal instead.
// Not-found is not an emergency case: a registered request must exist in Postgres.
func (j *PostgresJournal) Fail(ctx context.Context, requestID, groupRequestID journal.ID, cause error, tx *gorm.DB) error {
	db := j.db(ctx, tx)

	var request journal.Request
	if err := db.First(&request, "id = ? AND group_id = ?", requestID, groupRequestID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return journal.NewNotFound(requestID, groupRequestID)
		}

		j.emergency.add(requestID, groupRequestID)
		return journal.NewInternalErr(err)
	}

	if request.Status != journal.Status_Running {
		return journal.NewStatusConflict(requestID, groupRequestID, journal.Status_Failed, request.Status)
	}

	request.Status = journal.Status_Failed
	request.Error = journal.ErrorTextFrom(cause)
	if err := db.Save(&request).Error; err != nil {
		j.emergency.add(requestID, groupRequestID)
		return journal.NewInternalErr(err)
	}
	return nil
}

// Find returns the request from Postgres.
// If the id is marked in emergency (Fail could not be persisted), returns ErrAlreadyFailed —
// including when Postgres still shows Running.
func (j *PostgresJournal) Find(ctx context.Context, requestID, groupRequestID journal.ID) (journal.Request, error) {
	if j.emergency.find(requestID, groupRequestID) {
		return journal.Request{}, journal.ErrAlreadyFailed
	}

	var request journal.Request
	err := j.connection.DB().WithContext(ctx).First(&request, "id = ? AND group_id = ?", requestID, groupRequestID).Error
	if err == nil {
		return request, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return journal.Request{}, journal.NewInternalErr(err)
	}

	return journal.Request{}, journal.NewNotFound(requestID, groupRequestID)
}

// Close sets ExpiresAt on every Postgres request in the group (idempotent for already-set rows).
func (j *PostgresJournal) Close(ctx context.Context, groupRequestID journal.ID, deadline time.Time) error {
	err := j.connection.DB().WithContext(ctx).
		Model(&journal.Request{}).
		Where("group_id = ? AND expires_at IS NULL", groupRequestID).
		Update("expires_at", deadline).Error
	if err != nil {
		return journal.NewInternalErr(err)
	}

	j.emergency.delete(groupRequestID)
	return nil
}

// IsGroupFree reports whether Postgres has no requests for the group.
// Emergency markers are ignored.
func (j *PostgresJournal) IsGroupFree(ctx context.Context, groupRequestID journal.ID) (bool, error) {
	var count int64
	err := j.connection.DB().WithContext(ctx).
		Model(&journal.Request{}).
		Where("group_id = ?", groupRequestID).
		Count(&count).Error
	if err != nil {
		return false, journal.NewInternalErr(err)
	}
	return count == 0, nil
}

// DeleteAllExpired removes Postgres groups whose every request is expired under soft/hard rules,
// and clears emergency markers for deleted groups.
func (j *PostgresJournal) DeleteAllExpired(ctx context.Context, hardTTL time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var groupIDs []journal.ID
	if err := j.connection.DB().WithContext(ctx).
		Model(&journal.Request{}).
		Distinct("group_id").
		Pluck("group_id", &groupIDs).Error; err != nil {
		return journal.NewInternalErr(err)
	}

	now := time.Now().UTC()
	for _, groupID := range groupIDs {
		if err := ctx.Err(); err != nil {
			return err
		}

		var requests []journal.Request
		if err := j.connection.DB().WithContext(ctx).
			Where("group_id = ?", groupID).
			Find(&requests).Error; err != nil {
			return journal.NewInternalErr(err)
		}
		if len(requests) == 0 {
			continue
		}

		allExpired := true
		for _, req := range requests {
			if !req.IsExpired(now, hardTTL) {
				allExpired = false
				break
			}
		}
		if !allExpired {
			continue
		}

		if err := j.connection.DB().WithContext(ctx).
			Where("group_id = ?", groupID).
			Delete(&journal.Request{}).Error; err != nil {
			return journal.NewInternalErr(err)
		}

		j.emergency.delete(groupID)
	}

	return nil
}

// groupClosed reports whether the group was closed: any request already has ExpiresAt set.
func (j *PostgresJournal) groupClosed(tx *gorm.DB, groupRequestID journal.ID) (bool, error) {
	var count int64
	err := tx.Model(&journal.Request{}).
		Where("group_id = ? AND expires_at IS NOT NULL", groupRequestID).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return false, journal.NewInternalErr(err)
	}
	return count > 0, nil
}
