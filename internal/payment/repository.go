package payment

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type queryExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error
	GetOrderTotal(ctx context.Context, orderID string) (float64, error)
	GetTotalPaid(ctx context.Context, orderID string) (float64, error)
	InsertPayment(ctx context.Context, p *Payment) error
	UpdateOrderStatus(ctx context.Context, orderID string, status string) error
	GetByOrderID(ctx context.Context, orderID string) ([]Payment, error)
	ListAll(ctx context.Context) ([]Payment, error)
	GetDailyReport(ctx context.Context, day time.Time) (*DailyReport, error)
	GetByID(ctx context.Context, id int) (*Payment, error)
	RefundPayment(ctx context.Context, paymentID int) error
}

type repository struct {
	db *pgxpool.Pool
	q  queryExecutor
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db, q: db}
}

func EnsureSchema(ctx context.Context, db *pgxpool.Pool) error {
	type schemaStatement struct {
		sql              string
		allowOwnerDenied bool
	}

	statements := []schemaStatement{
		{sql: `
		CREATE TABLE IF NOT EXISTS payments (
			id SERIAL PRIMARY KEY,
			order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			amount DOUBLE PRECISION NOT NULL,
			method TEXT NOT NULL,
			status TEXT NOT NULL,
			transaction_reference TEXT NOT NULL DEFAULT '',
			paid_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`},
		{sql: `CREATE INDEX IF NOT EXISTS idx_payments_order_id ON payments(order_id)`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments(created_at DESC)`, allowOwnerDenied: true},
	}

	for _, statement := range statements {
		if _, err := db.Exec(ctx, statement.sql); err != nil {
			if statement.allowOwnerDenied && isOwnerPrivilegeError(err) {
				continue
			}
			return err
		}
	}

	return nil
}

func isOwnerPrivilegeError(err error) bool {
	var pgErr *pgconn.PgError
	if ok := errors.As(err, &pgErr); ok {
		return pgErr.Code == "42501"
	}

	errMessage := strings.ToLower(err.Error())
	return strings.Contains(errMessage, "must be owner") || strings.Contains(errMessage, "permission denied")
}

func (r *repository) WithTx(ctx context.Context, fn func(txRepo Repository) error) error {
	if tx, ok := r.q.(pgx.Tx); ok && tx != nil {
		return fn(r)
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	txRepo := &repository{db: r.db, q: tx}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (r *repository) GetOrderTotal(ctx context.Context, orderID string) (float64, error) {
	query := `SELECT total_amount FROM orders WHERE id::text = $1`

	var total float64
	err := r.q.QueryRow(ctx, query, orderID).Scan(&total)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	return total, nil
}

func (r *repository) GetTotalPaid(ctx context.Context, orderID string) (float64, error) {
	query := `
	SELECT COALESCE(SUM(amount), 0)
	FROM payments
	WHERE order_id::text = $1 AND status = 'paid'
	`

	var total float64
	err := r.q.QueryRow(ctx, query, orderID).Scan(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
}

func (r *repository) InsertPayment(ctx context.Context, p *Payment) error {
	var paidAt *time.Time
	if p.Status == PaymentStatusPaid {
		now := time.Now().UTC()
		paidAt = &now
	}

	queryBothColumns := `
	INSERT INTO payments (order_id, amount, method, payment_method, status, transaction_reference, paid_at)
	VALUES ($1::uuid, $2::numeric, $3::text, $3::text, $4::text, $5::text, $6::timestamp)
	RETURNING id, paid_at, created_at
	`
	if err := r.insertPaymentWithQuery(ctx, queryBothColumns, p, paidAt); err == nil {
		return nil
	} else if !isUndefinedColumnError(err) {
		return err
	}

	queryMethodColumn := `
	INSERT INTO payments (order_id, amount, method, status, transaction_reference, paid_at)
	VALUES ($1::uuid, $2::numeric, $3::text, $4::text, $5::text, $6::timestamp)
	RETURNING id, paid_at, created_at
	`
	if err := r.insertPaymentWithQuery(ctx, queryMethodColumn, p, paidAt); err == nil {
		return nil
	} else if !isUndefinedColumnError(err) && !isNotNullViolationForColumn(err, "payment_method") {
		return err
	}

	queryPaymentMethodColumn := `
	INSERT INTO payments (order_id, amount, payment_method, status, transaction_reference, paid_at)
	VALUES ($1::uuid, $2::numeric, $3::text, $4::text, $5::text, $6::timestamp)
	RETURNING id, paid_at, created_at
	`
	return r.insertPaymentWithQuery(ctx, queryPaymentMethodColumn, p, paidAt)
}

func (r *repository) insertPaymentWithQuery(ctx context.Context, query string, p *Payment, paidAt *time.Time) error {
	var paidAtValue pgtype.Timestamptz
	var createdAtValue pgtype.Timestamptz

	err := r.q.QueryRow(ctx, query, p.OrderID, p.Amount, p.Method, p.Status, p.Reference, paidAt).Scan(
		&p.ID,
		&paidAtValue,
		&createdAtValue,
	)
	if err != nil {
		return err
	}

	if paidAtValue.Valid {
		paidAtTime := paidAtValue.Time
		p.PaidAt = &paidAtTime
	} else {
		p.PaidAt = nil
	}

	if createdAtValue.Valid {
		p.CreatedAt = createdAtValue.Time
	} else {
		p.CreatedAt = time.Now().UTC()
	}

	return nil
}

func isUndefinedColumnError(err error) bool {
	var pgErr *pgconn.PgError
	if ok := errors.As(err, &pgErr); ok {
		return pgErr.Code == "42703"
	}

	return false
}

func isNotNullViolationForColumn(err error, column string) bool {
	var pgErr *pgconn.PgError
	if ok := errors.As(err, &pgErr); ok {
		if pgErr.Code != "23502" {
			return false
		}

		if strings.EqualFold(pgErr.ColumnName, column) {
			return true
		}

		message := strings.ToLower(pgErr.Message)
		return strings.Contains(message, "column \""+strings.ToLower(column)+"\"")
	}

	return false
}

func (r *repository) UpdateOrderStatus(ctx context.Context, orderID string, status string) error {
	query := `UPDATE orders SET status = $1, updated_at = NOW() WHERE id::text = $2`

	cmd, err := r.q.Exec(ctx, query, status, orderID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *repository) GetByOrderID(ctx context.Context, orderID string) ([]Payment, error) {
	queryMethodColumn := `
	SELECT id, order_id::text, amount, method, status, transaction_reference, paid_at, created_at
	FROM payments
	WHERE order_id::text = $1
	ORDER BY id ASC
	`
	queryPaymentMethodColumn := `
	SELECT id, order_id::text, amount, payment_method AS method, status, transaction_reference, paid_at, created_at
	FROM payments
	WHERE order_id::text = $1
	ORDER BY id ASC
	`

	rows, err := r.q.Query(ctx, queryMethodColumn, orderID)
	if err != nil && isUndefinedColumnError(err) {
		rows, err = r.q.Query(ctx, queryPaymentMethodColumn, orderID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPayments(rows)
}

func (r *repository) ListAll(ctx context.Context) ([]Payment, error) {
	queryMethodColumn := `
	SELECT id, order_id::text, amount, method, status, transaction_reference, paid_at, created_at
	FROM payments
	ORDER BY created_at DESC
	`
	queryPaymentMethodColumn := `
	SELECT id, order_id::text, amount, payment_method AS method, status, transaction_reference, paid_at, created_at
	FROM payments
	ORDER BY created_at DESC
	`

	rows, err := r.q.Query(ctx, queryMethodColumn)
	if err != nil && isUndefinedColumnError(err) {
		rows, err = r.q.Query(ctx, queryPaymentMethodColumn)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPayments(rows)
}

func scanPayments(rows pgx.Rows) ([]Payment, error) {

	var payments []Payment
	for rows.Next() {
		var payment Payment
		var paidAtValue pgtype.Timestamptz
		var createdAtValue pgtype.Timestamptz
		if err := rows.Scan(
			&payment.ID,
			&payment.OrderID,
			&payment.Amount,
			&payment.Method,
			&payment.Status,
			&payment.Reference,
			&paidAtValue,
			&createdAtValue,
		); err != nil {
			return nil, err
		}

		if paidAtValue.Valid {
			paidAtTime := paidAtValue.Time
			payment.PaidAt = &paidAtTime
		} else {
			payment.PaidAt = nil
		}

		if createdAtValue.Valid {
			payment.CreatedAt = createdAtValue.Time
		}

		payments = append(payments, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return payments, nil
}

func (r *repository) GetDailyReport(ctx context.Context, day time.Time) (*DailyReport, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	end := start.Add(24 * time.Hour)

	totalsQuery := `
	SELECT COUNT(*), COALESCE(SUM(amount), 0)
	FROM payments
	WHERE created_at >= $1 AND created_at < $2 AND status = 'paid'
	`

	var count int
	var total float64
	if err := r.q.QueryRow(ctx, totalsQuery, start, end).Scan(&count, &total); err != nil {
		return nil, err
	}

	methodsQueryMethodColumn := `
	SELECT method, COUNT(*), COALESCE(SUM(amount), 0)
	FROM payments
	WHERE created_at >= $1 AND created_at < $2 AND status = 'paid'
	GROUP BY method
	ORDER BY method ASC
	`
	methodsQueryPaymentMethodColumn := `
	SELECT payment_method AS method, COUNT(*), COALESCE(SUM(amount), 0)
	FROM payments
	WHERE created_at >= $1 AND created_at < $2 AND status = 'paid'
	GROUP BY payment_method
	ORDER BY payment_method ASC
	`

	rows, err := r.q.Query(ctx, methodsQueryMethodColumn, start, end)
	if err != nil && isUndefinedColumnError(err) {
		rows, err = r.q.Query(ctx, methodsQueryPaymentMethodColumn, start, end)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	methods := make([]PaymentMethodTotal, 0)
	for rows.Next() {
		var m PaymentMethodTotal
		if err := rows.Scan(&m.Method, &m.Count, &m.Total); err != nil {
			return nil, err
		}
		methods = append(methods, m)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &DailyReport{
		Date:           start.Format("2006-01-02"),
		TotalPayments:  count,
		TotalCollected: total,
		ByMethod:       methods,
	}, nil
}

func (r *repository) GetByID(ctx context.Context, id int) (*Payment, error) {
	queryMethodColumn := `
	SELECT id, order_id::text, amount, method, status, transaction_reference, paid_at, created_at
	FROM payments
	WHERE id = $1
	`
	queryPaymentMethodColumn := `
	SELECT id, order_id::text, amount, payment_method AS method, status, transaction_reference, paid_at, created_at
	FROM payments
	WHERE id = $1
	`

	payment, err := r.getByIDWithQuery(ctx, queryMethodColumn, id)
	if err != nil && isUndefinedColumnError(err) {
		payment, err = r.getByIDWithQuery(ctx, queryPaymentMethodColumn, id)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return payment, nil
}

func (r *repository) getByIDWithQuery(ctx context.Context, query string, id int) (*Payment, error) {

	var payment Payment
	var paidAtValue pgtype.Timestamptz
	var createdAtValue pgtype.Timestamptz
	err := r.q.QueryRow(ctx, query, id).Scan(
		&payment.ID,
		&payment.OrderID,
		&payment.Amount,
		&payment.Method,
		&payment.Status,
		&payment.Reference,
		&paidAtValue,
		&createdAtValue,
	)
	if err != nil {
		return nil, err
	}

	if paidAtValue.Valid {
		paidAtTime := paidAtValue.Time
		payment.PaidAt = &paidAtTime
	} else {
		payment.PaidAt = nil
	}

	if createdAtValue.Valid {
		payment.CreatedAt = createdAtValue.Time
	}

	return &payment, nil
}

func (r *repository) RefundPayment(ctx context.Context, paymentID int) error {
	query := `
	UPDATE payments
	SET status = 'refunded', updated_at = NOW()
	WHERE id = $1 AND status = 'paid'
	`

	cmd, err := r.q.Exec(ctx, query, paymentID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
