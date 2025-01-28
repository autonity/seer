package net

import (
	"log/slog"
	"math/rand"
	"sync"

	"github.com/autonity/autonity/ethclient"
	"github.com/autonity/autonity/rpc"
)

type clientType interface {
	*ethclient.Client | *rpc.Client
}

type Connection[T clientType] struct {
	Client T
	URL    string
}

type ConnectionPool[T clientType] struct {
	initialConnections int
	urls               []string
	connections        []*Connection[T]
	sync.RWMutex
}

func (cp *ConnectionPool[T]) newConnection() *Connection[T] {
	url := cp.urls[rand.Intn(len(cp.urls))]
	var client T
	switch any(client).(type) {
	case *ethclient.Client:
		wsClient, err := ethclient.Dial(url)
		if err != nil {
			slog.Error("dial error", "err", err, "url", url)
			return nil
		}
		client = any(wsClient).(T)
	case *rpc.Client:
		rpcClient, err := rpc.Dial(url)
		if err != nil {
			slog.Error("dial error", "err", err, "url", url)
			return nil
		}
		client = any(rpcClient).(T)
	}

	cp.Lock()
	defer cp.Unlock()
	con := &Connection[T]{client, url}
	cp.connections = append(cp.connections, con)
	return con
}

func NewConnectionPool[T clientType](urls []string, capacity int) *ConnectionPool[T] {
	cp := &ConnectionPool[T]{
		urls:               urls,
		initialConnections: capacity,
		connections:        make([]*Connection[T], 0),
	}
	for i := 0; i < capacity; i++ {
		cp.newConnection()
	}
	return cp
}

func (cp *ConnectionPool[T]) Get() *Connection[T] {
	cp.RLock()
	defer cp.RUnlock()

	if len(cp.connections) == 0 {
		return nil
	}
	return cp.connections[rand.Intn(len(cp.connections))]
}

func (cp *ConnectionPool[T]) Close(c *Connection[T]) {
	switch cl := any(c.Client).(type) {
	case *rpc.Client:
		cl.Close()
	case *ethclient.Client:
		cl.Close()
	}
}
