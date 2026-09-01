package orders

import (
	"context"
	"errors"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/fees"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const StatusPending = "pending"
const StatusFilled = "filled"
const StatusRejected = "rejected"
const StatusCanceled = "canceled"

type Row struct {
	ID                uuid.UUID
	SimulationID      uuid.UUID
	AgentID           uuid.UUID
	Symbol            string
	OrderType         string
	Side              string
	Quantity          int
	Price             *float64
	MatchOnDate       time.Time
	MatchOnTs         *time.Time
	FilledQuantity    int
	RemainingQuantity *int
	CreatedAt         time.Time
}

func (r Row) WorkingQty() int {
	if r.RemainingQuantity != nil {
		return *r.RemainingQuantity
	}
	if r.FilledQuantity > 0 && r.FilledQuantity < r.Quantity {
		return r.Quantity - r.FilledQuantity
	}
	return r.Quantity
}

func InsertPending(ctx context.Context, pool *pgxpool.Pool, simulationID, agentID uuid.UUID, symbol, orderType, side string, quantity int, price *float64, matchOnDate time.Time, matchOnTs *time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO orders (simulation_id, agent_id, symbol, order_type, side, quantity, price, status, match_on_date, match_on_ts, remaining_quantity, filled_quantity)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::date, $10, $11, 0)
		RETURNING id
	`, simulationID, agentID, symbol, orderType, side, quantity, price, StatusPending, matchOnDate, matchOnTs, quantity).Scan(&id)
	return id, err
}

func scanPending(rows pgx.Rows) ([]Row, error) {
	var out []Row
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.SimulationID, &r.AgentID, &r.Symbol, &r.OrderType, &r.Side, &r.Quantity, &r.Price, &r.MatchOnDate, &r.MatchOnTs, &r.RemainingQuantity, &r.FilledQuantity, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func ListPendingForDate(ctx context.Context, q pgx.Tx, simulationID uuid.UUID, tradeDate time.Time) ([]Row, error) {
	rows, err := q.Query(ctx, `
		SELECT id, simulation_id, agent_id, symbol, order_type, side, quantity, price, match_on_date, match_on_ts,
		       remaining_quantity, COALESCE(filled_quantity, 0), created_at
		FROM orders
		WHERE simulation_id = $1 AND status = $2 AND match_on_date = $3::date
		ORDER BY created_at ASC, id ASC
	`, simulationID, StatusPending, tradeDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPending(rows)
}

func ListPendingForTs(ctx context.Context, q pgx.Tx, simulationID uuid.UUID, barTs time.Time) ([]Row, error) {
	rows, err := q.Query(ctx, `
		SELECT id, simulation_id, agent_id, symbol, order_type, side, quantity, price, match_on_date, match_on_ts,
		       remaining_quantity, COALESCE(filled_quantity, 0), created_at
		FROM orders
		WHERE simulation_id = $1 AND status = $2 AND match_on_ts = $3
		ORDER BY created_at ASC, id ASC
	`, simulationID, StatusPending, barTs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPending(rows)
}

func MarkRejected(ctx context.Context, tx pgx.Tx, id uuid.UUID, reason string) error {
	_, err := tx.Exec(ctx, `
		UPDATE orders SET status = $2, rejection_reason = $3 WHERE id = $1 AND status = $4
	`, id, StatusRejected, reason, StatusPending)
	return err
}

func MarkFilled(ctx context.Context, tx pgx.Tx, id uuid.UUID, fillPrice float64, filledDay time.Time, br fees.Breakdown, tradeValue float64) error {
	_, err := tx.Exec(ctx, `
		UPDATE orders SET
			status = $2,
			filled_price = $3,
			filled_at = $4::date,
			filled_at_ts = $4::timestamptz,
			filled_quantity = quantity,
			remaining_quantity = 0,
			fees_total = $5,
			trade_value = $6,
			fee_brokerage = $7,
			fee_stt = $8,
			fee_gst = $9,
			fee_exchange = $10,
			fee_sebi = $11,
			fee_stamp = $12,
			rejection_reason = NULL
		WHERE id = $1 AND status = $13
	`, id, StatusFilled, fillPrice, filledDay, br.Total, tradeValue,
		br.Brokerage, br.STT, br.GST, br.Exchange, br.SEBI, br.Stamp, StatusPending)
	return err
}

func ApplySliceFill(ctx context.Context, tx pgx.Tx, id uuid.UUID, fillQty int, remainingAfter int, fillPrice float64, filledDay, filledTs time.Time, nextMatchTs *time.Time, br fees.Breakdown, tradeValue float64) error {
	status := StatusPending
	if remainingAfter <= 0 {
		status = StatusFilled
	}
	_, err := tx.Exec(ctx, `
		UPDATE orders SET
			status = $2,
			filled_quantity = COALESCE(filled_quantity, 0) + $3,
			remaining_quantity = $4,
			filled_price = CASE
				WHEN COALESCE(filled_quantity, 0) + $3 <= 0 THEN $5
				ELSE (COALESCE(filled_price, 0) * COALESCE(filled_quantity, 0) + $5 * $3)
				     / (COALESCE(filled_quantity, 0) + $3)
			END,
			filled_at = $6::date,
			filled_at_ts = $7,
			match_on_ts = COALESCE($8, match_on_ts),
			fees_total = COALESCE(fees_total, 0) + $9,
			trade_value = COALESCE(trade_value, 0) + $10,
			fee_brokerage = COALESCE(fee_brokerage, 0) + $11,
			fee_stt = COALESCE(fee_stt, 0) + $12,
			fee_gst = COALESCE(fee_gst, 0) + $13,
			fee_exchange = COALESCE(fee_exchange, 0) + $14,
			fee_sebi = COALESCE(fee_sebi, 0) + $15,
			fee_stamp = COALESCE(fee_stamp, 0) + $16,
			rejection_reason = NULL
		WHERE id = $1 AND status = $17
	`, id, status, fillQty, remainingAfter, fillPrice, filledDay, filledTs, nextMatchTs,
		br.Total, tradeValue, br.Brokerage, br.STT, br.GST, br.Exchange, br.SEBI, br.Stamp, StatusPending)
	return err
}

func UpdateMatchOnDate(ctx context.Context, tx pgx.Tx, id uuid.UUID, next time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE orders SET match_on_date = $2::date WHERE id = $1 AND status = $3
	`, id, next, StatusPending)
	return err
}

