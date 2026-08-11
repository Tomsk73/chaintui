package api

// PageOpts controls a single cursor-paginated list request.
type PageOpts struct {
	PageSize  int32
	PageToken string
	OrderBy   string
	// Query is free-text search where the API supports it (e.g. advisories).
	Query string
}

// DefaultPageSize matches the API default when PageSize is unset or invalid.
const DefaultPageSize int32 = 50

func (o PageOpts) size() int32 {
	if o.PageSize <= 0 {
		return DefaultPageSize
	}
	if o.PageSize > 200 {
		return 200
	}
	return o.PageSize
}

// Page is one page of list results from a v2beta1 List RPC.
type Page[T any] struct {
	Items         []T
	NextPageToken string
	TotalCount    int64 // 0 if the API omitted it
}
