package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"

	edl "github.com/evilcouncil/goec/pkg/etcdlib"
	"github.com/joho/godotenv"
	"go.etcd.io/etcd/client/v3"
	"golang.org/x/sync/errgroup"
)

func handleEtcd(ctx context.Context, servers []string, g *errgroup.Group) {
	ea, err := edl.NewEtcdAgent(ctx, servers)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	g.Go(func() error {
		return ea.KeepAlive(ctx)
	})

}

type ServiceDef struct {
	ServicePort int64  `json:"service_port"`
	MetricsPort int64  `json:"metrics_port"`
	MetricsUrl  string `json:"metrics_url"`
}

type ServiceInfo struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func GetServices(c *clientv3.Client) ([]ServiceInfo, error) {
	slog.Info("GetServices called")
	si := []ServiceInfo{}
	data, err := c.Get(context.Background(), "/", clientv3.WithPrefix())
	if err != nil {
		return si, err
	}
	for _, kv := range data.Kvs {
		fmt.Printf("k: %s, v: %s", kv.Key, kv.Value)
		fields := strings.Split(string(kv.Key), "/")
		sd := ServiceDef{}
		if err := json.Unmarshal(kv.Value, &sd); err != nil {
			return si, err
		}
		si = append(si, ServiceInfo{Targets: []string{fmt.Sprintf("%s:%d", fields[3], sd.MetricsPort)}, Labels: map[string]string{"job": fields[2]}})
	}
	return si, nil
}

func encode[T any](w http.ResponseWriter, _ *http.Request, status int, v T) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return err
	}

	return nil
}

func handleServerBlock(c *clientv3.Client) http.Handler {
	slog.Info("HandleServerBlock called")
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			sib, err := GetServices(c)
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)

			} else {
				encode(w, r, http.StatusOK, sib)
			}
		})
}
func handleServer(ctx context.Context, port string) {
	slog.InfoContext(ctx, "Server Listenting", "port", port)
}

func NewServer(c *clientv3.Client) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, c)

	return mux
}

func run(
	ctx context.Context,
	w io.Writer,
	_ []string, // args
	getenv func(key string) string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	logHandler := slog.NewTextHandler(w, nil)
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	etcdServers := strings.Split(getenv("ETCD_SERVERS"), ",")
	slog.InfoContext(ctx, "Connecting", "etcd servers", etcdServers)

	g, ctx := errgroup.WithContext(ctx)
	ea, err := edl.NewEtcdAgent(ctx, etcdServers)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	g.Go(func() error {
		return ea.KeepAlive(ctx)
	})
	//handleEtcd(ctx, etcdServers, g)

	port := getenv("PORT")
	slog.Info(fmt.Sprintf("listening on %s", port))
	svr := &http.Server{
		Handler: NewServer(ea.Client),
		Addr:    fmt.Sprintf(":%s", port),
	}

	g.Go(func() error {

		return svr.ListenAndServe()
	})

	// wait for context to close
	<-ctx.Done()
	slog.Info("Shutdown time")

	if err := svr.Shutdown(ctx); err != nil {
		return err
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func main() {
	slog.Info("goprohtds online")

	if err := godotenv.Load(); err != nil {
		slog.Info(fmt.Sprintf("Unabled to load env file: %v\n", err))
	}

	ctx := context.Background()
	if err := run(ctx, os.Stdout, os.Args, os.Getenv); err != nil {
		slog.ErrorContext(ctx, err.Error())
		os.Exit(1)
	}
}
