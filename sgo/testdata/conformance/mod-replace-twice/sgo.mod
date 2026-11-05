module example.com/mod-replace-twice

sgo 0.20.1
require example.com/lib v1.0.0
replace example.com/lib => ./a
replace example.com/lib => ./b
