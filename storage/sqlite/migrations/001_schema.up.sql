create table series (
	id integer primary key autoincrement,
	time integer not null,
	label text not null,
	status integer not null,
	elapsed integer not null
);
