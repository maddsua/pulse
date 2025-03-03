-- name: InsertUptime :exec
insert into uptime (
	time,
	label,
	status,
	http_status,
	elapsed,
	latency
) values (
	sqlc.arg(time),
	sqlc.arg(label),
	sqlc.arg(status),
	sqlc.arg(http_status),
	sqlc.arg(elapsed),
	sqlc.arg(latency)
);

-- name: GetUptimeSeriesRange :many
select * from uptime
where time >= sqlc.arg(range_from)
	and time <= sqlc.arg(range_to);
