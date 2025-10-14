package generic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
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

// createRPCConnection wraps an RPC client to create a proper jsonrpc2 connection
func createRPCConnection(ctx context.Context, rpc provider.RPCClient, log logr.Logger) (base.RPCConn, error) {
	log.Info("Creating RPC connection wrapper")
	if rpc == nil {
		log.Error(fmt.Errorf("RPC client is nil"), "Failed to create RPC connection")
		return nil, fmt.Errorf("RPC client is nil")
	}
	log.Info("Successfully created RPC connection wrapper")
	return base.NewRPCConnWrapper(rpc, log), nil
}

func (g *GenericServiceClientBuilder) Init(ctx context.Context, log logr.Logger, c provider.InitConfig) (provider.ServiceClient, error) {
	// Check for RPC mode first (same pattern as Java provider)
	if c.RPC != nil {
		log.Info("Initializing GenericServiceClient in RPC mode")
		sc := &GenericServiceClient{
			rpc:    c.RPC,
			config: c,
			log:    log,
		}

		log.Info("Creating RPC connection wrapper for IDE communication")
		// Create a real jsonrpc2.Connection using RPC client as transport
		conn, err := createRPCConnection(ctx, c.RPC, log)
		if err != nil {
			log.Error(err, "Failed to create RPC connection wrapper")
			return nil, fmt.Errorf("failed to create RPC connection: %w", err)
		}

		log.Info("Creating LSPServiceClientBase with RPC connection")
		scBase := &base.LSPServiceClientBase{
			Conn: conn,
			Log:  log,
			Ctx:  ctx,
			BaseConfig: base.LSPServiceClientConfig{
				WorkspaceFolders: []string{c.Location},
			},
			// Set default server capabilities for RPC mode
			ServerCapabilities: protocol.ServerCapabilities{
				WorkspaceSymbolProvider: &protocol.WorkspaceSymbolOptions{},
			},
		}
		sc.LSPServiceClientBase = scBase

		log.Info("Creating LSP service client evaluator")
		eval, err := base.NewLspServiceClientEvaluator[*GenericServiceClient](sc, g.GetGenericServiceClientCapabilities(log))
		if err != nil {
			log.Error(err, "Failed to create LSP service client evaluator")
			return nil, fmt.Errorf("lsp service client evaluator error: %w", err)
		}
		sc.LSPServiceClientEvaluator = eval

		log.Info("Successfully initialized GenericServiceClient in RPC mode")
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

func (sc *GenericServiceClient) Evaluate(ctx context.Context, cap string, conditionInfo []byte) (provider.ProviderEvaluateResponse, error) {
	sc.log.Info("GenericServiceClient: evaluating capability", "capability", cap)
	sc.log.V(2).Info("GenericServiceClient: condition info", "capability", cap, "conditionInfo", string(conditionInfo))
	
	result, err := sc.LSPServiceClientEvaluator.Evaluate(ctx, cap, conditionInfo)
	if err != nil {
		sc.log.Error(err, "GenericServiceClient: evaluation failed", "capability", cap)
	} else {
		sc.log.Info("GenericServiceClient: evaluation completed", "capability", cap, "matched", result.Matched, "incidents", len(result.Incidents))
	}
	
	return result, err
}

func (sc *GenericServiceClient) Stop() {
	if sc.LSPServiceClientBase != nil {
		sc.LSPServiceClientBase.Stop()
	}
}
