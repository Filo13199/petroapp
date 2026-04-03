package db

import (
	"petroapp/models"
	"sync"
)

type InMemoryDB struct {
	//connection pool or whatever
	//the key of the map is the station_id since most intensive operations (read for summary will be by station id)
	StationEvents map[string][]models.Event

	//for simplicity, the unique index will be a simple map to query the existence of an event_id
	UniqueIndex map[string]struct{}
	mu          sync.RWMutex
}

type DB interface {
	InsertEvent(event models.Event) (bool, error)
	GetStationEventsByStationId(id string) (map[string]models.Event, error)
}
