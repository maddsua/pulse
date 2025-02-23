create table series (
	id int8 generated by default as identity primary key,
	time timestamp with time zone not null,
	label text not null,
	status int2 not null,
	elapsed interval not null
);
