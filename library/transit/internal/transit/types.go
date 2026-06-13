package transit

import "time"

type ProductFlags struct {
	Suburban bool `json:"suburban"`
	Subway   bool `json:"subway"`
	Tram     bool `json:"tram"`
	Bus      bool `json:"bus"`
	Ferry    bool `json:"ferry"`
	Express  bool `json:"express"`
	Regional bool `json:"regional"`
}

type Location struct {
	Type        string        `json:"type,omitempty"`
	ID          string        `json:"id,omitempty"`
	Name        string        `json:"name,omitempty"`
	Latitude    float64       `json:"latitude,omitempty"`
	Longitude   float64       `json:"longitude,omitempty"`
	Address     string        `json:"address,omitempty"`
	Location    *Coordinates  `json:"location,omitempty"`
	Products    ProductFlags  `json:"products,omitempty"`
	Lines       []Line        `json:"lines,omitempty"`
	Distance    int           `json:"distance,omitempty"`
	StationID   string        `json:"stationDHID,omitempty"`
	Stopovers   []Stopover    `json:"stopovers,omitempty"`
	Remarks     []Remark      `json:"remarks,omitempty"`
	RawProducts *ProductFlags `json:"-"`
}

type Coordinates struct {
	Type      string  `json:"type,omitempty"`
	ID        string  `json:"id,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Address   string  `json:"address,omitempty"`
}

type Line struct {
	Type        string `json:"type,omitempty"`
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	ProductName string `json:"productName,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Product     string `json:"product,omitempty"`
}

type Remark struct {
	Type string `json:"type,omitempty"`
	Code string `json:"code,omitempty"`
	Text string `json:"text,omitempty"`
}

type DepartureResponse struct {
	Departures []Departure `json:"departures"`
}

type Departure struct {
	TripID          string     `json:"tripId,omitempty"`
	Stop            Location   `json:"stop"`
	When            *time.Time `json:"when,omitempty"`
	PlannedWhen     *time.Time `json:"plannedWhen,omitempty"`
	Delay           *int       `json:"delay,omitempty"`
	Platform        string     `json:"platform,omitempty"`
	PlannedPlatform string     `json:"plannedPlatform,omitempty"`
	Cancelled       bool       `json:"cancelled,omitempty"`
	Remarks         []Remark   `json:"remarks,omitempty"`
	Line            Line       `json:"line"`
	Direction       string     `json:"direction,omitempty"`
}

type JourneyResponse struct {
	EarlierRef string    `json:"earlierRef,omitempty"`
	LaterRef   string    `json:"laterRef,omitempty"`
	Journeys   []Journey `json:"journeys"`
}

type Journey struct {
	Type          string   `json:"type,omitempty"`
	Legs          []Leg    `json:"legs"`
	RefreshToken  string   `json:"refreshToken,omitempty"`
	Remarks       []Remark `json:"remarks,omitempty"`
	Cycle         *Journey `json:"cycle,omitempty"`
	Price         any      `json:"price,omitempty"`
	ScheduledDays any      `json:"scheduledDays,omitempty"`
}

type Leg struct {
	Origin                   Location   `json:"origin"`
	Destination              Location   `json:"destination"`
	Departure                *time.Time `json:"departure,omitempty"`
	PlannedDeparture         *time.Time `json:"plannedDeparture,omitempty"`
	DepartureDelay           *int       `json:"departureDelay,omitempty"`
	Arrival                  *time.Time `json:"arrival,omitempty"`
	PlannedArrival           *time.Time `json:"plannedArrival,omitempty"`
	ArrivalDelay             *int       `json:"arrivalDelay,omitempty"`
	Reachable                *bool      `json:"reachable,omitempty"`
	Public                   bool       `json:"public,omitempty"`
	Walking                  bool       `json:"walking,omitempty"`
	Distance                 int        `json:"distance,omitempty"`
	TripID                   string     `json:"tripId,omitempty"`
	Line                     Line       `json:"line,omitempty"`
	Direction                string     `json:"direction,omitempty"`
	Cancelled                bool       `json:"cancelled,omitempty"`
	Remarks                  []Remark   `json:"remarks,omitempty"`
	Stopovers                []Stopover `json:"stopovers,omitempty"`
	DeparturePlatform        string     `json:"departurePlatform,omitempty"`
	PlannedDeparturePlatform string     `json:"plannedDeparturePlatform,omitempty"`
	ArrivalPlatform          string     `json:"arrivalPlatform,omitempty"`
	PlannedArrivalPlatform   string     `json:"plannedArrivalPlatform,omitempty"`
}

