package radio

// catalogState is the catalog part of the service (lane B): the Radio Browser sync, the merge into
// stations, the facets, the in-memory index behind lists, facets and search, the favourites and
// preferences of a profile and the station administration.
type catalogState struct{}

func newCatalogState() *catalogState { return &catalogState{} }
