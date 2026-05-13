package orderbook

import (
	"github.com/agentic-sim-trading/market-simulator/pkg/models"
	"github.com/google/uuid"
)

// OrderBook wraps roadmap concepts for Phase 2 matching (filled out in Step 8).
type OrderBook struct {
	orders []*QueuedOrder
}

type QueuedOrder struct {
	ID     uuid.UUID
	Order  models.Order
	Ticket int
}

func NewOrderBook() *OrderBook {
	return &OrderBook{}
}

func (ob *OrderBook) Len() int { return len(ob.orders) }

func (ob *OrderBook) Enqueue(q QueuedOrder) {
	ob.orders = append(ob.orders, &q)
}
