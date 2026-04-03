package db

import (
	"fmt"
	"petroapp/models"
)

func NewInMemoryDB() *InMemoryDB {
	return &InMemoryDB{
		StationEvents: make(map[string][]models.Event),
		UniqueIndex:   make(map[string]struct{}),
	}
}

// InsertEvent inserts the event if its event_id hasn't been seen before.
// Returns (true, nil) on insert, (false, nil) if it's a duplicate.
// The write lock is held for the full check-then-insert to prevent races.
func (d *InMemoryDB) InsertEvent(event models.Event) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.UniqueIndex[event.EventId]; exists {
		return false, nil
	}

	d.UniqueIndex[event.EventId] = struct{}{}
	d.StationEvents[event.StationId] = append(d.StationEvents[event.StationId], event)
	return true, nil
}

func (d *InMemoryDB) GetStationEventsByStationId(id string) (map[string]models.Event, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	events, ok := d.StationEvents[id]
	if !ok {
		return nil, fmt.Errorf("station %s not found", id)
	}

	result := make(map[string]models.Event, len(events))
	for _, e := range events {
		result[e.EventId] = e
	}
	return result, nil
}
