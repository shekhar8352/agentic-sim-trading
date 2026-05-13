package orderbook

import "context"

// Queue holds pending orders per simulation tick (see roadmap Step 8).
type Queue struct {
	book *OrderBook
}

func NewQueue(book *OrderBook) *Queue {
	return &Queue{book: book}
}

func (q *Queue) PendingCount() int {
	if q.book == nil {
		return 0
	}
	return q.book.Len()
}

// Drain is a placeholder until persistence-backed pending orders exist.
func (q *Queue) Drain(context.Context) ([]QueuedOrder, error) {
	return nil, nil
}
