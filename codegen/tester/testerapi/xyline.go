/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

package testerapi

// XYLine is a non-primitive type with a nested non-primitive type.
type XYLine struct {
	Start XYCoord `json:"start,omitempty"`
	End   XYCoord `json:"end,omitempty"`
}