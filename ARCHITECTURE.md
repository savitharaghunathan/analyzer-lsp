# Konveyor Analyzer Rule Engine - Comprehensive Architecture Documentation

## Table of Contents

1. [Overview](#overview)
2. [Core Architecture](#core-architecture)
3. [Component Deep Dive](#component-deep-dive)
4. [Provider System](#provider-system)
5. [Rule Engine Implementation](#rule-engine-implementation)
6. [Rule System & Parser](#rule-system--parser)
7. [Output & Violation Management](#output--violation-management)
8. [Execution Flow](#execution-flow)
9. [Configuration Management](#configuration-management)
10. [Performance & Scalability](#performance--scalability)
11. [Advanced Features](#advanced-features)
12. [Development Guide](#development-guide)
13. [Best Practices](#best-practices)

## Overview

The **Konveyor Analyzer Rule Engine** is a sophisticated, multi-language static code analysis framework designed for application modernization, migration assessments, and code quality analysis. Built in Go, it leverages the Language Server Protocol (LSP) to provide deep, accurate analysis across different programming languages and technologies.

### Key Characteristics
- **Multi-language Support**: Java, Go, Generic LSP-compliant languages
- **Rule-based Analysis**: YAML-defined, declarative rule system
- **Parallel Processing**: Concurrent rule execution with configurable worker pools
- **Extensible Architecture**: Pluggable provider system via gRPC and in-process providers
- **Rich Output**: Structured violations, dependency analysis, and metadata

## Core Architecture

### System Overview
```
┌─────────────────────────────────────────────────────────────┐
│                     Analyzer Engine                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Parser    │  │    Engine   │  │  Output Generator   │  │
│  │             │  │             │  │                     │  │
│  │ • YAML      │  │ • Workers   │  │ • Violations        │  │
│  │ • Rules     │  │ • Scheduler │  │ • Dependencies      │  │
│  │ • Validation│  │ • Context   │  │ • Reports           │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                    Provider Layer                           │
├─────────────────────────────────────────────────────────────┤
│  ┌───────────┐  ┌─────────────┐  ┌─────────────────────────┐│
│  │ Built-in  │  │    Java     │  │      External           ││
│  │           │  │             │  │                         ││
│  │ • XML     │  │ • Eclipse   │  │ • Go Provider           ││
│  │ • JSON    │  │   JDT-LS    │  │ • Generic LSP           ││
│  │ • Files   │  │ • Bundles   │  │ • gRPC Interface        ││
│  └───────────┘  └─────────────┘  └─────────────────────────┘│
├─────────────────────────────────────────────────────────────┤
│                Communication Layer                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   JSONRPC2  │  │    gRPC     │  │      Tracing        │  │
│  │             │  │             │  │                     │  │
│  │ • LSP       │  │ • External  │  │ • OpenTelemetry     │  │
│  │ • Protocol  │  │   Providers │  │ • Jaeger            │  │
│  │ • Transport │  │ • Streaming │  │ • Distributed       │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Directory Structure
```
analyzer-lsp/
├── cmd/analyzer/           # Main CLI application entry point
├── engine/                 # Core rule execution engine
│   ├── engine.go          # Main engine implementation  
│   ├── conditions.go      # Condition evaluation logic
│   ├── scopes.go          # Analysis scope management
│   └── labels/            # Label selector system
├── provider/              # Provider system implementation
│   ├── provider.go        # Provider interfaces
│   ├── lib.go            # Provider utilities and file search
│   └── server.go         # Provider server implementation
├── parser/                # Rule parsing and validation
│   ├── rule_parser.go    # YAML rule parser
│   └── open_api.go       # Schema generation
├── jsonrpc2/             # LSP communication protocol
├── output/v1/konveyor/   # Output structure definitions
├── external-providers/   # External provider examples
├── examples/             # Test applications
└── docs/                 # Documentation
```

## Component Deep Dive

### 1. Rule Engine Core (`engine/engine.go`)

#### Engine Interface
```go
type RuleEngine interface {
    RunRules(ctx context.Context, rules []RuleSet, selectors ...RuleSelector) []konveyor.RuleSet
    RunRulesScoped(ctx context.Context, ruleSets []RuleSet, scopes Scope, selectors ...RuleSelector) []konveyor.RuleSet
    Stop()
}
```

#### Key Components

**Worker Pool Management**:
```go
type ruleEngine struct {
    ruleProcessing chan ruleMessage  // Buffered channel (size: 10)
    cancelFunc     context.CancelFunc
    logger         logr.Logger
    wg            *sync.WaitGroup
    
    // Configuration options
    incidentLimit    int     // Max incidents per rule
    codeSnipLimit    int     // Max code snippets per file
    contextLines     int     // Context lines around violations
    incidentSelector string  // Incident filtering expression
    locationPrefixes []string // Path prefixes for analysis
}
```

**Rule Processing Workflow**:
1. **Rule Distribution**: Rules are distributed to available workers via buffered channels
2. **Context Management**: Each rule gets its own condition context with templates and tags
3. **Parallel Execution**: Workers process rules concurrently with proper span propagation
4. **Result Aggregation**: Responses are collected and violations are generated

**Worker Implementation**:
```go
func processRuleWorker(ctx context.Context, ruleMessages chan ruleMessage, logger logr.Logger, wg *sync.WaitGroup) {
    prop := otel.GetTextMapPropagator()
    for {
        select {
        case m := <-ruleMessages:
            // Extract tracing context
            ctx = prop.Extract(ctx, m.carrier)
            
            // Create isolated condition context
            m.conditionContext.Template = make(map[string]ChainTemplate)
            if m.scope != nil {
                m.scope.AddToContext(&m.conditionContext)
            }
            
            // Process rule
            response, err := processRule(ctx, m.rule, m.conditionContext, logger)
            
            // Return result
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

#### Engine Creation and Configuration
```go
func CreateRuleEngine(ctx context.Context, workers int, log logr.Logger, options ...Option) RuleEngine {
    ruleProcessor := make(chan ruleMessage, 10)
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

### 2. Condition System (`engine/conditions.go`)

#### Core Interfaces
```go
type Conditional interface {
    Evaluate(ctx context.Context, log logr.Logger, condCtx ConditionContext) (ConditionResponse, error)
}

type ConditionContext struct {
    Tags     map[string]interface{}   // Generated tags from tagging rules
    Template map[string]ChainTemplate // Template variables for condition chaining
    RuleID   string                   // Current rule identifier
}

type ConditionResponse struct {
    Matched         bool                    // Whether condition matched
    Incidents       []IncidentContext       // Specific violation instances
    TemplateContext map[string]interface{}  // Variables for message templating
}
```

#### Incident Context Structure
```go
type IncidentContext struct {
    FileURI      uri.URI                 // File where violation occurred
    Effort       *int                    // Effort points for fixing
    LineNumber   *int                    // Line number of violation
    Variables    map[string]interface{}  // Custom variables for templating
    Links        []konveyor.Link         // External documentation links
    CodeLocation *Location               // Precise location in code
}

type Location struct {
    StartPosition Position
    EndPosition   Position
}

type Position struct {
    Line      int  // Zero-based line number
    Character int  // Zero-based character offset
}
```

#### Logical Conditions
```go
type AndCondition struct {
    And []ConditionEntry
}

type OrCondition struct {
    Or []ConditionEntry
}

type ConditionEntry struct {
    From                   string      // Variable source for chaining
    As                     string      // Variable name assignment
    Ignorable              bool        // Don't count for rule matching
    Not                    bool        // Negate condition result
    ProviderSpecificConfig Conditional // Actual provider condition
}
```

### 3. Rule Parsing System (`parser/rule_parser.go`)

#### Parser Structure
```go
type RuleParser struct {
    ProviderNameToClient map[string]provider.InternalProviderClient
    Log                  logr.Logger
    NoDependencyRules    bool
    DepLabelSelector     *labels.LabelSelector[*provider.Dep]
}
```

#### Rule Loading Process
```go
func (r *RuleParser) LoadRules(filepath string) ([]engine.RuleSet, map[string]provider.InternalProviderClient, error) {
    info, err := os.Stat(filepath)
    if err != nil {
        return nil, nil, err
    }
    
    if info.Mode().IsRegular() {
        // Single file - load as rule file
        rules, providers, err := r.LoadRule(filepath)
        ruleSet := r.loadRuleSet(path.Dir(filepath))
        if ruleSet == nil {
            ruleSet = defaultRuleSet  // Use default if no ruleset.yaml
        }
        ruleSet.Rules = rules
        return []engine.RuleSet{*ruleSet}, providers, err
    } else {
        // Directory - load as ruleset
        return r.loadRuleSetFromDir(filepath)
    }
}
```

#### Rule Validation
The parser performs extensive validation:
- **Syntax validation**: YAML structure compliance
- **Semantic validation**: Provider capability verification
- **Dependency validation**: Provider availability checks
- **Schema validation**: Rule format compliance

## Provider System

### Provider Architecture

#### Core Provider Interface
```go
type InternalProviderClient interface {
    Capabilities() []Capability
    ProviderInit(ctx context.Context, configs []InitConfig) ([]InitConfig, error)
    Evaluate(ctx context.Context, cap string, conditionInfo []byte) (ProviderEvaluateResponse, error)
    GetDependencies(ctx context.Context) (map[uri.URI][]*Dep, error)
    GetDependenciesDAG(ctx context.Context) (map[uri.URI][]DepDAGItem, error)
    Stop()
}

type Startable interface {
    Start(ctx context.Context) error
}
```

#### Provider Capabilities
```go
type Capability struct {
    Name   string
    Input  CapabilityInput
    Output CapabilityOutput
}

type CapabilityInput struct {
    Schema *openapi3.Schema
}

type CapabilityOutput struct {
    Schema *openapi3.Schema
}
```

### Built-in Provider Capabilities

#### File System Operations
- **file**: Pattern-based file matching
- **filecontent**: Regex content searching within files
- **xml**: XPath-based XML querying
- **json**: JSONPath-based JSON querying
- **hasTags**: Tag existence checking

#### Advanced File Search (`provider/lib.go`)
```go
type FileSearcher struct {
    BasePath                  string
    AdditionalPaths          []string
    ProviderConfigConstraints IncludeExcludeConstraints
    RuleScopeConstraints     IncludeExcludeConstraints
    FailFast                 bool
    Log                      logr.Logger
}

type SearchCriteria struct {
    Patterns           []string  // File patterns to match
    ConditionFilepaths []string  // Specific file paths from conditions
}
```

**Search Priority Order**:
1. **Search-time constraints** (highest priority)
2. **Rule scope constraints**
3. **Provider config constraints**

### Language-Specific Providers

#### Java Provider
- **Eclipse JDT-LS Integration**: Uses Eclipse's Java Development Tools Language Server
- **Custom Analysis Bundles**: Extended capabilities via OSGi bundles
- **Capabilities**:
  - `referenced`: Find references with location-specific queries
  - `dependency`: Maven/Gradle dependency analysis
  - `inheritance`: Class hierarchy analysis
  - `annotation`: Annotation-based pattern matching

**Java Location Types**:
- `IMPORT`: Import statement analysis
- `PACKAGE`: Package usage detection
- `TYPE`: Type reference matching
- `METHOD`: Method declaration/call analysis
- `CONSTRUCTOR_CALL`: Constructor invocation
- `ANNOTATION`: Annotation presence/analysis
- `INHERITANCE`: Class inheritance patterns
- `IMPLEMENTS_TYPE`: Interface implementation
- `VARIABLE_DECLARATION`: Variable type analysis

#### Go Provider
- **gopls Integration**: Uses official Go language server
- **Capabilities**:
  - `referenced`: Symbol reference analysis
  - `dependency`: go.mod dependency tracking

#### Generic Provider
- **LSP 3.17 Compliance**: Works with any LSP-compliant language server
- **Configuration**:
```json
{
    "name": "go",
    "binaryPath": "/path/to/generic/provider/binary",
    "initConfig": [{
        "location": "/path/to/source",
        "providerSpecificConfig": {
            "name": "go",
            "lspServerPath": "/path/to/gopls",
            "lspArgs": ["arg1", "arg2"],
            "dependencyProviderPath": "/path/to/dep/provider"
        }
    }]
}
```

## Rule System & Parser

### Rule Structure Deep Dive

#### Complete Rule Format
```yaml
# Rule Metadata
ruleID: "unique-rule-identifier"
description: "Human-readable description of the rule"
labels:
  - "technology=java"
  - "category=migration"  
  - "effort=high"
category: mandatory|optional|potential
effort: 1-10

# Custom Variables for Templating
customVariables:
  - pattern: '([A-z]+)\.get\(\)'
    name: VariableName

# Rule Condition
when:
  <condition_expression>

# Actions
message: "Templated message: Found {{ VariableName }}"
tag:
  - "deprecated-api"
  - "Category=spring-migration"

# Documentation
links:
  - url: "https://migration.guide"
    title: "Migration Documentation"
```

#### Advanced Rule Conditions

**Provider Conditions**:
```yaml
# Java reference analysis
java.referenced:
  pattern: "org.springframework.*"
  location: IMPORT
  annotated:
    pattern: "org.framework.Bean"
    elements:
      - name: "url"
        value: "http://.*"

# Dependency analysis
java.dependency:
  name: "junit.junit"
  upperbound: "4.12.2"
  lowerbound: "4.4.0"

# XML analysis
builtin.xml:
  xpath: "//dependencies/dependency"
  namespaces:
    m: "http://maven.apache.org/POM/4.0.0"
  filepaths: ["pom.xml"]
```

**Condition Chaining**:
```yaml
when:
  or:
    # First condition: find pom files and save as 'poms'
    - builtin.file:
        pattern: "pom.xml"
      as: poms
      ignore: true  # Don't count for rule matching
    
    # Second condition: use pom file paths from first condition  
    - builtin.xml:
        xpath: "//dependencies/dependency"
        filepaths: "{{poms.filepaths}}"
      from: poms
```

**Complex Logical Conditions**:
```yaml
when:
  and:
    - or:
        - java.referenced:
            pattern: "javax.servlet.*"
            location: IMPORT
        - java.referenced:
            pattern: "jakarta.servlet.*"
            location: IMPORT
    - not:
        java.dependency:
          name: "org.springframework.boot"
```

### Rule Categories and Effort

#### Category Definitions
- **mandatory**: Must be fixed for successful migration/upgrade
- **optional**: Should be fixed but won't break functionality
- **potential**: Requires investigation to determine necessity

#### Effort Scale
- **1-3**: Low effort (simple replacements, configuration changes)
- **4-6**: Medium effort (refactoring, API changes)
- **7-10**: High effort (architectural changes, major rewrites)

### Custom Variables and Templating

#### Variable Definition
```yaml
customVariables:
  - pattern: '([A-z]+)\.get\(\)'           # Regex with capture groups
    name: VariableName                      # Variable name for templating
  - pattern: 'import\s+([a-zA-Z0-9\.]+)'
    name: ImportedPackage
```

#### Message Templating
```yaml
message: |
  Found deprecated method call: {{ VariableName }}
  Consider replacing with modern alternative
  Imported package: {{ ImportedPackage }}
```

## Output & Violation Management

### Output Structure (`output/v1/konveyor/violations.go`)

#### RuleSet Output
```go
type RuleSet struct {
    Name        string                  `yaml:"name"`
    Description string                  `yaml:"description"`
    Tags        []string                `yaml:"tags"`
    Violations  map[string]Violation    `yaml:"violations"`  // Rule ID -> Violation
    Insights    map[string]Violation    `yaml:"insights"`    // Informational rules
    Errors      map[string]string       `yaml:"errors"`      // Rule ID -> Error
    Unmatched   []string                `yaml:"unmatched"`   // Unmatched rule IDs
    Skipped     []string                `yaml:"skipped"`     // Skipped rule IDs
}
```

#### Violation Structure
```go
type Violation struct {
    Description string      `yaml:"description"`
    Category    *Category   `yaml:"category"`
    Labels      []string    `yaml:"labels"`
    Incidents   []Incident  `yaml:"incidents"`
    Links       []Link      `yaml:"links"`
    Effort      *int        `yaml:"effort"`
    Extras      json.RawMessage `yaml:"extras"`
}
```

#### Incident Details
```go
type Incident struct {
    URI        uri.URI                 `yaml:"uri"`         // File location
    Message    string                  `yaml:"message"`     // Specific incident message
    CodeSnip   string                  `yaml:"codeSnip"`    // Code context
    LineNumber *int                    `yaml:"lineNumber"`  // Line in file
    Variables  map[string]interface{}  `yaml:"variables"`   // Template variables
}
```

### Dependency Analysis Output

#### Flat Dependencies
```go
type DepsFlatItem struct {
    FileURI      string `yaml:"fileURI"`      // File containing dependencies
    Provider     string `yaml:"provider"`     // Provider that found dependencies
    Dependencies []*Dep `yaml:"dependencies"` // List of dependencies
}
```

#### Hierarchical Dependencies
```go
type DepsTreeItem struct {
    FileURI      string       `yaml:"fileURI"`
    Provider     string       `yaml:"provider"`
    Dependencies []DepDAGItem `yaml:"dependencies"`
}

type DepDAGItem struct {
    Dep       Dep          `yaml:"dep"`
    AddedDeps []DepDAGItem `yaml:"addedDep"`  // Transitive dependencies
}
```

#### Dependency Structure
```go
type Dep struct {
    Name               string                 `yaml:"name"`
    Version            string                 `yaml:"version"`
    Classifier         string                 `yaml:"classifier"`
    Type               string                 `yaml:"type"`
    Indirect           bool                   `yaml:"indirect"`
    ResolvedIdentifier string                 `yaml:"resolvedIdentifier"`
    Labels             []string               `yaml:"labels"`
    FileURIPrefix      string                 `yaml:"prefix"`
    Extras             map[string]interface{} `yaml:"extras"`
}
```

## Execution Flow

### Main Application Flow (`cmd/analyzer/main.go`)

#### 1. Initialization Phase
```go
func main() {
    // Configure logging
    logrusLog := logrus.New()
    log := logrusr.New(logrusLog)
    
    // Initialize tracing
    tracerOptions := tracing.Options{
        EnableJaeger:   enableJaeger,
        JaegerEndpoint: jaegerEndpoint,
    }
    tp, err := tracing.InitTracerProvider(log, tracerOptions)
    defer tracing.Shutdown(ctx, log, tp)
    
    // Create main span
    ctx, mainSpan := tracing.StartNewSpan(ctx, "main")
    defer mainSpan.End()
}
```

#### 2. Provider Configuration
```go
// Load provider configurations
configs, err := provider.GetConfig(settingsFile)

// Add builtin providers for all locations
finalConfigs := []provider.Config{}
defaultBuiltinConfigs := []provider.InitConfig{}

for _, config := range configs {
    // Process each provider configuration
    for _, initConf := range config.InitConfig {
        // Auto-add builtin provider for each location
        if initConf.Location != "" {
            builtinConf := provider.InitConfig{Location: initConf.Location}
            defaultBuiltinConfigs = append(defaultBuiltinConfigs, builtinConf)
        }
    }
    finalConfigs = append(finalConfigs, config)
}

// Add default builtin configuration
finalConfigs = append(finalConfigs, provider.Config{
    Name:       "builtin",
    InitConfig: defaultBuiltinConfigs,
})
```

#### 3. Provider Initialization
```go
providers := map[string]provider.InternalProviderClient{}
for _, config := range finalConfigs {
    // Override analysis mode if specified via CLI
    if analysisMode != "" {
        for i, initConfig := range config.InitConfig {
            config.InitConfig[i].AnalysisMode = provider.AnalysisMode(analysisMode)
        }
    }
    
    // Create provider client
    prov, err := lib.GetProviderClient(config, log)
    providers[config.Name] = prov
    
    // Start provider if it implements Startable
    if s, ok := prov.(provider.Startable); ok {
        err := s.Start(ctx)
    }
}
```

#### 4. Engine Creation
```go
eng := engine.CreateRuleEngine(ctx,
    10,  // Number of workers
    log,
    engine.WithIncidentLimit(limitIncidents),
    engine.WithCodeSnipLimit(limitCodeSnips),
    engine.WithContextLines(contextLines),
    engine.WithIncidentSelector(incidentSelector),
    engine.WithLocationPrefixes(providerLocations),
)
```

#### 5. Rule Processing
```go
// Create rule parser
parser := parser.RuleParser{
    ProviderNameToClient: providers,
    Log:                  log.WithName("parser"),
    NoDependencyRules:    noDependencyRules,
    DepLabelSelector:     dependencyLabelSelector,
}

// Load and parse rules
ruleSets := []engine.RuleSet{}
needProviders := map[string]provider.InternalProviderClient{}

for _, ruleFile := range rulesFile {
    ruleSet, providers, err := parser.LoadRules(ruleFile)
    ruleSets = append(ruleSets, ruleSet...)
    
    // Collect required providers
    for k, v := range providers {
        needProviders[k] = v
    }
}
```

#### 6. Provider Initialization
```go
// Initialize required providers
for name, provider := range needProviders {
    if name == "builtin" {
        continue // Handle builtin separately
    }
    
    initCtx, initSpan := tracing.StartNewSpan(ctx, "init",
        attribute.Key("provider").String(name))
    
    additionalBuiltinConfs, err := provider.ProviderInit(initCtx, nil)
    initSpan.End()
}

// Initialize builtin provider with additional configs
if builtinClient, ok := needProviders["builtin"]; ok {
    builtinClient.ProviderInit(ctx, additionalBuiltinConfigs)
}
```

#### 7. Rule Execution
```go
// Run analysis with selectors
rulesets := eng.RunRules(ctx, ruleSets, selectors...)

// Stop engine and providers
eng.Stop()
for _, provider := range needProviders {
    provider.Stop()
}
```

#### 8. Output Generation
```go
// Sort results
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
```

### Rule Execution Flow

#### 1. Rule Filtering and Categorization
```go
func (r *ruleEngine) filterRules(ruleSets []RuleSet, selectors ...RuleSelector) (
    taggingRules []ruleMessage,
    otherRules []ruleMessage, 
    mapRuleSets map[string]*konveyor.RuleSet) {
    
    for _, ruleSet := range ruleSets {
        rs := r.createRuleSet(ruleSet)
        mapRuleSets[ruleSet.Name] = rs
        
        for _, rule := range ruleSet.Rules {
            // Apply rule selectors
            for _, selector := range selectors {
                if !selector.Matches(rule) {
                    continue
                }
            }
            
            // Categorize rules
            if len(rule.Tag) > 0 {
                taggingRules = append(taggingRules, ruleMessage{...})
            } else {
                otherRules = append(otherRules, ruleMessage{...})
            }
        }
    }
    return
}
```

#### 2. Tagging Rules Execution (Synchronous)
```go
func (r *ruleEngine) runTaggingRules(ctx context.Context, taggingRules []ruleMessage, 
    mapRuleSets map[string]*konveyor.RuleSet, conditionContext ConditionContext, 
    scopes Scope) ConditionContext {
    
    for _, ruleMsg := range taggingRules {
        // Execute tagging rule synchronously
        response, err := processRule(ctx, ruleMsg.rule, conditionContext, r.logger)
        
        if err == nil && response.Matched {
            // Add generated tags to context
            for _, tag := range ruleMsg.rule.Tag {
                conditionContext.Tags[tag] = struct{}{}
            }
        }
    }
    return conditionContext
}
```

#### 3. Main Rules Execution (Asynchronous)
```go
func (r *ruleEngine) RunRulesScoped(ctx context.Context, ruleSets []RuleSet, 
    scopes Scope, selectors ...RuleSelector) []konveyor.RuleSet {
    
    _, otherRules, mapRuleSets := r.filterRules(ruleSets, selectors...)
    
    ret := make(chan response)
    wg := &sync.WaitGroup{}
    
    // Start result handler goroutine
    go func() {
        for response := range ret {
            defer wg.Done()
            
            if response.Err != nil {
                // Handle rule execution error
                mapRuleSets[response.RuleSetName].Errors[response.Rule.RuleID] = response.Err.Error()
            } else if response.ConditionResponse.Matched {
                // Create violation from successful match
                violation, err := r.createViolation(ctx, response.ConditionResponse, response.Rule, scopes)
                mapRuleSets[response.RuleSetName].Violations[response.Rule.RuleID] = violation
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

### Violation Creation Process

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

## Configuration Management

### Provider Configuration Schema

#### Base Configuration
```json
{
    "name": "provider-name",
    "binaryPath": "/path/to/provider/binary",
    "address": "localhost:port",
    "proxyConfig": {
        "httpproxy": "http://user:pass@proxy:port",
        "httpsproxy": "https://user:pass@proxy:port", 
        "noproxy": "localhost,127.0.0.1"
    },
    "initConfig": [...]
}
```

#### Init Configuration
```json
{
    "location": "/path/to/source/or/binary",
    "dependencyPath": "/path/to/dependencies",
    "lspServerPath": "/path/to/language/server",
    "analysisMode": "full|source-only",
    "providerSpecificConfig": {...}
}
```

### Java Provider Configuration
```json
{
    "name": "java",
    "binaryPath": "/path/to/jdtls",
    "initConfig": [{
        "location": "/path/to/java/project",
        "analysisMode": "full",
        "providerSpecificConfig": {
            "bundles": "/path/to/analyzer-bundle.jar",
            "workspace": "/tmp/java-workspace",
            "depOpenSourceLabelsFile": "/etc/maven.default.index",
            "mavenSettingsFile": "/path/to/settings.xml",
            "excludePackages": ["test.packages.*"],
            "jvmMaxMem": "4096m"
        }
    }]
}
```

### Go Provider Configuration
```json
{
    "name": "go",
    "binaryPath": "/path/to/generic-provider",
    "initConfig": [{
        "location": "/path/to/go/project",
        "analysisMode": "full",
        "providerSpecificConfig": {
            "name": "go",
            "lspServerPath": "/usr/local/bin/gopls",
            "lspArgs": ["-logfile", "/tmp/gopls.log"],
            "dependencyProviderPath": "/path/to/go-dep-provider"
        }
    }]
}
```

## Performance & Scalability

### Concurrency Model

#### Worker Pool Architecture
- **Configurable Workers**: Default 10 workers, adjustable via engine creation
- **Buffered Channels**: Rule queue with buffer size 10 to prevent blocking
- **Graceful Shutdown**: Context-based cancellation with proper cleanup

#### Memory Management
- **Incident Limits**: Configurable limits to prevent memory exhaustion
- **Code Snippet Limits**: Control over code context size
- **Streaming Results**: Results are processed as they arrive

#### Performance Tuning Parameters
```go
engine.CreateRuleEngine(ctx, workers,
    engine.WithIncidentLimit(1500),     // Max incidents per rule
    engine.WithCodeSnipLimit(20),       // Max code snippets per file
    engine.WithContextLines(10),        // Lines of context around violations
)
```

### Optimization Strategies

#### 1. Rule Execution Optimization
- **Tagging Rules First**: Synchronous execution to build context
- **Dependency Optimization**: Skip dependency rules when not needed
- **Early Termination**: Stop processing when incident limits are reached

#### 2. Provider Optimization
- **Connection Pooling**: Reuse LSP connections
- **Batch Operations**: Group related queries
- **Caching**: Cache file system operations and provider responses

#### 3. Memory Optimization
- **Streaming Processing**: Process results as they arrive
- **Context Isolation**: Separate contexts per rule to prevent leaks
- **Resource Cleanup**: Proper cleanup of providers and connections

## Advanced Features

### Label Selector System

#### Label Syntax
```bash
# Select rules with specific labels
--label-selector "technology=java,category=migration"

# Complex expressions
--label-selector "technology in (java,go),!category=test"

# Dependency label selection
--dep-label-selector "konveyor.io/dep-source=open-source"
```

#### Label Implementation
```go
type LabelSelector[T any] struct {
    expression string
    compiled   *gval.Evaluable
}

func (s *LabelSelector[T]) Matches(item T) bool {
    labels := extractLabels(item)
    result, _ := s.compiled.EvalBool(context.Background(), labels)
    return result
}
```

### Incident Selector System

#### Custom Variable Filtering
```bash
# Filter incidents based on custom variables
--incident-selector "!package=io.konveyor.demo.config-utils"
--incident-selector "severity=high,confidence>0.8"
```

### Distributed Tracing

#### OpenTelemetry Integration
```go
func initTracing() {
    tp, err := tracing.InitTracerProvider(log, tracing.Options{
        EnableJaeger:   true,
        JaegerEndpoint: "http://localhost:14268/api/traces",
    })
    
    // Create spans for major operations
    ctx, span := tracing.StartNewSpan(ctx, "rule-execution")
    defer span.End()
}
```

#### Trace Propagation
- **Context Propagation**: Distributed trace context across rule executions
- **Span Hierarchy**: Nested spans for detailed performance analysis
- **Custom Attributes**: Rule ID, provider name, file paths in traces

### OpenAPI Schema Generation

#### Dynamic Schema Creation
```go
func createOpenAPISchema(providers map[string]provider.InternalProviderClient) openapi3.Spec {
    spec := parser.CreateSchema()
    
    // Add provider capabilities to schema
    for provName, prov := range providers {
        for _, capability := range prov.Capabilities() {
            spec.MapOfSchemaOrRefValues[fmt.Sprintf("%s.%s", provName, capability.Name)] = 
                openapi3.SchemaOrRef{Schema: capability.Input.Schema}
        }
    }
    
    return spec
}
```

## Development Guide

### Adding New Providers

#### 1. Implement Provider Interface
```go
type MyProvider struct {
    client LSPClient
    config Config
}

func (p *MyProvider) Capabilities() []Capability {
    return []Capability{
        {
            Name: "referenced",
            Input: CapabilityInput{Schema: &openapi3.Schema{...}},
            Output: CapabilityOutput{Schema: &openapi3.Schema{...}},
        },
    }
}

func (p *MyProvider) Evaluate(ctx context.Context, cap string, conditionInfo []byte) (ProviderEvaluateResponse, error) {
    switch cap {
    case "referenced":
        return p.evaluateReferenced(ctx, conditionInfo)
    default:
        return ProviderEvaluateResponse{}, fmt.Errorf("unsupported capability: %s", cap)
    }
}
```

#### 2. Register Provider
```go
func GetProviderClient(config Config, log logr.Logger) (InternalProviderClient, error) {
    switch config.Name {
    case "my-provider":
        return NewMyProvider(config, log)
    default:
        return nil, fmt.Errorf("unknown provider: %s", config.Name)
    }
}
```

### Adding New Rule Types

#### 1. Define Condition Structure
```go
type MyCondition struct {
    Pattern    string   `json:"pattern"`
    Options    []string `json:"options,omitempty"`
    Threshold  float64  `json:"threshold,omitempty"`
}

func (c MyCondition) Evaluate(ctx context.Context, log logr.Logger, condCtx ConditionContext) (ConditionResponse, error) {
    // Implementation
}
```

#### 2. Register in Parser
```go
func (p *RuleParser) parseCondition(conditionMap map[string]interface{}) (Conditional, error) {
    for key, value := range conditionMap {
        switch key {
        case "my-provider.my-capability":
            return parseMyCondition(value)
        }
    }
}
```

### Testing Guidelines

#### Unit Testing
```go
func TestRuleEngine(t *testing.T) {
    // Create test providers
    providers := map[string]provider.InternalProviderClient{
        "test": &MockProvider{},
    }
    
    // Create test rules
    rules := []engine.RuleSet{{
        Name: "test-rules",
        Rules: []engine.Rule{{
            RuleID: "test-001",
            When: engine.OrCondition{...},
        }},
    }}
    
    // Execute and verify
    engine := engine.CreateRuleEngine(ctx, 1, log)
    results := engine.RunRules(ctx, rules)
    
    assert.Equal(t, 1, len(results))
}
```

#### Integration Testing
```go
func TestEndToEnd(t *testing.T) {
    // Setup test project
    testDir := setupTestProject()
    defer os.RemoveAll(testDir)
    
    // Create provider config
    config := provider.Config{
        Name: "java",
        InitConfig: []provider.InitConfig{{
            Location: testDir,
        }},
    }
    
    // Run analysis
    main.AnalysisCmd().Execute()
    
    // Verify output
    verifyOutputFile("output.yaml")
}
```

## Best Practices

### Rule Development

#### 1. Rule Design Principles
- **Specific Patterns**: Use precise patterns to avoid false positives
- **Meaningful Messages**: Provide actionable violation messages
- **Appropriate Effort**: Assign realistic effort estimates
- **Good Documentation**: Include links to migration guides

#### 2. Performance Considerations
- **Efficient Patterns**: Use optimized regex patterns
- **Scope Limiting**: Use file patterns to limit search scope
- **Incident Limits**: Set reasonable incident limits for broad rules

#### 3. Rule Organization
```yaml
# Group related rules in rulesets
# ruleset.yaml
name: "spring-boot-migration"
description: "Rules for Spring Boot 2.x to 3.x migration"
labels:
  - "technology=spring"
  - "migration=3.x"

# rules/deprecated-apis.yaml
- ruleID: "spring-boot-001"
  description: "Replace deprecated WebSecurityConfigurerAdapter"
  category: mandatory
  effort: 5
  when:
    java.referenced:
      pattern: "org.springframework.security.config.annotation.web.configuration.WebSecurityConfigurerAdapter"
      location: INHERITANCE
```

### Provider Development

#### 1. LSP Integration Best Practices
- **Error Handling**: Robust error handling for LSP communication
- **Resource Management**: Proper cleanup of LSP connections
- **Timeout Handling**: Implement timeouts for long-running operations

#### 2. Configuration Design
- **Sensible Defaults**: Provide good default configurations
- **Validation**: Validate configuration at startup
- **Documentation**: Document all configuration options

#### 3. Performance Optimization
- **Connection Reuse**: Reuse LSP connections when possible
- **Batch Queries**: Group related queries for efficiency
- **Caching**: Cache expensive operations

### Deployment Considerations

#### 1. Container Deployment
```dockerfile
FROM registry.access.redhat.com/ubi8/ubi:latest

# Install dependencies
RUN yum install -y java-11-openjdk-devel go

# Copy analyzer
COPY analyzer-lsp /usr/local/bin/
COPY provider_settings.json /etc/analyzer/

# Run analysis
ENTRYPOINT ["/usr/local/bin/analyzer-lsp"]
```

#### 2. Resource Requirements
- **Memory**: Minimum 2GB, recommended 4GB+ for large codebases
- **CPU**: Benefits from multiple cores (worker parallelism)
- **Disk**: Temporary space for LSP workspaces and analysis cache

#### 3. Security Considerations
- **Input Validation**: Validate all file paths and patterns
- **Sandboxing**: Run analysis in isolated environments
- **Access Control**: Limit file system access appropriately

This comprehensive architecture documentation provides the deep technical details needed to understand, extend, and maintain the Konveyor Analyzer Rule Engine effectively.