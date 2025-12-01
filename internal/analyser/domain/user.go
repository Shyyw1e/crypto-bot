package domain

import (
	"time"
)

type UserSettings struct {
	ChatID				int64
	WatchFact			bool
	WatchPotential		bool
	MinDiffFact			float64
	MinDiffPotential	float64
	MaxNotional			float64		//only potential
	IsActive			bool
	CreatedAt			time.Time
	UpdatedAt			time.Time
}

