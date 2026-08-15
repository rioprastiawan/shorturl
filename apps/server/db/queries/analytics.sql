-- Rollup maintenance, called by the analytics worker per drained batch.

-- name: UpsertClickHourly :exec
INSERT INTO click_hourly (workspace_id, link_id, bucket, clicks)
VALUES ($1, $2, $3, $4)
ON CONFLICT (link_id, bucket)
DO UPDATE SET clicks = click_hourly.clicks + EXCLUDED.clicks;

-- name: UpsertClickDimensionDaily :exec
INSERT INTO click_dimension_daily (workspace_id, day, dimension, value, clicks)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, day, dimension, value)
DO UPDATE SET clicks = click_dimension_daily.clicks + EXCLUDED.clicks;


-- Reporting. Every query below reads a rollup table, never click_events.

-- name: WorkspaceClickTotals :one
SELECT
    coalesce(sum(clicks), 0)::bigint AS total_clicks,
    coalesce(sum(clicks) FILTER (WHERE bucket >= date_trunc('day', now())), 0)::bigint AS clicks_today
FROM click_hourly
WHERE workspace_id = $1;

-- name: ClicksOverTime :many
SELECT
    date_trunc(sqlc.arg(granularity)::text, bucket)::timestamptz AS period,
    sum(clicks)::bigint AS clicks
FROM click_hourly
WHERE workspace_id = sqlc.arg(workspace_id)
  AND bucket >= sqlc.arg(from_time)
  AND bucket < sqlc.arg(to_time)
GROUP BY period
ORDER BY period ASC;

-- name: ClicksInRange :one
SELECT coalesce(sum(clicks), 0)::bigint
FROM click_hourly
WHERE workspace_id = sqlc.arg(workspace_id)
  AND bucket >= sqlc.arg(from_time)
  AND bucket < sqlc.arg(to_time);

-- name: ClicksByHourOfDay :many
SELECT
    extract(hour FROM bucket)::integer AS hour,
    sum(clicks)::bigint AS clicks
FROM click_hourly
WHERE workspace_id = sqlc.arg(workspace_id)
  AND bucket >= sqlc.arg(from_time)
  AND bucket < sqlc.arg(to_time)
GROUP BY hour
ORDER BY hour;

-- name: ClicksByWeekday :many
SELECT
    extract(isodow FROM bucket)::integer AS weekday,
    sum(clicks)::bigint AS clicks
FROM click_hourly
WHERE workspace_id = sqlc.arg(workspace_id)
  AND bucket >= sqlc.arg(from_time)
  AND bucket < sqlc.arg(to_time)
GROUP BY weekday
ORDER BY weekday;

-- name: ClicksOverTimeForLink :many
SELECT
    date_trunc(sqlc.arg(granularity)::text, bucket)::timestamptz AS period,
    sum(clicks)::bigint AS clicks
FROM click_hourly
WHERE link_id = sqlc.arg(link_id)
  AND bucket >= sqlc.arg(from_time)
  AND bucket < sqlc.arg(to_time)
GROUP BY period
ORDER BY period ASC;

-- name: TopLinks :many
SELECT
    links.id,
    links.slug,
    links.title,
    links.destination_url,
    domains.hostname,
    sum(click_hourly.clicks)::bigint AS clicks
FROM click_hourly
JOIN links   ON links.id = click_hourly.link_id
JOIN domains ON domains.id = links.domain_id
WHERE click_hourly.workspace_id = sqlc.arg(workspace_id)
  AND click_hourly.bucket >= sqlc.arg(from_time)
  AND click_hourly.bucket < sqlc.arg(to_time)
GROUP BY links.id, links.slug, links.title, links.destination_url, domains.hostname
ORDER BY clicks DESC
LIMIT sqlc.arg(row_limit);

-- name: TopDimensionValues :many
SELECT
    value,
    sum(clicks)::bigint AS clicks
FROM click_dimension_daily
WHERE workspace_id = sqlc.arg(workspace_id)
  AND dimension = sqlc.arg(dimension)
  AND day >= sqlc.arg(from_day)
  AND day <= sqlc.arg(to_day)
GROUP BY value
ORDER BY clicks DESC
LIMIT sqlc.arg(row_limit);

-- name: UniqueVisitorsInRange :one
SELECT count(DISTINCT value)::bigint
FROM click_dimension_daily
WHERE workspace_id = sqlc.arg(workspace_id)
  AND dimension = 'visitor'
  AND day >= sqlc.arg(from_day)
  AND day <= sqlc.arg(to_day);

-- name: LinkClickTotals :one
SELECT
    coalesce(sum(clicks), 0)::bigint AS total_clicks,
    coalesce(sum(clicks) FILTER (WHERE bucket >= date_trunc('day', now())), 0)::bigint AS clicks_today
FROM click_hourly
WHERE link_id = $1;

-- name: RecentClickEvents :many
SELECT * FROM click_events
WHERE link_id = $1
ORDER BY clicked_at DESC
LIMIT $2;

-- Retention: raw events age out, rollups are kept indefinitely because they
-- are tiny. Called by the worker's periodic maintenance pass.
-- name: DeleteClickEventsBefore :execrows
DELETE FROM click_events WHERE clicked_at < $1;

-- Visitor hashes have the same retention window as raw click events. Other
-- aggregate dimensions remain tiny and are retained indefinitely.
-- name: DeleteVisitorDimensionsBefore :execrows
DELETE FROM click_dimension_daily
WHERE dimension = 'visitor' AND day < $1;
