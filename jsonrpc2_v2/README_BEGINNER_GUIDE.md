# JSON-RPC 2.0 Implementation - Comprehensive Guide

## What is JSON-RPC 2.0?

JSON-RPC 2.0 is a **remote procedure call (RPC) protocol** that allows you to call functions on a remote server over the internet. Think of it like making a phone call to ask someone to do something for you, but instead of talking, you send JSON messages.

### Simple Example
Instead of calling a function directly:
```go
result := calculateSum(5, 3)  // Local function call
```

You send a JSON message over the network:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "calculateSum",
  "params": [5, 3]
}
```

And get back:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": 8
}
```

## Deep Dive: Protocol Specification

### JSON-RPC 2.0 Specification Details

The JSON-RPC 2.0 specification defines a stateless, light-weight remote procedure call (RPC) protocol. Here are the key aspects:

#### Message Structure
Every JSON-RPC 2.0 message must contain:
- `jsonrpc`: Must be exactly "2.0"
- `method`: String containing the method name to invoke
- `params`: Structured value (array or object) with method parameters
- `id`: Request identifier (for calls only, not notifications)

#### Request Types

**1. Call (Request-Response)**
```json
{
  "jsonrpc": "2.0",
  "method": "subtract",
  "params": [42, 23],
  "id": 1
}
```

**2. Notification (Fire-and-Forget)**
```json
{
  "jsonrpc": "2.0",
  "method": "notify_hello",
  "params": [7]
}
```

**3. Response (Success)**
```json
{
  "jsonrpc": "2.0",
  "result": 19,
  "id": 1
}
```

**4. Response (Error)**
```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32601,
    "message": "Method not found"
  },
  "id": 1
}
```

#### Error Codes
- **-32700** to **-32799**: Reserved for implementation-defined server-errors
- **-32600** to **-32699**: Reserved for implementation-defined server-errors
- **-32600**: Invalid Request
- **-32601**: Method not found
- **-32602**: Invalid params
- **-32603**: Internal error
- **-32000** to **-32099**: Reserved for implementation-defined server-errors

## File Structure Overview

This implementation is split into several files, each handling a specific part of the JSON-RPC protocol:

```
jsonrpc2_v2/
├── jsonrpc2.go    # Core interfaces and types
├── messages.go    # Request/Response data structures
├── wire.go        # JSON wire format and error codes
├── frame.go       # Message transport (how to send/receive)
├── conn.go        # Connection management (the main logic)
├── serve.go       # Server functionality
├── net.go         # Network transport (TCP, Unix sockets)
└── rpcerr.go      # Error utilities
```

## 1. Core Concepts (`jsonrpc2.go`) - Deep Dive

### Architecture Overview

The JSON-RPC 2.0 implementation follows a layered architecture:

```
┌─────────────────────────────────────┐
│           Application Layer         │  ← Your handlers
├─────────────────────────────────────┤
│         JSON-RPC Protocol           │  ← Request/Response logic
├─────────────────────────────────────┤
│         Message Framing             │  ← Raw vs Header framing
├─────────────────────────────────────┤
│         Transport Layer             │  ← TCP, Unix sockets, etc.
└─────────────────────────────────────┘
```

### Core Interfaces Explained

#### Handler Interface - The Heart of Request Processing

```go
type Handler interface {
    Handle(ctx context.Context, req *Request) (result interface{}, err error)
}
```

**What it does:**
- Processes incoming JSON-RPC requests sequentially
- Must be thread-safe (only one request at a time per connection)
- Returns either a result or an error
- Can return `ErrAsyncResponse` for asynchronous processing

**Implementation patterns:**
```go
// Simple handler
type SimpleHandler struct{}

func (h *SimpleHandler) Handle(ctx context.Context, req *Request) (interface{}, error) {
    switch req.Method {
    case "add":
        var params []int
        if err := json.Unmarshal(req.Params, &params); err != nil {
            return nil, ErrInvalidParams
        }
        return params[0] + params[1], nil
    default:
        return nil, ErrMethodNotFound
    }
}

// Handler with context awareness
func (h *Handler) Handle(ctx context.Context, req *Request) (interface{}, error) {
    // Check if request was cancelled
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // Process with timeout
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    return h.processRequest(ctx, req)
}
```

#### Preempter Interface - High-Priority Message Processing

```go
type Preempter interface {
    Preempt(ctx context.Context, req *Request) (result interface{}, err error)
}
```

**What it does:**
- Handles messages BEFORE they enter the main handler queue
- Used for urgent messages like cancellation requests
- Must NOT block (fast, non-blocking operations only)
- Can return `ErrNotHandled` to pass to main handler

**Use cases:**
```go
type CancellationPreempter struct{}

func (p *CancellationPreempter) Preempt(ctx context.Context, req *Request) (interface{}, error) {
    // Handle cancellation requests immediately
    if req.Method == "$/cancelRequest" {
        // Cancel the specified request
        return nil, nil
    }
    
    // Handle progress notifications
    if strings.HasPrefix(req.Method, "$/progress") {
        // Update progress immediately
        return nil, nil
    }
    
    // Let other requests go to main handler
    return nil, ErrNotHandled
}
```

### Advanced Error Handling

#### Error Type Hierarchy

