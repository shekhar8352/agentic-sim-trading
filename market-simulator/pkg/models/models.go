package models

import (
	"time"

	"github.com/google/uuid"
)

type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

type OrderType string

const (
	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"
	OrderTypeStop   OrderType = "stop"
)

type OrderStatus string

const (
	StatusPending  OrderStatus = "pending"
	StatusFilled   OrderStatus = "filled"
	StatusRejected OrderStatus = "rejected"
	StatusCanceled OrderStatus = "canceled"
)

// Order is the domain shape aligned with init.sql `orders` plus IDs.
type Order struct {
	ID           uuid.UUID   `json:"id"`
	SimulationID uuid.UUID   `json:"simulation_id"`
	AgentID      uuid.UUID   `json:"agent_id"`
	Symbol       string      `json:"symbol"`
	OrderType    OrderType   `json:"order_type"`
	Side         Side        `json:"side"`
	Quantity     int         `json:"quantity"`
	Price        float64     `json:"price,omitempty"`
	Status       OrderStatus `json:"status"`
	FilledPrice  float64     `json:"filled_price,omitempty"`
	FilledAt     *time.Time  `json:"filled_at,omitempty"`
	RejectReason string      `json:"rejection_reason,omitempty"`
	CreatedAt    time.Time   `json:"created_at,omitempty"`
}

// Trade represents a fill record used by the matching engine and Redis events.
type Trade struct {
	ID           uuid.UUID `json:"id"`
	SimulationID uuid.UUID `json:"simulation_id"`
	AgentID      uuid.UUID `json:"agent_id"`
	Symbol       string    `json:"symbol"`
	Side         Side      `json:"side"`
	Quantity     int       `json:"quantity"`
	Price        float64   `json:"price"`
	Date         time.Time `json:"date"`
}

const (
	Interval1d  = "1d"
	Interval60m = "60m"
)

// Quote is the simulated price context for an agent at the current clock date.
type Quote struct {
	Symbol   string    `json:"symbol"`
	Date     time.Time `json:"date"`
	Ts       time.Time `json:"ts,omitempty"`
	Interval string    `json:"interval,omitempty"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	Volume   int64     `json:"volume"`
}
