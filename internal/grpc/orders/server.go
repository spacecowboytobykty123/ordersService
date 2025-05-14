package orders

import (
	"context"
	"fmt"
	ords "github.com/spacecowboytobykty123/ordersProto/gen/go/orders"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"ordersService/internal/data"
	"ordersService/internal/validator"
	"ordersService/storage/postgres"
	"strings"
)

type serverAPI struct {
	ords.UnimplementedOrdersServer
	orders Orders
}

type Orders interface {
	CreateOrder(ctx context.Context, toys []*data.OrderItem, address string) (int64, ords.OperationStatus, string)
	DeleteOrder(ctx context.Context, orderId int64) ords.OperationStatus
	GetOrder(ctx context.Context, orderId int64) data.Order
	ChangeOrderStatus(ctx context.Context, orderId int64, newStatus ords.OrderStatus) ords.OperationStatus
	OrderHistory(ctx context.Context, page int32, pageSize int32) ([]*data.Order, int32)
}

func Register(gRPC *grpc.Server, orders Orders) {
	ords.RegisterOrdersServer(gRPC, &serverAPI{orders: orders})
}

func (s *serverAPI) CreateOrder(ctx context.Context, r *ords.CreateOrderRequest) (*ords.CreateOrderResponse, error) {
	v := validator.New()

	toys := r.GetToys()
	address := r.GetAddress()

	if postgres.ValidateOrder(v, toys, address); !v.Valid() {
		return nil, collectErrors(v)
	}
	validToys := ToDomainOrderItems(toys)

	orderId, opStatus, errMsg := s.orders.CreateOrder(ctx, validToys, address)

	return &ords.CreateOrderResponse{
		OrderId:      orderId,
		OrderStatus:  opStatus,
		ErrorMessage: errMsg,
	}, nil

}

func (s *serverAPI) DeleteOrder(ctx context.Context, r *ords.DeleteOrderRequest) (*ords.DeleteOrderResponse, error) {
	v := validator.New()
	order_id := r.GetOrderId()
	v.Check(order_id != 0, "text", "order_id must be provided!")
	if !v.Valid() {
		return nil, collectErrors(v)
	}

	opStatus := s.orders.DeleteOrder(ctx, order_id)

	if opStatus != ords.OperationStatus_STATUS_OK {
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &ords.DeleteOrderResponse{Status: opStatus}, nil

}

func (s *serverAPI) GetOrder(ctx context.Context, r *ords.GetOrderRequest) (*ords.GetOrderResponse, error) {
	v := validator.New()
	order_id := r.GetOrderId()
	v.Check(order_id != 0, "text", "order_id must be provided!")
	if !v.Valid() {
		return nil, collectErrors(v)
	}

	order := s.orders.GetOrder(ctx, order_id)

	validOrder := fromDataToProtocSingleOrder(order)

	return &ords.GetOrderResponse{Order: validOrder}, nil
}

func (s *serverAPI) ChangeOrderStatus(ctx context.Context, r *ords.ChangeOrderStatusRequest) (*ords.ChangeOrderStatusResponse, error) {
	v := validator.New()

	order_id := r.GetOrderId()
	newOrderStatus := r.GetNewOrderStatus()
	v.Check(order_id != 0, "text", "order id must be provided")
	if !v.Valid() {
		return nil, collectErrors(v)
	}

	opStatus := s.orders.ChangeOrderStatus(ctx, order_id, newOrderStatus)

	return &ords.ChangeOrderStatusResponse{Status: opStatus}, nil
}

func (s *serverAPI) ListOrderHistory(ctx context.Context, r *ords.OrderHistoryRequest) (*ords.OrderHistoryResponse, error) {
	v := validator.New()

	pages := r.GetPage()
	pageSize := r.GetPageSize()

	if postgres.ValidateFilters(v, pages, pageSize); !v.Valid() {
		return nil, collectErrors(v)
	}

	orders, totalPages := s.orders.OrderHistory(ctx, pages, pageSize)

	ordersList := ToDomainOrder(orders)

	if len(ordersList) == 0 {
		return nil, status.Error(codes.NotFound, "You have no orders! Buy some!")
	}

	return &ords.OrderHistoryResponse{
		Orders:     ordersList,
		TotalPages: totalPages,
	}, nil
}

func collectErrors(v *validator.Validator) error {
	var b strings.Builder
	for field, msg := range v.Errors {
		fmt.Fprintf(&b, "%s:%s; ", field, msg)
	}
	return status.Error(codes.InvalidArgument, b.String())
}

func ToDomainOrderItems(orders []*ords.OrderItem) []*data.OrderItem {
	domainItems := make([]*data.OrderItem, 0, len(orders))
	for _, o := range orders {
		domainItems = append(domainItems, &data.OrderItem{
			ToyId:    o.ToyId,
			ToyName:  o.ToyName,
			Quantity: o.Quantity,
		})
	}
	return domainItems
}

func fromDataToProtocSingleOrder(order data.Order) *ords.Order {
	return &ords.Order{
		OrderId:     order.OrderId,
		UserId:      order.UserId,
		Address:     order.Address,
		OrderStatus: order.OrderStatus,
		CreatedAt:   order.CreatedAt,
		Items:       fromOrderItemDataToProto(order.OrderItems),
	}
}

func fromDataToProtocSingleOrderItem(order *data.OrderItem) *ords.OrderItem {
	return &ords.OrderItem{
		ToyId:    order.ToyId,
		ToyName:  order.ToyName,
		Quantity: order.Quantity,
	}
}

func ToDomainOrder(orders []*data.Order) []*ords.Order {
	domainOrders := make([]*ords.Order, 0, len(orders))
	for _, o := range orders {
		domainOrders = append(domainOrders, &ords.Order{
			OrderId:     o.OrderId,
			UserId:      o.UserId,
			Address:     o.Address,
			OrderStatus: o.OrderStatus,
			CreatedAt:   o.CreatedAt,
			Items:       fromOrderItemDataToProto(o.OrderItems),
		})
	}
	return domainOrders
}

func fromOrderItemDataToProto(orders []*data.OrderItem) []*ords.OrderItem {
	domainItems := make([]*ords.OrderItem, 0, len(orders))
	for _, o := range orders {
		domainItems = append(domainItems, &ords.OrderItem{
			ToyId:    o.ToyId,
			ToyName:  o.ToyName,
			Quantity: o.Quantity,
		})
	}
	return domainItems
}
