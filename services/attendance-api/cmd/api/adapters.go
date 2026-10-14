package main

import (
	"context"

	"github.com/jmoiron/sqlx"

	"restaurant-api/internal/attendance"
	"restaurant-api/internal/employee"
)

// ============================================================
// employeeReaderAdapter — يربط attendance.EmployeeReader مع employee.Service
// ============================================================
type employeeReaderAdapter struct {
	svc *employee.Service
}

func (a employeeReaderAdapter) GetByID(ctx context.Context, id string) (*attendance.EmployeeInfo, error) {
	emp, err := a.svc.Get(ctx, employee.GetInput{
		EmployeeID:    id,
		ActorRole:     "CEO", // internal read
		ActorBranchID: "",
	})
	if err != nil {
		return nil, err
	}
	if emp == nil {
		return nil, nil
	}
	return &attendance.EmployeeInfo{
		ID:       emp.ID,
		BranchID: emp.BranchID,
		Active:   emp.Active,
	}, nil
}

// ============================================================
// branchReaderAdapter — reads directly from DB (branches table)
// ============================================================
type branchReaderAdapter struct {
	db *sqlx.DB
}

func (a branchReaderAdapter) GetByID(ctx context.Context, id string) (*attendance.BranchInfo, error) {
	var row struct {
		ID                string  `db:"id"`
		OfficialStartTime string  `db:"official_start_time"`
		LateRate          float64 `db:"late_rate"`
		Type              string  `db:"type"`
	}

	// بنستخدم to_char عشان نرجّع TIME بصيغة "HH:MM" (مش "HH:MM:SS")
	const q = `
		SELECT
			id,
			to_char(official_start_time, 'HH24:MI') AS official_start_time,
			late_rate,
			type
		FROM branches
		WHERE id = $1
	`

	err := a.db.GetContext(ctx, &row, q, id)
	if err != nil {
		// لو مش موجود → نرجّع nil
		return nil, nil
	}

	return &attendance.BranchInfo{
		ID:                  row.ID,
		OfficialStartTime:   row.OfficialStartTime,
		LatenessRatePerHour: row.LateRate,
		IsCentralKitchen:    row.Type == "central_kitchen",
	}, nil
}