type Stopover struct {
	Stop                     Location   `json:"stop"`
	Arrival                  *time.Time `json:"arrival,omitempty"`
	PlannedArrival           *time.Time `json:"plannedArrival,omitempty"`
	ArrivalDelay             *int       `json:"arrivalDelay,omitempty"`
	ArrivalPlatform          string     `json:"arrivalPlatform,omitempty"`
	PlannedArrivalPlatform   string     `json:"plannedArrivalPlatform,omitempty"`
	Departure                *time.Time `json:"departure,omitempty"`
	PlannedDeparture         *time.Time `json:"plannedDeparture,omitempty"`
	DepartureDelay           *int       `json:"departureDelay,omitempty"`
	DeparturePlatform        string     `json:"departurePlatform,omitempty"`
	PlannedDeparturePlatform string     `json:"plannedDeparturePlatform,omitempty"`
	Remarks                  []Remark   `json:"remarks,omitempty"`
}

type RadarResponse struct {
	Movements []Movement `json:"movements"`
}

type Movement struct {
	Direction     string       `json:"direction,omitempty"`
	TripID        string       `json:"tripId,omitempty"`
	Line          Line         `json:"line"`
	Location      *Coordinates `json:"location,omitempty"`
	NextStopovers []Stopover   `json:"nextStopovers,omitempty"`
	Frames        []RadarFrame `json:"frames,omitempty"`
}

type RadarFrame struct {
	Origin      Location     `json:"origin,omitempty"`
	Destination Location     `json:"destination,omitempty"`
	T           any          `json:"t,omitempty"`
	Location    *Coordinates `json:"location,omitempty"`
}

type TripResponse struct {
	Trip Trip `json:"trip"`
}

type Trip struct {
	Origin                   Location     `json:"origin"`
	Destination              Location     `json:"destination"`
	Departure                *time.Time   `json:"departure,omitempty"`
	PlannedDeparture         *time.Time   `json:"plannedDeparture,omitempty"`
	DepartureDelay           *int         `json:"departureDelay,omitempty"`
	Arrival                  *time.Time   `json:"arrival,omitempty"`
	PlannedArrival           *time.Time   `json:"plannedArrival,omitempty"`
	ArrivalDelay             *int         `json:"arrivalDelay,omitempty"`
	Line                     Line         `json:"line,omitempty"`
	Direction                string       `json:"direction,omitempty"`
	CurrentLocation          *Coordinates `json:"currentLocation,omitempty"`
	ArrivalPlatform          string       `json:"arrivalPlatform,omitempty"`
	PlannedArrivalPlatform   string       `json:"plannedArrivalPlatform,omitempty"`
	DeparturePlatform        string       `json:"departurePlatform,omitempty"`
	PlannedDeparturePlatform string       `json:"plannedDeparturePlatform,omitempty"`
	Stopovers                []Stopover   `json:"stopovers,omitempty"`
	Remarks                  []Remark     `json:"remarks,omitempty"`
}

func (l Location) DisplayName() string {
	if l.Name != "" {
		return l.Name
	}
	if l.Address != "" {
		return l.Address
	}
	if l.Location != nil && l.Location.Address != "" {
		return l.Location.Address
	}
	if l.ID != "" {
		return l.ID
	}
	return "-"
}

func (l Location) Coordinates() (float64, float64, bool) {
	if l.Latitude != 0 || l.Longitude != 0 {
		return l.Latitude, l.Longitude, true
	}
	if l.Location != nil && (l.Location.Latitude != 0 || l.Location.Longitude != 0) {
		return l.Location.Latitude, l.Location.Longitude, true
	}
	return 0, 0, false
}
