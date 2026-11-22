package cmd

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/codearena-platform/codearena-cli/internal/auth"
	"github.com/codearena-platform/codearena-cli/internal/project"
	pb "github.com/codearena-platform/codearena-core/pkg/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestLoginCommandReturnsErrorsInsteadOfExiting(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing client id", []string{"--endpoint", "https://localhost:8080"}, "--client-id"},
		{"plaintext without opt-in", []string{"--client-id", "cli", "--endpoint", "http://localhost:8080"}, "--insecure"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newLoginCmd()
			cmd.SetArgs(tc.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Execute error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestLoginDefaultEndpointUsesGatewayPortAndTLS(t *testing.T) {
	flag := newLoginCmd().Flags().Lookup("endpoint")
	if flag == nil || flag.DefValue != auth.DefaultEndpoint {
		t.Fatalf("endpoint default = %v, want %s", flag, auth.DefaultEndpoint)
	}
	if !strings.HasPrefix(auth.DefaultEndpoint, "https://") || !strings.HasSuffix(auth.DefaultEndpoint, ":8080") {
		t.Fatalf("default endpoint %q must use TLS and port 8080", auth.DefaultEndpoint)
	}
}

func TestTransportCredentials(t *testing.T) {
	creds, err := transportCredentials(false)
	if err != nil {
		t.Fatal(err)
	}
	if got := creds.Info().SecurityProtocol; got != "tls" {
		t.Fatalf("default protocol = %q, want tls", got)
	}
	creds, err = transportCredentials(true)
	if err != nil {
		t.Fatal(err)
	}
	if got := creds.Info().SecurityProtocol; got == "tls" {
		t.Fatalf("dev opt-in protocol = %q, want plaintext", got)
	}
}

type fakeMatchServer struct {
	pb.UnimplementedMatchServiceServer
	request *pb.RegisterBotRequest
	auth    string
	fail    bool
}

func (s *fakeMatchServer) RegisterBot(ctx context.Context, req *pb.RegisterBotRequest) (*pb.RegisterBotResponse, error) {
	s.request = req
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get("authorization"); len(values) > 0 {
			s.auth = values[0]
		}
	}
	if s.fail {
		return &pb.RegisterBotResponse{Success: false, Message: "plagiarism detected"}, nil
	}
	return &pb.RegisterBotResponse{Success: true, BotId: "bot-1", VersionId: "v-1", Message: "ok"}, nil
}

func startFakeGRPC(t *testing.T, fake *fakeMatchServer) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	pb.RegisterMatchServiceServer(server, fake)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return listener.Addr().String()
}

func TestPushBotDerivesIdentityFromTokenAndReportsErrors(t *testing.T) {
	fake := &fakeMatchServer{}
	addr := startFakeGRPC(t, fake)
	cfg := &auth.Config{Token: "jwt-token", ClientID: "cli"}
	manifest := &project.Manifest{Name: "my-bot", Version: "1.0.0", Language: "typescript"}
	var out bytes.Buffer

	if err := pushBot(context.Background(), addr, true, cfg, manifest, []byte("zip-bytes"), &out); err != nil {
		t.Fatalf("pushBot: %v", err)
	}
	if fake.request == nil {
		t.Fatal("RegisterBot was not called")
	}
	if fake.request.UserId != "" {
		t.Fatalf("UserId = %q, want empty (server derives identity)", fake.request.UserId)
	}
	if fake.request.Name != "my-bot" || fake.request.Version != "1.0.0" || fake.request.Language != "typescript" {
		t.Fatalf("unexpected request: %+v", fake.request)
	}
	if fake.request.SourceCode == "" {
		t.Fatal("source code payload missing")
	}
	if fake.auth != "Bearer jwt-token" {
		t.Fatalf("authorization metadata = %q", fake.auth)
	}
	if !strings.Contains(out.String(), "bot-1") {
		t.Fatalf("output = %q", out.String())
	}

	fake.fail = true
	if err := pushBot(context.Background(), addr, true, cfg, manifest, nil, &out); err == nil || !strings.Contains(err.Error(), "plagiarism detected") {
		t.Fatalf("server rejection error = %v", err)
	}
	if err := pushBot(context.Background(), addr, true, &auth.Config{}, manifest, nil, &out); err == nil {
		t.Fatal("missing token accepted")
	}

	// A connection failure must be returned, never silently ignored.
	unusedListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadAddr := unusedListener.Addr().String()
	unusedListener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pushBot(ctx, deadAddr, true, cfg, manifest, nil, &out); err == nil {
		t.Fatal("connection failure was swallowed")
	}
}
