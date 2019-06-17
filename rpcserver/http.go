package rpcserver

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/incognitochain/incognito-chain/common"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type HttpServer struct {
	started          int32
	shutdown         int32
	numClients       int32
	numSocketClients int32
	config           RpcServerConfig
	server           *http.Server
	statusLock       sync.RWMutex
	statusLines      map[int]string
	authSHA          []byte
	limitAuthSHA     []byte
	// channel
	cRequestProcessShutdown chan struct{}
}

func (httpServer *HttpServer) Init(config *RpcServerConfig) {
	httpServer.config = *config
	httpServer.statusLines = make(map[int]string)
	if config.RPCUser != "" && config.RPCPass != "" {
		login := config.RPCUser + ":" + config.RPCPass
		auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(login))
		httpServer.authSHA = common.HashB([]byte(auth))
	}
	if config.RPCLimitUser != "" && config.RPCLimitPass != "" {
		login := config.RPCLimitUser + ":" + config.RPCLimitPass
		auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(login))
		httpServer.limitAuthSHA = common.HashB([]byte(auth))
	}
}

// Start is used by server.go to start the rpc listener.
func (HttpServer *HttpServer) Start() error {
	if atomic.AddInt32(&HttpServer.started, 1) != 1 {
		return NewRPCError(ErrAlreadyStarted, nil)
	}
	httpServeMux := http.NewServeMux()
	HttpServer.server = &http.Server{
		Handler: httpServeMux,
		// Timeout connections which don't complete the initial
		// handshake within the allowed timeframe.
		ReadTimeout: time.Second * rpcAuthTimeoutSeconds,
	}
	
	httpServeMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		HttpServer.handleRequest(w, r)
	})
	for _, listen := range HttpServer.config.Listenters {
		go func(listen net.Listener) {
			Logger.log.Infof("RPC server listening on %s", listen.Addr())
			go HttpServer.server.Serve(listen)
			Logger.log.Infof("RPC listener done for %s", listen.Addr())
		}(listen)
	}
	HttpServer.started = 1
	return nil
}
// Stop is used by server.go to stop the rpc listener.
func (httpServer *HttpServer) Stop() {
	if atomic.AddInt32(&httpServer.shutdown, 1) != 1 {
		Logger.log.Info("RPC server is already in the process of shutting down")
	}
	Logger.log.Info("RPC server shutting down")
	if httpServer.started != 0 {
		httpServer.server.Close()
	}
	for _, listen := range httpServer.config.Listenters {
		listen.Close()
	}
	Logger.log.Warn("RPC server shutdown complete")
	httpServer.started = 0
	httpServer.shutdown = 1
}


/*
Handle all request to rpcserver
*/
func (httpServer *HttpServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	NewCorsHeader(w)
	r.Close = true
	
	// Limit the number of connections to max allowed.
	if httpServer.limitConnections(w, r.RemoteAddr) {
		return
	}
	
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	// Keep track of the number of connected clients.
	httpServer.IncrementClients()
	defer httpServer.DecrementClients()
	// Check authentication for rpc user
	ok, isLimitUser, err := httpServer.checkAuth(r, true)
	if err != nil || !ok {
		Logger.log.Error(err)
		AuthFail(w)
		return
	}
	
	httpServer.ProcessRpcRequest(w, r, isLimitUser)
}

/*
handles reading and responding to RPC messages.
*/

