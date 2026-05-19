package orders

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const StatusPending = "pending"
const StatusFilled = "filled"
const StatusRejected = "rejected"
const StatusCanceled = "canceled"

type Row struct {
	ID           uuid.UUID
	SimulationID uuid.UUID
	AgentID      uuid.UUID
	Symbol       string
	OrderType    string
	Side         string
	Quantity     int
	MatchOnDate  time.Time
	CreatedAt    time.Time
}

func InsertPending(ctx context.Context, pool *pgxpool.Pool, simulationID, agentID uuid.UUID, symbol, orderType, side string, quantity int, matchOnDate time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO orders (simulation_id, agent_id, symbol, order_type, side, quantity, price, status, match_on_date)
		VALUES ($1, $2, $3, $4, $5, $6, NULL, $7, $8::date)
		RETURNING id
	`, simulationID, agentID, symbol, orderType, side, quantity, StatusPending, matchOnDate).Scan(&id)
	return id, err
}

func ListPendingForDate(ctx context.Context, q pgx.Tx, simulationID uuid.UUID, tradeDate time.Time) ([]Row, error) {
	rows, err := q.Query(ctx, `
		SELECT id, simulation_id, agent_id, symbol, order_type, side, quantity, match_on_date, created_at
		FROM orders
		WHERE simulation_id = $1 AND status = $2 AND match_on_date = $3::date
		ORDER BY created_at ASC, id ASC
	`, simulationID, StatusPending, tradeDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.SimulationID, &r.AgentID, &r.Symbol, &r.OrderType, &r.Side, &r.Quantity, &r.MatchOnDate, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func MarkRejected(ctx context.Context, tx pgx.Tx, id uuid.UUID, reason string) error {
	_, err := tx.Exec(ctx, `
		UPDATE orders SET status = $2, rejection_reason = $3 WHERE id = $1 AND status = $4
	`, id, StatusRejected, reason, StatusPending)
	return err
}

func MarkFilled(ctx context.Context, tx pgx.Tx, id uuid.UUID, openPrice float64, filledDay time.Time, feesTotal, tradeValue float64) error {
	_, err := tx.Exec(ctx, `
		UPDATE orders SET
			status = $2,
			filled_price = $3,
			filled_at = $4::date,
			fees_total = $5,
			trade_value = $6,
			rejection_reason = NULL
		WHERE id = $1 AND status = $7
	`, id, StatusFilled, openPrice, filledDay, feesTotal, tradeValue, StatusPending)
	return err
}

func MarkCanceled(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (bool, error) {
	tag, err := pool.Exec(ctx, `
		UPDATE orders SET status = $2 WHERE id = $1 AND status = $3
	`, id, StatusCanceled, StatusPending)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// CancelPending marks a pending order canceled when it belongs to the simulation.
func CancelPending(ctx context.Context, pool *pgxpool.Pool, simulationID, orderID uuid.UUID) (bool, error) {
	tag, err := pool.Exec(ctx, `
		UPDATE orders SET status = $2
		WHERE id = $1 AND simulation_id = $3 AND status = $4
	`, orderID, StatusCanceled, simulationID, StatusPending)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// PendingOrderAgentID returns the owning agent for a pending order, if any.
func PendingOrderAgentID(ctx context.Context, pool *pgxpool.Pool, simulationID, orderID uuid.UUID) (uuid.UUID, bool, error) {
	var aid uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT agent_id FROM orders
		WHERE id = $1 AND simulation_id = $2 AND status = $3
	`, orderID, simulationID, StatusPending).Scan(&aid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, err
	}
	return aid, true, nil
}

func ListByAgent(ctx context.Context, pool *pgxpool.Pool, simulationID, agentID uuid.UUID, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := pool.Query(ctx, `
		SELECT id, symbol, order_type, side, quantity, price, status, filled_price, filled_at,
		       rejection_reason, match_on_date, fees_total, trade_value, created_at
		FROM orders
		WHERE simulation_id = $1 AND agent_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`, simulationID, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var symbol, orderType, side, status string
		var qty int
		var price, filledPrice, feesTotal, tradeVal *float64
		var filledAt, matchOn *time.Time
		var reject *string
		var createdAt time.Time
		if err := rows.Scan(&id, &symbol, &orderType, &side, &qty, &price, &status, &filledPrice, &filledAt,
			&reject, &matchOn, &feesTotal, &tradeVal, &createdAt); err != nil {
			return nil, err
		}
		m := map[string]any{
			"id": id.String(), "symbol": symbol, "order_type": orderType, "side": side,
			"quantity": qty, "status": status, "created_at": createdAt.UTC().Format(time.RFC3339),
		}
		if price != nil {
			m["price"] = *price
		}
		if filledPrice != nil {
			m["filled_price"] = *filledPrice
		}
		if filledAt != nil {
			m["filled_at"] = filledAt.Format(time.DateOnly)
		}
		if matchOn != nil {
			m["match_on_date"] = matchOn.Format(time.DateOnly)
		}
		if reject != nil {
			m["rejection_reason"] = *reject
		}
		if feesTotal != nil {
			m["fees_total"] = *feesTotal
		}
		if tradeVal != nil {
			m["trade_value"] = *tradeVal
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
