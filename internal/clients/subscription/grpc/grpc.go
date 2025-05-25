package grpc

import (
	"context"
	"fmt"
	grpclog "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	subs "github.com/spacecowboytobykty123/subsProto/gen/go/subscription"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"ordersService/internal/jsonlog"
	"time"
)

type Client struct {
	subApi subs.SubscriptionClient
	log    *jsonlog.Logger
}

func New(ctx context.Context, log *jsonlog.Logger, addr int, timeout time.Duration, retriesCount int) (*Client, error) {

	retryOpts := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.NotFound, codes.Aborted, codes.DeadlineExceeded),
		grpcretry.WithMax(uint(retriesCount)),
		grpcretry.WithPerRetryTimeout(timeout),
	}

	logOpts := []grpclog.Option{
		grpclog.WithLogOnEvents(grpclog.PayloadReceived, grpclog.PayloadReceived),
	}

	cc, err := grpc.DialContext(ctx, "localhost:3000",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclog.UnaryClientInterceptor(InterceptorLogger(log), logOpts...),
			grpcretry.UnaryClientInterceptor(retryOpts...),
			// NewJWTUnaryInterceptor("dadasdaw2"),
		),
	)

	if err != nil {
		return nil, fmt.Errorf("%s:%w", "grpc.New", err)
	}
	return &Client{
		subApi: subs.NewSubscriptionClient(cc),
		log:    log,
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

func (c *Client) CheckSubscription(ctx context.Context, userID int64) *subs.CheckSubsResponse {
	c.log.PrintInfo("checking subscription", map[string]string{
		"method": "grpc.CheckSubscription",
	})

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		c.log.PrintError(fmt.Errorf("missing metadata"), nil)
		return &subs.CheckSubsResponse{SubStatus: subs.Status_STATUS_INTERNAL_ERROR}
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		c.log.PrintError(fmt.Errorf("missing authorization token"), nil)
		return &subs.CheckSubsResponse{SubStatus: subs.Status_STATUS_INTERNAL_ERROR}
	}

	outCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", authHeader[0]))
	c.log.PrintInfo("forwarding JWT token", map[string]string{
		"token": authHeader[0],
	})

	resp, err := c.subApi.CheckSubscription(outCtx, &subs.CheckSubsRequest{})
	if err != nil {
		println(err.Error())
		return &subs.CheckSubsResponse{SubStatus: subs.Status_STATUS_INTERNAL_ERROR}
	}

	return resp
}

func (c *Client) ExtractFromBalance(ctx context.Context, value int64) *subs.ExtractFromBalanceResponse {
	c.log.PrintInfo("checking subscription", map[string]string{
		"method": "grpc.ExtractFromBalance",
	})

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		c.log.PrintError(fmt.Errorf("missing metadata"), nil)
		return &subs.ExtractFromBalanceResponse{
			OpStatus: subs.Status_STATUS_INTERNAL_ERROR,
			Msg:      "missing metadata from token",
			Left:     0,
		}
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		c.log.PrintError(fmt.Errorf("missing authorization token"), nil)
		return &subs.ExtractFromBalanceResponse{
			OpStatus: subs.Status_STATUS_INTERNAL_ERROR,
			Msg:      "missing auth from token",
			Left:     0,
		}
	}

	outCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", authHeader[0]))
	c.log.PrintInfo("forwarding JWT token", map[string]string{
		"token": authHeader[0],
	})
	resp, err := c.subApi.ExtractFromBalance(outCtx, &subs.ExtractFromBalanceRequest{Value: value})
	if err != nil {
		c.log.PrintError(err, map[string]string{
			"method": "grpc.ExtractFromBalance",
		})
	}
	println("extract")
	return resp

}

func (c *Client) AddFromBalance(ctx context.Context, value int64) *subs.AddToBalanceResponse {

	c.log.PrintInfo("checking subscription", map[string]string{
		"method": "grpc.AddToBalance",
	})

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		c.log.PrintError(fmt.Errorf("missing metadata"), nil)
		return &subs.AddToBalanceResponse{
			OpStatus: subs.Status_STATUS_INTERNAL_ERROR,
			Msg:      "missing metadata from token",
			Left:     0,
		}
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		c.log.PrintError(fmt.Errorf("missing authorization token"), nil)
		return &subs.AddToBalanceResponse{
			OpStatus: subs.Status_STATUS_INTERNAL_ERROR,
			Msg:      "missing auth from token",
			Left:     0,
		}
	}

	outCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", authHeader[0]))
	c.log.PrintInfo("forwarding JWT token", map[string]string{
		"token": authHeader[0],
	})
	resp, err := c.subApi.AddToBalance(outCtx, &subs.AddToBalanceRequest{Value: value})
	if err != nil {
		c.log.PrintError(err, map[string]string{
			"method": "grpc.AddToBalance",
		})
	}
	println("add")
	return resp
}
func NewJWTUnaryInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md := metadata.Pairs("authorization", "Bearer "+token)
		ctx = metadata.NewOutgoingContext(ctx, md)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
