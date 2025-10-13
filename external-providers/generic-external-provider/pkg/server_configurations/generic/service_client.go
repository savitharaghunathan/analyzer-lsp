package generic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/go-logr/logr"
	jsonrpc2 "github.com/konveyor/analyzer-lsp/jsonrpc2_v2"
	base "github.com/konveyor/analyzer-lsp/lsp/base_service_client"
	"github.com/konveyor/analyzer-lsp/lsp/protocol"
	"github.com/konveyor/analyzer-lsp/provider"
	"github.com/swaggest/openapi-go/openapi3"
	"gopkg.in/yaml.v2"
)

// **DELETE THIS COMMENT BLOCK FOR NEW SERVICE CLIENTS**
//
// Suppose the name of your language server is `foo-lsp`. The recommended
// pattern of adding new service clients is:
//
// 1. Create a new folder in `server_configurations` with the name of your lsp
//    server, `foo_lsp`. Copy `generic/service_client.go` into the new directory.
//
// 2. Change the package name to `foo_lsp`. Change the occurrences of
//    `GenericServiceClient` to `FooServiceClient`.
//
// 3. Add any variables you need to the `FooServiceClient` and
//    `FooServiceClientConfig` struct.
//
// 4. Modify any parameters related to the `initialize` request. If you need
//    additional `jsonrpc2_v2` handlers, say for responding to messages from the
//    server in a specific way, pass those into the base service client.
//
// 5. Implement your capabilities and update the `FooServiceClientCapabilities`
//    slice
//
// 6. In constants.go, add `NewFooServiceClient` to SupportedLanguages and
//    `FooServiceClientCapabilities` to SupportedCapabilities

// createRPCConnection creates a real jsonrpc2.Connection using RPC client as transport
func createRPCConnection(ctx context.Context, rpc provider.RPCClient) (*jsonrpc2.Connection, error) {
	log.Printf("[RPC DEBUG] createRPCConnection() starting...")
	dialer := &rpcDialer{rpc: rpc}
	
	log.Printf("[RPC DEBUG] Calling jsonrpc2.Dial()...")
	conn, err := jsonrpc2.Dial(ctx, dialer, jsonrpc2.ConnectionOptions{})
	if err != nil {
		log.Printf("[RPC DEBUG] jsonrpc2.Dial() failed: %v", err)
		return nil, fmt.Errorf("failed to create RPC connection: %w", err)
	}
	
	log.Printf("[RPC DEBUG] jsonrpc2.Dial() succeeded, connection created")
	return conn, nil
}

// rpcDialer implements jsonrpc2.Dialer using provider.RPCClient
type rpcDialer struct {
	rpc provider.RPCClient
}

func (d *rpcDialer) Dial(ctx context.Context) (io.ReadWriteCloser, error) {
	log.Printf("[RPC DEBUG] rpcDialer.Dial() called, creating rpcTransport...")
	transport := &rpcTransport{rpc: d.rpc, ctx: ctx}
	log.Printf("[RPC DEBUG] rpcDialer.Dial() completed, returning transport")
	return transport, nil
}

// rpcTransport implements io.ReadWriteCloser and bridges JSON-RPC clients
type rpcTransport struct {
	rpc provider.RPCClient
	ctx context.Context
}

func (t *rpcTransport) Read(p []byte) (n int, err error) {
	log.Printf("[RPC DEBUG] rpcTransport.Read() called with buffer size %d", len(p))
	// For JSON-RPC client bridge, we don't handle raw reads
	// The jsonrpc2 library will handle the protocol
	log.Printf("[RPC DEBUG] rpcTransport.Read() returning EOF")
	return 0, io.EOF
}

