package net

import (
	"fmt"
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
	pool               *sync.Pool
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

func NewConnectionPool[T clientType](urls []string, initialCapacity int) *ConnectionPool[T] {
	cp := &ConnectionPool[T]{
		urls:               urls,
		initialConnections: initialCapacity,
		connections:        make([]*Connection[T], 0),
	}
	cp.pool = &sync.Pool{
		New: func() any {
			return cp.newConnection()
		},
	}
	for i := 0; i < initialCapacity; i++ {
		con := cp.newConnection()
		if con != nil {
			cp.pool.Put(con)
		}
	}
	return cp
}

func (cp *ConnectionPool[T]) Get() *Connection[T] {
	con := cp.pool.Get().(*Connection[T])
	if con == nil {
		if len(cp.connections) > 0 {
			cp.RLock()
			defer cp.RUnlock()
			sharedConn := cp.connections[rand.Intn(len(cp.connections))]
			slog.Info("Sharing existing connection", "url", sharedConn.URL)
			return sharedConn
		}
		err := fmt.Sprintf("len of connections %d", len(cp.connections))
		panic(err)
	}
	return con
}

func (cp *ConnectionPool[T]) Put(c *Connection[T]) {
	cp.pool.Put(c)
}

func (cp *ConnectionPool[T]) Close(c *Connection[T]) {
	switch cl := any(c.Client).(type) {
	case *rpc.Client:
		cl.Close()
	case *ethclient.Client:
		cl.Close()
	}
}
