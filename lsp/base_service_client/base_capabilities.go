package base

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/konveyor/analyzer-lsp/lsp/protocol"
	"github.com/konveyor/analyzer-lsp/provider"
	"go.lsp.dev/uri"
	"gopkg.in/yaml.v2"
)

// Type aliases to make the function definitions shorter

type ctx = context.Context
type resp = provider.ProviderEvaluateResponse
type base = HasLSPServiceClientBase

// Technically not necessary
type NoOpCondition struct{}

// A simple no-op function. Returns a blank response
func EvaluateNoOp[T base](t T, ctx ctx, cap string, info []byte) (resp, error) {
	return resp{}, nil
}

// Generic referenced condition
type ReferencedCondition struct {
	Referenced struct {
		Pattern string `yaml:"pattern"`
	} `yaml:"referenced"`
}

// EvaluateReferenced evaluates references to a given entity based on a query
// pattern. The function uses the provided query pattern to find references to
// the specified entity within the workspace, filters out references in certain
// directories, and returns a list of incident contexts associated with these
// references.
func EvaluateReferenced[T base](t T, ctx ctx, cap string, info []byte) (resp, error) {
	sc := t.GetLSPServiceClientBase()
	sc.Log.Info("EvaluateReferenced: starting evaluation", "capability", cap)

	var cond ReferencedCondition
	err := yaml.Unmarshal(info, &cond)
	if err != nil {
		sc.Log.Error(err, "EvaluateReferenced: failed to unmarshal condition")
		return resp{}, fmt.Errorf("error unmarshaling query info")
	}

	query := cond.Referenced.Pattern
	if query == "" {
		sc.Log.Error(fmt.Errorf("empty query pattern"), "EvaluateReferenced: no query pattern provided")
		return resp{}, fmt.Errorf("unable to get query info")
	}

	sc.Log.Info("EvaluateReferenced: calling GetAllDeclarations", "query", query, "workspaceFolders", sc.BaseConfig.WorkspaceFolders)
	symbols := sc.GetAllDeclarations(ctx, sc.BaseConfig.WorkspaceFolders, query)
	sc.Log.Info("EvaluateReferenced: GetAllDeclarations returned", "symbolCount", len(symbols))

	incidents := []provider.IncidentContext{}
	incidentsMap := make(map[string]provider.IncidentContext) // Remove duplicates

	sc.Log.Info("EvaluateReferenced: starting to process symbols for references", "symbolCount", len(symbols))
	for i, s := range symbols {
		sc.Log.Info("EvaluateReferenced: processing symbol", "symbolIndex", i, "symbolName", s.Name)
		
		// Handle the union type properly
		var location protocol.Location
		switch v := s.Location.Value.(type) {
		case protocol.Location:
			location = v
		case *protocol.Location:
			location = *v
		default:
			sc.Log.Error(fmt.Errorf("unexpected location type: %T", v), "EvaluateReferenced: skipping symbol due to location type", "symbolName", s.Name)
			continue
		}
		
		sc.Log.Info("EvaluateReferenced: getting references for symbol", "symbolIndex", i, "symbolName", s.Name, "symbolURI", location.URI)
		references := sc.GetAllReferences(ctx, location)
		sc.Log.Info("EvaluateReferenced: got references for symbol", "symbolIndex", i, "symbolName", s.Name, "referenceCount", len(references))

		breakEarly := false
		for _, ref := range references {
			// Look for things that are in the location loaded,
			// Note may need to filter out vendor at some point
			if !strings.Contains(ref.URI, sc.BaseConfig.WorkspaceFolders[0]) {
				continue
			}

			for _, substr := range sc.BaseConfig.DependencyFolders {
				if substr == "" {
					continue
				}

				if strings.Contains(ref.URI, substr) {
					breakEarly = true
					break
				}
			}

			if breakEarly {
				break
			}

			u, err := uri.Parse(ref.URI)
			if err != nil {
				return resp{}, err
			}
			// ranges are 0 indexed
			lineNumber := int(ref.Range.Start.Line) + 1
			incident := provider.IncidentContext{
				FileURI:    u,
				LineNumber: &lineNumber,
				Variables: map[string]interface{}{
					"file": ref.URI,
				},
				CodeLocation: &provider.Location{
					StartPosition: provider.Position{
						Line:      float64(ref.Range.Start.Line) + 1,
						Character: float64(ref.Range.Start.Character) + 1,
					},
					EndPosition: provider.Position{
						Line:      float64(ref.Range.End.Line) + 1,
						Character: float64(ref.Range.End.Character) + 1,
					},
				},
			}
			b, _ := json.Marshal(incident)

			incidentsMap[string(b)] = incident
		}
	}

	for _, incident := range incidentsMap {
		incidents = append(incidents, incident)
	}

	// No results were found.
	if len(incidents) == 0 {
		return resp{Matched: false}, nil
	}
	return resp{
		Matched:   true,
		Incidents: incidents,
	}, nil
}
