package flight

import "context"

const (
	CabinEconomy        = "economy"
	CabinPremiumEconomy = "premium_economy"
	CabinBusiness       = "business"
	CabinFirst          = "first"

	BagsNone         = "none"
	BagsPersonalItem = "personal_item"
	BagsCabin        = "cabin"
	BagsChecked      = "checked"

	SortPrice    = "price"
	SortDuration = "duration"
	SortBest     = "best"
)

type Provider interface {
	Name() string
	Status(ctx context.Context) ProviderStatus
	Search(ctx context.Context, request FlightSearchRequest) ([]FlightSearchResult, error)
}

type FlexibleProvider interface {
	Cheapest(ctx context.Context, request FlexibleSearchRequest) ([]FlightSearchResult, error)
}

type ProviderStatus struct {
	Name      string   `json:"name"`
	Enabled   bool     `json:"enabled"`
	Available bool     `json:"available"`
	Mode      string   `json:"mode,omitempty"`
	Missing   []string `json:"missing,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Error     string   `json:"error,omitempty"`
}

type FlightSearchRequest struct {
	Origin              string `json:"origin"`
	Destination         string `json:"destination"`
	DepartDate          string `json:"departDate"`
	ReturnDate          string `json:"returnDate,omitempty"`
	OneWay              bool   `json:"oneWay,omitempty"`
	Adults              int    `json:"adults"`
	Children            int    `json:"children,omitempty"`
	Infants             int    `json:"infants,omitempty"`
	Currency            string `json:"currency"`
	Cabin               string `json:"cabin"`
	MaxStops            *int   `json:"maxStops,omitempty"`
	DirectOnly          bool   `json:"directOnly,omitempty"`
	IncludeSelfTransfer bool   `json:"includeSelfTransfer,omitempty"`
	Bags                string `json:"bags,omitempty"`
	Limit               int    `json:"limit,omitempty"`
	Sort                string `json:"sort,omitempty"`
}

type FlexibleSearchRequest struct {
	FlightSearchRequest
	Month    string  `json:"month"`
	MaxPrice float64 `json:"maxPrice,omitempty"`
	TripDays int     `json:"tripDays,omitempty"`
}

type FlightSearchResult struct {
	Provider         string   `json:"provider"`
	ProviderResultID string   `json:"providerResultId,omitempty"`
	PriceAvailable   bool     `json:"priceAvailable"`
	TotalPrice       float64  `json:"totalPrice,omitempty"`
	Currency         string   `json:"currency"`
	DeepLink         string   `json:"deepLink,omitempty"`
	BookingLink      string   `json:"bookingLink,omitempty"`
	Origin           string   `json:"origin"`
	Destination      string   `json:"destination"`
	DepartAt         string   `json:"departAt,omitempty"`
	ArriveAt         string   `json:"arriveAt,omitempty"`
	ReturnDepartAt   string   `json:"returnDepartAt,omitempty"`
	ReturnArriveAt   string   `json:"returnArriveAt,omitempty"`
	DurationMinutes  int      `json:"durationMinutes,omitempty"`
	Stops            int      `json:"stops"`
	Airlines         []string `json:"airlines,omitempty"`
	FlightNumbers    []string `json:"flightNumbers,omitempty"`
	Baggage          Baggage  `json:"baggage"`
	Risks            []string `json:"risks,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	Raw              any      `json:"raw,omitempty"`
	Score            float64  `json:"score,omitempty"`
}

type Baggage struct {
	PersonalItemIncluded *bool  `json:"personalItemIncluded,omitempty"`
	CabinBagIncluded     *bool  `json:"cabinBagIncluded,omitempty"`
	CheckedBagIncluded   *bool  `json:"checkedBagIncluded,omitempty"`
	Notes                string `json:"notes,omitempty"`
}

type ProviderError struct {
	Provider string `json:"provider"`
	Error    string `json:"error"`
}

type SearchSummary struct {
	Request        FlightSearchRequest  `json:"request"`
	Results        []FlightSearchResult `json:"results"`
	ProviderErrors []ProviderError      `json:"providerErrors,omitempty"`
}

type Watch struct {
	ID        string              `json:"id"`
	CreatedAt string              `json:"createdAt"`
	Request   FlightSearchRequest `json:"request"`
	MaxPrice  float64             `json:"maxPrice,omitempty"`
	Notify    string              `json:"notify,omitempty"`
	Providers []string            `json:"providers,omitempty"`
}
