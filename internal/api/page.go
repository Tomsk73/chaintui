package api

// PageOpts controls a single cursor-paginated list request.
type PageOpts struct {
	PageSize  int32
	PageToken string
	OrderBy string
	// Query is a server-side filter string. Advisories treat it as free-text
	// search; list RPCs with a Name field treat it as an exact name match.
	Query string
}

// DefaultPageSize matches the API default when PageSize is unset or invalid.
const DefaultPageSize int32 = 50

// MaxPageSize is the largest page the API accepts; larger requests are clamped.
const MaxPageSize int32 = 200

func (o PageOpts) size() int32 {
	if o.PageSize <= 0 {
		return DefaultPageSize
	}
	if o.PageSize > MaxPageSize {
		return MaxPageSize
	}
	return o.PageSize
}

// Page is one page of list results from a v2beta1 List RPC.
type Page[T any] struct {
	Items         []T
	NextPageToken string
	TotalCount    int64 // 0 if the API omitted it
}
