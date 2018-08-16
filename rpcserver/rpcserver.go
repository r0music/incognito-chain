package rpcserver

import (
	"github.com/internet-cash/prototype/blockchain"
	"sync/atomic"
	"net/http"
	"errors"
	"time"
	"log"
	"net"
	"io/ioutil"
	"fmt"
	"strconv"
	"github.com/internet-cash/prototype/jsonrpc"
	"encoding/json"
)

const (
	rpcAuthTimeoutSeconds = 10
)

// timeZeroVal is simply the zero value for a time.Time and is used to avoid
// creating multiple instances.
var timeZeroVal time.Time

type commandHandler func(*RpcServer, interface{}, <-chan struct{}) (interface{}, error)

var RpcHandler = map[string]commandHandler{
	"dosomething":       handleDoSomething,
	"createtransaction": handleCreateTransaction,
}

// rpcServer provides a concurrent safe RPC server to a chain server.
type RpcServer struct {
	started    int32
	shutdown   int32
	numClients int32

	Config     RpcServerConfig
	HttpServer *http.Server

	requestProcessShutdown chan struct{}
	quit                   chan int
}

type RpcServerConfig struct {
	Listenters    []net.Listener
	ChainParams   *blockchain.Params
	RPCMaxClients int
}

func (self RpcServer) Init(config *RpcServerConfig) (*RpcServer, error) {
	self.Config = *config
	return &self, nil
}

// RequestedProcessShutdown returns a channel that is sent to when an authorized
// RPC client requests the process to shutdown.  If the request can not be read
// immediately, it is dropped.
func (self RpcServer) RequestedProcessShutdown() <-chan struct{} {
	return self.requestProcessShutdown
}

// limitConnections responds with a 503 service unavailable and returns true if
// adding another client would exceed the maximum allow RPC clients.
//
// This function is safe for concurrent access.
func (self RpcServer) limitConnections(w http.ResponseWriter, remoteAddr string) bool {
	if int(atomic.LoadInt32(&self.numClients)+1) > self.Config.RPCMaxClients {
		log.Printf("Max RPC clients exceeded [%d] - "+
			"disconnecting client %s", self.Config.RPCMaxClients,
			remoteAddr)
		http.Error(w, "503 Too busy.  Try again later.",
			http.StatusServiceUnavailable)
		return true
	}
	return false
}

// genCertPair generates a key/cert pair to the paths provided.
func genCertPair(certFile, keyFile string) error {
	// TODO for using TCL
	/*log.Println("Generating TLS certificates...")

	org := "btcd autogenerated cert"
	validUntil := time.Now().Add(10 * 365 * 24 * time.Hour)
	cert, key, err := btcutil.NewTLSCertPair(org, validUntil, nil)
	if err != nil {
		return err
	}

	// Write cert and key files.
	if err = ioutil.WriteFile(certFile, cert, 0666); err != nil {
		return err
	}
	if err = ioutil.WriteFile(keyFile, key, 0600); err != nil {
		os.Remove(certFile)
		return err
	}

	rpcsLog.Infof("Done generating TLS certificates")*/
	return nil
}

func (self RpcServer) Start() (error) {
	if atomic.AddInt32(&self.started, 1) != 1 {
		return errors.New("RPC server is already started")
	}
	rpcServeMux := http.NewServeMux()
	self.HttpServer = &http.Server{
		Handler: rpcServeMux,

		// Timeout connections which don't complete the initial
		// handshake within the allowed timeframe.
		ReadTimeout: time.Second * rpcAuthTimeoutSeconds,
	}

	rpcServeMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		self.RpcHandleRequest(w, r)
	})
	for _, listen := range self.Config.Listenters {
		go func(listen net.Listener) {
			log.Printf("RPC server listening on %s", listen.Addr())
			go self.HttpServer.Serve(listen)
			log.Printf("RPC listener done for %s", listen.Addr())
		}(listen)
	}
	self.started = 1
	return nil
}

