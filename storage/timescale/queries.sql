-- name: InsertSeries :exec
insert into series (
	time,
	label,
	status,
	elapsed
) values (
	sqlc.arg(time),
	sqlc.arg(label),
	sqlc.arg(status),
	sqlc.arg(elapsed)
);

-- name: GetSeriesRange :many
select * from series
where sqlc.arg(range_from) >= time
	and sqlc.arg(range_to) <= time;
