package orders

import (
	"context"
	"fmt"
	bckt "github.com/spacecowboytobykty123/bucketProto/gen/go/bucket"
	ords "github.com/spacecowboytobykty123/ordersProto/gen/go/orders"
	cart_v1_crt "github.com/spacecowboytobykty123/protoCart/gen/go/cart"
	subs "github.com/spacecowboytobykty123/subsProto/gen/go/subscription"
	"github.com/spacecowboytobykty123/toysProto/gen/go/toys"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	bcktgrpc "ordersService/internal/clients/bucket/grpc"
	"ordersService/internal/clients/carts/grpc"
	subsgrpc "ordersService/internal/clients/subscription/grpc"
	toygrpc "ordersService/internal/clients/toys/grpc"
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
	cartClient     *grpc.CartClient
	toyClient      *toygrpc.ToyClient
	bucketClient   *bcktgrpc.BucketClient
}

type orderProvider interface {
	CreateOrder(ctx context.Context, toys []*data.OrderItem, address string, userID int64) (int64, ords.OperationStatus, string)
	DeleteOrder(ctx context.Context, orderId int64, userID int64) ords.OperationStatus
	GetOrder(ctx context.Context, orderId int64, userID int64) data.Order
	ChangeOrderStatus(ctx context.Context, orderId int64, newStatus ords.OrderStatus, userID int64) ords.OperationStatus
	OrderHistory(ctx context.Context, page int32, pageSize int32, userID int64) ([]*data.Order, int32)
	OrderBack(ctx context.Context, toys []*data.OrderItem, address string, userID int64) (int64, ords.OperationStatus, string)
}

func New(log *jsonlog.Logger, orderProvider orderProvider, tokenTTL time.Duration, subsClient *subsgrpc.Client, cartClient *grpc.CartClient, toyClient *toygrpc.ToyClient, bucketClient *bcktgrpc.BucketClient) *Orders {
	return &Orders{
		log:            log,
		ordersProvider: orderProvider,
		tokenTTL:       tokenTTL,
		subsClient:     subsClient,
		cartClient:     cartClient,
		toyClient:      toyClient,
		bucketClient:   bucketClient,
	}
}

func (o *Orders) CreateOrder(ctx context.Context, toys []*data.OrderItem, address string) (int64, ords.OperationStatus, string) {

	userID, err := getUserFromContext(ctx)
	if err != nil {
		return 0, ords.OperationStatus_STATUS_INVALID_USER, "invalid user!"
	}

	println(userID)

	subsResp := o.subsClient.CheckSubscription(ctx, userID)
	fmt.Printf("%T\n", subsResp.SubStatus)
	if subsResp.SubStatus != subs.Status_STATUS_SUBSCRIBED {
		return 0, ords.OperationStatus_STATUS_INVALID_USER, "user is not subscribed!"
	}

	cartResp := o.cartClient.GetCart(ctx)
	if cartResp.TotalQuantity == 0 {
		return 0, ords.OperationStatus_STATUS_INTERNAL_ERROR, "cart is empty!"
	}
	var toyIds []int64
	for _, o := range cartResp.Items {
		println(o.ToyId)
		toyIds = append(toyIds, o.ToyId)
	}
	println(toyIds)

	toyResp := o.toyClient.GetToysByIds(ctx, toyIds)
	validToys := mapCartItemsToOrders(toyResp.Toy, cartResp.Items)

	orderID, opStatus, msg := o.ordersProvider.CreateOrder(ctx, validToys, address, userID)

	var allValue int64
	for _, toy := range validToys {
		allValue += toy.Value
	}

	if opStatus == ords.OperationStatus_STATUS_OK {
		subsReps := o.subsClient.ExtractFromBalance(ctx, allValue)
		println("before check")
		println(subsReps.Msg)
		if subsReps.OpStatus != subs.Status_STATUS_OK {
			return 0, ords.OperationStatus_STATUS_INTERNAL_ERROR, "could not extract money from limit!"
		}
		println("after")
	}

	if opStatus != ords.OperationStatus_STATUS_OK {
		o.log.PrintError(o.MapStatusToError(opStatus), map[string]string{
			"method": "orders.CreateOrder",
		})

		return 0, opStatus, msg
	}

	return orderID, opStatus, msg
}