```go
// Base JSON-RPC errors (from wire.go)
var (
    ErrParse         = NewError(-32700, "JSON RPC parse error")
    ErrInvalidRequest = NewError(-32600, "JSON RPC invalid request")
    ErrMethodNotFound = NewError(-32601, "JSON RPC method not found")
    ErrInvalidParams  = NewError(-32602, "JSON RPC invalid params")
    ErrInternal       = NewError(-32603, "JSON RPC internal error")
)

// Implementation-specific errors
var (
    ErrServerOverloaded = NewError(-32000, "JSON RPC overloaded")
    ErrUnknown          = NewError(-32001, "JSON RPC unknown error")
    ErrServerClosing    = NewError(-32002, "JSON RPC server is closing")
    ErrClientClosing    = NewError(-32003, "JSON RPC client is closing")
)

// Connection-level errors (from jsonrpc2.go)
var (
    ErrIdleTimeout   = errors.New("timed out waiting for new connections")
    ErrNotHandled    = errors.New("JSON RPC not handled")
    ErrAsyncResponse = errors.New("JSON RPC asynchronous response")
)
```

#### Error Wrapping and Propagation

```go
// Custom error with additional context
type CustomError struct {
    Code    int64
    Message string
    Data    interface{}
}

func (e *CustomError) Error() string {
    return e.Message
}

func (e *CustomError) Is(other error) bool {
    if err, ok := other.(*CustomError); ok {
        return e.Code == err.Code
    }
    return false
}

// Usage in handler
func (h *Handler) Handle(ctx context.Context, req *Request) (interface{}, error) {
    if req.Method == "divide" {
        var params []float64
        json.Unmarshal(req.Params, &params)
        
        if len(params) != 2 {
            return nil, &CustomError{
                Code:    -32602,
                Message: "Invalid params: expected 2 numbers",
                Data:    "Expected [dividend, divisor]",
            }
        }
        
        if params[1] == 0 {
            return nil, &CustomError{
                Code:    -32001,
                Message: "Division by zero",
                Data:    map[string]interface{}{"dividend": params[0], "divisor": params[1]},
            }
        }
        
        return params[0] / params[1], nil
    }
    
    return nil, ErrMethodNotFound
}
```

### Async Helper - Managing Asynchronous Operations

```go
type async struct {
    ready    chan struct{} // closed when done
    firstErr chan error    // 1-buffered; contains either nil or the first non-nil error
}
```

**Design patterns:**
- **Single completion** - `ready` channel closed exactly once
- **Error preservation** - First error is stored and preserved
- **Thread safety** - Safe for concurrent access
- **Resource cleanup** - Prevents goroutine leaks

**Usage patterns:**
```go
// Long-running operation
func (h *Handler) Handle(ctx context.Context, req *Request) (interface{}, error) {
    if req.Method == "longTask" {
        // Start async operation
        async := newAsync()
        go func() {
            defer async.done()
            
            result, err := h.performLongTask(ctx)
            if err != nil {
                async.setError(err)
            }
        }()
        
        // Return async response indicator
        return nil, ErrAsyncResponse
    }
    
    return nil, ErrMethodNotFound
}

// Later, respond asynchronously
func (h *Handler) respondAsync(id ID, result interface{}, err error) {
    // This would be called from the async goroutine
    h.connection.Respond(id, result, err)
}
```

## 2. Message Types (`messages.go`) - Deep Dive

### Message Interface Design

The implementation uses a **closed interface pattern** to ensure type safety:

```go
type Message interface {
    // marshal builds the wire form from the API form.
    // It is private, which makes the set of Message implementations closed.
    marshal(to *wireCombined)
}
```

**Why this design?**
- **Type safety** - Only `*Request` and `*Response` can implement `Message`
- **Encapsulation** - Private `marshal` method prevents external implementations
- **Polymorphism** - Can handle both types uniformly
- **Performance** - No interface overhead for type checking

### Request Structure - Advanced Details

```go
type Request struct {
    ID     ID              // Unique identifier (nil for notifications)
    Method string          // What function to call
    Params json.RawMessage // Parameters to pass
}
```

#### ID Management - Sophisticated Design

```go
type ID struct {
    value interface{}  // Can be string, int64, or nil
}

// Type-safe constructors
func StringID(s string) ID { return ID{value: s} }
func Int64ID(i int64) ID { return ID{value: i} }

// Validation and access
func (id ID) IsValid() bool { return id.value != nil }
func (id ID) Raw() interface{} { return id.value }
```

**Why `interface{}` for ID?**
- **JSON-RPC 2.0 spec** allows both string and numeric IDs
- **Type coercion** - Handles float64 → int64 conversion from JSON
- **Flexibility** - Supports various ID schemes
- **Performance** - No boxing/unboxing overhead

**ID Usage Patterns:**
```go
// Generate sequential numeric IDs
id := Int64ID(atomic.AddInt64(&seq, 1))

// Use meaningful string IDs
id := StringID("user-123-session-456")

// UUID-based IDs
id := StringID(uuid.New().String())

// Timestamp-based IDs
id := Int64ID(time.Now().UnixNano())
```

#### Params as json.RawMessage - Performance Optimization

```go
Params json.RawMessage // Lazy parsing for performance
```

**Why `json.RawMessage`?**
- **Lazy parsing** - Only parse when needed
- **Memory efficiency** - Avoids intermediate allocations
- **Type flexibility** - Can unmarshal to any Go type
- **Performance** - Reduces JSON processing overhead

**Usage patterns:**
```go
// Parse params to specific types
func (h *Handler) Handle(ctx context.Context, req *Request) (interface{}, error) {
    switch req.Method {
    case "add":
        var params []int
        if err := json.Unmarshal(req.Params, &params); err != nil {
            return nil, ErrInvalidParams
        }
        return params[0] + params[1], nil
        
    case "user.create":
        var params struct {
            Name  string `json:"name"`
            Email string `json:"email"`
        }
        if err := json.Unmarshal(req.Params, &params); err != nil {
            return nil, ErrInvalidParams
        }
        return h.createUser(params.Name, params.Email)
        
    case "complex":
        // Parse to map for dynamic handling
        var params map[string]interface{}
        if err := json.Unmarshal(req.Params, &params); err != nil {
            return nil, ErrInvalidParams
        }
        return h.handleComplex(params)
    }
    
    return nil, ErrMethodNotFound
}
```

