package docextractionflowapi

// Rectangle is an axis-aligned region of a page image, in pixels.
type Rectangle struct {
	X int `json:"x,omitzero" jsonschema:"description=X is the left coordinate in pixels"`
	Y int `json:"y,omitzero" jsonschema:"description=Y is the top coordinate in pixels"`
	W int `json:"w,omitzero" jsonschema:"description=W is the width in pixels"`
	H int `json:"h,omitzero" jsonschema:"description=H is the height in pixels"`
}
