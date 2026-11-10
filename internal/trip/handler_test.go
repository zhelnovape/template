package trip

import (
	"strings"
	"testing"
)

func TestDecodeTripDataRejectsCaseVariantFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{
			name: "PRICE",
			body: `{
				"user_id": "11111111-1111-4111-8111-111111111111",
				"driver_id": "22222222-2222-4222-8222-222222222222",
				"start_point": {"latitude": 0, "longitude": 0},
				"end_point": {"latitude": 1, "longitude": 1},
				"price": 0, "PRICE": 500
			}`,
		},
		{
			name: "Latitude",
			body: `{
				"user_id": "11111111-1111-4111-8111-111111111111",
				"driver_id": "22222222-2222-4222-8222-222222222222",
				"start_point": {"latitude": 0, "longitude": 0, "Latitude": 40},
				"end_point": {"latitude": 1, "longitude": 1},
				"price": 0
			}`,
		},
		{
			name: "Longitude",
			body: `{
				"user_id": "11111111-1111-4111-8111-111111111111",
				"driver_id": "22222222-2222-4222-8222-222222222222",
				"start_point": {"latitude": 0, "longitude": 0, "Longitude": 50},
				"end_point": {"latitude": 1, "longitude": 1},
				"price": 0
			}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if req, ok := decodeTripData(strings.NewReader(tc.body)); ok {
				t.Fatalf("декодер принял лишнее поле %s: цена %d, координаты (%g, %g)",
					tc.name, req.Price, req.StartPoint.Latitude, req.StartPoint.Longitude)
			}
		})
	}
}