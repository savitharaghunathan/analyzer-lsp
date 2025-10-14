package generic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/go-logr/logr"
	jsonrpc2 "github.com/konveyor/analyzer-lsp/jsonrpc2_v2"
	base "github.com/konveyor/analyzer-lsp/lsp/base_service_client"
	"github.com/konveyor/analyzer-lsp/lsp/protocol"
	"github.com/konveyor/analyzer-lsp/provider"
	"github.com/konveyor/analyzer-lsp/rpc"
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
	// Check for RPC mode first (same pattern as Java provider)
	if c.RPC != nil {
		// Extract the underlying streams from kai RPC client to create jsonrpc2_v2 Connection
		// This allows us to reuse base client's evaluate functionality
		conn, err := extractConnectionFromKaiClient(ctx, log, c.RPC)
		if err != nil {
			return nil, fmt.Errorf("failed to extract connection from kai RPC client: %w", err)
		}

		// Wrap the jsonrpc2_v2 Connection to match provider.RPCClient interface
		wrappedRPC := rpc.NewClientWrapper(conn)

		sc := &GenericServiceClient{
			rpc:    wrappedRPC, // Use the wrapped RPC client
			config: c,
			log:    log,
		}

		// Create base service client with the jsonrpc2_v2 connection for evaluate functionality
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

// rwcDialer implements jsonrpc2.Dialer for an existing ReadWriteCloser
type rwcDialer struct {
	rwc io.ReadWriteCloser
}

func (d *rwcDialer) Dial(ctx context.Context) (io.ReadWriteCloser, error) {
	return d.rwc, nil
}

// extractConnectionFromKaiClient extracts the underlying streams from kai RPC client
// and creates a jsonrpc2_v2 Connection from them
func extractConnectionFromKaiClient(ctx context.Context, log logr.Logger, kaiClient provider.RPCClient) (*jsonrpc2.Connection, error) {
	// The kai client wraps an rpc2.Client which has access to the underlying streams
	// We need to extract those streams using reflection since they're not exposed

	clientValue := reflect.ValueOf(kaiClient)
	if clientValue.Kind() == reflect.Ptr {
		clientValue = clientValue.Elem()
	}

	// Look for the embedded rpc2.Client field
	var rpc2Client reflect.Value
	for i := 0; i < clientValue.NumField(); i++ {
		field := clientValue.Field(i)
		fieldType := clientValue.Type().Field(i)

		// Check if this is the embedded rpc2.Client
		if fieldType.Type.String() == "*rpc2.Client" || fieldType.Name == "Client" {
			rpc2Client = field
			break
		}
	}

	if !rpc2Client.IsValid() {
		return nil, fmt.Errorf("could not find rpc2.Client in kai client")
	}

	// Extract the streams from rpc2.Client using reflection
	// rpc2.Client should have a codec field that contains the streams
	codecField := rpc2Client.Elem().FieldByName("codec")
	if !codecField.IsValid() {
		return nil, fmt.Errorf("could not find codec field in rpc2.Client")
	}

	// The codec should have rwc (ReadWriteCloser) field
	rwcField := codecField.Elem().FieldByName("rwc")
	if !rwcField.IsValid() {
		return nil, fmt.Errorf("could not find rwc field in codec")
	}

	// Extract the ReadWriteCloser
	rwc, ok := rwcField.Interface().(io.ReadWriteCloser)
	if !ok {
		return nil, fmt.Errorf("rwc field is not a ReadWriteCloser")
	}

	// Create jsonrpc2_v2 Connection from the extracted streams
	// Use the correct jsonrpc2_v2 API with custom dialer
	rwcDialer := &rwcDialer{rwc: rwc}

	// Create connection options with handler
	connOptions := jsonrpc2.ConnectionOptions{
		Framer: jsonrpc2.HeaderFramer(),
		Handler: jsonrpc2.HandlerFunc(func(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
			// Basic logging handler for incoming requests
			log.V(5).Info("received jsonrpc2 request", "method", req.Method)
			return nil, jsonrpc2.ErrNotHandled
		}),
	}

	conn, err := jsonrpc2.Dial(ctx, rwcDialer, connOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonrpc2_v2 connection: %w", err)
	}

	return conn, nil
}

// Evaluate method that handles both RPC and process modes
func (sc *GenericServiceClient) Evaluate(ctx context.Context, cap string, conditionInfo []byte) (provider.ProviderEvaluateResponse, error) {
	return sc.LSPServiceClientEvaluator.Evaluate(ctx, cap, conditionInfo)
}

// Stop method that handles both RPC and process modes
func (sc *GenericServiceClient) Stop() {

	// Process mode: use base implementation (existing behavior)
	if sc.LSPServiceClientBase != nil {
		sc.LSPServiceClientBase.Stop()
	}
}
