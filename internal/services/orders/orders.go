package orders

import (
	"context"
	ords "github.com/spacecowboytobykty123/ordersProto/gen/go/orders"
	subs "github.com/spacecowboytobykty123/subsProto/gen/go/subscription"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	subsgrpc "ordersService/internal/clients/subscription/grpc"
	"ordersService/internal/contextkeys"
	"ordersService/internal/data"
	"ordersService/internal/jsonlog"
	"time"
)

type Orders struct {
	log            *jsonlog.Logger
	ordersProvider orderProvider
	tokenTTL       time.Duration
	subsClient     *subsgrpc.Client
}

type orderProvider interface {
	CreateOrder(ctx context.Context, toys []*data.OrderItem, address string, userID int64) (int64, ords.OperationStatus, string)
	DeleteOrder(ctx context.Context, orderId int64, userID int64) ords.OperationStatus
	GetOrder(ctx context.Context, orderId int64, userID int64) data.Order
	ChangeOrderStatus(ctx context.Context, orderId int64, newStatus ords.OrderStatus, userID int64) ords.OperationStatus
	OrderHistory(ctx context.Context, page int32, pageSize int32, userID int64) ([]*data.Order, int32)
}

func New(log *jsonlog.Logger, orderProvider orderProvider, tokenTTL time.Duration, subsClient *subsgrpc.Client) *Orders {
	return &Orders{
		log:            log,
		ordersProvider: orderProvider,
		tokenTTL:       tokenTTL,
		subsClient:     subsClient,
	}
}

func (o *Orders) CreateOrder(ctx context.Context, toys []*data.OrderItem, address string) (int64, ords.OperationStatus, string) {

	userID, err := getUserFromContext(ctx)
	if err != nil {
		return 0, ords.OperationStatus_STATUS_INVALID_USER, "invalid user!"
	}

	println(userID)

	subsResp := o.subsClient.CheckSubscription(ctx, userID)
	if subsResp.SubStatus != subs.Status_STATUS_SUBSCRIBED {
		return 0, ords.OperationStatus_STATUS_INVALID_USER, "user is not subscribed!"
	}
	orderID, opStatus, msg := o.ordersProvider.CreateOrder(ctx, toys, address, userID)
	if opStatus != ords.OperationStatus_STATUS_OK {
		o.log.PrintError(o.MapStatusToError(opStatus), map[string]string{
			"method": "orders.CreateOrder",
		})

		return 0, opStatus, msg
	}
	return orderID, opStatus, msg
}

func (o *Orders) DeleteOrder(ctx context.Context, orderId int64) ords.OperationStatus {
	userID, err := getUserFromContext(ctx)
	if err != nil {
		return ords.OperationStatus_STATUS_INVALID_USER
	}
	// TODO: maybe admin service to delete order

	opStatus := o.ordersProvider.DeleteOrder(ctx, orderId, userID)
	if opStatus != ords.OperationStatus_STATUS_OK {
		o.log.PrintError(o.MapStatusToError(opStatus), map[string]string{
			"method": "orders.DeleteOrder",
		})

		return opStatus
	}
	return opStatus

}

func (o *Orders) GetOrder(ctx context.Context, orderId int64) data.Order {
	userID, err := getUserFromContext(ctx)
	if err != nil {
		return data.Order{}
	}

	order := o.ordersProvider.GetOrder(ctx, orderId, userID)

	return order

	//TODO implement me
}

func (o *Orders) ChangeOrderStatus(ctx context.Context, orderId int64, newStatus ords.OrderStatus) ords.OperationStatus {
	userID, err := getUserFromContext(ctx)
	if err != nil {
		return ords.OperationStatus_STATUS_INVALID_USER
	}
	// TODO: maybe admin service to change order

	opStatus := o.ordersProvider.ChangeOrderStatus(ctx, orderId, newStatus, userID)
	if opStatus != ords.OperationStatus_STATUS_OK {
		o.log.PrintError(o.MapStatusToError(opStatus), map[string]string{
			"method": "orders.ChangeOrderStatus",
		})

		return opStatus
	}
	return opStatus

}

func (o *Orders) OrderHistory(ctx context.Context, page int32, pageSize int32) ([]*data.Order, int32) {
	userID, err := getUserFromContext(ctx)
	if err != nil {
		return []*data.Order{}, 0
	}

	orders, totalPages := o.ordersProvider.OrderHistory(ctx, page, pageSize, userID)

	if orders == nil {
		o.log.PrintError(status.Error(codes.NotFound, "failed to fetch orders"), map[string]string{
			"method": "orders.OrderHistory",
		})
		return nil, 0
	}

	return orders, totalPages

}

func getUserFromContext(ctx context.Context) (int64, error) {
	val := ctx.Value(contextkeys.UserIDKey)
	userID, ok := val.(int64)
	if !ok {
		return 0, status.Error(codes.Unauthenticated, "user id is missing or invalid in context")
	}

	return userID, nil

}

func (o *Orders) MapStatusToError(code ords.OperationStatus) error {
	switch code {
	case ords.OperationStatus_STATUS_INVALID_USER:
		return status.Error(codes.InvalidArgument, "invalid user!")
	case ords.OperationStatus_STATUS_INVALID_TOY:
		return status.Error(codes.InvalidArgument, "invalid toy!")
	case ords.OperationStatus_STATUS_INVALID_ORDER:
		return status.Error(codes.InvalidArgument, "invalid order!")
	case ords.OperationStatus_STATUS_INVALID_ADDRESS:
		return status.Error(codes.InvalidArgument, "invalid address!")
	default:
		return status.Error(codes.Internal, "internal error!")
	}

}
