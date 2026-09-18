# FinTrack Domain Context

FinTrack is a personal finance management system for tracking individual income, expenses, categories, and category-level monthly budgets.

## Language

### Identity & Tenancy

**User**:
An individual account holder who owns transactions, categories, and monthly budgets.
_Avoid_: Account, Customer, Client, Member

### Financial Records

**Money**:
A monetary amount in Vietnamese Đồng (VND), represented as a strictly non-negative whole integer.
_Avoid_: Cents, Minor units, Float, Decimal, Currency code

**Transaction**:
A record of financial inflow or outflow occurring on a specific calendar date.
_Avoid_: Transfer, Record, Ledger entry, Payment

**Income**:
A transaction representing money earned or received.
_Avoid_: Deposit, Revenue, Inflow

**Expense**:
A transaction representing money spent.
_Avoid_: Outflow, Charge, Debit, Bill

**Category**:
A classification assigned to a transaction, partitioned strictly into Income or Expense types and scoped uniquely per user.
_Avoid_: Tag, Label, Folder, Group

**Uncategorized**:
The state of a transaction that has no category assigned.
_Avoid_: Miscellaneous, Default category, General

### Budgeting & Analytics

**Monthly Budget**:
A spending limit set exclusively for an expense category within a single calendar month.
_Avoid_: Target, Quota, Global budget, Overall budget, Yearly budget

**Budget Consumption**:
The dynamically aggregated total of expenses incurred for an expense category within a calendar month.
_Avoid_: Actual spend, Spent amount, Burn rate

**Financial Summary**:
The aggregated total income, total expense, net savings, and budget status for a calendar month.
_Avoid_: Balance sheet, Overview, Statement

**Expense Analytics**:
Categorical breakdowns and multi-month trends of historical expenditure.
_Avoid_: Insights, Metrics, Statistics

### Data Intake

**CSV Import**:
A batch intake of external transaction records parsed and mapped into FinTrack transactions.
_Avoid_: Sync, Bank feed, Statement upload
