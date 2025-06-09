package main

import (
	"go.etcd.io/etcd/client/v3"
	"net/http"
)

func addRoutes(
	mux *http.ServeMux,
	c *clientv3.Client,
) {
	mux.Handle("/services", handleServerBlock(c))
	mux.Handle("/", http.NotFoundHandler())
}