### Response Structure - Error Handling

```go
type Response struct {
    ID     ID              // Same ID as the request
    Result json.RawMessage // The answer (if successful)
    Error  error           // Error (if failed)
}
```

#### Mutual Exclusivity - Result vs Error

**Critical invariant:** A response can have EITHER a result OR an error, never both.

```go
// Validation in processResult
if result != nil && err != nil {
    c.internalErrorf("%#v returned a non-nil result with a non-nil error for %s:\n%v\n%#v", 
        from, req.Method, err, result)
    result = nil // Discard the spurious result and respond with err.
}
```

#### Error Conversion - Wire Format Compatibility

```go
func toWireError(err error) *wireError {
    if err == nil {
        return nil
    }
    if err, ok := err.(*wireError); ok {
        return err  // Already a wire error
    }
    
    result := &wireError{Message: err.Error()}
    var wrapped *wireError
    if errors.As(err, &wrapped) {
        // Preserve error code from wrapped error
        result.Code = wrapped.Code
    }
    return result
}
```

**Error wrapping support:**
```go
// Custom error that wraps a wire error
type MyError struct {
    *wireError
    AdditionalData string
}

func (e *MyError) Unwrap() error {
    return e.wireError
}

// Usage
func (h *Handler) Handle(ctx context.Context, req *Request) (interface{}, error) {
    if req.Method == "divide" {
        var params []float64
        json.Unmarshal(req.Params, &params)
        
        if params[1] == 0 {
            return nil, &MyError{
                wireError: &wireError{
                    Code:    -32001,
                    Message: "Division by zero",
                },
                AdditionalData: "Cannot divide by zero",
            }
        }
        
        return params[0] / params[1], nil
    }
    
    return nil, ErrMethodNotFound
}
```

### Message Construction - Factory Pattern

```go
// Notification constructor
func NewNotification(method string, params interface{}) (*Request, error) {
    p, merr := marshalToRaw(params)
    return &Request{Method: method, Params: p}, merr
}

// Call constructor
func NewCall(id ID, method string, params interface{}) (*Request, error) {
    p, merr := marshalToRaw(params)
    return &Request{ID: id, Method: method, Params: p}, merr
}

// Response constructor
func NewResponse(id ID, result interface{}, rerr error) (*Response, error) {
    r, merr := marshalToRaw(result)
    return &Response{ID: id, Result: r, Error: rerr}, merr
}
```

**Design benefits:**
- **Consistent error handling** - All constructors return errors
- **Lazy marshaling** - Parameters marshaled immediately to `json.RawMessage`
- **Type safety** - Compile-time checking of parameter types
- **Performance** - Single marshaling operation

### Message Serialization - Wire Format Conversion

```go
func EncodeMessage(msg Message) ([]byte, error) {
    wire := wireCombined{VersionTag: wireVersion}
    msg.marshal(&wire)
    data, err := json.Marshal(&wire)
    if err != nil {
        return data, fmt.Errorf("marshaling jsonrpc message: %w", err)
    }
    return data, nil
}

func DecodeMessage(data []byte) (Message, error) {
    msg := wireCombined{}
    if err := json.Unmarshal(data, &msg); err != nil {
        return nil, fmt.Errorf("unmarshaling jsonrpc message: %w", err)
    }
    
    // Version validation
    if msg.VersionTag != wireVersion {
        return nil, fmt.Errorf("invalid message version tag %s expected %s", 
            msg.VersionTag, wireVersion)
    }
    
    // ID type coercion
    id := ID{}
    switch v := msg.ID.(type) {
    case nil:
        // No ID (notification)
    case float64:
        // JSON numbers are float64, convert to int64
        id = Int64ID(int64(v))
    case int64:
        id = Int64ID(v)
    case string:
        id = StringID(v)
    default:
        return nil, fmt.Errorf("invalid message id type <%T>%v", v, v)
    }
    
    // Message type detection
    if msg.Method != "" {
        // Has method = Request
        return &Request{
            Method: msg.Method,
            ID:     id,
            Params: msg.Params,
        }, nil
    }
    
    // No method = Response
    if !id.IsValid() {
        return nil, ErrInvalidRequest
    }
    
    resp := &Response{
        ID:     id,
        Result: msg.Result,
    }
    if msg.Error != nil {
        resp.Error = msg.Error
    }
    return resp, nil
}
```

**Key features:**
- **Version validation** - Ensures JSON-RPC 2.0 compatibility
- **Type coercion** - Handles JSON number → int64 conversion
- **Message detection** - Uses presence of `method` field to determine type
- **Error handling** - Comprehensive error reporting with context

### Request Types - Detailed Analysis

#### 1. Call (Request-Response Pattern)

```go
// Call characteristics
func (msg *Request) IsCall() bool { 
    return msg.ID.IsValid() 
}
```

**Call lifecycle:**
1. **Client sends** call with unique ID
2. **Server processes** request
3. **Server responds** with same ID and result/error
4. **Client matches** response to original call

**Call examples:**
```json
// Simple call
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "add",
  "params": [2, 3]
}

// Call with named parameters
{
  "jsonrpc": "2.0",
  "id": "user-123",
  "method": "user.create",
  "params": {
    "name": "John Doe",
    "email": "john@example.com"
  }
}

// Call with no parameters
{
  "jsonrpc": "2.0",
  "id": 42,
  "method": "getTime"
}
```

#### 2. Notification (Fire-and-Forget Pattern)

```go
// Notification characteristics
func (msg *Request) IsNotification() bool { 
    return !msg.ID.IsValid() 
}
```