// Stop is used by server.go to stop the rpc listener.
func (self RpcServer) Stop() error {
	if atomic.AddInt32(&self.shutdown, 1) != 1 {
		log.Println("RPC server is already in the process of shutting down")
		return nil
	}
	log.Println("RPC server shutting down")
	self.HttpServer.Close()
	for _, listen := range self.Config.Listenters {
		listen.Close()
	}
	close(self.quit)
	log.Println("RPC server shutdown complete")
	self.started = 0
	self.shutdown = 1
	return nil
}

func (self RpcServer) RpcHandleRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Connection", "close")
	w.Header().Set("Content-Type", "application/json")
	r.Close = true

	// Limit the number of connections to max allowed.
	if self.limitConnections(w, r.RemoteAddr) {
		return
	}

	// Keep track of the number of connected clients.
	self.incrementClients()
	defer self.decrementClients()
	// TODO
	_, _, err := self.checkAuth(r, true)
	if err != nil {
		self.AuthFail(w)
		return
	}

	self.ProcessRpcRequest(w, r)
}

// checkAuth checks the HTTP Basic authentication supplied by a wallet
// or RPC client in the HTTP request r.  If the supplied authentication
// does not match the username and password expected, a non-nil error is
// returned.
//
// This check is time-constant.
//
// The first bool return value signifies auth success (true if successful) and
// the second bool return value specifies whether the user can change the state
// of the server (true) or whether the user is limited (false). The second is
// always false if the first is.
func (self RpcServer) checkAuth(r *http.Request, require bool) (bool, bool, error) {
	// TODO
	return true, true, nil
}

// incrementClients adds one to the number of connected RPC clients.  Note
// this only applies to standard clients.  Websocket clients have their own
// limits and are tracked separately.
//
// This function is safe for concurrent access.
func (self *RpcServer) incrementClients() {
	atomic.AddInt32(&self.numClients, 1)
}

// decrementClients subtracts one from the number of connected RPC clients.
// Note this only applies to standard clients.  Websocket clients have their own
// limits and are tracked separately.
//
// This function is safe for concurrent access.
func (self *RpcServer) decrementClients() {
	atomic.AddInt32(&self.numClients, -1)
}

// AuthFail sends a message back to the client if the http auth is rejected.
func (self RpcServer) AuthFail(w http.ResponseWriter) {
	w.Header().Add("WWW-Authenticate", `Basic realm="RPC"`)
	http.Error(w, "401 Unauthorized.", http.StatusUnauthorized)
}

/**
handles reading and responding to RPC messages.
 */
func (self RpcServer) ProcessRpcRequest(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&self.shutdown) != 0 {
		return
	}
	// Read and close the JSON-RPC request body from the caller.
	body, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		errCode := http.StatusBadRequest
		http.Error(w, fmt.Sprintf("%d error reading JSON message: %v",
			errCode, err), errCode)
		return
	}

	// Unfortunately, the http server doesn't provide the ability to
	// change the read deadline for the new connection and having one breaks
	// long polling.  However, not having a read deadline on the initial
	// connection would mean clients can connect and idle forever.  Thus,
	// hijack the connecton from the HTTP server, clear the read deadline,
	// and handle writing the response manually.
	hj, ok := w.(http.Hijacker)
	if !ok {
		errMsg := "webserver doesn't support hijacking"
		log.Print(errMsg)
		errCode := http.StatusInternalServerError
		http.Error(w, strconv.Itoa(errCode)+" "+errMsg, errCode)
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		log.Printf("Failed to hijack HTTP connection: %v", err)
		errCode := http.StatusInternalServerError
		http.Error(w, strconv.Itoa(errCode)+" "+err.Error(), errCode)
		return
	}
	defer conn.Close()
	defer buf.Flush()
	conn.SetReadDeadline(timeZeroVal)

	// Attempt to parse the raw body into a JSON-RPC request.
	var responseID interface{}
	var jsonErr error
	var result interface{}
	var request jsonrpc.Request
	if err := json.Unmarshal(body, &request); err != nil {
		jsonErr = &jsonrpc.RPCError{
			Code:    jsonrpc.ErrRPCParse.Code,
			Message: "Failed to parse request: " + err.Error(),
		}
	}
}
