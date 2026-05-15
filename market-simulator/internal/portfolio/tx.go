package portfolio

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Holding struct {
	Quantity    int
	AvgBuyPrice float64
}

func (m *Manager) PortfolioIDTx(ctx context.Context, tx pgx.Tx, simulationID, agentID uuid.UUID) (int64, error) {
	if m.pool == nil {
		return 0, errors.New("no database")
	}
	var id int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM portfolios WHERE simulation_id = $1 AND agent_id = $2
	`, simulationID, agentID).Scan(&id)
	return id, err
}

func (m *Manager) GetCashTx(ctx context.Context, tx pgx.Tx, portfolioID int64) (float64, error) {
	var cash float64
	err := tx.QueryRow(ctx, `SELECT cash::float8 FROM portfolios WHERE id = $1`, portfolioID).Scan(&cash)
	return cash, err
}

func (m *Manager) SetCashTx(ctx context.Context, tx pgx.Tx, portfolioID int64, cash float64) error {
	_, err := tx.Exec(ctx, `UPDATE portfolios SET cash = $2 WHERE id = $1`, portfolioID, cash)
	return err
}

func (m *Manager) LoadHoldingsTx(ctx context.Context, tx pgx.Tx, portfolioID int64) (map[string]Holding, error) {
	rows, err := tx.Query(ctx, `
		SELECT symbol, quantity, avg_buy_price::float8 FROM holdings WHERE portfolio_id = $1
	`, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]Holding)
	for rows.Next() {
		var sym string
		var q int
		var avg float64
		if err := rows.Scan(&sym, &q, &avg); err != nil {
			return nil, err
		}
		out[sym] = Holding{Quantity: q, AvgBuyPrice: avg}
	}
	return out, rows.Err()
}

func (m *Manager) UpsertHoldingAfterBuy(ctx context.Context, tx pgx.Tx, portfolioID int64, symbol string, buyQty int, fillPrice float64) error {
	var curQty int
	var curAvg float64
	err := tx.QueryRow(ctx, `
		SELECT quantity, avg_buy_price::float8 FROM holdings
		WHERE portfolio_id = $1 AND symbol = $2
	`, portfolioID, symbol).Scan(&curQty, &curAvg)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `
			INSERT INTO holdings (portfolio_id, symbol, quantity, avg_buy_price)
			VALUES ($1, $2, $3, $4)
		`, portfolioID, symbol, buyQty, fillPrice)
		return err
	}
	if err != nil {
		return err
	}
	newQty := curQty + buyQty
	newAvg := (float64(curQty)*curAvg + float64(buyQty)*fillPrice) / float64(newQty)
	_, err = tx.Exec(ctx, `
		UPDATE holdings SET quantity = $3, avg_buy_price = $4
		WHERE portfolio_id = $1 AND symbol = $2
	`, portfolioID, symbol, newQty, newAvg)
	return err
}

func (m *Manager) ReduceHoldingAfterSell(ctx context.Context, tx pgx.Tx, portfolioID int64, symbol string, sellQty int) error {
	var curQty int
	err := tx.QueryRow(ctx, `
		SELECT quantity FROM holdings WHERE portfolio_id = $1 AND symbol = $2
	`, portfolioID, symbol).Scan(&curQty)
	if err != nil {
		return err
	}
	newQty := curQty - sellQty
	if newQty < 0 {
		return errors.New("negative holdings")
	}
	if newQty == 0 {
		_, err = tx.Exec(ctx, `DELETE FROM holdings WHERE portfolio_id = $1 AND symbol = $2`, portfolioID, symbol)
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE holdings SET quantity = $3 WHERE portfolio_id = $1 AND symbol = $2`, portfolioID, symbol, newQty)
	return err
}

func (m *Manager) ListAgentIDsTx(ctx context.Context, tx pgx.Tx, simulationID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `SELECT agent_id FROM portfolios WHERE simulation_id = $1 ORDER BY agent_id`, simulationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (m *Manager) UpsertSnapshotTx(ctx context.Context, tx pgx.Tx, simulationID, agentID uuid.UUID, day time.Time, totalValue, cash, invested float64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO portfolio_snapshots (simulation_id, agent_id, date, total_value, cash, invested_value)
		VALUES ($1, $2, $3::date, $4, $5, $6)
		ON CONFLICT (simulation_id, agent_id, date) DO UPDATE SET
			total_value = EXCLUDED.total_value,
			cash = EXCLUDED.cash,
			invested_value = EXCLUDED.invested_value
	`, simulationID, agentID, day, totalValue, cash, invested)
	return err
}

func (m *Manager) HoldingQtyTx(ctx context.Context, tx pgx.Tx, portfolioID int64, symbol string) (int, error) {
	var q int
	err := tx.QueryRow(ctx, `
		SELECT quantity FROM holdings WHERE portfolio_id = $1 AND symbol = $2
	`, portfolioID, symbol).Scan(&q)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return q, err
}
