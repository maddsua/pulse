select
	id,
	time,
	label,
	status,
	elapsed
from series
where ? >= time && ? <= time;
