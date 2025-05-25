package grpc

import (
	"context"
	"fmt"
	grpclog "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	bckt "github.com/spacecowboytobykty123/bucketProto/gen/go/bucket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"ordersService/internal/jsonlog"
	"time"
)

type BucketClient struct {
	bucketApi bckt.BucketClient
	log       *jsonlog.Logger
}

func New(ctx context.Context, log *jsonlog.Logger, timeout time.Duration, retriesCount int) (*BucketClient, error) {
	retryOpts := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.NotFound, codes.Aborted, codes.DeadlineExceeded),
		grpcretry.WithMax(uint(retriesCount)),
		grpcretry.WithPerRetryTimeout(timeout),
	}

	logOpts := []grpclog.Option{
		grpclog.WithLogOnEvents(grpclog.PayloadSent, grpclog.PayloadReceived),
	}

	cc, err := grpc.DialContext(ctx, "localhost:2000",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclog.UnaryClientInterceptor(InterceptorLogger(log), logOpts...),
			grpcretry.UnaryClientInterceptor(retryOpts...),
		),
	)

	if err != nil {
		return nil, fmt.Errorf("%s:%w", "grpc.New", err)
	}
	return &BucketClient{
		bucketApi: bckt.NewBucketClient(cc),
		log:       log,
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

func (b *BucketClient) GetBucket(ctx context.Context) *bckt.GetBucketResponse {
	b.log.PrintInfo("getting toys from bucket service", map[string]string{
		"method":  "bucket.grpc.getBucket",
		"service": "bucket",
	})
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		b.log.PrintError(fmt.Errorf("missing metadata"), map[string]string{
			"method":  "bucket.grpc.getBucket",
			"service": "bucket",
		})
		return &bckt.GetBucketResponse{
			Toys:     nil,
			Quantity: 0,
		}
	}
	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		b.log.PrintError(fmt.Errorf("missing authorization token"), nil)
		return &bckt.GetBucketResponse{
			Toys:     nil,
			Quantity: 0,
		}
	}
	outctx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", authHeader[0]))
	b.log.PrintInfo("forwarding JWT token", map[string]string{
		"token": authHeader[0],
	})

	resp, err := b.bucketApi.GetBucket(outctx, &bckt.GetBucketRequest{})
	if err != nil {
		b.log.PrintError(fmt.Errorf("cannot get response from bucket service"), map[string]string{
			"method":  "bucket.grpc.getBucket",
			"service": "bucket",
		})
		return &bckt.GetBucketResponse{}
	}
	return resp
}

func (b *BucketClient) AddToBucket(ctx context.Context, toys []*bckt.ToyBucket) *bckt.AddToBucketResponse {
	b.log.PrintInfo("adding toys to bucket", map[string]string{
		"method":  "bucket.grpc.AddToBucket",
		"service": "bucket",
	})
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		b.log.PrintError(fmt.Errorf("missing metadata"), map[string]string{
			"method":  "bucket.grpc.AddToBucket",
			"service": "bucket",
		})
		return &bckt.AddToBucketResponse{
			Status: bckt.OperationStatus_STATUS_INTERNAL_ERROR,
			Msg:    "Failed to get metadata",
		}
	}
	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		b.log.PrintError(fmt.Errorf("missing authorization token"), nil)
		return &bckt.AddToBucketResponse{
			Status: bckt.OperationStatus_STATUS_UNAUTHORIZED,
			Msg:    "missing token",
		}
	}
	outctx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", authHeader[0]))
	b.log.PrintInfo("forwarding JWT token", map[string]string{
		"token": authHeader[0],
	})

	resp, err := b.bucketApi.AddToBucket(outctx, &bckt.AddToBucketRequest{Toys: toys})
	if err != nil {
		b.log.PrintError(err, map[string]string{
			"method":  "bucket.grpc.AddToBucket",
			"service": "bucket",
		})
	}
	return resp
}

func (b *BucketClient) DeleteFromBucket(ctx context.Context, toysId []int64) *bckt.DelFromBucketResponse {
	b.log.PrintInfo("adding toys to bucket", map[string]string{
		"method":  "bucket.grpc.DeleteFromBucket",
		"service": "bucket",
	})
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		b.log.PrintError(fmt.Errorf("missing metadata"), map[string]string{
			"method":  "bucket.grpc.AddToBucket",
			"service": "bucket",
		})
		return &bckt.DelFromBucketResponse{
			Status: bckt.OperationStatus_STATUS_INTERNAL_ERROR,
			Msg:    "Failed to get metadata",
		}
	}
	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		b.log.PrintError(fmt.Errorf("missing authorization token"), nil)
		return &bckt.DelFromBucketResponse{
			Status: bckt.OperationStatus_STATUS_UNAUTHORIZED,
			Msg:    "missing token",
		}
	}
	outctx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", authHeader[0]))
	b.log.PrintInfo("forwarding JWT token", map[string]string{
		"token": authHeader[0],
	})
	resp, err := b.bucketApi.DelFromBucket(outctx, &bckt.DelFromBucketRequest{ToyId: toysId})
	if err != nil {
		b.log.PrintError(err, map[string]string{
			"method":  "bucket.grpc.AddToBucket",
			"service": "bucket",
		})
	}
	return resp
}
