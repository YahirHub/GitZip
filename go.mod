module gitzip

go 1.23

require (
	github.com/git-pkgs/gitignore v1.1.2
	github.com/yeka/zip v0.0.0-00010101000000-000000000000
)

replace github.com/git-pkgs/gitignore => ./third_party/gitignore
replace github.com/yeka/zip => ./third_party/yeka_zip
