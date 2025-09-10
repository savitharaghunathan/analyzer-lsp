# Component Flow in Analyzer Rule Engine

This document explains the detailed flow between components in the Analyzer Rule Engine, showing how data moves through the system from startup through analysis completion.

## Table of Contents

1. [Application Startup Flow](#application-startup-flow)
2. [Rule Processing Pipeline](#rule-processing-pipeline)
3. [Provider Communication Patterns](#provider-communication-patterns)
4. [Data Flow Through Engine Components](#data-flow-through-engine-components)
5. [Output Generation Flow](#output-generation-flow)
6. [Error Handling & Recovery](#error-handling--recovery)
7. [Lifecycle Management](#lifecycle-management)

## Application Startup Flow

### Phase 1: CLI Initialization & Configuration Loading
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   main.go       │    │  Configuration   │    │   Validation    │
│   Entry Point   │───▶│   Loading        │───▶│   & Setup       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                       │
        ├─ Parse CLI flags        ├─ Load settings.json   ├─ Validate paths
        ├─ Setup logging         ├─ Load rule files      ├─ Check providers
        └─ Initialize tracing    └─ Parse configurations └─ Verify analysis mode
```

**Key Data Flows:**
1. **CLI Flags** → **Global Configuration Variables**
2. **Settings File** → **Provider Configuration Structs**  
3. **Rule Files** → **Rule Parser Input**

**Configuration Processing:**
```go
// CLI flag parsing
logrusLog := logrus.New()
log := logrusr.New(logrusLog)

// Tracing initialization
tracerOptions := tracing.Options{
    EnableJaeger:   enableJaeger,
    JaegerEndpoint: jaegerEndpoint,
}
tp, err := tracing.InitTracerProvider(log, tracerOptions)

// Create main span
ctx, mainSpan := tracing.StartNewSpan(ctx, "main")
defer mainSpan.End()
```

### Phase 2: Provider System Bootstrap
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Configuration  │    │   Provider       │    │   Provider      │
│   Processing    │───▶│   Creation       │───▶│  Initialization │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                       │
        │                        │                       │
        ▼                        ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│               Provider Hierarchy Creation                       │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Built-in  │  │    Java     │  │      External gRPC      │  │
│  │             │  │             │  │                         │  │
│  │ • Auto-added│  │ • JDT-LS    │  │ • TCP connections       │  │
│  │ • File ops  │  │ • Process   │  │ • Authentication        │  │
│  │ • XML/JSON  │  │ • JSONRPC2  │  │ • Session management    │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Provider Creation Flow:**
```go
// Load provider configurations
configs, err := provider.GetConfig(settingsFile)

// Auto-add builtin providers for each source location
defaultBuiltinConfigs := []provider.InitConfig{}
for _, config := range configs {
    for _, initConf := range config.InitConfig {
        if initConf.Location != "" {
            builtinLocation, _ := filepath.Abs(initConf.Location)
            builtinConf := provider.InitConfig{Location: builtinLocation}
            defaultBuiltinConfigs = append(defaultBuiltinConfigs, builtinConf)
        }
    }
}

// Create provider clients
providers := map[string]provider.InternalProviderClient{}
for _, config := range finalConfigs {
    prov, err := lib.GetProviderClient(config, log)
    providers[config.Name] = prov
    
    // Start provider if needed
    if s, ok := prov.(provider.Startable); ok {
        err := s.Start(ctx)
    }
}
```

## Rule Processing Pipeline

### Phase 3: Rule Loading & Parsing
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Rule Files    │    │   Rule Parser    │    │  Rule Validation│
│  (YAML/Dir)     │───▶│   Processing     │───▶│   & Provider    │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                       │
        │                        │                       │
        ▼                        ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                Rule Parsing Flow                               │
├─────────────────────────────────────────────────────────────────┤
│  rules/file.yaml  →  YAML Parse  →  Rule Struct  →  Validation  │
│       │                  │             │               │        │
│       │                  │             │               │        │
│  ┌────▼────┐    ┌────────▼──┐    ┌─────▼────┐    ┌─────▼────┐   │
│  │ Ruleset │    │ Condition │    │Provider  │    │Required  │   │
│  │ Golden  │    │ Parsing   │    │Capability│    │Providers │   │
│  │ File    │    │ Tree      │    │Discovery │    │Collection│   │
│  └─────────┘    └───────────┘    └──────────┘    └──────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

**Rule Parser Data Flow:**
```go
parser := parser.RuleParser{
    ProviderNameToClient: providers,
    Log:                  log.WithName("parser"),
    NoDependencyRules:    noDependencyRules,
    DepLabelSelector:     dependencyLabelSelector,
}

ruleSets := []engine.RuleSet{}
needProviders := map[string]provider.InternalProviderClient{}

for _, ruleFile := range rulesFile {
    // Parse each rule file
    internRuleSet, internNeedProviders, err := parser.LoadRules(ruleFile)
    
    // Accumulate results
    ruleSets = append(ruleSets, internRuleSet...)
    for k, v := range internNeedProviders {
        needProviders[k] = v  // Collect required providers
    }
}
```

**Rule Structure Processing:**
1. **YAML Parsing**: Convert YAML to Go structs
2. **Condition Analysis**: Parse when/and/or conditions
3. **Provider Discovery**: Identify required provider capabilities
4. **Validation**: Check rule syntax and provider availability

### Phase 4: Engine Creation & Rule Categorization
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Parsed Rules   │    │   Rule Engine    │    │ Rule            │
│   & Providers   │───▶│   Creation       │───▶│ Categorization  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                       │
        │                        │                       │
        ▼                        ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Engine Initialization                         │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Worker Pool  │  │ Channels    │  │   Rule Filtering        │  │
│  │             │  │             │  │                         │  │
│  │• 10 Workers │  │• ruleMsg    │  │ • Tagging Rules         │  │
│  │• Goroutines │  │• response   │  │ • Regular Rules         │  │
│  │• ctx Cancel │  │• buffered   │  │ • Label Selectors       │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Engine Creation:**
```go
eng := engine.CreateRuleEngine(ctx,
    10,  // 10 worker goroutines
    log,
    engine.WithIncidentLimit(limitIncidents),
    engine.WithCodeSnipLimit(limitCodeSnips),
    engine.WithContextLines(contextLines),
    engine.WithIncidentSelector(incidentSelector),
    engine.WithLocationPrefixes(providerLocations),
)
```

**Worker Pool Setup:**
```go
func CreateRuleEngine(ctx context.Context, workers int, log logr.Logger, options ...Option) RuleEngine {
    ruleProcessor := make(chan ruleMessage, 10)  // Buffered channel
    ctx, cancelFunc := context.WithCancel(ctx)
    wg := &sync.WaitGroup{}
    
    // Start worker goroutines
    for i := 0; i < workers; i++ {
        logger := log.WithValues("worker", i)
        wg.Add(1)
        go processRuleWorker(ctx, ruleProcessor, logger, wg)
    }
    
    return &ruleEngine{
        ruleProcessing: ruleProcessor,
        cancelFunc:     cancelFunc,
        logger:         log,
        wg:             wg,
    }
}
```

## Provider Communication Patterns

### Phase 5: Provider Initialization & LSP Startup
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│Required         │    │Provider          │    │LSP Server       │
│Providers        │───▶│Initialization    │───▶│Startup          │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                       │
        │                        │                       │
        ▼                        ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                Provider Initialization Flow                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Java Provider:                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Process      │  │JSONRPC2     │  │Workspace                │  │
│  │Spawn        │──│Connection   │──│Setup                    │  │
│  │JDT-LS       │  │initialize   │  │• Source indexing        │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  External gRPC Provider:                                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │TCP          │  │gRPC         │  │Session                  │  │
│  │Connection   │──│Handshake    │──│Creation                 │  │
│  │Establish    │  │• Auth       │  │• Unique ID              │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  Built-in Provider:                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │File System  │  │Index        │  │Ready                    │  │
│  │Scan         │──│Building     │──│for Queries              │  │
│  │Locations    │  │• Patterns   │  │                         │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Provider Init Flow:**
```go
// Initialize non-builtin providers first  
additionalBuiltinConfigs := []provider.InitConfig{}
for name, provider := range needProviders {
    switch name {
    case "builtin":
        continue  // Handle builtin last
    default:
        initCtx, initSpan := tracing.StartNewSpan(ctx, "init",
            attribute.Key("provider").String(name))
        
        // Init provider and collect additional builtin configs
        additionalBuiltinConfs, err := provider.ProviderInit(initCtx, nil)
        if additionalBuiltinConfs != nil {
            additionalBuiltinConfigs = append(additionalBuiltinConfigs, additionalBuiltinConfs...)
        }
        initSpan.End()
    }
}

// Initialize builtin provider with additional configs
if builtinClient, ok := needProviders["builtin"]; ok {
    builtinClient.ProviderInit(ctx, additionalBuiltinConfigs)
}
```

### Communication Protocols by Provider Type

#### Java Provider (JSONRPC2/LSP)
```go
// Process spawn and LSP initialization
conn := jsonrpc2.NewConn(stream, log)
go conn.Run(ctx)

// Initialize LSP server
initParams := &protocol.InitializeParams{
    RootURI: uri.File(location),
}
var result protocol.InitializeResult
err = conn.Call(ctx, "initialize", initParams, &result)
conn.Notify(ctx, "initialized", nil)
```

#### External gRPC Provider
```go
// Establish gRPC connection
conn, err := grpc.Dial(address, grpc.WithInsecure())
client := libgrpc.NewProviderServiceClient(conn)

// Discover capabilities
caps, err := client.Capabilities(ctx, &emptypb.Empty{})

// Initialize session
initResp, err := client.Init(ctx, &libgrpc.Config{
    Location:     location,
    AnalysisMode: "full",
})
sessionID := initResp.Id
```

#### Built-in Provider
```go
// File system initialization
client := &BuiltinProvider{
    config:   config,
    searcher: provider.FileSearcher{BasePath: location},
}
// No network communication - direct function calls
```

### Phase 6: Analysis Execution Flow

#### Two-Phase Rule Execution
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│Rule             │    │Phase 1:          │    │Phase 2:         │
│Categorization   │───▶│Tagging Rules     │───▶│Regular Rules    │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                       │
        ▼                        ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Rule Execution Phases                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Phase 1: Tagging Rules (Synchronous)                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Sequential   │  │Provider     │  │Tag                      │  │
│  │Execution    │──│Evaluation   │──│Generation               │  │
│  │(blocking)   │  │             │  │• Build context          │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  Phase 2: Regular Rules (Asynchronous)                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Worker Pool  │  │Parallel     │  │Violation                │  │
│  │Distribution │──│Evaluation   │──│Creation                 │  │
│  │(fan-out)    │  │             │  │• With tag context       │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Rule Categorization Logic:**
```go
func (r *ruleEngine) filterRules(ruleSets []RuleSet, selectors ...RuleSelector) (
    taggingRules []ruleMessage,
    otherRules []ruleMessage, 
    mapRuleSets map[string]*konveyor.RuleSet) {
    
    for _, ruleSet := range ruleSets {
        for _, rule := range ruleSet.Rules {
            // Apply rule selectors
            for _, selector := range selectors {
                if !selector.Matches(rule) {
                    continue
                }
            }
            
            // Categorize rules based on presence of tag action
            if len(rule.Tag) > 0 {
                taggingRules = append(taggingRules, ruleMessage{
                    rule:             rule,
                    ruleSetName:      ruleSet.Name,
                    conditionContext: conditionContext.Copy(),
                    scope:            scopes,
                })
            } else {
                otherRules = append(otherRules, ruleMessage{...})
            }
        }
    }
    return
}
```

**Tagging Rules Execution (Synchronous):**
```go
func (r *ruleEngine) runTaggingRules(ctx context.Context, taggingRules []ruleMessage, 
    mapRuleSets map[string]*konveyor.RuleSet, conditionContext ConditionContext, 
    scopes Scope) ConditionContext {
    
    for _, ruleMsg := range taggingRules {
        // Execute tagging rule synchronously
        response, err := processRule(ctx, ruleMsg.rule, conditionContext, r.logger)
        
        if err == nil && response.Matched {
            // Add generated tags to context for subsequent rules
            for _, tag := range ruleMsg.rule.Tag {
                conditionContext.Tags[tag] = struct{}{}
            }
        }
    }
    return conditionContext
}
```

## Data Flow Through Engine Components

### Worker Pool Architecture & Message Flow
```
┌─────────────────────────────────────────────────────────────────┐
│                    Rule Engine Data Flow                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Rule Distribution:                                             │
│  ┌─────────────┐   buffered   ┌─────────────────────────────┐   │
│  │Main Thread  │──channel(10)──│    Worker Pool (10)         │   │
│  │             │               │                             │   │
│  │ • Rules     │               │ ┌─────┐ ┌─────┐ ┌─────┐     │   │
│  │ • Context   │               │ │ W1  │ │ W2  │ │ W3  │ ... │   │
│  │ • Carrier   │               │ └─────┘ └─────┘ └─────┘     │   │
│  └─────────────┘               └─────────────────────────────┘   │
│                                                                 │
│  Response Collection:                                           │
│  ┌─────────────┐   response    ┌─────────────────────────────┐   │
│  │Result       │◄──channel─────│    Worker Results           │   │
│  │Aggregator   │               │                             │   │
│  │             │               │ • Violations                │   │
│  │ • Violations│               │ • Errors                    │   │
│  │ • Errors    │               │ • Unmatched                 │   │
│  │ • Stats     │               │                             │   │
│  └─────────────┘               └─────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

**Worker Processing Flow:**
```go
func processRuleWorker(ctx context.Context, ruleMessages chan ruleMessage, logger logr.Logger, wg *sync.WaitGroup) {
    for {
        select {
        case m := <-ruleMessages:
            // 1. Extract tracing context
            ctx = otel.GetTextMapPropagator().Extract(ctx, m.carrier)
            
            // 2. Setup isolated condition context
            m.conditionContext.Template = make(map[string]ChainTemplate)
            if m.scope != nil {
                m.scope.AddToContext(&m.conditionContext)
            }
            
            // 3. Process rule through provider
            response, err := processRule(ctx, m.rule, m.conditionContext, logger)
            
            // 4. Send response back
            m.returnChan <- response{
                ConditionResponse: response,
                Err:               err,
                Rule:              m.rule,
                RuleSetName:       m.ruleSetName,
            }
        case <-ctx.Done():
            wg.Done()
            return
        }
    }
}
```

### Condition Evaluation Data Flow
```
┌─────────────────────────────────────────────────────────────────┐
│              Condition Processing Pipeline                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Input: Rule + Context                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Rule         │  │Condition    │  │Provider                 │  │
│  │Condition    │──│Context      │──│Evaluation               │  │
│  │             │  │• Tags       │  │                         │  │
│  └─────────────┘  │• Templates  │  └─────────────────────────┘  │
│                   │• Variables  │                              │
│                   └─────────────┘                              │
│                                                                 │
│  Processing: Provider-Specific Logic                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Java         │  │Built-in     │  │External gRPC            │  │
│  │• LSP Query  │  │• File Ops   │  │• RPC Call               │  │
│  │• JDT-LS     │  │• Regex      │  │• Session                │  │
│  │• References │  │• XPath      │  │• Protobuf               │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  Output: Incident Contexts                                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Incidents    │  │Template     │  │Match                    │  │
│  │• FileURI    │  │Variables    │  │Status                   │  │
│  │• Location   │  │• Custom     │  │• Matched                │  │
│  │• Variables  │  │• Extracted  │  │• Count                  │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  Condition Chaining Example:                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │builtin.file │  │Save as      │  │builtin.xml              │  │
│  │pattern:     │──│"poms"       │──│filepaths:               │  │
│  │pom.xml      │  │ignore:true  │  │"{{poms.filepaths}}"     │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Result Aggregation Flow
```go
func (r *ruleEngine) RunRulesScoped(ctx context.Context, ruleSets []RuleSet, 
    scopes Scope, selectors ...RuleSelector) []konveyor.RuleSet {
    
    // ... setup ...
    
    ret := make(chan response)
    var matchedRules, unmatchedRules, failedRules int32
    wg := &sync.WaitGroup{}
    
    // Result aggregation goroutine
    go func() {
        for {
            select {
            case response := <-ret:
                defer wg.Done()
                
                if response.Err != nil {
                    // Handle rule execution error
                    atomic.AddInt32(&failedRules, 1)
                    mapRuleSets[response.RuleSetName].Errors[response.Rule.RuleID] = response.Err.Error()
                    
                } else if response.ConditionResponse.Matched && len(response.ConditionResponse.Incidents) > 0 {
                    // Create violation from successful match
                    violation, err := r.createViolation(ctx, response.ConditionResponse, response.Rule, scopes)
                    if len(violation.Incidents) == 0 {
                        // Incidents filtered out
                        atomic.AddInt32(&unmatchedRules, 1)
                        mapRuleSets[response.RuleSetName].Unmatched = append(mapRuleSets[response.RuleSetName].Unmatched, response.Rule.RuleID)
                    } else {
                        // Valid violation
                        atomic.AddInt32(&matchedRules, 1)
                        rs := mapRuleSets[response.RuleSetName]
                        
                        // Categorize as violation or insight based on effort
                        if response.Rule.Effort == nil || *response.Rule.Effort == 0 {
                            rs.Insights[response.Rule.RuleID] = violation
                        } else {
                            rs.Violations[response.Rule.RuleID] = violation
                        }
                    }
                    
                } else {
                    // Rule didn't match
                    atomic.AddInt32(&unmatchedRules, 1)
                    mapRuleSets[response.RuleSetName].Unmatched = append(mapRuleSets[response.RuleSetName].Unmatched, response.Rule.RuleID)
                }
            case <-ctx.Done():
                return
            }
        }
    }()
    
    // Dispatch rules to workers
    for _, ruleMsg := range otherRules {
        wg.Add(1)
        ruleMsg.returnChan = ret
        r.ruleProcessing <- ruleMsg
    }
    
    wg.Wait()
    close(ret)
    
    return convertToOutput(mapRuleSets)
}
```

## Output Generation Flow

### Phase 7: Violation Creation & Processing
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│Provider         │    │Violation         │    │Output           │
│Responses        │───▶│Creation          │───▶│Formatting       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                       │
        ▼                        ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Violation Processing Flow                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Step 1: Incident Processing                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Provider     │  │Incident     │  │Message                  │  │
│  │IncidentCtx │──│Filtering    │──│Templating               │  │
│  │             │  │• Selector   │  │• Variables              │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  Step 2: Code Snippet Extraction                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │File URI +   │  │Provider     │  │Code                     │  │
│  │Location     │──│CodeSnip     │──│Context                  │  │
│  │             │  │Query        │  │• Context lines          │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  Step 3: Violation Assembly                                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Incidents    │  │Rule         │  │Final                    │  │
│  │+ Code       │──│Metadata     │──│Violation                │  │
│  │+ Variables  │  │+ Links      │  │Object                   │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Violation Creation Process:**
```go
func (r *ruleEngine) createViolation(ctx context.Context, response ConditionResponse, 
    rule Rule, scopes Scope) (konveyor.Violation, error) {
    
    violation := konveyor.Violation{
        Description: rule.Description,
        Category:    &rule.Category,
        Labels:      rule.Labels,
        Links:       rule.Links,
        Effort:      rule.Effort,
        Incidents:   []konveyor.Incident{},
    }
    
    for _, incident := range response.Incidents {
        // Apply incident selector if configured
        if r.incidentSelector != "" {
            if !matchesIncidentSelector(incident, r.incidentSelector) {
                continue
            }
        }
        
        // Generate templated message
        message, err := renderTemplate(rule.Message, incident.Variables)
        
        // Get code snippet if available
        codeSnip := ""
        if incident.CodeLocation != nil {
            codeSnip, _ = r.getCodeSnippet(incident.FileURI, *incident.CodeLocation)
        }
        
        konveyorIncident := konveyor.Incident{
            URI:        incident.FileURI,
            Message:    message,
            CodeSnip:   codeSnip,
            LineNumber: incident.LineNumber,
            Variables:  incident.Variables,
        }
        
        violation.Incidents = append(violation.Incidents, konveyorIncident)
        
        // Apply incident limit
        if len(violation.Incidents) >= r.incidentLimit {
            break
        }
    }
    
    return violation, nil
}
```

### Phase 8: Parallel Dependency Analysis
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│Provider         │    │Dependency        │    │Dependency       │
│Initialization   │───▶│Collection        │───▶│Output           │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                       │
        ▼                        ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│             Parallel Dependency Analysis Flow                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Concurrent Execution:                                          │
│  ┌─────────────┐                    ┌─────────────────────────┐ │
│  │Main Rule    │                    │Dependency               │ │
│  │Processing   │ ◄──── goroutine ──▶│Analysis                 │ │
│  │             │                    │                         │ │
│  │• Workers    │                    │• Provider queries       │ │
│  │• Violations │                    │• Flat dependencies      │ │
│  │• Results    │                    │• Tree dependencies      │ │
│  └─────────────┘                    └─────────────────────────┘ │
│                                                                 │
│  Synchronization:                                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │WaitGroup    │  │Both         │  │Final                    │  │
│  │Coordination │──│Complete     │──│Output                   │  │
│  │             │  │             │  │Generation               │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Dependency Analysis Setup:**
```go
wg := &sync.WaitGroup{}
var depSpan trace.Span
var depCtx context.Context

if depOutputFile != "" {
    depCtx, depSpan = tracing.StartNewSpan(ctx, "dep")
    wg.Add(1)
    go DependencyOutput(depCtx, providers, log, errLog, depOutputFile, wg)
}

// Main rule processing (blocking)
rulesets := eng.RunRules(ctx, ruleSets, selectors...)
engineSpan.End()

// Wait for dependency analysis to complete
wg.Wait()
if depSpan != nil {
    depSpan.End()
}
```

### Phase 9: Final Output Assembly
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│Rule Results     │    │Output            │    │File             │
│+ Dependencies   │───▶│Marshaling        │───▶│Writing          │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                       │
        ▼                        ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Final Output Generation                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Data Sorting & Organization:                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │RuleSet      │  │Violation    │  │Incident                 │  │
│  │Sorting      │──│Sorting      │──│Sorting                  │  │
│  │• By name    │  │• By URI     │  │• By line number         │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  YAML Serialization:                                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Struct       │  │YAML         │  │Output                   │  │
│  │Marshaling   │──│Generation   │──│File                     │  │
│  │• Canonical  │  │• Formatted  │  │• violations.yaml        │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Final Output Generation:**
```go
// Sort results for deterministic output
sort.SliceStable(rulesets, func(i, j int) bool {
    return rulesets[i].Name < rulesets[j].Name
})

// Marshal to YAML
b, _ := yaml.Marshal(rulesets)

// Handle error-on-violation mode
if errorOnViolations && len(rulesets) != 0 {
    fmt.Printf("%s", string(b))
    os.Exit(EXIT_ON_ERROR_CODE)
}

// Write output file
err = os.WriteFile(outputViolations, b, 0644)
if err != nil {
    errLog.Error(err, "error writing output file", "file", outputViolations)
    os.Exit(1)
}
```

## Error Handling & Recovery

### Error Classification & Handling
```
┌─────────────────────────────────────────────────────────────────┐
│                    Error Handling Flow                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Provider Errors:                                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Connection   │  │Timeout      │  │Recovery                 │  │
│  │Failures     │──│Handling     │──│Strategy                 │  │
│  │• LSP        │  │• Backoff    │  │• Retry                  │  │
│  │• gRPC       │  │• Cancel     │  │• Fallback               │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  Rule Errors:                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Parse        │  │Evaluation   │  │Error                    │  │
│  │Failures     │──│Failures     │──│Aggregation              │  │
│  │• Syntax     │  │• Provider   │  │• Per rule               │  │
│  │• Validation │  │• Condition  │  │• In output              │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  System Errors:                                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Resource     │  │Context      │  │Graceful                 │  │
│  │Exhaustion   │──│Cancellation │──│Shutdown                 │  │
│  │• Memory     │  │• Timeouts   │  │• Cleanup                │  │
│  │• Files      │  │• User abort │  │• Reports                │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Lifecycle Management

### Cleanup & Resource Management
```
┌─────────────────────────────────────────────────────────────────┐
│                    Cleanup & Shutdown Flow                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Analysis Completion:                                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Rule Engine  │  │Provider     │  │Resource                 │  │
│  │Stop         │──│Cleanup      │──│Deallocation             │  │
│  │• Workers    │  │• LSP stop   │  │• Files                  │  │
│  │• Channels   │  │• gRPC close │  │• Memory                 │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  Context Cancellation:                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │Signal       │  │Propagation  │  │Graceful                 │  │
│  │Handling     │──│Tree         │──│Termination              │  │
│  │• SIGINT     │  │• Workers    │  │• Output flush           │  │
│  │• SIGTERM    │  │• Providers  │  │• Temp cleanup           │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Cleanup Sequence:**
```go
// Stop rule engine
eng.Stop()

// Stop all providers
for _, provider := range needProviders {
    provider.Stop()
}

// Tracing cleanup
defer tracing.Shutdown(ctx, log, tp)

// Context cancellation propagates to all goroutines
defer cancelFunc()
```

This comprehensive component flow documentation shows how the Analyzer Rule Engine orchestrates complex multi-provider analysis through well-defined phases, with proper error handling, resource management, and parallel processing to deliver efficient static code analysis.