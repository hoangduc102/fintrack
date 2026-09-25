package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/service"
)

func ptr(s string) *string {
	return &s
}

func TestTransactionService_ImportCSV_Valid(t *testing.T) {
	txRepo := newFakeTransactionRepository()
	catRepo := newFakeCategoryRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	userID := uuid.New()

	catFood := &domain.Category{ID: uuid.New(), UserID: userID, Name: "Food", Type: domain.CategoryTypeExpense}
	catSalary := &domain.Category{ID: uuid.New(), UserID: userID, Name: "Salary", Type: domain.CategoryTypeIncome}
	_ = catRepo.Create(ctx, catFood)
	_ = catRepo.Create(ctx, catSalary)

	csvContent := `date,type,amount,category,description
2026-09-01,EXPENSE,50000,Food,Lunch
2026-09-02,INCOME,25000000,Salary,Monthly salary
2026-09-03,EXPENSE,100000,,Coffee
`
	result, err := svc.ImportCSV(ctx, userID, strings.NewReader(csvContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("expected total 3, got %d", result.Total)
	}
	if result.Imported != 3 {
		t.Errorf("expected imported 3, got %d", result.Imported)
	}
	if result.Failed != 0 {
		t.Errorf("expected failed 0, got %d", result.Failed)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}

	// Verify records in repo
	if len(txRepo.transactions) != 3 {
		t.Fatalf("expected 3 transactions in repository, got %d", len(txRepo.transactions))
	}
}

func TestTransactionService_ImportCSV_PreflightHeaderValidation(t *testing.T) {
	txRepo := newFakeTransactionRepository()
	catRepo := newFakeCategoryRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	userID := uuid.New()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "empty content",
			content: "",
		},
		{
			name:    "wrong columns",
			content: "id,type,amount,category,description\n",
		},
		{
			name:    "missing description column",
			content: "date,type,amount,category\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ImportCSV(ctx, userID, strings.NewReader(tt.content))
			if err == nil {
				t.Fatal("expected preflight validation error, got nil")
			}
			var valErr domain.ValidationError
			if !errors.As(err, &valErr) {
				t.Errorf("expected domain.ValidationError, got %v", err)
			}
		})
	}
}

func TestTransactionService_ImportCSV_PartialFailures(t *testing.T) {
	txRepo := newFakeTransactionRepository()
	catRepo := newFakeCategoryRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	userID := uuid.New()

	csvContent := `date,type,amount,category,description
2026-09-01,EXPENSE,50000,Food,Lunch
2026/09/02,EXPENSE,50000,Food,Invalid Date Format
2026-09-03,TRANSFER,50000,Food,Invalid Type
2026-09-04,EXPENSE,-50000,Food,Negative Amount
2026-09-05,EXPENSE,0,Food,Zero Amount
2026-09-06,EXPENSE,invalid,Food,Non-numeric Amount
2026-09-07,INCOME,1000000,,Valid Uncategorized Income
`
	result, err := svc.ImportCSV(ctx, userID, strings.NewReader(csvContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 7 {
		t.Errorf("expected total 7, got %d", result.Total)
	}
	if result.Imported != 2 {
		t.Errorf("expected imported 2, got %d", result.Imported)
	}
	if result.Failed != 5 {
		t.Errorf("expected failed 5, got %d", result.Failed)
	}
	if len(result.Errors) != 5 {
		t.Fatalf("expected 5 errors, got %d", len(result.Errors))
	}

	// Verify error rows are 3, 4, 5, 6, 7 (1-indexed CSV row including header)
	expectedRows := []int{3, 4, 5, 6, 7}
	for i, exp := range expectedRows {
		if result.Errors[i].Row != exp {
			t.Errorf("expected error %d row to be %d, got %d (error: %s)", i, exp, result.Errors[i].Row, result.Errors[i].Error)
		}
	}

	// Verify only 2 valid transactions persisted
	if len(txRepo.transactions) != 2 {
		t.Errorf("expected 2 transactions persisted in repository, got %d", len(txRepo.transactions))
	}
}

func TestTransactionService_ImportCSV_UnknownCategoryMapsToNil(t *testing.T) {
	txRepo := newFakeTransactionRepository()
	catRepo := newFakeCategoryRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	userID := uuid.New()

	csvContent := `date,type,amount,category,description
2026-09-01,EXPENSE,50000,NonExistentCategory,Lunch
`
	result, err := svc.ImportCSV(ctx, userID, strings.NewReader(csvContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Imported != 1 {
		t.Fatalf("expected 1 imported, got %d", result.Imported)
	}

	for _, tx := range txRepo.transactions {
		if tx.CategoryID != nil {
			t.Errorf("expected CategoryID to be nil for unknown category, got %v", tx.CategoryID)
		}
	}
}

func TestTransactionService_ImportCSV_Cancellation(t *testing.T) {
	txRepo := newFakeTransactionRepository()
	catRepo := newFakeCategoryRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	userID := uuid.New()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	csvContent := `date,type,amount,category,description
2026-09-01,EXPENSE,50000,,Lunch
2026-09-02,EXPENSE,60000,,Dinner
`
	_, err := svc.ImportCSV(ctx, userID, strings.NewReader(csvContent))
	if err == nil {
		t.Fatal("expected context canceled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestTransactionService_ImportCSV_ConcurrencyStress(t *testing.T) {
	txRepo := newFakeTransactionRepository()
	catRepo := newFakeCategoryRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	userID := uuid.New()

	// Generate 300 rows
	var sb strings.Builder
	sb.WriteString("date,type,amount,category,description\n")
	for i := 1; i <= 300; i++ {
		sb.WriteString(fmt.Sprintf("2026-09-%02d,EXPENSE,%d,,Item %d\n", (i%28)+1, i*1000, i))
	}

	result, err := svc.ImportCSV(ctx, userID, strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("unexpected error on 300 rows: %v", err)
	}

	if result.Total != 300 || result.Imported != 300 || result.Failed != 0 {
		t.Errorf("unexpected stress result: %+v", result)
	}
	if len(txRepo.transactions) != 300 {
		t.Errorf("expected 300 transactions in repo, got %d", len(txRepo.transactions))
	}
}
