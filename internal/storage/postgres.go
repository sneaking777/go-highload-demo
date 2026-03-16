// Package storage реализует конкретные адаптеры хранилищ для сервиса уведомлений.
package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/go-highload-demo/internal/model"
)

// PostgresStore — хранилище уведомлений на базе PostgreSQL.
// Реализует интерфейс repository.Store.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore создаёт новое хранилище с подключением к PostgreSQL.
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect failed: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping failed: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

// Close закрывает пул соединений.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// Migrate выполняет SQL-миграцию для создания необходимых таблиц.
func (s *PostgresStore) Migrate(ctx context.Context, sql string) error {
	_, err := s.pool.Exec(ctx, sql)
	if err != nil {
		return fmt.Errorf("postgres: migrate failed: %w", err)
	}
	return nil
}

// Save сохраняет уведомление в PostgreSQL.
func (s *PostgresStore) Save(ctx context.Context, n *model.Notification) error {
	query := `INSERT INTO notifications (id, user_id, channel, payload, status, retry_count, last_error, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := s.pool.Exec(ctx, query,
		n.ID, n.UserID, n.Channel, n.Payload, n.Status,
		n.RetryCount, n.LastError, n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: save failed: %w", err)
	}
	return nil
}

// GetByID возвращает уведомление по идентификатору.
func (s *PostgresStore) GetByID(ctx context.Context, id string) (*model.Notification, error) {
	query := `SELECT id, user_id, channel, payload, status, retry_count, last_error, created_at, updated_at
		FROM notifications WHERE id = $1`
	var n model.Notification
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&n.ID, &n.UserID, &n.Channel, &n.Payload, &n.Status,
		&n.RetryCount, &n.LastError, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("notification %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get by id failed: %w", err)
	}
	return &n, nil
}

// UpdateStatus обновляет статус уведомления и сообщение об ошибке.
func (s *PostgresStore) UpdateStatus(ctx context.Context, id string, status model.NotificationStatus, lastError string) error {
	query := `UPDATE notifications SET status = $1, last_error = $2, updated_at = NOW() WHERE id = $3`
	tag, err := s.pool.Exec(ctx, query, status, lastError, id)
	if err != nil {
		return fmt.Errorf("postgres: update status failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("notification %s not found", id)
	}
	return nil
}

// GetPending возвращает до limit уведомлений со статусом pending.
func (s *PostgresStore) GetPending(ctx context.Context, limit int) ([]*model.Notification, error) {
	query := `SELECT id, user_id, channel, payload, status, retry_count, last_error, created_at, updated_at
		FROM notifications WHERE status = 'pending' ORDER BY created_at LIMIT $1`
	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: get pending failed: %w", err)
	}
	defer rows.Close()

	var result []*model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.Channel, &n.Payload, &n.Status,
			&n.RetryCount, &n.LastError, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan failed: %w", err)
		}
		result = append(result, &n)
	}
	return result, rows.Err()
}