func (o *Orders) OrderBack(ctx context.Context, toys []*data.OrderItem, address string) (int64, ords.OperationStatus, string) {
	println("orders business part")
	userID, err := getUserFromContext(ctx)
	if err != nil {
		return 0, ords.OperationStatus_STATUS_INVALID_USER, "invalid user!"
	}

	println(userID)

	subsResp := o.subsClient.CheckSubscription(ctx, userID)
	if subsResp.SubStatus != subs.Status_STATUS_SUBSCRIBED {
		return 0, ords.OperationStatus_STATUS_INVALID_USER, "user is not subscribed!"
	}

	bucketResp := o.bucketClient.GetBucket(ctx)
	if bucketResp.Quantity == 0 {
		return 0, ords.OperationStatus_STATUS_INTERNAL_ERROR, "bucket is empty!"
	}

	filteredToys := idsInBucket(bucketResp.Toys, toys)
	println(len(filteredToys))

	orderId, opStatus, msg := o.ordersProvider.OrderBack(ctx, filteredToys, address, userID)

	var allValue int64
	var toysId []int64

	for _, t := range filteredToys {
		allValue += t.Value
		toysId = append(toysId, t.ToyId)
	}
	println(allValue)

	if opStatus == ords.OperationStatus_STATUS_OK {
		subsReps := o.subsClient.AddFromBalance(ctx, allValue)
		if subsReps.OpStatus != subs.Status_STATUS_OK {
			return 0, ords.OperationStatus_STATUS_INTERNAL_ERROR, "could not give money to limit back!"
		}

		delFromBckrResp := o.bucketClient.DeleteFromBucket(ctx, toysId)
		if delFromBckrResp.Status != bckt.OperationStatus_STATUS_OK {
			return 0, ords.OperationStatus_STATUS_INTERNAL_ERROR, "could not delete toy from bucket!"
		}

	}

	if opStatus != ords.OperationStatus_STATUS_OK {
		return 0, opStatus, "could not create order back"
	}
	return orderId, opStatus, msg

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
	println("buz")

	userID, err := getUserFromContext(ctx)
	if err != nil {
		return data.Order{}
	}

	order := o.ordersProvider.GetOrder(ctx, orderId, userID)

	var toyList []int64

	for _, item := range order.OrderItems {
		toyList = append(toyList, item.ToyId)
	}

	toyResp := o.toyClient.GetToysByIds(ctx, toyList)

	// TODO: тут добавить value
	toyMap := make(map[int64]*toys.ToySummary) // Adjust Toy type to match your toyResp.Toy type
	for _, toy := range toyResp.Toy {
		toyMap[toy.Id] = toy
	}

	// Safely assign data to order items
	for _, item := range order.OrderItems {
		if toy, ok := toyMap[item.ToyId]; ok {
			item.ImageURL = toy.ImageUrl
			item.Value = toy.Value
		} else {
			// Optional: handle missing toy info
			fmt.Printf("Toy info not found for ToyID: %d\n", item.ToyId)
		}
	}

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
	if newStatus == ords.OrderStatus_STATUS_DELIVERED {
		order := o.ordersProvider.GetOrder(ctx, orderId, userID)

		var toyBucket []*bckt.ToyBucket

		for _, toy := range order.OrderItems {
			toyBucket = append(toyBucket, &bckt.ToyBucket{
				ToyId:    toy.ToyId,
				Quantity: toy.Quantity,
			})
		}

		bucketResp := o.bucketClient.AddToBucket(ctx, toyBucket)
		if bucketResp.Status != bckt.OperationStatus_STATUS_OK {
			o.log.PrintError(fmt.Errorf("could not save toys to users bucket"), map[string]string{
				"method":  "orders.ChangeOrderStatus",
				"service": "bucket",
			})
			return ords.OperationStatus_STATUS_INTERNAL_ERROR
		}

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

func mapCartItemsToOrders(items []*toys.ToySummary, qtys []*cart_v1_crt.CartItem) []*data.OrderItem {
	println("orders.mapcart")
	toyList := make([]*data.OrderItem, 0, len(items))
	for i := range items {
		toyList = append(toyList, &data.OrderItem{
			ToyId:    items[i].Id,
			ToyName:  items[i].Title,
			Quantity: qtys[i].Quantity,
			Value:    items[i].Value,
			ImageURL: items[i].ImageUrl,
		})
		fmt.Printf("Item %d -> ToyID: %d, ImageURL: '%s'\n", i, items[i].Id, items[i].ImageUrl)
	}
	println(toyList)
	return toyList
}

func idsInBucket(bucketToys []*bckt.Toy, toys []*data.OrderItem) []*data.OrderItem {
	println("idsinBucket")
	bucketMap := make(map[int64]struct{})
	for _, b := range bucketToys {
		bucketMap[b.ToyId] = struct{}{}
	}

	// Filter toys that appear in the bucket
	var result []*data.OrderItem
	for _, t := range toys {
		if _, found := bucketMap[t.ToyId]; found {
			result = append(result, t)
		}
	}

	return result
}
