package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
)

func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23503":
			return domain.ErrConflict
		case "22P02", "23502":
			return domain.ErrInvalidInput
		}
	}
	return err
}
