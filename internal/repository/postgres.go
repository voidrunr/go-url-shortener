package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgres(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (repo *PostgresRepository) Write(ctx context.Context, url model.URL) error {
	const query = `
		INSERT INTO shortener_urls (code, original_url, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (original_url) DO NOTHING
		RETURNING code
	`

	var insertedCode string
	err := repo.db.QueryRowContext(
		ctx,
		query,
		url.Code,
		url.Original,
		url.UserID,
		url.CreatedAt,
		url.UpdatedAt,
	).Scan(&insertedCode)
	if errors.Is(err, sql.ErrNoRows) {
		existing, findErr := repo.getByOriginal(ctx, url.Original)
		if findErr != nil {
			return findErr
		}
		return &DuplicateURLError{URL: existing}
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrConflict
		}
		return err
	}

	return nil
}

func (repo *PostgresRepository) WriteBatch(ctx context.Context, urls []model.URL) error {
	if len(urls) == 0 {
		return nil
	}

	valueStrings := make([]string, 0, len(urls))
	valueArgs := make([]any, 0, len(urls)*5)
	for i, url := range urls {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)", i*5+1, i*5+2, i*5+3, i*5+4, i*5+5))
		valueArgs = append(valueArgs, url.Code, url.Original, url.UserID, url.CreatedAt, url.UpdatedAt)
	}

	query := fmt.Sprintf(
		`INSERT INTO shortener_urls (code, original_url, user_id, created_at, updated_at)
		 VALUES %s
		 ON CONFLICT (original_url) DO NOTHING
		 RETURNING code`,
		strings.Join(valueStrings, ","),
	)

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, query, valueArgs...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrConflict
		}
		return err
	}
	defer rows.Close()

	inserted := make(map[string]struct{}, len(urls))
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return err
		}
		inserted[code] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, url := range urls {
		if _, ok := inserted[url.Code]; !ok {
			existing, findErr := repo.getByOriginal(ctx, url.Original)
			if findErr != nil {
				return findErr
			}
			return &DuplicateURLError{URL: existing}
		}
	}

	return tx.Commit()
}

func (repo *PostgresRepository) getByOriginal(ctx context.Context, original string) (model.URL, error) {
	const query = `
		SELECT code, original_url, user_id, is_deleted, created_at, updated_at
		FROM shortener_urls
		WHERE original_url = $1
	`

	var url model.URL
	err := repo.db.QueryRowContext(ctx, query, original).
		Scan(&url.Code, &url.Original, &url.UserID, &url.Deleted, &url.CreatedAt, &url.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.URL{}, ErrNotFound
	}
	if err != nil {
		return model.URL{}, err
	}

	return url, nil
}

func (repo *PostgresRepository) Get(ctx context.Context, code string) (model.URL, error) {
	const query = `
		SELECT code, original_url, user_id, is_deleted, created_at, updated_at
		FROM shortener_urls
		WHERE code = $1
	`

	var url model.URL
	err := repo.db.QueryRowContext(ctx, query, code).
		Scan(&url.Code, &url.Original, &url.UserID, &url.Deleted, &url.CreatedAt, &url.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.URL{}, ErrNotFound
	}
	if err != nil {
		return model.URL{}, err
	}

	return url, nil
}

func (repo *PostgresRepository) GetByUser(userID string) ([]model.URL, error) {
	const query = `
		SELECT code, original_url, user_id, is_deleted, created_at, updated_at
		FROM shortener_urls
		WHERE user_id = $1
		ORDER BY created_at, id
	`

	ctx := context.Background()

	rows, err := repo.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urls := make([]model.URL, 0)
	for rows.Next() {
		var url model.URL
		if err := rows.Scan(&url.Code, &url.Original, &url.UserID, &url.Deleted, &url.CreatedAt, &url.UpdatedAt); err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return urls, nil
}

func (repo *PostgresRepository) DeleteBatch(userID string, codes []string) error {
	if len(codes) == 0 {
		return nil
	}

	const query = `
		UPDATE shortener_urls
		SET is_deleted = TRUE, updated_at = NOW()
		WHERE user_id = $1 AND code = ANY($2)
	`

	_, err := repo.db.ExecContext(context.Background(), query, userID, codes)
	return err
}