**Notification characteristics:**
- **No ID** - Cannot be matched to a response
- **No response** - Server never sends a response
- **Fire-and-forget** - Client doesn't wait for confirmation
- **Best effort** - No guarantee of delivery

**Notification examples:**
```json
// Progress notification
{
  "jsonrpc": "2.0",
  "method": "progress",
  "params": {
    "taskId": "task-123",
    "progress": 75
  }
}

// Log notification
{
  "jsonrpc": "2.0",
  "method": "log",
  "params": {
    "level": "info",
    "message": "User logged in"
  }
}

// Heartbeat notification
{
  "jsonrpc": "2.0",
  "method": "ping"
}
```

## 3. Wire Format (`wire.go`)

### What is the "wire format"?
This is how the JSON actually looks when sent over the network. The `wireCombined` struct represents this:

```go
type wireCombined struct {
    VersionTag string          `json:"jsonrpc"`    // Always "2.0"
    ID         interface{}     `json:"id"`         // Request ID
    Method     string          `json:"method"`     // Function name
    Params     json.RawMessage `json:"params"`     // Parameters
    Result     json.RawMessage `json:"result"`     // Response data
    Error      *wireError      `json:"error"`      // Error info
}
```

### Error Codes
The protocol defines standard error codes:
- `-32700` - Parse error (invalid JSON)
- `-32600` - Invalid request
- `-32601` - Method not found
- `-32602` - Invalid parameters
- `-32603` - Internal error

## 4. Message Transport (`frame.go`)

### How are messages sent over the network?

There are two ways to package JSON-RPC messages:

#### Raw Framer
Just send the JSON directly:
```
{"jsonrpc":"2.0","id":1,"method":"hello","params":[]}
{"jsonrpc":"2.0","id":1,"result":"world"}
```

#### Header Framer (used by LSP)
Send with HTTP-style headers:
```
Content-Length: 45

{"jsonrpc":"2.0","id":1,"method":"hello","params":[]}
Content-Length: 52

{"jsonrpc":"2.0","id":1,"result":"world"}
```

## 5. Connection Management (`conn.go`) - Deep Dive

### Connection Architecture - The Heart of the System

A `Connection` is the central orchestrator that manages the entire JSON-RPC conversation. It's **bidirectional** - can act as both client and server simultaneously.

```go
type Connection struct {
    seq int64 // must only be accessed using atomic operations

    stateMu sync.Mutex
    state   inFlightState // accessed only in updateInFlight
    done    chan struct{} // closed (under stateMu) when state.closed is true and all goroutines have completed

    writer chan Writer // 1-buffered; stores the writer when not in use

    handler Handler

    onInternalError func(error)
    onDone          func()
}
```

### State Management - Thread-Safe Operations

The connection uses a sophisticated state management system to ensure thread safety and proper resource cleanup.

#### In-Flight State Tracking

```go
type inFlightState struct {
    // Connection lifecycle
    connClosing bool  // true when the Connection's Close method has been called
    reading     bool  // true while the readIncoming goroutine is running
    readErr     error // non-nil when the readIncoming goroutine exits (typically io.EOF)
    writeErr    error // non-nil if a call to the Writer has failed with a non-canceled Context

    // Resource management
    closer   io.Closer
    closeErr error // error returned from closer.Close

    // Outgoing request tracking
    outgoingCalls         map[ID]*AsyncCall // calls only
    outgoingNotifications int               // # of notifications awaiting "write"

    // Incoming request tracking
    incoming    int                    // total number of incoming calls and notifications
    incomingByID map[ID]*incomingRequest // calls only

    // Handler queue management
    handlerQueue   []*incomingRequest
    handlerRunning bool
}
```

#### Atomic State Updates

All state changes go through the `updateInFlight` function to ensure consistency:

```go
func (c *Connection) updateInFlight(f func(*inFlightState)) {
    c.stateMu.Lock()
    defer c.stateMu.Unlock()

    s := &c.state
    f(s)  // Apply the state change

    // Check if connection should be closed
    if s.idle() && s.shuttingDown(ErrUnknown) != nil {
        if s.closer != nil {
            s.closeErr = s.closer.Close()
            s.closer = nil // prevent duplicate Close calls
        }
        
        if s.reading {
            // readIncoming goroutine will handle cleanup
        } else {
            // Everything is idle, close the connection
            if c.onDone != nil {
                c.onDone()
            }
            close(c.done)
        }
    }
}
```

**Why this pattern?**
- **Atomic updates** - All state changes are atomic
- **Automatic cleanup** - Connection closes when idle and shutting down
- **Thread safety** - Single mutex protects all state
- **Resource management** - Prevents resource leaks

### Outgoing Operations - Client-Side Functionality

#### Call Method - Request-Response Pattern

```go
func (c *Connection) Call(ctx context.Context, method string, params interface{}) *AsyncCall {
    // Generate unique ID atomically
    id := Int64ID(atomic.AddInt64(&c.seq, 1))
    
    // Create async call object
    ac := &AsyncCall{
        id:      id,
        ready:   make(chan struct{}),
        ctx:     ctx,
        endSpan: endSpan,
    }
    
    // Create the call message
    call, err := NewCall(ac.id, method, params)
    if err != nil {
        ac.retire(&Response{ID: id, Error: err})
        return ac
    }
    
    // Register the call in outgoing map
    c.updateInFlight(func(s *inFlightState) {
        err = s.shuttingDown(ErrClientClosing)
        if err != nil {
            return
        }
        if s.outgoingCalls == nil {
            s.outgoingCalls = make(map[ID]*AsyncCall)
        }
        s.outgoingCalls[ac.id] = ac
    })
    
    // Send the call
    if err := c.write(ctx, call); err != nil {
        // Send failed, retire the call
        c.updateInFlight(func(s *inFlightState) {
            if s.outgoingCalls[ac.id] == ac {
                delete(s.outgoingCalls, ac.id)
                ac.retire(&Response{ID: id, Error: err})
            }
        })
    }
    
    return ac
}
```

