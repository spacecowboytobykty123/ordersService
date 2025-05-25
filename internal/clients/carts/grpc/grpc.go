package grpc

import (
	"context"
	"fmt"
	grpclog "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	cart_v1_crt "github.com/spacecowboytobykty123/protoCart/gen/go/cart"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"ordersService/internal/jsonlog"
	"time"
)

type CartClient struct {
	cartApi cart_v1_crt.CartClient
	log     *jsonlog.Logger
}

func New(ctx context.Context, log *jsonlog.Logger, timeout time.Duration, retriesCount int) (*CartClient, error) {
	retryOpts := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.NotFound, codes.Aborted, codes.DeadlineExceeded),
		grpcretry.WithMax(uint(retriesCount)),
		grpcretry.WithPerRetryTimeout(timeout),
	}

	logOpts := []grpclog.Option{
		grpclog.WithLogOnEvents(grpclog.PayloadSent, grpclog.PayloadReceived),
	}

	cc, err := grpc.DialContext(ctx, "localhost:5000",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclog.UnaryClientInterceptor(InterceptorLogger(log), logOpts...),
			grpcretry.UnaryClientInterceptor(retryOpts...),
		),
	)

	if err != nil {
		return nil, fmt.Errorf("%s:%w", "grpc.New", err)
	}

	return &CartClient{
		cartApi: cart_v1_crt.NewCartClient(cc),
		log:     log,
	}, nil

}

func InterceptorLogger(logger *jsonlog.Logger) grpclog.Logger {
	return grpclog.LoggerFunc(func(ctx context.Context, lvl grpclog.Level, msg string, fields ...any) {
		logger.PrintInfo(msg, map[string]string{
			"lvl": string(lvl),
		})
	},
	)
}

func (c *CartClient) GetCart(ctx context.Context) *cart_v1_crt.GetCartResponse {
	c.log.PrintInfo("getting toy from toy microservice", map[string]string{
		"method":  "carts.grpc.GetCart",
		"service": "Carts",
	})

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		c.log.PrintError(fmt.Errorf("missing metadata"), map[string]string{
			"method":  "carts.grpc.GetCart",
			"service": "Carts",
		})
		return &cart_v1_crt.GetCartResponse{}
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		c.log.PrintError(fmt.Errorf("missing authorization token"), nil)
		return &cart_v1_crt.GetCartResponse{}
	}

	outctx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", authHeader[0]))
	c.log.PrintInfo("forwarding JWT token", map[string]string{
		"token": authHeader[0],
	})

	resp, err := c.cartApi.GetCart(outctx, &cart_v1_crt.GetCartRequest{})
	if err != nil {
		c.log.PrintError(fmt.Errorf("cannot get response from cart service"), map[string]string{
			"method":  "carts.grpc.GetCart",
			"service": "Carts",
		})
		return &cart_v1_crt.GetCartResponse{}
	}

	return resp
}
