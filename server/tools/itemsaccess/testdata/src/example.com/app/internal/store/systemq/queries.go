package systemq

const getItem = `-- name: GetItem :one
SELECT id, kind FROM items WHERE id = ?`
