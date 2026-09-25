CREATE TABLE IF NOT EXISTS budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL CHECK (amount > 0),
    year INT NOT NULL CHECK (year >= 2000 AND year <= 2100),
    month INT NOT NULL CHECK (month >= 1 AND month <= 12),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_budgets_user_category_year_month UNIQUE (user_id, category_id, year, month)
);

CREATE INDEX IF NOT EXISTS idx_budgets_user_year_month ON budgets (user_id, year, month);
CREATE INDEX IF NOT EXISTS idx_budgets_category_id ON budgets (category_id);
