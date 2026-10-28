package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const tripsTable = "trips"

func TestSpanName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sql            string
		wantOperation  string
		wantCollection string
		wantSpanName   string
	}{
		{
			sql:            "INSERT INTO trips (id,user_id) VALUES ($1,$2)",
			wantOperation:  "INSERT",
			wantCollection: tripsTable,
			wantSpanName:   "INSERT trips",
		},
		{
			sql:            "SELECT id, status FROM trips WHERE id = $1",
			wantOperation:  "SELECT",
			wantCollection: tripsTable,
			wantSpanName:   "SELECT trips",
		},
		{
			sql:            "UPDATE trips SET status = $1 WHERE id = $2 RETURNING updated_at",
			wantOperation:  "UPDATE",
			wantCollection: tripsTable,
			wantSpanName:   "UPDATE trips",
		},
		{
			sql:            "INSERT INTO trip_status_history (trip_id) VALUES ($1)",
			wantOperation:  "INSERT",
			wantCollection: "trip_status_history",
			wantSpanName:   "INSERT trip_status_history",
		},
		{
			sql:           "begin isolation level read committed",
			wantOperation: "BEGIN",
			wantSpanName:  "BEGIN",
		},
		{
			sql:           "",
			wantOperation: "UNKNOWN",
			wantSpanName:  "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.wantSpanName, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantOperation, operationName(tt.sql))
			assert.Equal(t, tt.wantCollection, collectionName(tt.sql))
			assert.Equal(t, tt.wantSpanName, spanName(tt.sql))
		})
	}
}
