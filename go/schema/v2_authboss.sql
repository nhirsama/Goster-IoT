-- Authboss Users Table Schema
-- Replaces the old 'users' table.
-- 'pid' (Primary ID) is used by Authboss for cookies/sessions.
-- 'email' is the primary identifier for login/recovery.

DROP TABLE IF EXISTS users;

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    pid           TEXT UNIQUE NOT NULL,    -- Authboss Cookie/Session ID
    email         TEXT UNIQUE NOT NULL,    -- Login Identifier
    username      TEXT UNIQUE,             -- Display Name / Legacy Support
    password      TEXT NOT NULL,           -- Bcrypt Hash
    permission    INTEGER DEFAULT 0,       -- 0: None, 1: ReadOnly, 2: ReadWrite, 3: Admin
    
    -- Authboss Modules
    recover_token        TEXT,
    recover_token_expiry DATETIME,
    
    confirm_token        TEXT,
    confirmed            BOOLEAN DEFAULT FALSE,
    
    last_login           DATETIME,
    created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for fast lookups
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_pid ON users(pid);
CREATE INDEX idx_users_recover_token ON users(recover_token);