func (t *rpcTransport) Write(p []byte) (n int, err error) {
	log.Printf("[RPC DEBUG] rpcTransport.Write() called with message: %s", string(p))
	
	// Parse the JSON-RPC message and route to appropriate RPC client method
	var msg map[string]interface{}
	if err := json.Unmarshal(p, &msg); err != nil {
		log.Printf("[RPC DEBUG] Failed to parse JSON-RPC message: %v", err)
		return 0, fmt.Errorf("failed to parse JSON-RPC message: %w", err)
	}

	method, _ := msg["method"].(string)
	params := msg["params"]
	id := msg["id"]

	log.Printf("[RPC DEBUG] Parsed message - method: %s, id: %v, params: %v", method, id, params)

	if id != nil {
		// Request with response expected
		log.Printf("[RPC DEBUG] Making RPC Call to method: %s", method)
		err := t.rpc.Call(t.ctx, method, params, nil)
		if err != nil {
			log.Printf("[RPC DEBUG] RPC Call failed: %v", err)
			return 0, err
		}
		log.Printf("[RPC DEBUG] RPC Call succeeded for method: %s", method)
	} else {
		// Notification
		log.Printf("[RPC DEBUG] Making RPC Notify to method: %s", method)
		err := t.rpc.Notify(t.ctx, method, params)
		if err != nil {
			log.Printf("[RPC DEBUG] RPC Notify failed: %v", err)
			return 0, err
		}
		log.Printf("[RPC DEBUG] RPC Notify succeeded for method: %s", method)
	}

	log.Printf("[RPC DEBUG] rpcTransport.Write() completed successfully")
	return len(p), nil
}

func (t *rpcTransport) Close() error {
	return nil
}

type GenericServiceClientConfig struct {
	base.LSPServiceClientConfig `yaml:",inline"`
}

// Tidy aliases
type serviceClientFn = base.LSPServiceClientFunc[*GenericServiceClient]

type GenericServiceClient struct {
	*base.LSPServiceClientBase
	*base.LSPServiceClientEvaluator[*GenericServiceClient]

	Config GenericServiceClientConfig

	// RPC mode fields (same as Java provider)
	rpc    provider.RPCClient
	config provider.InitConfig
	log    logr.Logger
}

type GenericServiceClientBuilder struct{}

func (g *GenericServiceClientBuilder) Init(ctx context.Context, log logr.Logger, c provider.InitConfig) (provider.ServiceClient, error) {

	if c.RPC != nil {
		sc := &GenericServiceClient{
			rpc:    c.RPC,
			config: c,
			log:    log,
		}

		// Create a real jsonrpc2.Connection using RPC client as transport
		conn, err := createRPCConnection(ctx, c.RPC)
		if err != nil {
			return nil, fmt.Errorf("failed to create RPC connection: %w", err)
		}
		
		scBase := &base.LSPServiceClientBase{
			Conn: conn,
		}
		sc.LSPServiceClientBase = scBase

		eval, err := base.NewLspServiceClientEvaluator[*GenericServiceClient](sc, g.GetGenericServiceClientCapabilities(log))
		if err != nil {
			return nil, fmt.Errorf("lsp service client evaluator error: %w", err)
		}
		sc.LSPServiceClientEvaluator = eval

		return sc, nil
	}

	sc := &GenericServiceClient{}

	// Unmarshal the config
	b, _ := yaml.Marshal(c.ProviderSpecificConfig)
	err := yaml.Unmarshal(b, &sc.Config)
	if err != nil {
		return nil, fmt.Errorf("generic providerSpecificConfig Unmarshal error: %w", err)
	}

	// Create the parameters for the `initialize` request
	//
	// TODO(jsussman): Support more than one folder. This hack with only taking
	// the first item in WorkspaceFolders is littered throughout.
	params := protocol.InitializeParams{}

	if c.Location != "" {
		sc.Config.WorkspaceFolders = []string{c.Location}
	}

	if len(sc.Config.WorkspaceFolders) == 0 {
		params.RootURI = ""
	} else {
		params.RootURI = sc.Config.WorkspaceFolders[0]
	}

	params.Capabilities = protocol.ClientCapabilities{}

	var InitializationOptions map[string]any
	err = json.Unmarshal([]byte(sc.Config.LspServerInitializationOptions), &InitializationOptions)
	if err != nil {
		// fmt.Printf("Could not unmarshal into map[string]any: %s\n", sc.Config.LspServerInitializationOptions)
		params.InitializationOptions = map[string]any{}
	} else {
		params.InitializationOptions = InitializationOptions
	}

	// Initialize the base client
	scBase, err := base.NewLSPServiceClientBase(
		ctx, log, c,
		base.LogHandler(log),
		params,
	)
	if err != nil {
		return nil, fmt.Errorf("base client initialization error: %w", err)
	}
	sc.LSPServiceClientBase = scBase

	// Initialize the fancy evaluator (dynamic dispatch ftw)
	eval, err := base.NewLspServiceClientEvaluator[*GenericServiceClient](sc, g.GetGenericServiceClientCapabilities(log))
	if err != nil {
		return nil, fmt.Errorf("lsp service client evaluator error: %w", err)
	}
	sc.LSPServiceClientEvaluator = eval

	return sc, nil
}

