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

-- name: InsertTls :exec
insert into tlscert (
	time,
	label,
	security,
	cert_subject,
	cert_issuer,
	cert_expires,
	cert_fingerprint
) values (
	sqlc.arg(time),
	sqlc.arg(label),
	sqlc.arg(security),
	sqlc.arg(cert_subject),
	sqlc.arg(cert_issuer),
	sqlc.arg(cert_expires),
	sqlc.arg(cert_fingerprint)
);

-- name: GetTlsSeriesRange :many
select * from tlscert
where time >= sqlc.arg(range_from)
	and time <= sqlc.arg(range_to);