**Key features:**
- **Atomic ID generation** - Thread-safe sequence numbers
- **Immediate error handling** - Errors returned immediately if send fails
- **AsyncCall return** - Allows awaiting response later
- **State tracking** - Call registered in outgoing map

#### AsyncCall - Managing Asynchronous Responses

```go
type AsyncCall struct {
    id       ID
    ready    chan struct{} // closed after response has been set and span has been ended
    response *Response
    ctx      context.Context // for event logging only
    endSpan  func()          // close the tracing span when all processing for the message is complete
}

func (ac *AsyncCall) Await(ctx context.Context, result interface{}) error {
    select {
    case <-ctx.Done():
        return fmt.Errorf("ctx.Done error: %w", ctx.Err())
    case <-ac.ready:
    }
    
    if ac.response.Error != nil {
        return fmt.Errorf("ac.response error: %w", ac.response.Error)
    }
    
    if result == nil {
        return nil
    }
    
    err := json.Unmarshal(ac.response.Result, result)
    if err != nil {
        return fmt.Errorf("json.Unmarshal error: %w", err)
    }
    
    return nil
}
```

**Advanced usage patterns:**
```go
// Multiple concurrent calls
func makeMultipleCalls(conn *Connection) {
    var wg sync.WaitGroup
    
    // Call 1
    wg.Add(1)
    go func() {
        defer wg.Done()
        call := conn.Call(ctx, "getUser", map[string]string{"id": "123"})
        var user User
        if err := call.Await(ctx, &user); err != nil {
            log.Printf("getUser failed: %v", err)
        } else {
            log.Printf("User: %+v", user)
        }
    }()
    
    // Call 2
    wg.Add(1)
    go func() {
        defer wg.Done()
        call := conn.Call(ctx, "getProfile", map[string]string{"id": "123"})
        var profile Profile
        if err := call.Await(ctx, &profile); err != nil {
            log.Printf("getProfile failed: %v", err)
        } else {
            log.Printf("Profile: %+v", profile)
        }
    }()
    
    wg.Wait()
}

// Call with timeout
func callWithTimeout(conn *Connection) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    call := conn.Call(ctx, "slowOperation", nil)
    var result interface{}
    if err := call.Await(ctx, &result); err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            log.Printf("Operation timed out")
        } else {
            log.Printf("Operation failed: %v", err)
        }
    }
}

// Call with cancellation
func callWithCancellation(conn *Connection) {
    ctx, cancel := context.WithCancel(context.Background())
    
    // Cancel after 3 seconds
    go func() {
        time.Sleep(3 * time.Second)
        cancel()
    }()
    
    call := conn.Call(ctx, "longOperation", nil)
    var result interface{}
    if err := call.Await(ctx, &result); err != nil {
        if errors.Is(err, context.Canceled) {
            log.Printf("Operation was cancelled")
        } else {
            log.Printf("Operation failed: %v", err)
        }
    }
}
```

#### Notification Method - Fire-and-Forget Pattern

```go
func (c *Connection) Notify(ctx context.Context, method string, params interface{}) (err error) {
    // Event tracing setup
    ctx, done := event.Start(ctx, method,
        tag.Method.Of(method),
        tag.RPCDirection.Of(tag.Outbound),
    )
    attempted := false

    defer func() {
        labelStatus(ctx, err)
        done()
        if attempted {
            c.updateInFlight(func(s *inFlightState) {
                s.outgoingNotifications--
            })
        }
    }()

    // Check if connection is shutting down
    c.updateInFlight(func(s *inFlightState) {
        // Allow notifications only if there are calls in flight
        if len(s.outgoingCalls) == 0 && len(s.incomingByID) == 0 {
            err = s.shuttingDown(ErrClientClosing)
            if err != nil {
                return
            }
        }
        s.outgoingNotifications++
        attempted = true
    })
    
    if err != nil {
        return err
    }

    // Create and send notification
    notify, err := NewNotification(method, params)
    if err != nil {
        return fmt.Errorf("marshaling notify parameters: %v", err)
    }

    event.Metric(ctx, tag.Started.Of(1))
    return c.write(ctx, notify)
}
```

**Notification characteristics:**
- **No response expected** - Fire-and-forget pattern
- **Shutdown logic** - Only allowed if calls are in flight
- **Event tracing** - Full observability
- **Error handling** - Immediate error if send fails

### Incoming Operations - Server-Side Functionality

#### Request Processing Pipeline

The connection processes incoming requests through a sophisticated pipeline:

1. **Message Reading** - `readIncoming` goroutine reads messages
2. **Preemption** - `Preempter` handles urgent messages
3. **Handler Queue** - Regular messages queued for processing
4. **Handler Processing** - `Handler` processes messages sequentially
5. **Response Generation** - Responses sent back to client

#### readIncoming - Message Reading Loop

