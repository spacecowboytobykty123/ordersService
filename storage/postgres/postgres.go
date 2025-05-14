package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	ords "github.com/spacecowboytobykty123/ordersProto/gen/go/orders"
	"math"
	"ordersService/internal/data"
	"ordersService/internal/validator"
	"strings"
	"time"
)

type Storage struct {
	db *sql.DB
}

const (
	emptyValue = 0
)

type StorageDetails struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	MaxIdleTime  string
}

func OpenDB(details StorageDetails) (*Storage, error) {
	db, err := sql.Open("postgres", details.DSN)

	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(details.MaxOpenConns)
	db.SetMaxIdleConns(details.MaxIdleConns)

	duration, err := time.ParseDuration(details.MaxIdleTime)

	db.SetConnMaxIdleTime(duration)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}
	return &Storage{
		db: db,
	}, err
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func ValidateFilters(v *validator.Validator, page int32, pageSize int32) {
	v.Check(page != emptyValue, "text", "page number is required!")
	v.Check(pageSize != emptyValue, "text", "page sizing number is required!")
}

func ValidateOrder(v *validator.Validator, toys []*ords.OrderItem, address string) {
	v.Check(len(toys) > 0, "text", "at least 1 toy should be included!")
	v.Check(strings.TrimSpace(address) != "", "address", "delivery address is required!")

	for i, item := range toys {
		prefix := fmt.Sprintf("toys[%d]", i)

		v.Check(item.ToyId != emptyValue, prefix+".toy_id", "toy id must be provided")
		v.Check(strings.TrimSpace(item.ToyName) != "", prefix+".toy_name", "toy name should be provided")
		v.Check(item.Quantity != emptyValue, prefix+".quantity", "quantity should be provided")
	}
}

func (s *Storage) CreateOrder(ctx context.Context, toys []*data.OrderItem, address string, userID int64) (int64, ords.OperationStatus, string) {
	query := `INSERT INTO orders (user_id, address) 
VALUES ($1, $2)
RETURNING id`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	args := []any{userID, address}
	var ordID int64

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, ords.OperationStatus_STATUS_INTERNAL_ERROR, "could not start transaction"
	}

	status := tx.QueryRowContext(ctx, query, args...).Scan(&ordID)
	if status != nil {

		return 0, ords.OperationStatus_STATUS_INTERNAL_ERROR, "internal error! order was not created"
	}
	defer tx.Rollback()

	query1 := `INSERT INTO order_items (order_id, toy_id, toy_name, quantity)
	VALUES ($1, $2, $3, $4)
	RETURNING id`

	for _, toy := range toys {
		_, err := tx.ExecContext(ctx, query1, ordID, toy.ToyId, toy.ToyName, toy.Quantity)
		if err != nil {
			tx.Rollback()
			return 0, ords.OperationStatus_STATUS_INTERNAL_ERROR, "failed to create a toy"
		}

	}

	if err := tx.Commit(); err != nil {
		return 0, ords.OperationStatus_STATUS_INTERNAL_ERROR, "could not commit transaction"
	}

	return ordID, ords.OperationStatus_STATUS_OK, "order creation was successful"

}

func (s *Storage) DeleteOrder(ctx context.Context, orderId int64, userId int64) ords.OperationStatus {
	query := `DELETE FROM orders 
	WHERE user_id = $1 AND id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	results, err := s.db.ExecContext(ctx, query, userId, orderId)
	if err != nil {
		println(err.Error())
		return ords.OperationStatus_STATUS_INTERNAL_ERROR
	}
	rowsAffected, err := results.RowsAffected()
	if err != nil {
		println(err.Error())
		return ords.OperationStatus_STATUS_INTERNAL_ERROR
	}
	if rowsAffected == 0 {
		println("no matching order found")
		return ords.OperationStatus_STATUS_INVALID_ORDER
	}

	return ords.OperationStatus_STATUS_OK
}

func (s *Storage) GetOrder(ctx context.Context, orderId int64, userId int64) data.Order {
	query := `SELECT id, user_id, address, status, created_at FROM orders
WHERE id = $1 AND user_id = $2
`
	query1 := `SELECT id, toy_name, quantity FROM order_items
WHERE order_id = $1
`

	println("GetOrder.postgres")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	order := data.Order{}

	var statusStr string
	err := s.db.QueryRowContext(ctx, query, orderId, userId).Scan(
		&order.OrderId,
		&order.UserId,
		&order.Address,
		&statusStr,
		&order.CreatedAt,
	)

	if err != nil {
		println(err.Error())
		if errors.Is(err, sql.ErrNoRows) {
			return data.Order{} // or nil + status
		}
		return data.Order{}
	}

	status := ords.OrderStatus(ords.OrderStatus_value[statusStr])
	println(status)

	order.OrderStatus = status

	toyRows, err := s.db.QueryContext(ctx, query1, orderId)
	defer toyRows.Close()

	toys := []*data.OrderItem{}

	for toyRows.Next() {
		var toy data.OrderItem

		err := toyRows.Scan(
			&toy.ToyId,
			&toy.ToyName,
			&toy.Quantity,
		)
		if err != nil {
			println(err.Error())
			return data.Order{}
		}

		toys = append(toys, &toy)
	}
	if err = toyRows.Err(); err != nil {
		return data.Order{}
	}

	order.OrderItems = toys
	return order
}

func (s *Storage) ChangeOrderStatus(ctx context.Context, orderId int64, newStatus ords.OrderStatus, userId int64) ords.OperationStatus {
	println("changeOrderDBPart")
	query := `UPDATE orders
SET status = $1
WHERE id = $2
RETURNING id
`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var ordId int64

	statusStr := newStatus.String()

	err := s.db.QueryRowContext(ctx, query, statusStr, orderId).Scan(&ordId)
	if err != nil {
		println(err.Error())
		switch {

		case errors.Is(err, sql.ErrNoRows):
			return ords.OperationStatus_STATUS_INVALID_ORDER
		default:
			return ords.OperationStatus_STATUS_INTERNAL_ERROR
		}
	}
	return ords.OperationStatus_STATUS_OK
}

func (s *Storage) OrderHistory(ctx context.Context, page int32, pageSize int32, userId int64) ([]*data.Order, int32) {
	offset := (page - 1) * pageSize
	query := `SELECT id, address, status, created_at from orders
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	args := []any{userId, pageSize, offset}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		println(err.Error())
		return nil, 0
	}
	defer rows.Close()

	orders := []*data.Order{}

	for rows.Next() {
		var order data.Order
		order.UserId = userId
		var statusStr string

		err := rows.Scan(
			&order.OrderId,
			&order.Address,
			&statusStr,
			&order.CreatedAt,
		)

		status := ords.OrderStatus(ords.OrderStatus_value[statusStr])
		println(status)

		order.OrderStatus = status

		println(order.Address)

		if err != nil {
			println(err.Error())
			return nil, 0
		}

		orders = append(orders, &order)
	}

	if err = rows.Err(); err != nil {
		println(err.Error())
		return nil, 0
	}

	var total int32

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE user_id= $1`, userId).Scan(&total)
	if err != nil {
		println(err.Error())
		return nil, 0
	}

	totalPages := int32(math.Ceil(float64(total) / float64(pageSize)))

	return orders, totalPages
}
