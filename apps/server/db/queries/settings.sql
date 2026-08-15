-- name: GetSetting :one
SELECT * FROM app_settings WHERE key = $1;

-- name: SetSetting :one
INSERT INTO app_settings (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
RETURNING *;