```go
func (c *Connection) readIncoming(ctx context.Context, reader Reader, preempter Preempter) {
    var err error
    for {
        var (
            msg Message
            n   int64
        )
        msg, n, err = reader.Read(ctx)
        if err != nil {
            break
        }

        switch msg := msg.(type) {
        case *Request:
            c.acceptRequest(ctx, msg, n, preempter)

        case *Response:
            c.updateInFlight(func(s *inFlightState) {
                if ac, ok := s.outgoingCalls[msg.ID]; ok {
                    delete(s.outgoingCalls, msg.ID)
                    ac.retire(msg)
                } else {
                    // TODO: How should we report unexpected responses?
                }
            })

        default:
            c.internalErrorf("Read returned an unexpected message of type %T", msg)
        }
    }

    // Cleanup on read error
    c.updateInFlight(func(s *inFlightState) {
        s.reading = false
        s.readErr = err

        // Retire any outgoing requests that were still in flight
        for id, ac := range s.outgoingCalls {
            ac.retire(&Response{ID: id, Error: fmt.Errorf("reader no longer being processed, cannot receive response: %w", err)})
        }
        s.outgoingCalls = nil
    })
}
```

#### acceptRequest - Request Acceptance Logic

```go
func (c *Connection) acceptRequest(ctx context.Context, msg *Request, msgBytes int64, preempter Preempter) {
    // Event tracing setup
    labels := append(make([]label.Label, 0, 3),
        tag.Method.Of(msg.Method),
        tag.RPCDirection.Of(tag.Inbound),
    )
    if msg.IsCall() {
        labels = append(labels, tag.RPCID.Of(fmt.Sprintf("%q", msg.ID)))
    }
    ctx, endSpan := event.Start(ctx, msg.Method, labels...)
    event.Metric(ctx,
        tag.Started.Of(1),
        tag.ReceivedBytes.Of(msgBytes))

    // Create cancelable context
    ctx, cancel := context.WithCancel(ctx)
    req := &incomingRequest{
        Request: msg,
        ctx:     ctx,
        cancel:  cancel,
        endSpan: endSpan,
    }

    // Register call in incoming map
    var err error
    c.updateInFlight(func(s *inFlightState) {
        s.incoming++

        if req.IsCall() {
            if s.incomingByID[req.ID] != nil {
                err = fmt.Errorf("%w: request ID %v already in use", ErrInvalidRequest, req.ID)
                req.ID = ID{} // Don't misattribute this error
                return
            }

            if s.incomingByID == nil {
                s.incomingByID = make(map[ID]*incomingRequest)
            }
            s.incomingByID[req.ID] = req

            // Check if shutting down
            err = s.shuttingDown(ErrServerClosing)
        }
    })
    
    if err != nil {
        c.processResult("acceptRequest", req, nil, err)
        return
    }

    // Try preempter first
    if preempter != nil {
        result, err := preempter.Preempt(req.ctx, req.Request)

        if req.IsCall() && errors.Is(err, ErrAsyncResponse) {
            // Request will remain in flight until Respond is called
            return
        }

        if !errors.Is(err, ErrNotHandled) {
            c.processResult("Preempt", req, result, err)
            return
        }
    }

    // Queue for handler processing
    c.updateInFlight(func(s *inFlightState) {
        err = s.shuttingDown(ErrServerClosing)
        if err != nil {
            return
        }

        s.handlerQueue = append(s.handlerQueue, req)
        if !s.handlerRunning {
            s.handlerRunning = true
            go c.handleAsync()
        }
    })
    
    if err != nil {
        c.processResult("acceptRequest", req, nil, err)
    }
}
```

#### handleAsync - Sequential Handler Processing

```go
func (c *Connection) handleAsync() {
    for {
        var req *incomingRequest
        c.updateInFlight(func(s *inFlightState) {
            if len(s.handlerQueue) > 0 {
                req, s.handlerQueue = s.handlerQueue[0], s.handlerQueue[1:]
            } else {
                s.handlerRunning = false
            }
        })
        if req == nil {
            return
        }

        // Check if already canceled
        if err := req.ctx.Err(); err != nil {
            c.updateInFlight(func(s *inFlightState) {
                if s.writeErr != nil {
                    err = fmt.Errorf("%w: %v", ErrServerClosing, s.writeErr)
                }
            })
            c.processResult("handleAsync", req, nil, err)
            continue
        }

        // Process the request
        result, err := c.handler.Handle(req.ctx, req.Request)
        c.processResult(c.handler, req, result, err)
    }
}
```

**Key features:**
- **Sequential processing** - One request at a time per connection
- **Automatic queue management** - Handler goroutine starts/stops as needed
- **Cancellation support** - Respects context cancellation
- **Error handling** - Comprehensive error reporting

#### processResult - Response Generation

```go
func (c *Connection) processResult(from interface{}, req *incomingRequest, result interface{}, err error) error {
    switch err {
    case ErrAsyncResponse:
        if !req.IsCall() {
            return c.internalErrorf("%#v returned ErrAsyncResponse for a %q Request without an ID", from, req.Method)
        }
        return nil // Request remains in flight

    case ErrNotHandled, ErrMethodNotFound:
        err = fmt.Errorf("%w: %q", ErrMethodNotFound, req.Method)
    }

    // Validate response
    if req.endSpan == nil {
        return c.internalErrorf("%#v produced a duplicate %q Response", from, req.Method)
    }

    if result != nil && err != nil {
        c.internalErrorf("%#v returned a non-nil result with a non-nil error for %s:\n%v\n%#v", from, req.Method, err, result)
        result = nil // Discard spurious result
    }

    if req.IsCall() {
        // Generate response for call
        if result == nil && err == nil {
            err = c.internalErrorf("%#v returned a nil result and nil error for a %q Request that requires a Response", from, req.Method)
        }

        response, respErr := NewResponse(req.ID, result, err)

        // Remove from incoming map before sending
        c.updateInFlight(func(s *inFlightState) {
            delete(s.incomingByID, req.ID)
        })
        
        if respErr == nil {
            writeErr := c.write(notDone{req.ctx}, response)
            if err == nil {
                err = writeErr
            }
        } else {
            err = c.internalErrorf("%#v returned a malformed result for %q: %w", from, req.Method, respErr)
        }
    } else { // req is a notification
        if result != nil {
            err = c.internalErrorf("%#v returned a non-nil result for a %q Request without an ID", from, req.Method)
        } else if err != nil {
            err = fmt.Errorf("%w: %q notification failed: %v", ErrInternal, req.Method, err)
        }
        if err != nil {
            event.Label(req.ctx, keys.Err.Of(err))
        }
    }

    // Cleanup
    labelStatus(req.ctx, err)
    req.cancel()
    req.endSpan()
    req.endSpan = nil
    c.updateInFlight(func(s *inFlightState) {
        if s.incoming == 0 {
            panic("jsonrpc2_v2: processResult called when incoming count is already zero")
        }
        s.incoming--
    })
    return nil
}
```

