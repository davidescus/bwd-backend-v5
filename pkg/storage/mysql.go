package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"

	_ "github.com/go-sql-driver/mysql"
)

type Mysql struct {
	db *sql.DB
}

func NewMysql(connString string) (*Mysql, error) {
	db, err := sql.Open("mysql", connString)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	instance := Mysql{
		db: db,
	}

	if err = instance.createSchemaIfNotExists(); err != nil {
		return nil, err
	}

	return &instance, nil
}

func (s *Mysql) Apps() ([]App, error) {
	rows, err := s.db.Query(`
        SELECT
            app_id,
            run_interval,
            exchange,
            market_order_fees,
            limit_order_fees,
            base,
            quote,
            min_base_price,
            max_base_price,
            step_quote_volume,
            steps_type,
            steps_details,
            compound_type,
            compound_details,
            publish_orders_number,
            status
        FROM apps
   `)

	if err != nil {
		return []App{}, err
	}
	defer rows.Close()

	var apps []App

	for rows.Next() {
		var app App
		var runInterval string

		err := rows.Scan(
			&app.ID,
			&runInterval,
			&app.Exchange,
			&app.MarketOrderFees,
			&app.LimitOrderFees,
			&app.Base,
			&app.Quote,
			&app.MinBasePrice,
			&app.MaxBasePrice,
			&app.StepQuoteVolume,
			&app.StepsType,
			&app.StepsDetails,
			&app.CompoundType,
			&app.CompoundDetails,
			&app.PublishOrderNumber,
			&app.Status,
		)
		if err != nil {
			return []App{}, err
		}

		interval, err := time.ParseDuration(runInterval)
		if err != nil {
			return []App{}, err
		}
		app.Interval = interval

		apps = append(apps, app)
	}

	return apps, nil
}