func (httpServer *HttpServer) ProcessRpcRequest(w http.ResponseWriter, r *http.Request, isLimitedUser bool) {
	if atomic.LoadInt32(&httpServer.shutdown) != 0 {
		return
	}
	// Read and close the JSON-RPC request body from the caller.
	body, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		errCode := http.StatusBadRequest
		http.Error(w, fmt.Sprintf("%d error reading JSON Message: %+v", errCode, err), errCode)
		return
	}
	// Logger.log.Info(string(body))
	// log.Println(string(body))
	
	// Unfortunately, the http server doesn't provide the ability to
	// change the read deadline for the new connection and having one breaks
	// long polling.  However, not having a read deadline on the initial
	// connection would mean clients can connect and idle forever.  Thus,
	// hijack the connecton from the HTTP server, clear the read deadline,
	// and handle writing the response manually.
	hj, ok := w.(http.Hijacker)
	if !ok {
		errMsg := "webserver doesn't support hijacking"
		Logger.log.Error(errMsg)
		errCode := http.StatusInternalServerError
		http.Error(w, strconv.Itoa(errCode)+" "+errMsg, errCode)
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		Logger.log.Errorf("Failed to hijack HTTP connection: %s", err.Error())
		Logger.log.Error(err)
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
	var request RpcRequest
	if err := json.Unmarshal(body, &request); err != nil {
		jsonErr = NewRPCError(ErrRPCParse, err)
	}
	
	if jsonErr == nil {
		// The JSON-RPC 1.0 spec defines that notifications must have their "id"
		// set to null and states that notifications do not have a response.
		//
		// A JSON-RPC 2.0 notification is a request with "json-rpc":"2.0", and
		// without an "id" member. The specification states that notifications
		// must not be responded to. JSON-RPC 2.0 permits the null value as a
		// valid request id, therefore such requests are not notifications.
		//
		// coin Core serves requests with "id":null or even an absent "id",
		// and responds to such requests with "id":null in the response.
		//
		// Rpc does not respond to any request without and "id" or "id":null,
		// regardless the indicated JSON-RPC protocol version unless RPC quirks
		// are enabled. With RPC quirks enabled, such requests will be responded
		// to if the reqeust does not indicate JSON-RPC version.
		//
		// RPC quirks can be enabled by the user to avoid compatibility issues
		// with software relying on Core's behavior.
		if request.Id == nil && !(httpServer.config.RPCQuirks && request.Jsonrpc == "") {
			return
		}
		
		// The parse was at least successful enough to have an Id so
		// set it for the response.
		responseID = request.Id
		
		// Setup a close notifier.  Since the connection is hijacked,
		// the CloseNotifer on the ResponseWriter is not available.
		closeChan := make(chan struct{}, 1)
		go func() {
			_, err := conn.Read(make([]byte, 1))
			if err != nil {
				close(closeChan)
			}
		}()
		
		// Check if the user is limited and set error if method unauthorized
		if !isLimitedUser {
			if function, ok := LimitedHttpHandler[request.Method]; ok {
				_ = function
				jsonErr = NewRPCError(ErrRPCInvalidMethodPermission, errors.New(""))
			}
		}
		if jsonErr == nil {
			// Attempt to parse the JSON-RPC request into a known concrete
			// command.
			command := HttpHandler[request.Method]
			if command == nil {
				if isLimitedUser {
					command = LimitedHttpHandler[request.Method]
				} else {
					result = nil
					jsonErr = NewRPCError(ErrRPCMethodNotFound, nil)
				}
			}
			if command != nil {
				result, jsonErr = command(httpServer, request.Params, closeChan)
			} else {
				jsonErr = NewRPCError(ErrRPCMethodNotFound, nil)
			}
		}
	}
	if jsonErr.(*RPCError) != nil && r.Method != "OPTIONS" {
		// Logger.log.Errorf("RPC function process with err \n %+v", jsonErr)
		fmt.Println(request.Method)
		if request.Method != getTransactionByHash {
			Logger.log.Errorf("RPC function process with err \n %+v", jsonErr)
		}
	}
	// Marshal the response.
	msg, err := createMarshalledReply(responseID, result, jsonErr)
	if err != nil {
		Logger.log.Errorf("Failed to marshal reply: %s", err.Error())
		Logger.log.Error(err)
		return
	}
	
	// Write the response.
	err = httpServer.writeHTTPResponseHeaders(r, w.Header(), http.StatusOK, buf)
	if err != nil {
		Logger.log.Error(err)
		return
	}
	if _, err := buf.Write(msg); err != nil {
		Logger.log.Errorf("Failed to write marshalled reply: %s", err.Error())
		Logger.log.Error(err)
	}
	
	// Terminate with newline to maintain compatibility with coin Core.
	if err := buf.WriteByte('\n'); err != nil {
		Logger.log.Errorf("Failed to append terminating newline to reply: %s", err.Error())
		Logger.log.Error(err)
	}
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
func (httpServer *HttpServer) checkAuth(r *http.Request, require bool) (bool, bool, error) {
	if httpServer.config.DisableAuth {
		return true, true, nil
	}
	authhdr := r.Header["Authorization"]
	if len(authhdr) <= 0 {
		if require {
			Logger.log.Warnf("RPC authentication failure from %s",
				r.RemoteAddr)
			return false, false, errors.New("auth failure")
		}
		
		return false, false, nil
	}
	
	authsha := common.HashB([]byte(authhdr[0]))
	
	// Check for limited auth first as in environments with limited users, those
	// are probably expected to have a higher volume of calls
	limitcmp := subtle.ConstantTimeCompare(authsha[:], httpServer.limitAuthSHA[:])
	if limitcmp == 1 {
		return true, true, nil
	}
	
	// Check for admin-level auth
	cmp := subtle.ConstantTimeCompare(authsha[:], httpServer.authSHA[:])
	if cmp == 1 {
		return true, false, nil
	}
	
	// RpcRequest's auth doesn't match either user
	Logger.log.Warnf("RPC authentication failure from %s", r.RemoteAddr)
	return false, false, NewRPCError(ErrAuthFail, nil)
}
// AuthFail sends a Message back to the client if the http auth is rejected.
func AuthFail(w http.ResponseWriter) {
	w.Header().Add("WWW-Authenticate", `Basic realm="RPC"`)
	http.Error(w, "401 Unauthorized.", http.StatusUnauthorized)
}
func NewCorsHeader(w http.ResponseWriter) {
	// Set CORS Header
	w.Header().Set("Connection", "close")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Origin, Device-Type, Device-Id, Authorization, Accept-Language, Access-Control-Allow-Headers, Access-Control-Allow-Credentials, Access-Control-Allow-Origin, Access-Control-Allow-Methods, *")
	w.Header().Set("Access-Control-Allow-Methods", "POST, PUT, GET, OPTIONS, DELETE")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}
// writeHTTPResponseHeaders writes the necessary response headers prior to
// writing an HTTP body given a request to use for protocol negotiation, headers
// to write, a status Code, and a writer.
func (httpServer *HttpServer) writeHTTPResponseHeaders(req *http.Request, headers http.Header, code int, w io.Writer) error {
	_, err := io.WriteString(w, httpServer.httpStatusLine(req, code))
	if err != nil {
		return err
	}
	err = headers.Write(w)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "\r\n")
	return err
}

