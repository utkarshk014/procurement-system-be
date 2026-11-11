package pgutil

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// DBADelete names the DBA's objects for deleting one row. Every field is a constant, never input.
type DBADelete struct {
	Lock      string // the table name used for the advisory lock, the same one its writes take
	Validate  string // e.g. pms.mst_place_validate_delete(code) RETURNS (is_valid, message)
	Procedure string // e.g. pms.mst_place_delete(code)
	What      string // "Place", for our own messages
	// Guard, when set, runs inside the lock before the DBA's check: our own rule that must hold
	// at the moment of the delete (for example "not the current year").
	Guard func(ctx context.Context, tx pgx.Tx) error
}

// Delete runs the DBA's delete check and then his delete procedure in one transaction, under the
// same lock as the table's writes, so nothing can start using the row between the check and the
// delete. His "does not exist" becomes notFound; any other refusal ("Cannot delete! … already
// used in …", the current year) is a 409 carrying his message.
func Delete(ctx context.Context, pool *pgxpool.Pool, d DBADelete, key string, notFound error) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, d.Lock); err != nil {
			return domain.Internal("Could not lock "+d.What, err)
		}
		if d.Guard != nil {
			if err := d.Guard(ctx, tx); err != nil {
				return err
			}
		}
		var valid bool
		var message string
		if err := tx.QueryRow(ctx, `SELECT is_valid, message FROM `+d.Validate+`($1::varchar)`, key).Scan(&valid, &message); err != nil {
			return domain.Internal("Could not run the "+d.What+" delete check", err)
		}
		if !valid {
			return deleteRefused(message, notFound)
		}
		if _, err := tx.Exec(ctx, `CALL `+d.Procedure+`($1::varchar)`, key); err != nil {
			var pgErr *pgconn.PgError
			switch {
			case errors.As(err, &pgErr) && pgErr.Code == "P0001":
				return deleteRefused(pgErr.Message, notFound)
			case errors.As(err, &pgErr) && pgErr.Code == "23503": // a foreign key still points at the row
				return domain.Conflict("Cannot delete! " + d.What + " is still in use")
			default:
				return domain.Internal("Could not delete "+d.What, err)
			}
		}
		return nil
	})
}

func deleteRefused(message string, notFound error) error {
	message = strings.TrimSpace(message)
	if strings.Contains(strings.ToLower(message), "does not exist") {
		return notFound
	}
	return domain.Conflict(message)
}
