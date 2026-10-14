package provider

import (
	"context"
	"sort"
)

// RegisteredRoute has a stable platform identity; Model is the upstream wire
// name. Keys and clients remain inside each immutable Provider snapshot.
type RegisteredRoute struct {
	Option   ModelOption
	Provider Provider
	Model    string
}

type Registry struct {
	primary              Provider
	primaryCatalog       *Catalog
	routes               map[string]RegisteredRoute
	defaultModel, vision string
}

func NewRegistry(primary Provider, allowed []string, routes []RegisteredRoute, defaultModel, vision string) *Registry {
	r := &Registry{primary: primary, primaryCatalog: NewCatalogWithAllowed(primary, allowed), routes: map[string]RegisteredRoute{}, defaultModel: defaultModel, vision: vision}
	for _, route := range routes {
		r.routes[route.Option.ID] = route
	}
	return r
}

func PrimaryProvider(p Provider) Provider {
	if r, ok := p.(*Registry); ok {
		return r.primary
	}
	return p
}

func (r *Registry) Name() string { return "model_registry" }
func (r *Registry) Model() string {
	if r.defaultModel != "" {
		return r.defaultModel
	}
	return r.primary.Model()
}
func (r *Registry) ModelFor(req Request) string {
	if req.Model != "" {
		return req.Model
	}
	for _, m := range req.Messages {
		if len(m.ImageDataURLs) > 0 {
			if r.vision != "" {
				return r.vision
			}
			return r.primary.ModelFor(req)
		}
	}
	return r.Model()
}

func (r *Registry) listModels(ctx context.Context) ([]ModelOption, error) {
	base, err := r.primaryCatalog.List(ctx)
	if err != nil && len(r.routes) == 0 {
		return nil, err
	}
	items := append([]ModelOption{}, base.List...)
	for _, route := range r.routes {
		items = append(items, route.Option)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func ResolveProvider(ctx context.Context, source Provider, req Request) (Provider, Request, error) {
	r, ok := source.(*Registry)
	if !ok {
		return source, req, nil
	}
	id := r.ModelFor(req)
	if route, found := r.routes[id]; found {
		req.Model = route.Model
		return route.Provider, req, nil
	}
	selected, err := r.primaryCatalog.Select(ctx, id, false)
	if err != nil {
		return nil, req, err
	}
	req.Model = selected
	return r.primary, req, nil
}

func (r *Registry) Complete(ctx context.Context, req Request) (*Response, error) {
	p, req, err := ResolveProvider(ctx, r, req)
	if err != nil {
		return nil, err
	}
	return p.Complete(ctx, req)
}

func (r *Registry) Stream(ctx context.Context, req Request, onDelta func(StreamDelta) error) (*Response, error) {
	p, req, err := ResolveProvider(ctx, r, req)
	if err != nil {
		return nil, err
	}
	return p.Stream(ctx, req, onDelta)
}