func (s *Mysql) ActiveTrades(appID int) ([]Trade, error) {
	q := fmt.Sprintf(`
		SELECT
			id,   
			app_id,
			open_base_price,
			close_base_price,
			open_type,
			close_type,
			base_volume,
			buy_order_id,
			sell_order_id,
			status,
			converted_sell_limit_at,
			closed_at,
			updated_at,   
			created_at
		FROM trades
		WHERE 1
			AND app_id = %d
			AND status != 'CLOSED'
       `,
		appID,
	)
	rows, err := s.db.Query(q)

	if err != nil {
		return []Trade{}, err
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var trade Trade
		var convertedSellLimitAt, closedAt, updatedAt, createdAt mysql.NullTime

		err := rows.Scan(
			&trade.ID,
			&trade.AppID,
			&trade.OpenBasePrice,
			&trade.CloseBasePrice,
			&trade.OpenType,
			&trade.CloseType,
			&trade.BaseVolume,
			&trade.BuyOrderID,
			&trade.SellOrderID,
			&trade.Status,
			&convertedSellLimitAt,
			&closedAt,
			&updatedAt,
			&createdAt,
		)
		if err != nil {
			return []Trade{}, err
		}

		if convertedSellLimitAt.Valid {
			trade.ConvertedSellLimitAt = convertedSellLimitAt.Time
		}
		if closedAt.Valid {
			trade.ClosedAt = closedAt.Time
		}
		if updatedAt.Valid {
			trade.UpdatedAt = updatedAt.Time
		}
		if createdAt.Valid {
			trade.CreatedAt = createdAt.Time
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

func (s *Mysql) LatestAppClosedTradeByOpenPrice(appID int, openPrice float64) (Trade, error) {
	q := `
		SELECT
			id,   
			app_id,
			open_base_price,
			close_base_price,
			open_type,
			close_type,
			base_volume,
			buy_order_id,
			sell_order_id,
			status,
			converted_sell_limit_at,
			closed_at,
			updated_at,   
			created_at
		FROM trades
		WHERE 1
			AND app_id = ?
            AND open_base_price = ?
			AND status = 'CLOSED'
    `

	rows, err := s.db.Query(q, appID, openPrice)

	if err != nil {
		return Trade{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		return Trade{}, nil
	}

	var trade Trade
	var convertedSellLimitAt, closedAt, updatedAt, createdAt mysql.NullTime

	err = rows.Scan(
		&trade.ID,
		&trade.AppID,
		&trade.OpenBasePrice,
		&trade.CloseBasePrice,
		&trade.OpenType,
		&trade.CloseType,
		&trade.BaseVolume,
		&trade.BuyOrderID,
		&trade.SellOrderID,
		&trade.Status,
		&convertedSellLimitAt,
		&closedAt,
		&updatedAt,
		&createdAt,
	)
	if err != nil {
		return Trade{}, err
	}

	if convertedSellLimitAt.Valid {
		trade.ConvertedSellLimitAt = convertedSellLimitAt.Time
	}
	if closedAt.Valid {
		trade.ClosedAt = closedAt.Time
	}
	if updatedAt.Valid {
		trade.UpdatedAt = updatedAt.Time
	}
	if createdAt.Valid {
		trade.CreatedAt = createdAt.Time
	}

	return trade, nil
}

func (s *Mysql) AddTrade(trade Trade) (int, error) {
	q := `
		INSERT INTO trades (
		    app_id,
		    open_base_price,
		    close_base_price,
		    open_type,
		    close_type,
		    base_volume,
		    buy_order_id,
		    sell_order_id,
		    status,
	        converted_sell_limit_at,
		    closed_at,
		    created_at
	    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	resp, err := s.db.Exec(q,
		trade.AppID,
		trade.OpenBasePrice,
		trade.CloseBasePrice,
		trade.OpenType,
		trade.CloseType,
		trade.BaseVolume,
		trade.BuyOrderID,
		trade.SellOrderID,
		trade.Status,
		sqlNullableTime(trade.ConvertedSellLimitAt),
		sqlNullableTime(trade.ClosedAt),
		sqlNullableTime(trade.CreatedAt),
	)

	lastInsertedId, err := resp.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(lastInsertedId), nil
}

func (s *Mysql) UpdateTrade(trade Trade) error {
	q := `
		UPDATE trades SET
			open_type = ?,
		    close_type = ?,
		    buy_order_id = ?,
		    sell_order_id = ?,
		    status = ?,
	        converted_sell_limit_at = ?,
		    closed_at = ?,
		    updated_at = ?           
		WHERE id = ?
	`

	_, err := s.db.Exec(q,
		trade.OpenType,
		trade.CloseType,
		trade.BuyOrderID,
		trade.SellOrderID,
		trade.Status,
		sqlNullableTime(trade.ConvertedSellLimitAt),
		sqlNullableTime(trade.ClosedAt),
		sqlNullableTime(time.Now().UTC()),
		trade.ID,
	)

	return err
}

// BalanceHistory ...
func (s *Mysql) LatestBalanceHistory(appID int) (BalanceHistory, error) {
	var ab BalanceHistory

	q := fmt.Sprintf(`
        SELECT 
            app_id,
            action,
            quote_volume,
            total_quote_net_income,
            total_quote_reinvested,
            trade_id,
            created_at
        FROM balance_history
        WHERE app_id = %d
        ORDER BY id DESC
        LIMIT 1`,
		appID,
	)

	row, err := s.db.Query(q)
	if err != nil {
		return ab, err
	}
	defer row.Close()

	if !row.Next() {
		return ab, nil
	}

	var createdAt mysql.NullTime
	err = row.Scan(
		&ab.AppID,
		&ab.Action,
		&ab.QuoteVolume,
		&ab.TotalNetIncome,
		&ab.TotalReinvested,
		&ab.InternalTradeID,
		&createdAt,
	)
	if err != nil {
		return ab, err
	}

	if createdAt.Valid {
		ab.CreatedAt = createdAt.Time
	}

	return ab, nil
}

// LatestTradeBalanceHistory ...
func (s *Mysql) LatestTradeBalanceHistory(appID int, tradeID int) (BalanceHistory, error) {
	var ab BalanceHistory

	q := fmt.Sprintf(`
        SELECT 
            app_id,
            action,
            quote_volume,
            total_quote_net_income,
            total_quote_reinvested,
            trade_id,
            created_at
        FROM balance_history
        WHERE 1 
           AND app_id = %d
           AND trade_id = %d
        ORDER BY id DESC
        LIMIT 1`,
		appID,
		tradeID,
	)

	row, err := s.db.Query(q)
	if err != nil {
		return ab, err
	}
	defer row.Close()

	if !row.Next() {
		return ab, nil
	}

	var createdAt mysql.NullTime
	err = row.Scan(
		&ab.AppID,
		&ab.Action,
		&ab.QuoteVolume,
		&ab.TotalNetIncome,
		&ab.TotalReinvested,
		&ab.InternalTradeID,
		&createdAt,
	)
	if err != nil {
		return ab, err
	}

	if createdAt.Valid {
		ab.CreatedAt = createdAt.Time
	}

	return ab, nil
}

// AddAppBalanceEntry ...
func (s *Mysql) AddBalanceHistory(appID int, balance BalanceHistory) error {
	q := `
		INSERT INTO balance_history (
			app_id,
		    action,
		    quote_volume,
		    total_quote_net_income,
		    total_quote_reinvested,
		    trade_id,
		    created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?)
    `

	_, err := s.db.Exec(q,
		appID,
		balance.Action,
		balance.QuoteVolume,
		balance.TotalNetIncome,
		balance.TotalReinvested,
		balance.InternalTradeID,
		sqlNullableTime(balance.CreatedAt),
	)

	return err
}

func sqlNullableTime(t time.Time) mysql.NullTime {
	if t.IsZero() {
		return mysql.NullTime{}
	}
	return mysql.NullTime{
		Time:  t,
		Valid: true,
	}
}

func (s *Mysql) createSchemaIfNotExists() error {
	q := `
        CREATE TABLE IF NOT EXISTS apps (
            id INT PRIMARY KEY AUTO_INCREMENT,
            app_id INT UNIQUE,
            run_interval VARCHAR(32) DEFAULT '',
            exchange VARCHAR(32) DEFAULT '',
            market_order_fees DECIMAL(5,4) DEFAULT 0,
            limit_order_fees DECIMAL(5,4) DEFAULT 0,
            base VARCHAR(32) DEFAULT '',
            quote VARCHAR(32) DEFAULT '',
            min_base_price DECIMAL(16,10) DEFAULT 0,
            max_base_price DECIMAL(16,10) DEFAULT 0,
            step_quote_volume DECIMAL(16,10) DEFAULT 0,
            steps_type VARCHAR(32) DEFAULT '',
            steps_details VARCHAR(32) DEFAULT '',
            compound_type VARCHAR(32) DEFAULT '',
            compound_details VARCHAR(32) DEFAULT '',
            publish_orders_number INT DEFAULT 0,
            status VARCHAR(32) DEFAULT ''
        )    
    `

	stmt, err := s.db.Prepare(q)
	if err != nil {
		return err
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	q = `
        CREATE TABLE IF NOT EXISTS trades (
            id INT PRIMARY KEY AUTO_INCREMENT,
            app_id INT,
            open_base_price DECIMAL(16,10) DEFAULT 0,
            close_base_price DECIMAL(16,10) DEFAULT 0,
            open_type VARCHAR(32) DEFAULT '',
            close_type VARCHAR(32) DEFAULT '',
            base_volume DECIMAL(16,10) DEFAULT 0,
            buy_order_id VARCHAR(256) DEFAULT '',
            sell_order_id VARCHAR(256) DEFAULT '',
            status VARCHAR(64) DEFAULT '',
            converted_sell_limit_at TIMESTAMP,
            closed_at TIMESTAMP,
            updated_at TIMESTAMP,
            created_at TIMESTAMP
        )    
    `

	stmt, err = s.db.Prepare(q)
	if err != nil {
		return err
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	q = `
        CREATE TABLE IF NOT EXISTS balance_history (
            id INT PRIMARY KEY AUTO_INCREMENT,
            app_id INT,
            action VARCHAR(32) DEFAULT '',
            quote_volume DECIMAL(16,10) DEFAULT 0,
            total_quote_net_income DECIMAL(16,10) DEFAULT 0,
            total_quote_reinvested DECIMAL(16,10) DEFAULT 0,
            trade_id INT,
            created_at TIMESTAMP,
            INDEX APP_ID (app_id)
        )  
    `

	stmt, err = s.db.Prepare(q)
	if err != nil {
		return err
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	return nil
}
