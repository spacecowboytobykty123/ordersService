package grpcapp

import (
	"context"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net"
	"ordersService/internal/contextkeys"
	ordsgrpc "ordersService/internal/grpc/orders"
	"ordersService/internal/jsonlog"
	"strings"
)
import "google.golang.org/grpc"

type App struct {
	Log        *jsonlog.Logger
	GRPCServer *grpc.Server
	Port       int
}

func UnaryJWTInterceptor(secret []byte) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md["authorization"]
		if len(authHeader) == 0 || !strings.HasPrefix(authHeader[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid authorization header")
		}

		tokenStr := strings.TrimPrefix(authHeader[0], "Bearer ")
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return secret, nil
		})

		if err != nil || !token.Valid {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, status.Error(codes.Internal, "cannot parse claims")
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			return nil, status.Error(codes.Internal, "user ID not found or invalid type in token")
		}

		// Convert to int64
		userID := int64(userIDFloat)
		if err != nil {
			return nil, status.Error(codes.Internal, "invalid user ID format")
		}

		ctx = context.WithValue(ctx, contextkeys.UserIDKey, userID)
		return handler(ctx, req)

	}
}

func New(log *jsonlog.Logger, port int, ordersService ordsgrpc.Orders) *App {
	gRPCServer := grpc.NewServer(
		grpc.UnaryInterceptor(UnaryJWTInterceptor([]byte("test-secret"))),
	)

	ordsgrpc.Register(gRPCServer, ordersService)

	return &App{
		Log:        log,
		GRPCServer: gRPCServer,
		Port:       port,
	}

}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.Port))
	if err != nil {
		return fmt.Errorf("%s: %w", "grpcapp.Run", err)
	}
	a.Log.PrintInfo("Running GRPC server", nil)

	if err := a.GRPCServer.Serve(l); err != nil {
		return fmt.Errorf("%s:%d", "grpcapp.Run", err)
	}
	return nil
}

func (a *App) Stop() {
	a.GRPCServer.GracefulStop()
}
