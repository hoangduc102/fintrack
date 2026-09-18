# Store Transaction Dates as PostgreSQL DATE

We decided to store `transaction_date` as a PostgreSQL `DATE` rather than a full timestamp (`TIMESTAMPTZ`). Monthly budgets, daily expense tracking, and analytics operate strictly on calendar days and calendar months. Storing dates without time components avoids timezone shift bugs across month and day boundaries while matching user expectations for personal finance tracking.
