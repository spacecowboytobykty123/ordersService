package data

import (
	ords "github.com/spacecowboytobykty123/ordersProto/gen/go/orders"
)

type Order struct {
	OrderId     int64
	UserId      int64
	Address     string
	OrderStatus ords.OrderStatus
	CreatedAt   string // плохо но по другому никак
	OrderItems  []*OrderItem
}
