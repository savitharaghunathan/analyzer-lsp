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

func (g *GenericServiceClientBuilder) Init(ctx context.Context, log logr.Logger, c provider.InitConfig) (provider.ServiceClient, error) {
	// Check for RPC mode first (same pattern as Java provider)
	if c.RPC != nil {
		return &GenericServiceClient{
			rpc:    c.RPC,
			config: c,
			log:    log,
		}, nil
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

func (sc *GenericServiceClient) isRPCMode() bool {
	return sc.rpc != nil
}

// Evaluate method that handles both RPC and process modes
func (sc *GenericServiceClient) Evaluate(ctx context.Context, cap string, conditionInfo []byte) (provider.ProviderEvaluateResponse, error) {
	if sc.isRPCMode() {
		return sc.evaluateRPC(ctx, cap, conditionInfo)
	}

	// Process mode: use evaluator (existing behavior)
	return sc.LSPServiceClientEvaluator.Evaluate(ctx, cap, conditionInfo)
}

// Stop method that handles both RPC and process modes
func (sc *GenericServiceClient) Stop() {
	if sc.isRPCMode() {
		// RPC mode: just cleanup, don't kill external process
		sc.log.Info("stopping RPC-based generic service client")
		return
	}

	// Process mode: use base implementation (existing behavior)
	if sc.LSPServiceClientBase != nil {
		sc.LSPServiceClientBase.Stop()
	}
}

// RPC-based evaluation (forwards to external RPC client like Java provider)
func (sc *GenericServiceClient) evaluateRPC(ctx context.Context, cap string, conditionInfo []byte) (provider.ProviderEvaluateResponse, error) {
	switch cap {
	case "referenced":
		return sc.evaluateReferencedRPC(ctx, conditionInfo)
	case "echo":
		// Reuse existing echo implementation
		return sc.EvaluateEcho(ctx, cap, conditionInfo)
	default:
		return provider.ProviderEvaluateResponse{}, fmt.Errorf("capability '%s' not supported in RPC mode", cap)
	}
}

// RPC-based referenced evaluation using standard LSP methods
func (sc *GenericServiceClient) evaluateReferencedRPC(ctx context.Context, conditionInfo []byte) (provider.ProviderEvaluateResponse, error) {
	var cond base.ReferencedCondition
	err := yaml.Unmarshal(conditionInfo, &cond)
	if err != nil {
		return provider.ProviderEvaluateResponse{}, fmt.Errorf("unable to get query info: %v", err)
	}

	// Use standard LSP workspace/symbol method (supported by most language servers)
	params := map[string]interface{}{
		"query": cond.Referenced.Pattern,
	}

	var symbols []interface{}
	err = sc.rpc.Call(ctx, "workspace/symbol", params, &symbols)
	if err != nil {
		sc.log.Error(err, "failed to call workspace/symbol via RPC")
		return provider.ProviderEvaluateResponse{}, err
	}

	// Convert symbols to incidents
	incidents := []provider.IncidentContext{}
	for _, symbol := range symbols {
		if symbolMap, ok := symbol.(map[string]interface{}); ok {
			incident := provider.IncidentContext{
				Variables: map[string]interface{}{
					"name":     symbolMap["name"],
					"kind":     symbolMap["kind"],
					"location": symbolMap["location"],
				},
			}
			incidents = append(incidents, incident)
		}
	}

	return provider.ProviderEvaluateResponse{
		Matched:   len(incidents) > 0,
		Incidents: incidents,
	}, nil
}