// httpStatusLine returns a response Status-Line (RFC 2616 Section 6.1)
// for the given request and response status Code.  This function was lifted and
// adapted from the standard library HTTP server Code since it's not exported.
func (httpServer *HttpServer) httpStatusLine(req *http.Request, code int) string {
	// Fast path:
	key := code
	proto11 := req.ProtoAtLeast(1, 1)
	if !proto11 {
		key = -key
	}
	httpServer.statusLock.RLock()
	line, ok := httpServer.statusLines[key]
	httpServer.statusLock.RUnlock()
	if ok {
		return line
	}
	
	// Slow path:
	proto := "HTTP/1.0"
	if proto11 {
		proto = "HTTP/1.1"
	}
	codeStr := strconv.Itoa(code)
	text := http.StatusText(code)
	if text != "" {
		line = proto + " " + codeStr + " " + text + "\r\n"
		httpServer.statusLock.Lock()
		httpServer.statusLines[key] = line
		httpServer.statusLock.Unlock()
	} else {
		text = "status Code " + codeStr
		line = proto + " " + codeStr + " " + text + "\r\n"
	}
	return line
}

// limitConnections responds with a 503 service unavailable and returns true if
// adding another client would exceed the maximum allow RPC clients.
//
// This function is safe for concurrent access.
func (httpServer *HttpServer) limitConnections(w http.ResponseWriter, remoteAddr string) bool {
	if int(atomic.LoadInt32(&httpServer.numClients)+1) > httpServer.config.RPCMaxClients {
		Logger.log.Infof("Max RPC clients exceeded [%d] - "+
			"disconnecting client %s", httpServer.config.RPCMaxClients,
			remoteAddr)
		http.Error(w, "503 Too busy.  Try again later.",
			http.StatusServiceUnavailable)
		return true
	}
	return false
}
// DecrementClients subtracts one from the number of connected RPC clients.
// Note this only applies to standard clients.
//
// This function is safe for concurrent access.
func (httpServer *HttpServer) DecrementClients() {
	atomic.AddInt32(&httpServer.numClients, -1)
}
// IncrementClients adds one to the number of connected RPC clients.  Note
// this only applies to standard clients.
//
// This function is safe for concurrent access.
func (httpServer *HttpServer) IncrementClients() {
	atomic.AddInt32(&httpServer.numClients, 1)
}
