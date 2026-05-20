# 数据库迁移

## 概览

本目录包含数据库 schema 变更所需的 SQL 迁移文件。迁移系统使用 SHA256 checksum 保证迁移文件不可变，并确保不同环境之间保持一致。

## 迁移文件命名

格式：`NNN_description.sql`

- `NNN`：顺序编号，例如 001、002、003。
- `description`：使用 snake_case 的简短描述。

示例：`017_add_gemini_tier_id.sql`

### `_notx.sql` 命名与执行语义（并发索引专用）

当迁移包含 `CREATE INDEX CONCURRENTLY` 或 `DROP INDEX CONCURRENTLY` 时，必须使用 `_notx.sql` 后缀，例如：

- `062_add_accounts_priority_indexes_notx.sql`
- `063_drop_legacy_indexes_notx.sql`

运行规则：

1. `*.sql`（不带 `_notx`）按事务执行。
2. `*_notx.sql` 按非事务执行，不会包在 `BEGIN/COMMIT` 中。
3. `*_notx.sql` 仅允许并发索引语句，不允许混入事务控制语句或其它 DDL/DML。

幂等要求（必须）：

- 创建索引：`CREATE INDEX CONCURRENTLY IF NOT EXISTS ...`
- 删除索引：`DROP INDEX CONCURRENTLY IF EXISTS ...`

这样可以保证灾备重放、重复执行时不会因为对象已存在或不存在而失败。

## 迁移文件结构

本项目使用自定义迁移 runner（`internal/repository/migrations_runner.go`），会按原样执行整个 SQL 文件内容。

- 常规迁移（`*.sql`）：在事务中执行。
- 非事务迁移（`*_notx.sql`）：按语句拆分，并在事务外执行，主要用于 `CONCURRENTLY`。

```sql
-- Forward-only migration (recommended)
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS example_column VARCHAR(100);
```

> 注意：不要在同一个文件中放置可执行的 "Down" SQL。当前 runner 不解析 goose Up/Down 段落，会执行文件中的所有 SQL 语句。

## 重要规则

### 不可变原则

**迁移文件一旦应用到任何环境（dev、staging、production），就不得再修改。**

原因：

- 每个迁移的 SHA256 checksum 会存储在 `schema_migrations` 表中。
- 修改已应用的迁移会导致 checksum mismatch 错误。
- 不同环境会出现不一致的数据库状态。
- 会破坏审计轨迹和可复现性。

### 正确流程

1. **创建新迁移**

   ```bash
   # Create new file with next sequential number
   touch migrations/018_your_change.sql
   ```

2. **编写 forward-only 迁移 SQL**

   - 文件中只放本次预期的 schema 变更。
   - 如果需要回滚，创建新的迁移文件来反向修正。

3. **本地测试**

   ```bash
   # Apply migration
   make migrate-up

   # Test rollback
   make migrate-down
   ```

4. **提交并部署**

   ```bash
   git add migrations/018_your_change.sql
   git commit -m "feat(db): add your change"
   ```

### 不要做这些事

- 不要修改已经应用过的迁移文件。
- 不要删除迁移文件。
- 不要修改迁移文件名。
- 不要重排迁移编号。

### 如果误改了已应用迁移

**错误信息：**

```text
migration 017_add_gemini_tier_id.sql checksum mismatch (db=abc123... file=def456...)
```

**处理方式：**

```bash
# 1. Find the original version
git log --oneline -- migrations/017_add_gemini_tier_id.sql

# 2. Revert to the commit when it was first applied
git checkout <commit-hash> -- migrations/017_add_gemini_tier_id.sql

# 3. Create a NEW migration for your changes
touch migrations/018_your_new_change.sql
```

## 迁移系统细节

- **Checksum 算法**：对 trim 后的文件内容计算 SHA256。
- **跟踪表**：`schema_migrations`（filename、checksum、applied_at）。
- **Runner**：`internal/repository/migrations_runner.go`。
- **自动运行**：服务启动时自动运行迁移。

## 最佳实践

1. **保持迁移小而聚焦**

   - 每个迁移只做一个逻辑变更。
   - 更容易审查和回滚。

2. **编写可反向修正的迁移**

   - 当前 runner 不执行同文件 Down SQL；如需回滚，使用新的迁移文件修正。
   - 提交前先测试反向修正方案。

3. **尽量使用事务**

   - 能放进事务的 DDL 语句尽量使用事务。
   - 保证原子性。

4. **添加注释**

   - 说明为什么需要这个变更。
   - 记录特殊注意事项。

5. **先在开发环境测试**

   - 本地应用迁移。
   - 验证数据完整性。
   - 测试回滚或反向修正方案。

## 迁移示例

```sql
-- Add tier_id field to Gemini OAuth accounts for quota tracking
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{tier_id}',
    '"LEGACY"',
    true
)
WHERE platform = 'gemini'
  AND type = 'oauth'
  AND credentials->>'tier_id' IS NULL;
```

## 故障排查

### Checksum Mismatch

参见上文“如果误改了已应用迁移”。

### Migration Failed

```bash
# Check migration status
psql -d sub2api -c "SELECT * FROM schema_migrations ORDER BY applied_at DESC;"

# Manually rollback if needed (use with caution)
# Better to fix the migration and create a new one
```

### 需要跳过某个迁移（仅紧急情况）

```sql
-- DANGEROUS: Only use in development or with extreme caution
INSERT INTO schema_migrations (filename, checksum, applied_at)
VALUES ('NNN_migration.sql', 'calculated_checksum', NOW());
```

## 参考

- Migration runner：`internal/repository/migrations_runner.go`
- PostgreSQL docs：https://www.postgresql.org/docs/
