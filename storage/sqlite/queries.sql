-- name: InsertSeries :exec
insert into series (
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

-- name: GetSeriesRange :many
select * from series
where sqlc.arg(range_from) >= time
	and sqlc.arg(range_to) <= time;
