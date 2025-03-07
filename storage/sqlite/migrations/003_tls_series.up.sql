create table tlscert (
	id integer primary key autoincrement,
	time integer not null,
	label text not null,
	security text not null,
	cert_subject text,
	cert_issuer text,
	cert_expires integer,
	cert_fingerprint text
);