### Advanced Connection Features

#### Asynchronous Response Handling

```go
// Handler that returns ErrAsyncResponse
func (h *Handler) Handle(ctx context.Context, req *Request) (interface{}, error) {
    switch req.Method {
    case "longTask":
        // Start long-running task
        go h.performLongTask(req.ID, req.Params)
        return nil, ErrAsyncResponse
        
    case "quickTask":
        // Handle immediately
        return h.performQuickTask(req.Params), nil
    }
    
    return nil, ErrMethodNotFound
}

// Later, respond asynchronously
func (h *Handler) performLongTask(id ID, params json.RawMessage) {
    // Do long work...
    result, err := h.doLongWork(params)
    
    // Respond when done
    h.connection.Respond(id, result, err)
}
```

#### Connection Lifecycle Management

```go
// Graceful shutdown
func (c *Connection) Close() error {
    // Stop accepting new requests
    c.updateInFlight(func(s *inFlightState) { 
        s.connClosing = true 
    })
    
    // Wait for all requests to complete
    return c.Wait()
}

// Wait for connection to close
func (c *Connection) Wait() error {
    var err error
    <-c.done
    c.updateInFlight(func(s *inFlightState) {
        err = s.closeErr
    })
    return err
}
```

#### Error Handling and Recovery

```go
// Custom error handler
func (c *Connection) onInternalError(err error) {
    log.Printf("JSON-RPC internal error: %v", err)
    
    // Could implement retry logic, circuit breaker, etc.
    if isRetryableError(err) {
        go c.retryConnection()
    }
}

// Connection retry logic
func (c *Connection) retryConnection() {
    time.Sleep(5 * time.Second)
    // Attempt to reconnect...
}
```

### Performance Considerations

#### Memory Management
- **Lazy parsing** - `json.RawMessage` avoids unnecessary allocations
- **Connection pooling** - Reuse connections when possible
- **Goroutine management** - Handler goroutines start/stop as needed

#### Concurrency Patterns
- **Single writer** - Writer channel ensures atomic writes
- **Sequential processing** - One request at a time per connection
- **State synchronization** - All state changes through `updateInFlight`

#### Monitoring and Observability
- **Event tracing** - Full request/response lifecycle tracking
- **Metrics** - Byte counts, request counts, error rates
- **Context propagation** - Request context flows through entire pipeline

## 6. Server Functionality (`serve.go`)

### How do you create a server?

```go
// Create a listener (e.g., TCP port 8080)
listener, err := NetListener(ctx, "tcp", ":8080", NetListenOptions{})

// Create a server with a handler
server := NewServer(ctx, listener, ConnectionOptions{
    Handler: myHandler,
})

// Wait for the server to finish
err = server.Wait()
```

### What does the server do?
- Accepts incoming connections
- Creates a new connection for each client
- Routes requests to your handler
- Manages connection lifecycle

## 7. Network Transport (`net.go`)

### How do you connect over the network?

#### TCP Connection
```go
// Create a dialer
dialer := NetDialer("tcp", "localhost:8080", net.Dialer{})

// Connect
conn, err := Dial(ctx, dialer, ConnectionOptions{
    Handler: myHandler,
})
```

#### Unix Socket
```go
// For local communication
dialer := NetDialer("unix", "/tmp/mysocket", net.Dialer{})
```

## Complete Example

Here's a simple client-server example:

### Server
```go
func main() {
    // Create a handler
    handler := HandlerFunc(func(ctx context.Context, req *Request) (interface{}, error) {
        switch req.Method {
        case "add":
            // Parse parameters
            var params []int
            json.Unmarshal(req.Params, &params)
            
            // Calculate result
            sum := 0
            for _, n := range params {
                sum += n
            }
            
            return sum, nil
        default:
            return nil, ErrMethodNotFound
        }
    })
    
    // Create server
    listener, _ := NetListener(ctx, "tcp", ":8080", NetListenOptions{})
    server := NewServer(ctx, listener, ConnectionOptions{Handler: handler})
    
    // Run server
    server.Wait()
}
```

### Client
```go
func main() {
    // Connect to server
    dialer := NetDialer("tcp", "localhost:8080", net.Dialer{})
    conn, _ := Dial(ctx, dialer, ConnectionOptions{})
    
    // Make a call
    call := conn.Call(ctx, "add", []int{5, 3})
    
    // Wait for result
    var result int
    err := call.Await(ctx, &result)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Result: %d\n", result) // Prints: Result: 8
}
```

## Key Concepts Summary

1. **JSON-RPC 2.0** - A protocol for calling remote functions using JSON
2. **Request/Response** - Messages with IDs that expect answers
3. **Notifications** - Messages without IDs that don't expect answers
4. **Handlers** - Functions that process incoming requests
5. **Connections** - Manage the conversation between client and server
6. **Framing** - How messages are packaged for transport
7. **Error Handling** - Standard error codes and proper error propagation

## Why Use JSON-RPC?

