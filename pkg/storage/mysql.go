package storage

import (
	"database/sql"
	"strconv"
	"time"

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
            quote_percent_usage,
            min_base_price,
            max_base_price,
            step_quote_volume,
            steps_type,
            steps_details,
            compound_type,
            compound_details,
            publish_orders_number,
            status,
            is_done   
        FROM apps
   `)

	if err != nil {
		return []App{}, err
	}
	defer rows.Close()

	var apps []App

	for rows.Next() {
		var app App
		var runInterval, marketOrderFees, limitOrderFees, quotePercentUsage string
		var minBasePrice, maxBasePrice, stepQuoteVol string

		err := rows.Scan(
			&app.ID,
			&runInterval,
			&app.Exchange,
			&marketOrderFees,
			&limitOrderFees,
			&app.Base,
			&app.Quote,
			&quotePercentUsage,
			&minBasePrice,
			&maxBasePrice,
			&stepQuoteVol,
			&app.StepsType,
			&app.StepsDetails,
			&app.CompoundType,
			&app.CompoundDetails,
			&app.PublishOrderNumber,
			&app.Status,
			&app.IsDone,
		)
		// TODO if one app not work, nothing will work
		// TODO should return valid apps
		if err != nil {
			return []App{}, err
		}

		interval, err := time.ParseDuration(runInterval)
		if err != nil {
			return []App{}, err
		}
		app.Interval = interval

		marketOrderFeesFloat, err := strconv.ParseFloat(marketOrderFees, 64)
		if err != nil {
			return []App{}, err
		}
		app.MarketOrderFees = marketOrderFeesFloat

		limitOrderFeesFloat, err := strconv.ParseFloat(limitOrderFees, 64)
		if err != nil {
			return []App{}, err
		}
		app.LimitOrderFees = limitOrderFeesFloat

		quotePercentageUsageFloat, err := strconv.ParseFloat(quotePercentUsage, 64)
		if err != nil {
			return []App{}, err
		}
		app.QuotePercentUse = quotePercentageUsageFloat

		minBasePriceFloat, err := strconv.ParseFloat(minBasePrice, 64)
		if err != nil {
			return []App{}, err
		}
		app.MinBasePrice = minBasePriceFloat

		maxBasePriceFloat, err := strconv.ParseFloat(maxBasePrice, 64)
		if err != nil {
			return []App{}, err
		}
		app.MaxBasePrice = maxBasePriceFloat

		stepQuoteVolumeFloat, err := strconv.ParseFloat(stepQuoteVol, 64)
		if err != nil {
			return []App{}, err
		}
		app.StepQuoteVolume = stepQuoteVolumeFloat

		apps = append(apps, app)
	}

	return apps, nil
}

func (s *Mysql) createSchemaIfNotExists() error {
	q := `
        CREATE TABLE IF NOT EXISTS apps (
            id INT PRIMARY KEY AUTO_INCREMENT,
            app_id INT UNIQUE,
            run_interval VARCHAR(32),
            exchange VARCHAR(32),
            market_order_fees VARCHAR(32),
            limit_order_fees VARCHAR(32),
            base VARCHAR(32),
            quote VARCHAR(32),
            quote_percent_usage VARCHAR(32),
            min_base_price VARCHAR(32),
            max_base_price VARCHAR(32),
            step_quote_volume VARCHAR(32),
            steps_type VARCHAR(32),
            steps_details VARCHAR(32),
            compound_type VARCHAR(32),
            compound_details VARCHAR(32),
            publish_orders_number INT,
            status VARCHAR(32),
            is_done INT
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

	return nil
}
