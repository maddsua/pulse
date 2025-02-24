create table series (
	id integer primary key autoincrement,
	time integer not null,
	label text not null,
	status text not null,
	http_status integer,
	elapsed integer not null,
	latency integer
);