func (g *GenericServiceClientBuilder) GetGenericServiceClientCapabilities(log logr.Logger) []base.LSPServiceClientCapability {
	caps := []base.LSPServiceClientCapability{}
	r := openapi3.NewReflector()
	refCap, err := provider.ToProviderCap(r, log, base.ReferencedCondition{}, "referenced")
	if err != nil {
		log.Error(err, "unable to get referenced cap")
	} else {
		caps = append(caps, base.LSPServiceClientCapability{
			Capability: refCap,
			Fn:         serviceClientFn(base.EvaluateReferenced[*GenericServiceClient]),
		})
	}
	depCap, err := provider.ToProviderCap(r, log, base.NoOpCondition{}, "dependency")
	if err != nil {
		log.Error(err, "unable to get referenced cap")
	} else {
		caps = append(caps, base.LSPServiceClientCapability{
			Capability: depCap,
			Fn:         serviceClientFn(base.EvaluateNoOp[*GenericServiceClient]),
		})
	}
	echoCap, err := provider.ToProviderCap(r, log, echoCondition{}, "echo")
	if err != nil {
		log.Error(err, "unable to get referenced cap")
	} else {
		caps = append(caps, base.LSPServiceClientCapability{
			Capability: echoCap,
			Fn:         serviceClientFn((*GenericServiceClient).EvaluateEcho),
		})
	}
	return caps

}

// Example condition
type echoCondition struct {
	Echo struct {
		Input string `yaml:"input" json:"input"`
	} `yaml:"echo" json:"input"`
}

// Example evaluate
func (sc *GenericServiceClient) EvaluateEcho(ctx context.Context, cap string, info []byte) (provider.ProviderEvaluateResponse, error) {
	var cond echoCondition
	err := yaml.Unmarshal(info, &cond)
	if err != nil {
		return provider.ProviderEvaluateResponse{}, fmt.Errorf("error unmarshaling query info")
	}

	return provider.ProviderEvaluateResponse{
		Matched: true,
		Incidents: []provider.IncidentContext{
			{
				Variables: map[string]interface{}{
					"output": cond.Echo.Input,
				},
			},
		},
	}, nil
}

// Evaluate method - now handled by LSPServiceClientEvaluator for both modes
func (sc *GenericServiceClient) Evaluate(ctx context.Context, cap string, conditionInfo []byte) (provider.ProviderEvaluateResponse, error) {
	log.Printf("[RPC DEBUG] GenericServiceClient.Evaluate() called with capability: %s", cap)
	log.Printf("[RPC DEBUG] GenericServiceClient has RPC client: %v", sc.rpc != nil)
	log.Printf("[RPC DEBUG] GenericServiceClient has LSPServiceClientBase: %v", sc.LSPServiceClientBase != nil)
	log.Printf("[RPC DEBUG] GenericServiceClient has LSPServiceClientEvaluator: %v", sc.LSPServiceClientEvaluator != nil)
	
	if sc.LSPServiceClientBase != nil && sc.LSPServiceClientBase.Conn != nil {
		log.Printf("[RPC DEBUG] LSPServiceClientBase.Conn exists")
	} else {
		log.Printf("[RPC DEBUG] LSPServiceClientBase.Conn is nil")
	}
	
	log.Printf("[RPC DEBUG] Calling LSPServiceClientEvaluator.Evaluate()...")
	result, err := sc.LSPServiceClientEvaluator.Evaluate(ctx, cap, conditionInfo)
	
	if err != nil {
		log.Printf("[RPC DEBUG] LSPServiceClientEvaluator.Evaluate() failed: %v", err)
	} else {
		log.Printf("[RPC DEBUG] LSPServiceClientEvaluator.Evaluate() succeeded, matched: %v, incidents: %d", 
			result.Matched, len(result.Incidents))
	}
	
	return result, err
}

// Stop method that handles both RPC and process modes
func (sc *GenericServiceClient) Stop() {
	if sc.rpc != nil {
		// RPC mode: just cleanup, don't kill external process
		sc.log.Info("stopping RPC-based generic service client")
		return
	}

	// Process mode: use base implementation (existing behavior)
	if sc.LSPServiceClientBase != nil {
		sc.LSPServiceClientBase.Stop()
	}
}