- **Language agnostic** - Any language can implement it
- **Simple** - Easy to understand and debug
- **Efficient** - Minimal overhead
- **Standardized** - Well-defined protocol
- **Used by LSP** - Language Server Protocol uses JSON-RPC 2.0

## Advanced Topics and Best Practices

### Production-Ready Patterns

#### Connection Pooling
```go
type ConnectionPool struct {
    connections chan *Connection
    dialer      Dialer
    maxSize     int
    mu          sync.RWMutex
}

func NewConnectionPool(dialer Dialer, maxSize int) *ConnectionPool {
    return &ConnectionPool{
        connections: make(chan *Connection, maxSize),
        dialer:      dialer,
        maxSize:     maxSize,
    }
}

func (p *ConnectionPool) Get(ctx context.Context) (*Connection, error) {
    select {
    case conn := <-p.connections:
        return conn, nil
    default:
        // Create new connection
        return Dial(ctx, p.dialer, ConnectionOptions{
            Handler: p.createHandler(),
        })
    }
}
```

#### Circuit Breaker Pattern
```go
type CircuitBreaker struct {
    maxFailures int
    timeout     time.Duration
    failures    int
    lastFailure time.Time
    state       State
    mu          sync.RWMutex
}

func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if cb.state == StateOpen {
        if time.Since(cb.lastFailure) < cb.timeout {
            return errors.New("circuit breaker is open")
        }
        cb.state = StateHalfOpen
    }

    err := fn()
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        if cb.failures >= cb.maxFailures {
            cb.state = StateOpen
        }
        return err
    }

    cb.failures = 0
    cb.state = StateClosed
    return nil
}
```

### Security Considerations

#### Authentication and Authorization
```go
type AuthHandler struct {
    next     Handler
    authFunc func(ctx context.Context, req *Request) (context.Context, error)
}

func (h *AuthHandler) Handle(ctx context.Context, req *Request) (interface{}, error) {
    // Authenticate request
    ctx, err := h.authFunc(ctx, req)
    if err != nil {
        return nil, &wireError{
            Code:    -32001,
            Message: "Authentication failed",
        }
    }
    
    // Process with authenticated context
    return h.next.Handle(ctx, req)
}
```

#### Rate Limiting
```go
type RateLimiter struct {
    limiter *rate.Limiter
    next    Handler
}

func (rl *RateLimiter) Handle(ctx context.Context, req *Request) (interface{}, error) {
    if !rl.limiter.Allow() {
        return nil, &wireError{
            Code:    -32000,
            Message: "Rate limit exceeded",
        }
    }
    
    return rl.next.Handle(ctx, req)
}
```

### Performance Optimization

#### Connection Reuse
```go
type ConnectionManager struct {
    connections map[string]*Connection
    mu          sync.RWMutex
    dialer      Dialer
}

func (cm *ConnectionManager) GetConnection(ctx context.Context, endpoint string) (*Connection, error) {
    cm.mu.RLock()
    conn, exists := cm.connections[endpoint]
    cm.mu.RUnlock()
    
    if exists && conn != nil {
        return conn, nil
    }
    
    // Create new connection if needed
    cm.mu.Lock()
    defer cm.mu.Unlock()
    
    conn, err := Dial(ctx, cm.dialer, ConnectionOptions{
        Handler: cm.createHandler(),
    })
    if err != nil {
        return nil, err
    }
    
    cm.connections[endpoint] = conn
    return conn, nil
}
```

### Testing Strategies

#### Unit Testing
```go
func TestHandler(t *testing.T) {
    handler := &TestHandler{}
    
    // Test valid request
    req := &Request{
        ID:     Int64ID(1),
        Method: "add",
        Params: json.RawMessage(`[2, 3]`),
    }
    
    result, err := handler.Handle(context.Background(), req)
    assert.NoError(t, err)
    assert.Equal(t, 5, result)
}
```

#### Integration Testing
```go
func TestServerIntegration(t *testing.T) {
    // Start server
    listener, err := NetListener(context.Background(), "tcp", ":0", NetListenOptions{})
    require.NoError(t, err)
    
    server := NewServer(context.Background(), listener, ConnectionOptions{
        Handler: &TestHandler{},
    })
    
    // Connect client
    dialer := NetDialer("tcp", listener.Addr().String(), net.Dialer{})
    conn, err := Dial(context.Background(), dialer, ConnectionOptions{})
    require.NoError(t, err)
    
    // Test call
    call := conn.Call(context.Background(), "add", []int{2, 3})
    var result int
    err = call.Await(context.Background(), &result)
    
    assert.NoError(t, err)
    assert.Equal(t, 5, result)
    
    // Cleanup
    conn.Close()
    server.Shutdown()
}
```

## Conclusion

This implementation provides a robust, production-ready JSON-RPC 2.0 library that handles all the complexity of the protocol while providing a clean, easy-to-use API. The comprehensive guide above covers:

- **Protocol fundamentals** - Understanding JSON-RPC 2.0 specification
- **Implementation details** - Deep dive into the codebase architecture  
- **Advanced patterns** - Production-ready patterns and best practices
- **Performance optimization** - Memory management and concurrency patterns
- **Security considerations** - Authentication, authorization, and rate limiting
- **Testing strategies** - Unit and integration testing approaches

The library is designed to be:
- **Performant** - Efficient memory usage and minimal allocations
- **Reliable** - Comprehensive error handling and recovery
- **Observable** - Full tracing and metrics support
- **Secure** - Built-in security patterns and best practices
- **Testable** - Clean interfaces and comprehensive test support

Whether you're building a simple client-server application or a complex distributed system, this JSON-RPC 2.0 implementation provides the foundation you need for reliable, efficient communication between services.
