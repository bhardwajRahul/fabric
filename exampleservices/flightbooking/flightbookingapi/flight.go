package flightbookingapi

// Flight is a single candidate itinerary on a route.
type Flight struct {
	Airline     string   `json:"airline,omitzero" jsonschema_description:"Airline is the operating carrier"`
	FlightNo    string   `json:"flightNo,omitzero" jsonschema_description:"FlightNo is the flight number"`
	Origin      string   `json:"origin,omitzero" jsonschema_description:"Origin is the departure city"`
	Destination string   `json:"destination,omitzero" jsonschema_description:"Destination is the arrival city"`
	Depart      string   `json:"depart,omitzero" jsonschema_description:"Depart is the local departure time, e.g. 08:15"`
	Arrive      string   `json:"arrive,omitzero" jsonschema_description:"Arrive is the local arrival time, e.g. 16:40"`
	Stops       int      `json:"stops,omitzero" jsonschema_description:"Stops is the number of connections, 0 for non-stop"`
	PriceUSD    int      `json:"priceUSD,omitzero" jsonschema_description:"PriceUSD is the fare in US dollars"`
	Seats       []string `json:"seats,omitzero" jsonschema_description:"Seats are the available seat labels on this flight"`
}
