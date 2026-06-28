// Atlas 配置只负责数据库资产，不参与运行时代码装配。
// PostgreSQL 是主库目标，SQLite 仅保留给本地轻量验证。

variable "database_url" {
  type        = string
  description = "目标 PostgreSQL 数据库连接串"
}

variable "sqlite_url" {
  type        = string
  default     = "sqlite://file?mode=memory"
  description = "本地 SQLite 校验时使用的连接串"
}

env "postgres" {
  url = var.database_url
  dev = "docker://postgres/16/dev?search_path=public"

  migration {
    dir = "file://migrations/postgres"
  }

  schema {
    src = "file://schema/postgres.sql"
  }
}

env "sqlite" {
  url = var.sqlite_url
  dev = "sqlite://file?mode=memory"

  migration {
    dir = "file://migrations/sqlite"
  }

  schema {
    src = "file://schema/sqlite.sql"
  }
}
