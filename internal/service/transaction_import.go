package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

const (
	numImportWorkers = 4
	channelBufferSize = 100
)

type csvJob struct {
	rowNum   int
	record   []string
	parseErr string
}

type csvResult struct {
	rowNum int
	valid  bool
	tx     domain.Transaction
	err    string
}

// ImportCSV parses, validates, and concurrently imports transactions from a CSV stream.
func (s *TransactionService) ImportCSV(ctx context.Context, userID uuid.UUID, r io.Reader) (*domain.ImportResult, error) {
	// 0. Preflight context check
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	reader := csv.NewReader(r)

	// 1. Preflight Header Validation
	header, err := reader.Read()
	if err != nil {
		return nil, domain.ValidationError{
			Field:   "header",
			Message: "empty or invalid CSV file",
		}
	}

	expectedHeaders := []string{"date", "type", "amount", "category", "description"}
	if len(header) != len(expectedHeaders) {
		return nil, domain.ValidationError{
			Field:   "header",
			Message: "header must be date,type,amount,category,description",
		}
	}

	for i, h := range header {
		if strings.ToLower(strings.TrimSpace(h)) != expectedHeaders[i] {
			return nil, domain.ValidationError{
				Field:   "header",
				Message: "header must be date,type,amount,category,description",
			}
		}
	}

	// 2. Preload user categories into memory cache
	categories, err := s.categoryRepo.FindByUserAndType(ctx, userID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to preload user categories: %w", err)
	}

	catCache := make(map[string]domain.Category, len(categories))
	for _, c := range categories {
		catCache[strings.ToLower(strings.TrimSpace(c.Name))] = c
	}

	// 3. Worker Pool Setup
	jobs := make(chan csvJob, channelBufferSize)
	results := make(chan csvResult, channelBufferSize)

	// Producer goroutine: reads CSV rows and submits jobs
	go func() {
		defer close(jobs)
		rowNum := 1 // Header is row 1
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			record, err := reader.Read()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				rowNum++
				select {
				case <-ctx.Done():
					return
				case jobs <- csvJob{rowNum: rowNum, parseErr: err.Error()}:
				}
				continue
			}

			rowNum++
			select {
			case <-ctx.Done():
				return
			case jobs <- csvJob{rowNum: rowNum, record: record}:
			}
		}
	}()

	// 4 Workers goroutines
	var workerWg sync.WaitGroup
	for i := 0; i < numImportWorkers; i++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				res := validateCSVRow(job, userID, catCache)

				select {
				case <-ctx.Done():
					return
				case results <- res:
				}
			}
		}()
	}

	// Coordinator: waits for workers, then closes results
	go func() {
		workerWg.Wait()
		close(results)
	}()

	// 4. Collector: collects results from results channel
	var validTxs []domain.Transaction
	var importErrors []domain.ImportError
	totalRows := 0

	for res := range results {
		totalRows++
		if res.valid {
			validTxs = append(validTxs, res.tx)
		} else {
			importErrors = append(importErrors, domain.ImportError{
				Row:   res.rowNum,
				Error: res.err,
			})
		}
	}

	// If context was cancelled during processing, do not persist
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Sort errors by row ascending
	sort.Slice(importErrors, func(i, j int) bool {
		return importErrors[i].Row < importErrors[j].Row
	})

	// 5. Batch persist valid records inside single database transaction
	if len(validTxs) > 0 {
		if err := s.txRepo.CreateBatch(ctx, validTxs); err != nil {
			return nil, fmt.Errorf("failed to persist batch transactions: %w", err)
		}
	}

	return &domain.ImportResult{
		Total:    totalRows,
		Imported: len(validTxs),
		Failed:   len(importErrors),
		Errors:   importErrors,
	}, nil
}

func validateCSVRow(job csvJob, userID uuid.UUID, catCache map[string]domain.Category) csvResult {
	if job.parseErr != "" {
		return csvResult{
			rowNum: job.rowNum,
			valid:  false,
			err:    fmt.Sprintf("malformed CSV row: %s", job.parseErr),
		}
	}

	if len(job.record) != 5 {
		return csvResult{
			rowNum: job.rowNum,
			valid:  false,
			err:    "row must have exactly 5 columns (date,type,amount,category,description)",
		}
	}

	dateStr := strings.TrimSpace(job.record[0])
	typeStr := strings.ToUpper(strings.TrimSpace(job.record[1]))
	amountStr := strings.TrimSpace(job.record[2])
	categoryStr := strings.TrimSpace(job.record[3])
	descStr := strings.TrimSpace(job.record[4])

	// 1. Validate date
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		return csvResult{
			rowNum: job.rowNum,
			valid:  false,
			err:    "invalid date format, must be YYYY-MM-DD",
		}
	}

	// 2. Validate type
	txType := domain.TransactionType(typeStr)
	if txType != domain.TransactionTypeIncome && txType != domain.TransactionTypeExpense {
		return csvResult{
			rowNum: job.rowNum,
			valid:  false,
			err:    "type must be INCOME or EXPENSE",
		}
	}

	// 3. Validate amount
	amount, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil || amount <= 0 {
		return csvResult{
			rowNum: job.rowNum,
			valid:  false,
			err:    "amount must be a positive integer",
		}
	}

	// 4. Map category (unknown or blank maps to nil / Uncategorized)
	var categoryID *uuid.UUID
	if categoryStr != "" {
		if cat, exists := catCache[strings.ToLower(categoryStr)]; exists {
			// Category type must match transaction type
			if cat.Type == domain.CategoryType(txType) {
				id := cat.ID
				categoryID = &id
			}
		}
	}

	// 5. Description
	var desc *string
	if descStr != "" {
		desc = &descStr
	}

	now := time.Now().UTC()
	return csvResult{
		rowNum: job.rowNum,
		valid:  true,
		tx: domain.Transaction{
			ID:              uuid.New(),
			UserID:          userID,
			Amount:          amount,
			Type:            txType,
			CategoryID:      categoryID,
			TransactionDate: dateStr,
			Description:     desc,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
}