func UpdateMatchOnTs(ctx context.Context, tx pgx.Tx, id uuid.UUID, next time.Time, matchDate time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE orders SET match_on_ts = $2, match_on_date = $3::date WHERE id = $1 AND status = $4
	`, id, next, matchDate, StatusPending)
	return err
}

func FilledBuyQtyOnSession(ctx context.Context, tx pgx.Tx, simulationID, agentID uuid.UUID, symbol string, sessionDate time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(COALESCE(filled_quantity, quantity)), 0)
		FROM orders
		WHERE simulation_id = $1 AND agent_id = $2 AND symbol = $3
		  AND LOWER(side) = 'buy'
		  AND COALESCE(filled_quantity, 0) > 0
		  AND filled_at = $4::date
	`, simulationID, agentID, symbol, sessionDate).Scan(&n)
	return n, err
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
		SELECT id, symbol, order_type, side, quantity, price, status, filled_price, filled_at, filled_at_ts,
		       filled_quantity, remaining_quantity, rejection_reason, match_on_date, match_on_ts,
		       fees_total, trade_value, created_at,
		       fee_brokerage, fee_stt, fee_gst, fee_exchange, fee_sebi, fee_stamp
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
		var feeBrok, feeSTT, feeGST, feeExch, feeSEBI, feeStamp *float64
		var filledAt, filledAtTs, matchOn, matchOnTs *time.Time
		var filledQty, remainingQty *int
		var reject *string
		var createdAt time.Time
		if err := rows.Scan(&id, &symbol, &orderType, &side, &qty, &price, &status, &filledPrice, &filledAt, &filledAtTs,
			&filledQty, &remainingQty, &reject, &matchOn, &matchOnTs, &feesTotal, &tradeVal, &createdAt,
			&feeBrok, &feeSTT, &feeGST, &feeExch, &feeSEBI, &feeStamp); err != nil {
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
		if filledAtTs != nil {
			m["filled_at_ts"] = filledAtTs.UTC().Format(time.RFC3339)
		}
		if filledQty != nil {
			m["filled_quantity"] = *filledQty
		}
		if remainingQty != nil {
			m["remaining_quantity"] = *remainingQty
		}
		if matchOn != nil {
			m["match_on_date"] = matchOn.Format(time.DateOnly)
		}
		if matchOnTs != nil {
			m["match_on_ts"] = matchOnTs.UTC().Format(time.RFC3339)
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
		feesMap := map[string]any{}
		if feeBrok != nil {
			feesMap["brokerage"] = *feeBrok
		}
		if feeSTT != nil {
			feesMap["stt"] = *feeSTT
		}
		if feeGST != nil {
			feesMap["gst"] = *feeGST
		}
		if feeExch != nil {
			feesMap["exchange"] = *feeExch
		}
		if feeSEBI != nil {
			feesMap["sebi"] = *feeSEBI
		}
		if feeStamp != nil {
			feesMap["stamp"] = *feeStamp
		}
		if len(feesMap) > 0 {
			m["fees"] = feesMap
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
