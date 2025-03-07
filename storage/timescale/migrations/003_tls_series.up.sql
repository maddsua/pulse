create table tlscert (
	id int8 generated by default as identity primary key,
	time timestamp with time zone not null,
	label text not null,
	security text not null,
	cert_subject text,
	cert_issuer text,
	cert_expires timestamp with time zone,
	cert_fingerprint text
);
