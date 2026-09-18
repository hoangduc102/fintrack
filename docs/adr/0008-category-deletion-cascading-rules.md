# Category Deletion Cascading Rules for Transactions and Budgets

We decided that deleting a category sets `transactions.category_id = NULL` (`ON DELETE SET NULL`) while cascading deletions on `budgets.category_id` (`ON DELETE CASCADE`). 

This distinction preserves historical financial integrity: transactions represent immutable real-world events that remain valid as Uncategorized records, whereas a monthly budget exists solely as an active planning ceiling for a specific category and has no meaning once that category ceases to exist.
