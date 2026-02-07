module github.com/fruitsalade/fruitsalade/phase0

go 1.22

require (
	github.com/fruitsalade/fruitsalade/shared v0.0.0
	github.com/hanwen/go-fuse/v2 v2.5.1
)

replace github.com/fruitsalade/fruitsalade/shared => ../shared

require golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
