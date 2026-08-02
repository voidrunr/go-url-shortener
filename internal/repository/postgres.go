package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgres(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (repo *PostgresRepository) Write(url model.URL) error {
	return repo.WriteBatch([]model.URL{url})
}

func (repo *PostgresRepository) WriteBatch(urls []model.URL) error {
	const query = `
		INSERT INTO shortener_urls (code, original_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
	`

	ctx := context.Background()

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, url := range urls {
		_, err := stmt.ExecContext(
			ctx,
			url.Code,
			url.Original,
			url.CreatedAt,
			url.UpdatedAt,
		)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrConflict
			}
			return err
		}
	}

	return tx.Commit()
}

func (repo *PostgresRepository) Get(code string) (model.URL, error) {
	const query = `
		SELECT code, original_url, created_at, updated_at
		FROM shortener_urls
		WHERE code = $1
	`

	var url model.URL
	err := repo.db.QueryRowContext(context.Background(), query, code).
		Scan(&url.Code, &url.Original, &url.CreatedAt, &url.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.URL{}, ErrNotFound
	}
	if err != nil {
		return model.URL{}, err
	}

	return url, nil
}
