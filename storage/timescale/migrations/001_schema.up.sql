create table series (
	id int8 generated by default as identity primary key,
	time timestamp with time zone not null,
	label text not null,
	status text not null,
	http_status int2,
	elapsed_ms int8 not null,
	latency integer not null
);
